# Installation

Follow these steps to install and start using `Cosmopilot` in your Kubernetes cluster.

## Install using Helm

### 1. Prerequisites

Ensure you have all necessary prerequisites installed. Refer to the [Prerequisites](01-prerequisites) page for more details.

### 2. Install Cosmopilot

Use the following command to install `Cosmopilot` using Helm:

```bash
$ helm install \
    cosmopilot oci://ghcr.io/voluzi/helm/cosmopilot \
    --namespace cosmopilot-system \
    --create-namespace
```

Or, if you need to install a specific version:

```bash
$ helm install \
    cosmopilot oci://ghcr.io/voluzi/helm/cosmopilot \
    --namespace cosmopilot-system \
    --create-namespace \
    --version 1.35.2
```

---

If you opted for not installing cert-manager (one of the recommended controllers in [Prerequisites](01-prerequisites) page), you need to disable webhooks:

```bash
$ helm install \
    cosmopilot oci://ghcr.io/voluzi/helm/cosmopilot \
    --namespace cosmopilot-system \
    --create-namespace \
    --set webHooksEnabled=false
```

## Installation Options

A full list of available Helm values is available [here](https://github.com/voluzi/cosmopilot/blob/main/helm/cosmopilot/values.yaml).
For other advanced configurations please refer to the [Configuration](04-configuration) page.

