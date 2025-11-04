/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"sort" // Added for consistent hashing of multiple deployments
	"strconv"
	"strings" // Added for consistent hashing of multiple deployments
	"text/template"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitorv1 "deployment-email-operator/api/v1"
)

// SMTPConfig holds the SMTP server configuration
type SMTPConfig struct {
	Server   string `json:"SMTP_SERVER"`
	Port     int    `json:"SMTP_PORT"`
	Username string `json:"SMTP_USERNAME"`
	Password string `json:"SMTP_PASSWORD"`
}

// DeploymentMonitorReconciler reconciles a DeploymentMonitor object
type DeploymentMonitorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitor.bnei.dev,resources=deploymentmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitor.bnei.dev,resources=deploymentmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitor.bnei.dev,resources=deploymentmonitors/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeploymentMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the DeploymentMonitor instance
	deploymentMonitor := &monitorv1.DeploymentMonitor{}
	err := r.Get(ctx, req.NamespacedName, deploymentMonitor)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("DeploymentMonitor resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get DeploymentMonitor")
		return ctrl.Result{}, err
	}

	// List all Deployments to find those matching the monitor's criteria
	deploymentList := &appsv1.DeploymentList{}
	if err = r.List(ctx, deploymentList); err != nil {
		log.Error(err, "Failed to list Deployments")
		return ctrl.Result{}, err
	}

	// Filter deployments based on the DeploymentMonitor's spec
	monitoredDeployments := []appsv1.Deployment{}
	for _, dep := range deploymentList.Items {
		if r.isDeploymentMonitored(&dep, deploymentMonitor) {
			monitoredDeployments = append(monitoredDeployments, dep)
		}
	}

	if len(monitoredDeployments) == 0 {
		log.Info("No deployments found matching criteria for DeploymentMonitor", "DeploymentMonitor.Name", deploymentMonitor.Name)
		// If no deployments are monitored, and the status has a hash, clear it.
		if deploymentMonitor.Status.LastNotifiedDeploymentHash != "" {
			deploymentMonitor.Status.LastNotifiedDeploymentHash = ""
			deploymentMonitor.Status.LastNotificationTime = nil
			if err := r.Status().Update(ctx, deploymentMonitor); err != nil {
				log.Error(err, "Failed to clear DeploymentMonitor status when no deployments are monitored")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil // No deployments to monitor, no need to requeue immediately
	}

	// Calculate a combined hash for all currently monitored deployments
	// This ensures that if any of the monitored deployments change, the overall hash changes.
	currentMonitoredDeploymentsHash := calculateCombinedDeploymentsHash(monitoredDeployments)

	// If the current state of monitored deployments matches the last notified state, do nothing.
	if currentMonitoredDeploymentsHash == deploymentMonitor.Status.LastNotifiedDeploymentHash {
		log.V(1).Info("Monitored deployments state unchanged, no notification needed", "DeploymentMonitor.Name", deploymentMonitor.Name, "Hash", currentMonitoredDeploymentsHash)
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil // Requeue to check periodically
	}

	log.Info("Monitored Deployment(s) change detected", "DeploymentMonitor.Name", deploymentMonitor.Name, "OldHash", deploymentMonitor.Status.LastNotifiedDeploymentHash, "NewHash", currentMonitoredDeploymentsHash)

	// Check if SMTPSecretName is provided before attempting to fetch config
	if deploymentMonitor.Spec.SMTPSecretName == "" {
		log.Error(fmt.Errorf("SMTPSecretName is not defined"), "Cannot send email without SMTP configuration", "DeploymentMonitor.Name", deploymentMonitor.Name)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil // Requeue to allow user to fix the spec
	}

	// Fetch SMTP configuration from Secret
	smtpConfig, err := r.getSMTPConfig(ctx, deploymentMonitor)
	if err != nil {
		log.Error(err, "Failed to get SMTP configuration from secret", "Secret.Name", deploymentMonitor.Spec.SMTPSecretName)
		return ctrl.Result{}, err // Requeue with error to retry fetching secret
	}

	// Prepare a single email summarizing all changes
	subject := fmt.Sprintf("Deployment Change Alert for %s", deploymentMonitor.Name)
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("The following monitored deployments have changed:\n\n")

	// Data structure for template, if used
	type DeploymentInfo struct {
		Namespace string
		Name      string
		Image     string
		Replicas  int32
	}
	var templateData []DeploymentInfo

	for _, dep := range monitoredDeployments {
		image := "N/A"
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			image = dep.Spec.Template.Spec.Containers[0].Image
		}
		replicas := int32(1)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}

		templateData = append(templateData, DeploymentInfo{
			Namespace: dep.Namespace,
			Name:      dep.Name,
			Image:     image,
			Replicas:  replicas,
		})

		// Append to default body if no template or template fails
		bodyBuilder.WriteString(fmt.Sprintf("- Deployment %s/%s: Image=%s, Replicas=%d\n", dep.Namespace, dep.Name, image, replicas))
	}

	finalBody := bodyBuilder.String() // Default body

	if deploymentMonitor.Spec.EmailTemplate != "" {
		tmpl, err := template.New("email").Parse(deploymentMonitor.Spec.EmailTemplate)
		if err != nil {
			log.Error(err, "Failed to parse email template, falling back to default body", "DeploymentMonitor.Name", deploymentMonitor.Name)
		} else {
			var tpl bytes.Buffer
			// The template will receive a slice of DeploymentInfo
			err = tmpl.Execute(&tpl, templateData)
			if err != nil {
				log.Error(err, "Failed to execute email template, falling back to default body", "DeploymentMonitor.Name", deploymentMonitor.Name)
			} else {
				finalBody = tpl.String()
			}
		}
	}

	// Add a standard footer
	finalBody += "\nThis is an automated notification from your Kubernetes Deployment Monitor Operator."

	// Send email
	err = SendEmail(
		smtpConfig.Server,
		smtpConfig.Port,
		smtpConfig.Username,
		smtpConfig.Password,
		deploymentMonitor.Spec.RecipientEmail,
		subject,
		finalBody,
	)
	if err != nil {
		log.Error(err, "Failed to send email notification", "Recipient", deploymentMonitor.Spec.RecipientEmail)
		return ctrl.Result{}, err // Requeue with error to retry sending email
	}

	log.Info("Email notification sent successfully", "Recipient", deploymentMonitor.Spec.RecipientEmail, "DeploymentMonitor", deploymentMonitor.Name)

	// Update the DeploymentMonitor's status with the new combined hash
	// Only update if the hash has actually changed to avoid unnecessary status updates
	if deploymentMonitor.Status.LastNotifiedDeploymentHash != currentMonitoredDeploymentsHash {
		deploymentMonitor.Status.LastNotificationTime = &metav1.Time{Time: time.Now()}
		deploymentMonitor.Status.LastNotifiedDeploymentHash = currentMonitoredDeploymentsHash
		if err := r.Status().Update(ctx, deploymentMonitor); err != nil {
			log.Error(err, "Failed to update DeploymentMonitor status after sending email")
			return ctrl.Result{}, err
		}
	}

	// Requeue after a certain duration to periodically check for changes,
	// even if no event triggers it. This acts as a safeguard.
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// isDeploymentMonitored checks if a deployment matches the criteria defined in a DeploymentMonitor.
func (r *DeploymentMonitorReconciler) isDeploymentMonitored(dep *appsv1.Deployment, dm *monitorv1.DeploymentMonitor) bool {
	// Check for annotation match
	if dm.Spec.MonitoredAnnotationKey != "" {
		if val, ok := dep.Annotations[dm.Spec.MonitoredAnnotationKey]; ok {
			if dm.Spec.MonitoredAnnotationValue == "" || val == dm.Spec.MonitoredAnnotationValue {
				return true
			}
		}
	}

	// Check for label match
	if dm.Spec.MonitoredLabelKey != "" {
		if val, ok := dep.Labels[dm.Spec.MonitoredLabelKey]; ok {
			if dm.Spec.MonitoredLabelValue == "" || val == dm.Spec.MonitoredLabelValue {
				return true
			}
		}
	}

	return false
}

