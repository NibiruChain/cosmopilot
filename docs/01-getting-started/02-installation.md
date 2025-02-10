# Installation

Follow these steps to install and start using `Cosmopilot` in your Kubernetes cluster.

## Install using Helm

### 1. Prerequisites

Ensure you have all necessary prerequisites installed. Refer to the [Prerequisites](01-prerequisites) page for more details.

### 2. Install Cosmopilot

Use the following command to install `Cosmopilot` using Helm:

```bash
$ helm install \
    cosmopilot oci://ghcr.io/nibiruchain/helm/cosmopilot \
    --namespace cosmopilot-system \
    --create-namespace
```

Or, if you need to install a specific version:

```bash
$ helm install \
    cosmopilot oci://ghcr.io/nibiruchain/helm/cosmopilot \
    --namespace cosmopilot-system \
    --create-namespace \
    --version 1.35.2
```

---

If you opted for not installing any of the recomended controllers in [Prerequisites](01-prerequisites) page, you need to disable both webhooks and service monitors:

```bash
$ helm install \
    cosmopilot oci://ghcr.io/nibiruchain/helm/cosmopilot \
    --namespace cosmopilot-system \
    --create-namespace \
    --set serviceMonitorEnabled=false \
    --set webHooksEnabled=false
```

## Installation Options

A full list of available Helm values is available [here](https://github.com/NibiruChain/cosmopilot/blob/main/helm/cosmopilot/values.yaml).
For other advanced configurations please refer to the [Configuration](03-configuration) page.

