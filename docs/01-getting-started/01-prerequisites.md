# Prerequisites

To use `Cosmopilot`, ensure you meet the following prerequisites before proceeding with installation and configuration.

## **Kubernetes Cluster**
- **Version**: Kubernetes **v1.29** or higher is required.
- **Optional but strongly recommended for full functionality**:
  - **[NGINX Ingress Controller](https://docs.nginx.com/nginx-ingress-controller/)**: for exposing API endpoints.
  - **[cert-manager](https://cert-manager.io/docs/)**: for exposing API endpoints with TLS and for admission webhooks (which can be optionally disabled).
  - **[Volume Snapshot](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) controller**: for data volume snapshots support.


## **Tools**
- **[helm](https://helm.sh/)**: Required for installing `Cosmopilot`.
- **[kubectl](https://kubernetes.io/docs/reference/kubectl/)**: Required for creating and managing resources in your Kubernetes cluster.

## **Container Image Requirements**

Cosmopilot follows Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) and enforces the **restricted** profile by default:

- All containers run as non-root user (UID 1000)
- All Linux capabilities are dropped
- Privilege escalation is disabled
- Filesystem group is set to GID 1000

Your blockchain node images must support running as a non-root user. Most Cosmos SDK chain images are compatible with this requirement out of the box.

::: tip
If your image requires different security settings, you can override the security context. See [Security Context Overrides](/02-usage/04-node-config#security-context-overrides) for details.
:::

## **Chain Compatibility**

Cosmopilot works with chains built on the Cosmos SDK. Before deploying, verify your chain is compatible by checking the [Chain Compatibility](02-chain-compatibility) page, which lists:
- Required CLI commands your chain binary must support
- Required API endpoints (gRPC and RPC)
- Tested chains and their verification status

---

Ensure all dependencies are installed, your cluster is correctly configured, and your chain is compatible before proceeding to the [Installation](03-installation) page.