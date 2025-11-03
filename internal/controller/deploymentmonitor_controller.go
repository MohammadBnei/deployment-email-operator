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
	"context"
	"fmt"
	"net/smtp"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	monitorv1 "deployment-email-operator/api/v1"
)

// DeploymentMonitorReconciler reconciles a DeploymentMonitor object
type DeploymentMonitorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=monitor.example.com,resources=deploymentmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitor.example.com,resources=deploymentmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitor.example.com,resources=deploymentmonitors/finalizers,verbs=update
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
	}

	// Filter deployments based on the DeploymentMonitor's spec
	monitoredDeployments := []appsv1.Deployment{}
	for _, dep := range deploymentList.Items {
		if r.isDeploymentMonitored(&dep, deploymentMonitor) {
			monitoredDeployments = append(monitoredDeployments, dep)
		}
	}

	// For each monitored deployment, check for changes and send notifications
	for _, dep := range monitoredDeployments {
		// In a real-world scenario, you'd want to store the last observed state
		// of the deployment (e.g., in the DeploymentMonitor's status or a separate CR)
		// to detect actual changes. For this basic implementation, we'll just log
		// and send an email for any reconciliation of a monitored deployment.
		// A more robust solution would involve hashing the spec or comparing specific fields.

		log.Info("Monitored Deployment detected", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

		// Fetch SMTP password from Secret
		smtpPassword, err := r.getSMTPPassword(ctx, deploymentMonitor)
		if err != nil {
			log.Error(err, "Failed to get SMTP password from secret", "Secret.Name", deploymentMonitor.Spec.SMTPPasswordSecretRef.Name)
			// Don't block other reconciliations, but log the error.
			continue
		}

		// Construct email content
		subject := fmt.Sprintf("Deployment Change Alert: %s/%s", dep.Namespace, dep.Name)
		body := fmt.Sprintf("Deployment %s/%s has been updated or reconciled.\n\nDetails:\nImage: %s\nReplicas: %d\n\nThis is an automated notification from your Kubernetes Deployment Monitor Operator.",
			dep.Namespace, dep.Name, dep.Spec.Template.Spec.Containers[0].Image, *dep.Spec.Replicas) // Assuming first container for image

		// Send email
		err = SendEmail(
			deploymentMonitor.Spec.SMTPServer,
			deploymentMonitor.Spec.SMTPPort,
			deploymentMonitor.Spec.SMTPUsername,
			smtpPassword,
			deploymentMonitor.Spec.RecipientEmail,
			subject,
			body,
		)
		if err != nil {
			log.Error(err, "Failed to send email notification", "Recipient", deploymentMonitor.Spec.RecipientEmail)
			// Continue to process other deployments/monitors even if one email fails
		} else {
			log.Info("Email notification sent successfully", "Recipient", deploymentMonitor.Spec.RecipientEmail, "Deployment", dep.Name)
			// Update status to record last notification time, if status fields were defined
			// deploymentMonitor.Status.LastNotificationTime = metav1.NewTime(time.Now())
			// if err := r.Status().Update(ctx, deploymentMonitor); err != nil {
			// 	log.Error(err, "Failed to update DeploymentMonitor status")
			// 	return ctrl.Result{}, err
			// }
		}
	}

	// Requeue after a certain duration to periodically check for changes,
	// or rely solely on watch events for more immediate reactions.
	// For now, we'll rely on watch events.
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

// getSMTPPassword retrieves the SMTP password from the specified Kubernetes Secret.
func (r *DeploymentMonitorReconciler) getSMTPPassword(ctx context.Context, dm *monitorv1.DeploymentMonitor) (string, error) {
	if dm.Spec.SMTPPasswordSecretRef == nil {
		return "", fmt.Errorf("SMTPPasswordSecretRef is not defined in DeploymentMonitor %s/%s", dm.Namespace, dm.Name)
	}

	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Name:      dm.Spec.SMTPPasswordSecretRef.Name,
		Namespace: dm.Namespace, // Assuming secret is in the same namespace as DeploymentMonitor
	}

	err := r.Get(ctx, secretName, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", secretName.Namespace, secretName.Name, err)
	}

	passwordBytes, ok := secret.Data[dm.Spec.SMTPPasswordSecretRef.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s/%s", dm.Spec.SMTPPasswordSecretRef.Key, secretName.Namespace, secretName.Name)
	}

	return string(passwordBytes), nil
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
