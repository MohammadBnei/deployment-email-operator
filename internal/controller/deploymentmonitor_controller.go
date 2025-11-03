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
	"strconv"
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
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic,
			// refer to https://book.kubebuilder.io/reference/finalizers.html
			log.Info("DeploymentMonitor resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get DeploymentMonitor")
		return ctrl.Result{}, err
	}

	// List all Deployments to find those matching the monitor's criteria
	deploymentList := &appsv1.DeploymentList{}
	listOpts := []client.ListOption{
		// Consider adding a FieldSelector or LabelSelector here if performance becomes an issue
		// For now, we list all and filter in memory.
	}
	if err = r.List(ctx, deploymentList, listOpts...); err != nil {
		log.Error(err, "Failed to list Deployments")
		return ctrl.Result{}, err
		// TODO: Should we requeue here? If we can't list deployments, we can't monitor anything.
	}

	// Filter deployments based on the DeploymentMonitor's spec
	monitoredDeployments := []appsv1.Deployment{}
	for _, dep := range deploymentList.Items {
		if r.isDeploymentMonitored(&dep, deploymentMonitor) {
			monitoredDeployments = append(monitoredDeployments, dep)
		}
	}

	// If no deployments are monitored, we can stop here.
	if len(monitoredDeployments) == 0 {
		log.Info("No deployments found matching criteria for DeploymentMonitor", "DeploymentMonitor.Name", deploymentMonitor.Name)
		// Clear any existing observed deployments if none are monitored anymore
		if len(deploymentMonitor.Status.ObservedDeployments) > 0 {
			deploymentMonitor.Status.ObservedDeployments = []monitorv1.MonitoredDeploymentStatus{}
			if err := r.Status().Update(ctx, deploymentMonitor); err != nil {
				log.Error(err, "Failed to clear DeploymentMonitor status for observed deployments")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Check if SMTPSecretName is provided before attempting to fetch config
	if deploymentMonitor.Spec.SMTPSecretName == "" {
		log.Error(fmt.Errorf("SMTPSecretName is not defined"), "Cannot send email without SMTP configuration", "DeploymentMonitor.Name", deploymentMonitor.Name)
		// Requeue after a short period to allow user to fix the CRD
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Fetch SMTP configuration from Secret
	smtpConfig, err := r.getSMTPConfig(ctx, deploymentMonitor)
	if err != nil {
		log.Error(err, "Failed to get SMTP configuration from secret", "Secret.Name", deploymentMonitor.Spec.SMTPSecretName)
		// If we can't get SMTP config, we can't send emails, so requeue with error.
		return ctrl.Result{}, err
	}

	// Track if any status update is needed
	statusUpdated := false
	var newObservedDeployments []monitorv1.MonitoredDeploymentStatus

	// For each monitored deployment, check for changes and send notifications
	for _, dep := range monitoredDeployments {
		// Safely get image and replicas
		image := "N/A"
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			image = dep.Spec.Template.Spec.Containers[0].Image
		}
		replicas := int32(1) // Default value
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}

		currentDeploymentHash := calculateDeploymentHash(image, replicas)

		// Find the status for this specific deployment
		var deploymentStatus *monitorv1.MonitoredDeploymentStatus
		for i := range deploymentMonitor.Status.ObservedDeployments {
			if deploymentMonitor.Status.ObservedDeployments[i].Name == dep.Name && deploymentMonitor.Status.ObservedDeployments[i].Namespace == dep.Namespace {
				deploymentStatus = &deploymentMonitor.Status.ObservedDeployments[i]
				break
			}
		}

		if deploymentStatus == nil {
			// This is a new deployment being monitored, or its status was cleared.
			// Initialize its status and prepare to send notification.
			deploymentStatus = &monitorv1.MonitoredDeploymentStatus{
				Name:      dep.Name,
				Namespace: dep.Namespace,
			}
			// Add to the new list of observed deployments
			newObservedDeployments = append(newObservedDeployments, *deploymentStatus)
		} else {
			// Keep existing status for now, will update if notification is sent
			newObservedDeployments = append(newObservedDeployments, *deploymentStatus)
		}

		// Only send email if the deployment state has changed since the last notification
		if currentDeploymentHash == deploymentStatus.LastNotifiedDeploymentHash {
			log.V(1).Info("Deployment state unchanged, no notification needed", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name, "Hash", currentDeploymentHash)
			continue // Skip to the next deployment
		}

		log.Info("Monitored Deployment change detected", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name, "OldHash", deploymentStatus.LastNotifiedDeploymentHash, "NewHash", currentDeploymentHash)

		// Prepare data for template
		templateData := struct {
			Namespace string
			Name      string
			Image     string
			Replicas  int32
		}{
			Namespace: dep.Namespace,
			Name:      dep.Name,
			Image:     image,
			Replicas:  replicas,
		}

		subject := fmt.Sprintf("Deployment Change Alert: %s/%s", dep.Namespace, dep.Name)
		var body string

		if deploymentMonitor.Spec.EmailTemplate != "" {
			tmpl, err := template.New("email").Parse(deploymentMonitor.Spec.EmailTemplate)
			if err != nil {
				log.Error(err, "Failed to parse email template", "DeploymentMonitor.Name", deploymentMonitor.Name)
				// Fallback to default body if template parsing fails
				body = fmt.Sprintf("Deployment %s/%s has been updated or reconciled.\n\nDetails:\nImage: %s\nReplicas: %d\n\nThis is an automated notification from your Kubernetes Deployment Monitor Operator.",
					dep.Namespace, dep.Name, image, replicas)
			} else {
				var tpl bytes.Buffer
				err = tmpl.Execute(&tpl, templateData)
				if err != nil {
					log.Error(err, "Failed to execute email template", "DeploymentMonitor.Name", deploymentMonitor.Name)
					// Fallback to default body if template execution fails
					body = fmt.Sprintf("Deployment %s/%s has been updated or reconciled.\n\nDetails:\nImage: %s\nReplicas: %d\n\nThis is an automated notification from your Kubernetes Deployment Monitor Operator.",
						dep.Namespace, dep.Name, image, replicas)
				} else {
					body = tpl.String()
				}
			}
		} else {
			// Default email body if no template is provided
			body = fmt.Sprintf("Deployment %s/%s has been updated or reconciled.\n\nDetails:\nImage: %s\nReplicas: %d\n\nThis is an automated notification from your Kubernetes Deployment Monitor Operator.",
				dep.Namespace, dep.Name, image, replicas)
		}

		// Send email
		err = SendEmail(
			smtpConfig.Server,
			smtpConfig.Port,
			smtpConfig.Username,
			smtpConfig.Password,
			deploymentMonitor.Spec.RecipientEmail,
			subject,
			body,
		)
		if err != nil {
			log.Error(err, "Failed to send email notification", "Recipient", deploymentMonitor.Spec.RecipientEmail)
			// Continue to process other deployments/monitors even if one email fails
		} else {
			log.Info("Email notification sent successfully", "Recipient", deploymentMonitor.Spec.RecipientEmail, "Deployment", dep.Name)

			// Update the specific deployment's status
			deploymentStatus.LastNotificationTime = &metav1.Time{Time: time.Now()}
			deploymentStatus.LastNotifiedDeploymentHash = currentDeploymentHash
			statusUpdated = true // Mark that status needs an update
		}
	}

	// Update the DeploymentMonitor's status if any changes were made to observed deployments
	if statusUpdated || len(deploymentMonitor.Status.ObservedDeployments) != len(newObservedDeployments) {
		deploymentMonitor.Status.ObservedDeployments = newObservedDeployments
		if err := r.Status().Update(ctx, deploymentMonitor); err != nil {
			log.Error(err, "Failed to update DeploymentMonitor status")
			return ctrl.Result{}, err
		}
	}

	// Requeue after a certain duration to periodically check for changes,
	// or rely solely on watch events for more immediate reactions.
	// For now, we'll rely on watch events and a periodic requeue for robustness.
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
	// This check is now also performed before calling this function, but keeping it here for robustness
	if dm.Spec.SMTPSecretName == "" {
		return nil, fmt.Errorf("SMTPConfigSecretRef is not defined in DeploymentMonitor %s/%s", dm.Namespace, dm.Name)
	}

	secret := &corev1.Secret{}
	// Assuming secret is in the same namespace as DeploymentMonitor for now.
	// If DeploymentMonitor is cluster-scoped, the secret should be in a well-known namespace
	// or the secret reference should include a namespace.
	secretName := types.NamespacedName{
		Name:      dm.Spec.SMTPSecretName,
		Namespace: dm.Namespace,
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
					Name:      dm.Name,
					Namespace: dm.Namespace,
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
