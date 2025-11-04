// api/v1/deploymentmonitor_types.go
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentMonitorSpec defines the desired state of DeploymentMonitor
type DeploymentMonitorSpec struct {
	MonitoredAnnotationKey   string `json:"monitoredAnnotationKey,omitempty"`
	MonitoredAnnotationValue string `json:"monitoredAnnotationValue,omitempty"`
	MonitoredLabelKey        string `json:"monitoredLabelKey,omitempty"`
	MonitoredLabelValue      string `json:"monitoredLabelValue,omitempty"`
	RecipientEmail           string `json:"recipientEmail"`
	SMTPSecretName           string `json:"smtpSecretName,omitempty"`
	// EmailTemplate is an optional Go template string for the email body.
	// If provided, it will be used to format the email content.
	// The template will receive a data structure with fields:
	// .Namespace, .Name, .Image, .Replicas.
	// Example: "Deployment {{.Namespace}}/{{.Name}} updated. Image: {{.Image}}, Replicas: {{.Replicas}}"
	// +optional
	EmailTemplate string `json:"emailTemplate,omitempty"`
}

// DeploymentState defines the state of a single deployment for monitoring purposes.
type DeploymentState struct {
	Image    string `json:"image,omitempty"`
	Replicas int32  `json:"replicas,omitempty"`
}

// DeploymentMonitorStatus defines the observed state of DeploymentMonitor
type DeploymentMonitorStatus struct {
	// LastNotificationTime is the last time an email was sent for any monitored deployment.
	// +optional
	LastNotificationTime *metav1.Time `json:"lastNotificationTime,omitempty"`
	// LastNotifiedDeploymentMap stores the last observed state (image, replicas) for each monitored deployment.
	// The key is "namespace/name" of the deployment.
	// +optional
	LastNotifiedDeploymentMap map[string]DeploymentState `json:"lastNotifiedDeploymentMap,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// DeploymentMonitor is the Schema for the deploymentmonitors API
type DeploymentMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentMonitorSpec   `json:"spec,omitempty"`
	Status DeploymentMonitorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeploymentMonitorList contains a list of DeploymentMonitor
type DeploymentMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentMonitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentMonitor{}, &DeploymentMonitorList{})
}
