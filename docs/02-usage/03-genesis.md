# Genesis Download

This section explains the methods available for downloading or specifying the genesis file when deploying a node or initializing a network using `Cosmopilot`. The genesis file defines the blockchain's initial state and parameters, making it a critical component for connecting to the correct network.

## Download from a URL

You can specify a URL to fetch the genesis file directly. This is the simplest and most common method for established networks.

### Example Configuration

```yaml
genesis:
  url: https://raw.githubusercontent.com/NibiruChain/Networks/main/Mainnet/cataclysm-1/genesis.json
```

**Notes**
- The `URL` must point to a publicly accessible genesis file.
- Ensure the `URL` is updated to match the desired network or version.

## Fetch from an RPC Endpoint

Its also possible to configure your [ChainNode](/03-reference/crds/crds#chainnode) or [ChainNodeSet](/03-reference/crds/crds#chainnodeset) to fetch the genesis directly from another node’s `RPC` endpoint.

### Example Configuration

```yaml
genesis:
  fromNodeRPC:
    hostname: rpc.nibiru.fi
    port: 443 # Optional. Defaults to 26657
    secure: true # Optional. Defaults to false
```

**Notes**
- Ensure the `RPC` endpoint is accessible from the cluster.
- This method is useful for networks that regularly update their genesis file or for quickly bootstrapping nodes in test environments.

## Load from a ConfigMap

For private networks or custom configurations, you can use a Kubernetes `ConfigMap` to provide the genesis file. This is useful for managing genesis files directly within your cluster.

### Steps

1. Create a `ConfigMap` with the genesis file:

```bash
$ kubectl create configmap custom-genesis --from-file=genesis.json
```
Make sure the `ConfigMap` is created in the same namespace as your [ChainNode](/03-reference/crds/crds#chainnode) or [ChainNodeSet](/03-reference/crds/crds#chainnodeset).

2. Reference the `ConfigMap` in your [ChainNode](/03-reference/crds/crds#chainnode) or [ChainNodeSet](/03-reference/crds/crds#chainnodeset) manifest:

```yaml
genesis:
  configMap: custom-genesis
```

> [!IMPORTANT]
> `Cosmopilot` expects the genesis file name to be `genesis.json`.

## Large Genesis

For genesis files larger than the 1MiB limit of Kubernetes `ConfigMaps`, `Cosmopilot` provides the `useDataVolume` option. This allows the genesis file to be stored directly in the same volume as the node’s data.

### Example Configuration

```yaml
genesis:
  url: https://raw.githubusercontent.com/NibiruChain/Networks/main/Mainnet/cataclysm-1/genesis.json
  useDataVolume: true
```

**Downside**
When using `useDataVolume`, `Cosmopilot` will download the genesis file once for each node. This may lead to redundant downloads in scenarios with multiple nodes.