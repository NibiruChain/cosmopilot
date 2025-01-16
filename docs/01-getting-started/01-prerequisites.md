# Prerequisites

To use `Cosmopilot`, ensure you meet the following prerequisites before proceeding with installation and configuration.

## **Kubernetes Cluster**
- **Version**: Kubernetes **v1.29** or higher is required.
- **Optional but strongly recommended for full functionality**:
  - **[NGINX Ingress Controller](https://docs.nginx.com/nginx-ingress-controller/)**: for exposing API endpoints.
  - **[cert-manager](https://cert-manager.io/docs/)**: for exposing API endpoints with TLS and for admission webooks (which can be optionally disabled).
  - **[Volume Snapshot](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) controller**: for data volume snapshots support.
  - **[Prometheus Operator](https://prometheus-operator.dev)**: for `cosmopilot` to create service monitors (can be optionally disabled).


## **Tools**
- **[helm](https://helm.sh/)**: Required for installing `Cosmopilot`.
- **[kubectl](https://kubernetes.io/docs/reference/kubectl/)**: Required for creating and managing resources in your Kubernetes cluster.

---

Ensure all dependencies are installed and your cluster is correctly configured before proceeding to the [Installation](02-installation) page.