// getSMTPConfig retrieves the SMTP configuration from the specified Kubernetes Secret.
func (r *DeploymentMonitorReconciler) getSMTPConfig(ctx context.Context, dm *monitorv1.DeploymentMonitor) (*SMTPConfig, error) {
	if dm.Spec.SMTPSecretName == "" {
		return nil, fmt.Errorf("SMTPSecretName is not defined in DeploymentMonitor %s", dm.Name)
	}

	secret := &corev1.Secret{}
	// Assuming the secret is in the same namespace as the operator or a well-known namespace.
	// For a cluster-scoped DeploymentMonitor, it's common to place secrets in a dedicated namespace
	// or allow the DeploymentMonitor spec to define the secret's namespace.
	// For now, let's assume the secret is in the `default` namespace or the namespace where the operator runs.
	// A more robust solution would be to add a `SMTPSecretNamespace` field to the DeploymentMonitorSpec.
	secretName := types.NamespacedName{
		Name:      dm.Spec.SMTPSecretName,
		Namespace: "default", // Placeholder: Consider adding SMTPSecretNamespace to DeploymentMonitorSpec
	}

	err := r.Get(ctx, secretName, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", secretName.Namespace, secretName.Name, err)
	}

	smtpServer, ok := secret.Data["SMTP_SERVER"]
	if !ok {
		return nil, fmt.Errorf("key 'SMTP_SERVER' not found in secret %s/%s", secretName.Namespace, secretName.Name)
	}
	smtpPortStr, ok := secret.Data["SMTP_PORT"]
	if !ok {
		return nil, fmt.Errorf("key 'SMTP_PORT' not found in secret %s/%s", secretName.Namespace, secretName.Name)
	}
	smtpUsername, ok := secret.Data["SMTP_USERNAME"]
	if !ok {
		return nil, fmt.Errorf("key 'SMTP_USERNAME' not found in secret %s/%s", secretName.Namespace, secretName.Name)
	}
	smtpPassword, ok := secret.Data["SMTP_PASSWORD"]
	if !ok {
		return nil, fmt.Errorf("key 'SMTP_PASSWORD' not found in secret %s/%s", secretName.Namespace, secretName.Name)
	}

	port, err := strconv.Atoi(string(smtpPortStr))
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT value in secret %s/%s: %w", secretName.Namespace, secretName.Name, err)
	}

	return &SMTPConfig{
		Server:   string(smtpServer),
		Port:     port,
		Username: string(smtpUsername),
		Password: string(smtpPassword),
	}, nil
}

