# Configuration

This page describes all Helm configuration options available for installing and customizing `Cosmopilot`.
These settings allow you to tailor the deployment to your specific needs.

You can find the full list of available Helm values [here](https://github.com/NibiruChain/cosmopilot/blob/main/helm/cosmopilot/values.yaml), or you can run:

```bash
$ helm show values oci://ghcr.io/nibiruchain/helm/cosmopilot
```


## **General Settings**

### `replicas`
- **Description**: Number of replicas for the Cosmopilot deployment. If more than one, leader election will be enabled.
- **Default**: `1`

### `probesEnabled`
- **Description**: Enable or disable health and readiness probes for `cosmopilot` deployment.
- **Default**: `true`

### `image`
- **Description**: The container image repository for the Cosmopilot operator.
- **Default**: `ghcr.io/nibiruchain/cosmopilot`

### `imageTag`
- **Description**: The tag with the version to be used.
- **Default**: Defaults to specified app version in helm.

### `nodeUtilsImage`
- **Description**: The container image of `node-utils` (with version tag included). This is a container deployed by `cosmopilot` as a sidecar with helper methods for calculating data size, handling upgrades, and a few more utilities.
- **Default**: `ghcr.io/nibiruchain/node-utils`

### `cosmoGuardImage`
- **Description**: The container image of [CosmoGuard](https://github.com/NibiruChain/cosmoguard) (with version tag included).
- **Default**: `ghcr.io/nibiruchain/cosmoguard`

### `cosmoseedImage`
- **Description**: The container image of [Cosmoseed](https://github.com/NibiruChain/cosmoseed) (with version tag included). Used when deploying seed nodes.
- **Default**: `ghcr.io/nibiruchain/cosmoseed`

### `imagePullSecrets`
- **Description**: Secrets for pulling images from private repositories.
- **Default**: `[]`

## **Worker Configuration**

### `workerCount`
- **Description**: Number of workers to handle tasks within the `cosmopilot` operator.
- **Default**: `1`

### `workerName`
- **Description**: Name of the worker. Useful if you need multiple installations of `cosmopilot`. You can later define on resources which worker to use.
- **Default**: `""` (empty string)

## **Features and Functionality**

### `webHooksEnabled`
- **Description**: Enable or disable admission webhooks for validating and mutating requests. Ensure [cert-manager](https://cert-manager.io/docs/) is installed before enabling this.
- **Default**: `true`

### `debugMode`
- **Description**: Enable debug mode for additional logs.
- **Default**: `false`

### `disruptionChecksEnabled`
- **Description**: Enable or disable disruption checks to avoid nodes downtime during updates or maintenance.
- **Default**: `true`

## **Pod Priority Settings**

### `nodesPodPriority`
- **Description**: Priority for pods representing blockchain nodes.
- **Default**: `950`

### `validatorPodPriority`
- **Description**: Priority for validator pods.
- **Default**: `1050`

### `defaultPriority`
- **Description**: Default pod priority for all pods without specific roles.
- **Default**: `0`

## **Node Configuration**

### `nodeSelector`
- **Description**: Node selectors to control where `Cosmopilot` components are scheduled.
- **Default**: `{}` (no specific node selector)
