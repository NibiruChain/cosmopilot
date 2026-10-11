# Node Configurations

This page provides a comprehensive guide to configuring `ChainNode` (or `ChainNodeSet` group) resources in `Cosmopilot`, covering various options such as resource management, affinity, node configurations, and more. Some of below configuration fields are just reflected in corresponding `TOML` config files, and were created to simplify configuration, but its always possible to [override TOML config files](#overriding-toml-config-files) as explained below for more advanced configurations.

## State-Sync Snapshots

```yaml
config:
  stateSync:
    snapshotInterval: 250
    snapshotKeepRecent: 5 # optional. Defaults to 2.
```

## Overriding TOML Config Files

When a `ChainNode` is initialized for the first time, `Cosmopilot` generates all necessary configuration files (stored in the config directory) and saves them in a `ConfigMap` with the same name as the `ChainNode`. Among these files, `Cosmopilot` modifies only `app.toml` and `config.toml` to apply most settings. However, you can override these configurations — or settings in additional files — using the `.spec.config.override` field.

Although these configuration files are typically in `TOML` format, they should be defined in `YAML` format when using the override field. `Cosmopilot` will automatically convert the `YAML` into valid `TOML` syntax before applying the changes.

Only the specific values provided in the `override` field are changed. All other settings remain as their default values, as defined by the application. If you need to restore a configuration to its default state, simply remove it from the override field.

```yaml
config:
  override:
    app.toml:
      app-db-backend: pebbledb
      iavl-lazy-loading: true
      min-retain-blocks: 500000
      minimum-gas-prices: 0.025unibi
      pruning: custom
      pruning-interval: "10"
      pruning-keep-recent: "100"
    config.toml:
      db_backend: pebbledb
```

## Setting Pod Resources

Configure resource requests and limits for the main application container using the `resources` field:

```yaml
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "1"
    memory: "2Gi"
```

## Setting Image Pull Secrets

To use private container images, you can specify image pull secrets:

```yaml
config:
  imagePullSecrets:
  - name: my-private-registry-secret
```

## Node Selector and Affinity

You can control where the pod runs using node selectors and affinity rules:

### Node Selector
```yaml
nodeSelector:
  disktype: ssd
```

### Affinity

```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchLabels:
        chain-id: nibiru-testnet-1
        group: fullnode
        nodeset: nibiru-testnet-1
      topologyKey: topology.gke.io/zone
```

## Configuring Block Threshold

`Cosmopilot` provides a special feature to monitor block timestamps instead of relying solely on the `catching_up` field. This ensures more reliable health checks. By default, the block threshold is `15s`. To configure it:

```yaml
config:
  blockThreshold: 30s
```

## Seed Mode

To enable seed mode, configure the following:

```yaml
config:
  seedMode: true
```

## Additional ENV Variables

You can pass additional environment variables to the main application container:

```yaml
env:
  - name: CUSTOM_VAR_1
    value: custom_value_1
  - name: CUSTOM_VAR_2
    value: custom_value_2
```

## Pod Annotations

You can add custom annotations to the node's pod:

```yaml
config:
  podAnnotations:
    custom-annotation-key: custom-annotation-value
    another-key: another-value
```

These annotations will be merged with system-managed annotations. Note that system annotations (like `cosmopilot.voluzi.com/config-hash`) take precedence and cannot be overridden.

## Configuring Node Startup Time

The startup time corresponds to the startup probe timeout. It defaults to `1h`. If the node does not get helthy within this period it will be restarted. In some cases, like when starting a node with huge data, this might not be enough. You can adjust adjust it, using the following:

```yaml
config:
  startupTime: 3h
```

## `node-utils` Resources

You can configure resource requests and limits for the `node-utils` container:

```yaml
config:
  nodeUtilsResources:
    requests:
      cpu: "300m"
      memory: "100Mi"
    limits:
      cpu: "300m"
      memory: "100Mi"
```

The example above actually represents the defaults values.

## Persisting Address Book File

By default, the address book file is not persisted accross restarts, and is rebuilt on every new start. To persist the node's address book file, enable the following option:

```yaml
config:
  persistAddressBook: true
```

## Enable EVM

If the blockchain network supports EVM, enable it with the following configuration:

```yaml
evmEnabled: true
```

This will ensure that `EVM` `RPC` ports will be added to the node's service and will be available when [exposing the endpoints](07-exposing-endpoints).

## Startup Flags

In some cases you might need to append additional startup flags to the main application. For example in [osmosis](https://osmosis.zone/) nodes, the startup command will override some settings on both `config.toml` and `app.toml`, which will not work with `Cosmopilot` as it does manage those files. So an extra flag needs to be added to the main application, using the following:

```yaml
config:
  runFlags: ["--reject-config-defaults=true"]
```

## Additional Volumes

`Cosmopilot` creates a single main data `PVC` which will be mounted at `/home/app/data`. However, some applications might need to persist more data outside the `data` directory. When possible, it is advisable to [Override TOML config files](#overriding-toml-config-files) to store additional data in the volume (in `/home/app/data`). However, this is not always possible.

For configuring additional volumes, see [Persistence and Backups - Additional Volumes](/02-usage/05-persistence-and-backup#additional-volumes).

## Sidecar containers

In some networks, additional tools might need to run together with the node and have access to its data volume. On an environment where you have the possibility of having `ReadWriteMany` volumes this is not a problem, but on most cloud providers that is mostly not available, or extremely expensive. For this reason, `Cosmopilot` provides you with a way of appending additional containers to the same `Pod` as the node.

Please refer to [SidecarSpec](/03-reference/crds/crds#sidecarspec) for full specification on all available fields.

## Halt Height

In certain scenarios, you may need to stop a node at a specific block height. Cosmos-based nodes provide a `halt-height` configuration field in the `app.toml` file for this purpose. However, directly setting this field through [TOML config overrides](#overriding-toml-config-files) may not work as expected, as `Cosmopilot` will continuously attempt to restart the pod.

To properly handle this scenario, `Cosmopilot` offers its own `.spec.config.haltHeight` field, allowing you to set the desired halt height like this:

```yaml
config:
  haltHeight: 12345
```

When configured this way, `Cosmopilot` will:

- Automatically set the `halt-height` in the `app.toml` file.
- Prevent the pod from restarting after the node stops at the specified height.

This ensures a smooth and controlled node shutdown without unintended restarts.

## Security Context Overrides

By default, Cosmopilot applies a restricted security context to all containers following Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/):

- Runs as non-root user (UID 1000)
- Drops all Linux capabilities
- Disables privilege escalation
- Sets filesystem group to GID 1000

If your application requires different security settings (e.g., running as root or with specific capabilities), you can override these defaults.

### Container Security Context

Override security settings for the main application container:

```yaml
config:
  securityContext:
    runAsUser: 0
    runAsNonRoot: false
    allowPrivilegeEscalation: true
```

### Pod Security Context

Override pod-level security settings (affects all containers including sidecars):

```yaml
config:
  podSecurityContext:
    runAsUser: 0
    runAsGroup: 0
    fsGroup: 0
```

### Sidecar Security Context

Each sidecar container uses the restricted security context by default. To override it for a specific sidecar, use the `securityContext` field in the sidecar spec:

```yaml
config:
  sidecars:
    - name: my-privileged-sidecar
      image: some-image:latest
      securityContext:
        runAsUser: 0
        runAsNonRoot: false
```

::: warning
Only override security contexts if absolutely necessary. Running containers as root or with elevated privileges increases security risks. Ensure you understand the implications before making changes.
:::