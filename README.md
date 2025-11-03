# Kubernetes Deployment Monitor Operator

This operator monitors Kubernetes Deployments across all namespaces for specific annotations or labels and sends email notifications when changes are detected.

## Table of Contents

- [Kubernetes Deployment Monitor Operator](#kubernetes-deployment-monitor-operator)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Features](#features)
  - [Prerequisites](#prerequisites)
  - [Getting Started](#getting-started)
    - [1. Install Kubebuilder](#1-install-kubebuilder)
    - [2. Clone the Repository](#2-clone-the-repository)
    - [3. Build and Deploy the Operator](#3-build-and-deploy-the-operator)
    - [4. Create an SMTP Secret](#4-create-an-smtp-secret)
    - [5. Create a DeploymentMonitor Custom Resource](#5-create-a-deploymentmonitor-custom-resource)
    - [6. Test with a Monitored Deployment](#6-test-with-a-monitored-deployment)
  - [Custom Resource Definition (CRD)](#custom-resource-definition-crd)
    - [`DeploymentMonitor` Spec](#deploymentmonitor-spec)
  - [How it Works](#how-it-works)
  - [Contributing](#contributing)
  - [License](#license)

## Overview

The Kubernetes Deployment Monitor Operator is designed to provide proactive notifications for changes within your cluster's Deployments. By leveraging Kubernetes Custom Resources (CRDs), users can define notification rules based on Deployment annotations or labels. When a monitored Deployment changes (e.g., spec update, status change), the operator sends an email notification to the configured recipient.

This operator is built using [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) [2] and follows best practices for Kubernetes operator development.

## Features

- **Cluster-wide Monitoring**: Watches Deployments in all namespaces for specified annotations or labels.
- **Customizable Rules**: Define monitoring criteria using the `DeploymentMonitor` Custom Resource.
- **Email Notifications**: Sends email alerts when a change is detected in a monitored Deployment.
- **Secure SMTP Configuration**: Integrates with SMTP servers using Kubernetes Secrets for credentials.

## Prerequisites

- Go v1.24.5+
- Docker v17.03+
- kubectl v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster (e.g., minikube, kind, or a cloud-managed cluster).
- An accessible image registry (e.g., Docker Hub, Quay.io) for publishing operator images.

## Getting Started

Follow these steps to get your Deployment Monitor Operator up and running.

### 1. Install Kubebuilder

If you don't have Kubebuilder installed, follow the official installation guide [2]:

```bash
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
```

### 2. Clone the Repository

```bash
git clone https://github.com/your-username/deployment-monitor-operator.git
cd deployment-monitor-operator
```

*(Replace `https://github.com/your-username/deployment-monitor-operator.git` with your actual repository URL)*

### 3. Build and Deploy the Operator

First, build the operator's Docker image and push it to your registry. Remember to replace `your-registry/your-repo` with your actual image path.

```bash
# Set your image organization/repo
IMG="your-registry/your-repo/deployment-monitor-operator:latest"

# Build the Docker image
make docker-build IMG="${IMG}"

# Push the Docker image to your registry
make docker-push IMG="${IMG}"

# Deploy the operator to your Kubernetes cluster
make deploy IMG="${IMG}"
```

### 4. Create an SMTP Secret

Your operator needs credentials to send emails. Create a Kubernetes Secret in the same namespace where your operator is deployed (e.g., `deployment-monitor-system`, if you didn't change it).

```yaml
# config/samples/smtp-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-smtp-secret
  namespace: deployment-monitor-system # IMPORTANT: Ensure this matches your operator's namespace
type: Opaque
data:
  password: <base64-encoded-smtp-password> # Replace with `echo -n "your-smtp-password" | base64`
```

Apply this secret:

```bash
kubectl apply -f config/samples/smtp-secret.yaml
```

### 5. Create a DeploymentMonitor Custom Resource

Define a `DeploymentMonitor` custom resource that specifies what Deployments to watch and where to send notifications.

```yaml
# config/samples/monitor_v1_deploymentmonitor.yaml
apiVersion: monitor.example.com/v1
kind: DeploymentMonitor
metadata:
  name: critical-app-monitor
spec:
  monitoredAnnotationKey: "monitor.example.com/critical"
  monitoredAnnotationValue: "true" # Monitor Deployments with this annotation key and value
  # monitoredLabelKey: "app.kubernetes.io/component" # Uncomment and configure for label-based monitoring
  # monitoredLabelValue: "backend"
  recipientEmail: "your-alert-email@example.com" # Replace with your notification email
  smtpServer: "smtp.your-provider.com"          # Replace with your SMTP server
  smtpPort: 587                                # Replace with your SMTP port
  smtpUsername: "your-smtp-username@your-provider.com" # Replace with your SMTP username
  smtpPasswordSecretRef:
    name: my-smtp-secret
    key: password
```

Apply your `DeploymentMonitor` CR:

```bash
kubectl apply -f config/samples/monitor_v1_deploymentmonitor.yaml
```

### 6. Test with a Monitored Deployment

Create or update a Deployment with the annotation/label specified in your `DeploymentMonitor` CR.

```yaml
# test-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-critical-app
  namespace: default
  annotations:
    monitor.example.com/critical: "true" # This annotation matches your DeploymentMonitor CR
spec:
  replicas: 2
  selector:
    matchLabels:
      app: example-critical-app
  template:
    metadata:
      labels:
        app: example-critical-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.23.0 # Initial image
        ports:
        - containerPort: 80
```

Apply this Deployment:

```bash
kubectl apply -f test-deployment.yaml
```

You should observe the operator picking up this Deployment. Now, make a change to the Deployment, for example, update its image:

```yaml
# test-deployment-updated.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-critical-app
  namespace: default
  annotations:
    monitor.example.com/critical: "true"
spec:
  replicas: 2
  selector:
    matchLabels:
      app: example-critical-app
  template:
    metadata:
      labels:
        app: example-critical-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.24.0 # Updated image
        ports:
        - containerPort: 80
```

Apply the update:

```bash
kubectl apply -f test-deployment-updated.yaml
```

Check your configured email address for a notification. You can also view the operator logs:

```bash
kubectl logs -f -l control-plane=controller-manager -n deployment-monitor-system
```

## Custom Resource Definition (CRD)

### `DeploymentMonitor` Spec

The `DeploymentMonitor` Custom Resource defines the criteria for monitoring Deployments and the email notification settings.

| Field                     | Type                           | Description                                                                                                                              | Required |
| :------------------------ | :----------------------------- | :--------------------------------------------------------------------------------------------------------------------------------------- | :------- |
| `monitoredAnnotationKey`  | `string`                       | The key of the annotation to look for on Deployments.                                                                                    | No       |
| `monitoredAnnotationValue`| `string`                       | The value of the annotation key. If empty, the presence of the key is sufficient.                                                        | No       |
| `monitoredLabelKey`       | `string`                       | The key of the label to look for on Deployments.                                                                                         | No       |
| `monitoredLabelValue`     | `string`                       | The value of the label key. If empty, the presence of the key is sufficient.                                                             | No       |
| `recipientEmail`          | `string`                       | The email address to which notifications will be sent.                                                                                   | Yes      |
| `smtpServer`              | `string`                       | The hostname or IP address of the SMTP server.                                                                                           | Yes      |
| `smtpPort`                | `integer`                      | The port of the SMTP server (e.g., 587 for TLS/STARTTLS).                                                                                | Yes      |
| `smtpUsername`            | `string`                       | The username for SMTP authentication.                                                                                                    | Yes      |
| `smtpPasswordSecretRef`   | `SecretReference`              | A reference to a Kubernetes Secret containing the SMTP password.                                                                         | Yes      |
| `smtpPasswordSecretRef.name`| `string`                     | The name of the Secret that stores the SMTP password.                                                                                    | Yes      |
| `smtpPasswordSecretRef.key`| `string`                      | The key within the Secret's `data` field that holds the password.                                                                        | Yes      |

## How it Works

1. **`DeploymentMonitor` CR**: You create a `DeploymentMonitor` custom resource to configure your monitoring rules. This specifies the annotation/label to watch for, the recipient email, and SMTP server details.
2. **Controller Watches**: The operator's controller monitors two types of resources:
    - `DeploymentMonitor` instances: To know which Deployments to monitor and how to notify.
    - `Deployments` across all namespaces: To detect changes in any Deployment that matches a configured `DeploymentMonitor` rule. The manager's default behavior watches all namespaces when no specific namespace is configured [1].
3. **Change Detection**: When a Deployment is created, updated, or deleted, or when its status changes, the controller compares its current state against the criteria defined in any existing `DeploymentMonitor` CRs.
4. **Email Notification**: If a Deployment matches the monitoring criteria and a significant change is detected, the operator constructs an email and sends it to the specified recipient using the configured SMTP server and credentials stored securely in a Kubernetes Secret.

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for more details.

## License

This project is licensed under the Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.
