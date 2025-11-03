// api/v1/deploymentmonitor_types.go
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentMonitorSpec defines the desired state of DeploymentMonitor
type DeploymentMonitorSpec struct {
	MonitoredAnnotationKey   string           `json:"monitoredAnnotationKey,omitempty"`
	MonitoredAnnotationValue string           `json:"monitoredAnnotationValue,omitempty"`
	MonitoredLabelKey        string           `json:"monitoredLabelKey,omitempty"`
	MonitoredLabelValue      string           `json:"monitoredLabelValue,omitempty"`
	RecipientEmail           string           `json:"recipientEmail"`
	SMTPServergo             string           `json:"smtpServer"`
	SMTPPort                 int              `json:"smtpPort"`
	SMTPUsername             string           `json:"smtpUsername"`
	SMTPPasswordSecretRef    *SecretReference `json:"smtpPasswordSecretRef,omitempty"`
}

// SecretReference defines a reference to a secret key
type SecretReference struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// DeploymentMonitorStatus defines the observed state of DeploymentMonitor
type DeploymentMonitorStatus struct {
	// Add status fields if needed, e.g., lastNotificationTime, observedDeployments
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DeploymentMonitor is the Schema for the deploymentmonitors API
type DeploymentMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentMonitorSpec   `json:"spec,omitempty"`
	Status DeploymentMonitorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeploymentMonitorList contains a list of DeploymentMonitor
type DeploymentMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentMonitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentMonitor{}, &DeploymentMonitorList{})
}
