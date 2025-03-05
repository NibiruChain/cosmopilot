# Deploying a Node

This section provides a step-by-step guide to creating and managing a single node using `Cosmopilot`.

## Introduction

The [ChainNode](/03-reference/crds/crds#chainnode) resource is the foundational Custom Resource Definition (CRD) in Cosmopilot for deploying a single Cosmos-based blockchain node. It enables users to specify key configurations such as application details, genesis file, peer connections, and additional options, ensuring a smooth and streamlined deployment experience.

While deploying a single [ChainNode](/03-reference/crds/crds#chainnode) is practical in certain scenarios, such as development or lightweight setups, most production use cases—like operating a complete testnet or running a validator network—will benefit from using the [ChainNodeSet](/03-reference/crds/crds#chainnodeset) resource. The [ChainNodeSet](/03-reference/crds/crds#chainnodeset) allows you to deploy and manage multiple groups of nodes (e.g., validators, full nodes, archive nodes) in a single manifest. For more details, refer to the [Deploying a Node Set](02-deploy-node-set).

## Base Configuration

To deploy a single node, you need to define a [ChainNode](/03-reference/crds/crds#chainnode) resource. Below is a base manifest example for deploying a [Nibiru](https://nibiru.fi/) node:

```yaml
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNode
metadata:
  name: nibiru-cataclysm-1-fullnode
spec:
  app:
    app: nibid # Binary to be used
    image: ghcr.io/nibiruchain/nibiru # Container image repository of the application
    version: 1.0.0 # Version to be used

  genesis:
    url: https://raw.githubusercontent.com/NibiruChain/Networks/refs/heads/main/Mainnet/cataclysm-1/genesis.json

  peers:
  - id: 151acb0de556f4a059f9bd40d46190ee91f06422
    address: 34.38.151.176
  - id: d3c7f343d7ed815b73eef34d7d37948f10a1deab
    address: 34.76.80.206
  - id: 36a232cf6a3fb166750f003e3abd5249e86aeed8
    address: 15.235.115.154
    port: 16700
  - id: 98cadded622d291141f8a83972fa046267df94b6
    address: 38.109.200.36
    port: 44441
  - id: d2c01f9aee9fedbd9e14c42fff5179d7f53f72f9
    address: 174.138.180.190
    port: 60656
  - id: 8d8324141897243927359345bb4b1bb78a1e1df1
    address: 65.109.56.235
```

This example highlights the three mandatory configuration sections required to deploy a node:

- [app](/03-reference/crds/crds#appspec): Specifies the application to deploy, including the binary name, container image repository, and version.
- [genesis](/03-reference/crds/crds#genesisconfig): Defines the source of the genesis file required to initialize the node (not required if [Initializing a new Network](10-initializing-new-network)).
- [peers](/03-reference/crds/crds#peer): Lists the peer nodes to connect with for network communication.

To create the node, save the manifest to a file (e.g. `node.yaml`) and apply it to your Kubernetes cluster:

```bash
$ kubectl apply -f node.yaml
```

Once the node starts, you can check its status using the following command:

```bash
$ kubectl get chainnodes
NAME                          STATUS    IP          VERSION   CHAINID       VALIDATOR   BONDSTATUS   JAILED   DATAUSAGE   LATESTHEIGHT   AGE
nibiru-cataclysm-1-fullnode   Syncing   10.8.5.32   1.1.0     cataclysm-1   false                             1%          57             2m56s
```

At this point, the node will begin syncing to catch up with the network. While this is necessary for creating an archive node with the entire network history, it may take a significant amount of time for larger chains.

For a quicker setup of a full node, consider restoring from a data snapshot. You can find detailed instructions on this process in the [Restore from Snapshot](06-restoring-from-snapshot) page.


## Managed Resources

When you create a [ChainNode](/03-reference/crds/crds#chainnode), `Cosmopilot` automatically manages and creates several resources required to run the node. These include:

- **Pod**: Contains the main application container, a `node-utils` container, and any other optionally configured sidecar containers.
- **ConfigMaps**:
  - One `ConfigMap` contains all application configuration files (e.g., `config.toml`, `app.toml`, etc.).
  - Another `ConfigMap` contains information about upgrades.
- **Services**:
  - An internal service for cluster communication (ignores readiness probes).
  - A public service for external API access (respects readiness probes).
  - A service dedicated to peer-to-peer (P2P) traffic.
- **Secrets**:
  - Secrets are created only when necessary and may include:
    - **Private Consensus Key**: Generated for validators (not created if [TmKMS is being used](11-tmkms)). If TmKMS is in use, an additional secret with the KMS identity is created.
    - **Account Mnemonic**: Used with validators for starting a new network or submiting `create-validator` transaction.
- **Service Monitors**: A `ServiceMonitor` is created to enable Prometheus to scrape metrics from the node.

::: warning Important
Storing mnemonics and private keys in Kubernetes secrets may not be secure and is recommended only for testnets. For production networks, consider using [TmKMS](11-tmkms) for enhanced security.
:::

## Accessing Node Endpoints

Once the node is running, you can access its endpoints by using `kubectl` to port-forward traffic to either the node's containers or services. For example, to access the `RPC`, `LCD`, and `gRPC` endpoints of the node deployed earlier, you can run:

```bash
$ kubectl port-forward svc/nibiru-cataclysm-1-fullnode-internal 26657 1317 9090
```

After setting up port-forwarding:
- **RPC**: will be available at [localhost:26657](http://localhost:26657).
- **LCD**: will be available at [localhost:1317](http://localhost:1317).
- **gRPC**: will be available at `localhost:9090`.

You can test the `gRPC` endpoint using [grpcurl](https://github.com/fullstorydev/grpcurl):

```bash
$ grpcurl --plaintext localhost:9090 list
```

For more ways to access these endpoints, refer to the [Exposing Endpoints](07-exposing-endpoints) page.