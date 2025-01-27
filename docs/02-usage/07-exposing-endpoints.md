# Exposing Endpoints

This page provides guidance on exposing endpoints for nodes managed by `Cosmopilot`. Exposing `API` endpoints relies on the creation of ingress resources, which are designed to work exclusively with `NGINX` (and `cert-manager` for securing the endpoints). For this reason, `NGINX` and `cert-manager` are included in the requirements listed in the [Prerequisites](/01-getting-started/01-prerequisites) page.

The only exception is the `P2P` port, which can be exposed without `NGINX` and `cert-manager`.

## Exposing the P2P Port

The P2P port can be exposed by configuring the `.spec.expose` field on a `ChainNode` or the `.spec.nodes[].expose` field on a `ChainNodeSet`. By default, this feature is disabled.

```yaml
expose:
  p2p: true
  p2pServiceType: LoadBalancer # Optional. Default is `NodePort`.
  annotations: # Optional annotations to add to the P2P service.
    key1: value1
    key2: value2
```

### **Supported Service Types**
- **NodePort**: 
  - Exposes P2P traffic on a NodePort across all Kubernetes nodes.
  - This option is cost-effective as it uses existing Kubernetes node resources, but it requires Kubernetes nodes to have public access.
- **LoadBalancer**: 
  - On most cloud providers, this creates a dedicated Load Balancer mapped to the node.
  - Offers better performance but incurs higher costs due to the additional infrastructure.

## Exposing API Endpoints

API endpoints can be enabled either:
1. **Per Group of Nodes**: Configure under `.spec.nodes[].ingress`.
2. **Global Ingress**: Configure under `.spec.ingresses` to target multiple groups of nodes (this is the recommended approach, as per-group ingress configuration might be deprecated in the future).

### Per Group Ingress Example

To expose API endpoints for a specific group of nodes, use the following configuration under `.spec.nodes[].ingress`:

```yaml
ingress:
  host: nodes.example.com 
  enableRPC: true # optional. Defaults to `false`.
  enableGRPC: true # optional. Defaults to `false`.
  enableLCD: true # optional. Defaults to `false`.
  enableEvmRPC: false # optional. Defaults to `false`.
  enableEvmRpcWS: false # optional. Defaults to `false`.
  annotations: # optional annotations to ingress resource
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
  disableTLS: false # optional. Defaults to `false`.
  tlsSecretName: example-tls-secret # optional. Defaults to `<service-name>-tls`.
```

### **Global Ingress Example**

To expose API endpoints for multiple groups of nodes, configure them under `.spec.ingresses`. Here’s an example:

```yaml
ingresses:
  - name: global-ingress
    groups:
      - fullnode
      - archive
    host: api.nodes.example.com
    enableRPC: true # optional. Defaults to `false`.
    enableGRPC: true # optional. Defaults to `false`.
    enableLCD: true # optional. Defaults to `false`.
    enableEvmRPC: true # optional. Defaults to `false`.
    enableEvmRpcWS: true # optional. Defaults to `false`.
    annotations: # optional annotations to ingress resource
      nginx.ingress.kubernetes.io/proxy-body-size: "100m"
    disableTLS: false # optional. Defaults to `false`.
    tlsSecretName: global-tls-secret # optional. Defaults to `<service-name>-tls`.
```

::: info NOTE
Each `API` endpoint is exposed as a subdomain of the configured `host` as follows. These are not configurable.
- Tendermint RPC is available at `rpc.<host>`.
- Cosmos-SDK RPC is available at `lcd.<host>`.
- gRPC is available at `grpc.<host>`.
- EVM RPC is available at `evm-rpc.<host>`.
- EVM RPC Websocket is available at `evm-rpc-ws.<host>`.
:::

### Recommended Approach

For flexibility and better scalability, it is recommended to use `.spec.ingresses` to configure API endpoints instead of per-group ingress configurations.