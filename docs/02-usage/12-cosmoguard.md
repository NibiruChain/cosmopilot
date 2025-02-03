# Using CosmoGuard

[CosmoGuard](https://github.com/NibiruChain/cosmoguard) is a lightweight firewall designed specifically for protecting Cosmos nodes. With [CosmoGuard](https://github.com/NibiruChain/cosmoguard), you can control access at the API endpoint level, cache responses for performance, and limit WebSocket connections for better resource management.

`Cosmopilot` integrates seamlessly with [CosmoGuard](https://github.com/NibiruChain/cosmoguard) by allowing easy configuration of [CosmoGuard](https://github.com/NibiruChain/cosmoguard) rules through Kubernetes `ConfigMaps`.

## Why Use CosmoGuard?

- **Fine-Grained API Access Control:** Manage access on a per-endpoint level.
- **Performance Optimization with Caching:** Cache frequently accessed responses using either in-memory or Redis backends.
- **WebSocket Connection Management:** Control the number of WebSocket connections to nodes, preventing resource overload on nodes.
- **Hot-Reloading**: Apply configuration changes without needing to restart CosmoGuard.

## Setting Up CosmoGuard

### Step 1: Create the CosmoGuard Configuration

Create a configuration file following CosmoGuard's [rules structure](https://github.com/NibiruChain/cosmoguard/blob/main/CONFIG.md). An example configuration allowing only the `/status` endpoint on `RPC` and caching its response:

```yaml
cache:
  ttl: 10s

rpc:
  rules:
    - action: allow
      paths: 
        - /status
      methods: 
        - GET
      cache:
        enable: true
```

::: warning IMPORTANT
When using [CosmoGuard](https://github.com/NibiruChain/cosmoguard) with `Cosmopilot`, avoid manually configuring `Global Settings` and `Node Settings` in the `ConfigMap`. `Cosmopilot` will automatically handle these settings to ensure proper integration and functionality.
:::

### Step 2: Create a ConfigMap in Kubernetes

Use the configuration from Step 1 to create a Kubernetes ConfigMap:

```bash
kubectl create configmap cosmoguard-config --from-file=cosmoguard.yaml=/path/to/your/cosmoguard.yaml -n <namespace>
```

This ConfigMap will be referenced by the CosmoGuard integration in Cosmopilot.

### Step 3: Enable CosmoGuard in Cosmopilot

To enable CosmoGuard for a `ChainNode` or a node group within a `ChainNodeSet`, specify the following configuration in your manifest:

```yaml
config:
  cosmoGuard:
    enable: true
    config:
      name: cosmoguard-config  # Name of the ConfigMap created in Step 2.
      key: cosmoguard.yaml     # Key within the ConfigMap containing the configuration.
    restartPodOnFailure: true  # Optional: Restart the pod if CosmoGuard fails.
    resources:                 # Optional: Resource allocation for CosmoGuard. This example shows the default resources.
      requests:
        cpu: 200m
        memory: 250Mi
      limits:
        cpu: 200m
        memory: 250Mi
```

## Customizing Rules

Refer to the [CosmoGuard repo](https://github.com/NibiruChain/cosmoguard) for detailed information on creating custom rules. Here are a few tips:

- **Use Wildcards:**
  - `*` matches any single component in a path.
  - `**` matches multiple components.

- **Prioritize Rules:**
  Assign lower numbers to higher-priority rules using the `priority` field.

- **Enable Caching:**
  Enable caching selectively for frequently requested endpoints to reduce load on nodes.

Example rule allowing `/block` endpoint with caching:

```yaml
rpc:
  rules:
    - action: allow
      paths: 
        - /block/**
      methods: [GET]
      cache:
        enable: true
        ttl: 15s
```