// calculateDeploymentHash generates a SHA256 hash based on the deployment's image and replicas.
func calculateDeploymentHash(image string, replicas int32) string {
	data := fmt.Sprintf("%s-%d", image, replicas)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// calculateCombinedDeploymentsHash generates a SHA256 hash based on the combined state of multiple deployments.
// This ensures a consistent hash regardless of the order of deployments in the slice.
func calculateCombinedDeploymentsHash(deployments []appsv1.Deployment) string {
	var deploymentStates []string
	for _, dep := range deployments {
		image := "N/A"
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			image = dep.Spec.Template.Spec.Containers[0].Image
		}
		replicas := int32(1)
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		// Include namespace and name to differentiate deployments
		deploymentStates = append(deploymentStates, fmt.Sprintf("%s/%s:%s-%d", dep.Namespace, dep.Name, image, replicas))
	}

	// Sort the states to ensure a consistent hash regardless of the order in the input slice
	sort.Strings(deploymentStates)
	combinedData := strings.Join(deploymentStates, "|")

	hash := sha256.Sum256([]byte(combinedData))
	return hex.EncodeToString(hash[:])
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1.DeploymentMonitor{}).
		// Watch Deployments and enqueue owning DeploymentMonitors
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.findDeploymentMonitorsForDeployment),
		).
		Named("deploymentmonitor").
		Complete(r)
}

// findDeploymentMonitorsForDeployment lists all DeploymentMonitors and returns reconcile requests
// for those that would monitor the given deployment.
func (r *DeploymentMonitorReconciler) findDeploymentMonitorsForDeployment(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	deployment := obj.(*appsv1.Deployment)
	var requests []reconcile.Request

	deploymentMonitors := &monitorv1.DeploymentMonitorList{}
	err := r.List(ctx, deploymentMonitors)
	if err != nil {
		log.Error(err, "Failed to list DeploymentMonitors while processing Deployment event")
		return nil
	}

	for _, dm := range deploymentMonitors.Items {
		if r.isDeploymentMonitored(deployment, &dm) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: dm.Name,
					// DeploymentMonitor is cluster-scoped, so Namespace is empty.
					// The request is for the DeploymentMonitor, not the deployment.
				},
			})
		}
	}
	return requests
}

// SendEmail sends an email notification.
func SendEmail(smtpServer string, smtpPort int, username, password, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", smtpServer, smtpPort)

	// Set up authentication information.
	auth := smtp.PlainAuth("", username, password, smtpServer)

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body))

	// Send the email.
	err := smtp.SendMail(addr, auth, username, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}
