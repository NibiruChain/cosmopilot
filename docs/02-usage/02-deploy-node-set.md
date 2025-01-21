# Deploying a Node Set

This section provides a step-by-step guide to deploying and managing a group of nodes using the [ChainNodeSet](/03-reference/crds/crds#chainnodeset) resource in `Cosmopilot`.

## Introduction


The [ChainNodeSet](/03-reference/crds/crds#chainnodeset) resource offers a powerful and flexible way to deploy and manage multiple blockchain nodes within a single manifest. It is particularly well-suited for production environments and testnets where multiple node types need to work together seamlessly.

Compared to individual [ChainNode](/03-reference/crds/crds#chainnode) resources, [ChainNodeSet](/03-reference/crds/crds#chainnodeset) provides several key advantages:
- **Flexible Group Management**: Deploy and configure groups of nodes (e.g., full nodes, sentry nodes) with distinct settings, all in one resource.
- **Efficient Endpoint Exposure**: Configure group-level or global ingresses to expose API endpoints for multiple node groups, simplifying access management.
- **Disruption Checks**: Automatically ensure minimal downtime by performing disruption checks on each group during updates or maintenance.

With these features, [ChainNodeSet](/03-reference/crds/crds#chainnodeset) simplifies the deployment and management of complex Cosmos-based blockchain setups, making it the preferred choice for most use cases.

## Base Configuration

Below is a base manifest example for deploying a node set with one archive node and two full nodes:

```yaml
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNodeSet
metadata:
  name: nibiru-cataclysm-1
spec:
  app:
    app: nibid # Binary to be used
    image: ghcr.io/nibiruchain/nibiru # Container image repository of the application
    version: 1.0.0 # Version to be used

  genesis:
    url: https://raw.githubusercontent.com/NibiruChain/Networks/refs/heads/main/Mainnet/cataclysm-1/genesis.json

  nodes:
    - name: fullnode
      instances: 2

      config:
        override:
          app.toml:
            pruning: custom
            pruning-keep-recent: "100"
            pruning-interval: "10"

    - name: archive
      instances: 1

      config:
        override:
          app.toml:
            pruning: nothing
```

## Managed Resources

When you create a [ChainNodeSet](/03-reference/crds/crds#chainnodeset), `Cosmopilot` automatically manages and creates several resources required to deploy and manage groups of nodes. These include:

- **ChainNodes**: The [ChainNodeSet](/03-reference/crds/crds#chainnodeset) primarily creates and manages individual [ChainNode](/03-reference/crds/crds#chainnode) resources for each node in the set. As a result, all resources managed for [ChainNode](/03-reference/crds/crds#chainnode) (e.g., Pods, ConfigMaps, Services, Secrets, and Service Monitors) are implicitly created and managed as well.

- **Group Services**:
  - A service is created for each group of nodes in the [ChainNodeSet](/03-reference/crds/crds#chainnodeset). These services target all nodes within the corresponding group, simplifying internal and external communication for specific roles (e.g., all full nodes or sentries).

- **Ingresses**:
  - **Per-Group Ingresses**: Ingresses can be created for individual groups of nodes to expose their endpoints externally.
  - **Global Ingresses**: A single ingress can be created to target nodes across multiple groups, enabling centralized access to shared APIs.

- **ConfigMaps**:
  - If the [ChainNodeSet](/03-reference/crds/crds#chainnodeset) controller handles the genesis file, a `ConfigMap` is created to store it. This allows all nodes within the set to share a consistent genesis configuration.

The [ChainNodeSet](/03-reference/crds/crds#chainnodeset) simplifies managing large-scale deployments by automating the creation of multiple [ChainNode](/03-reference/crds/crds#chainnode) resources while providing additional flexibility through group-level services, ingresses, and centralized genesis configuration.

## Scaling Node Groups

To scale a specific group of nodes, modify the `instances` field in the manifest and re-apply it:

```yaml
groups:
  - name: fullnode
    instances: 2 // [!code --]
    instances: 3 // [!code ++]
```

Re-apply the configuration:

```bash
$ kubectl apply -f nodeset.yaml
```

The additional nodes will be created automatically.