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

## Configuring Service Monitor

Enable or disable service monitoring with Prometheus by configuring the following:

```yaml
config:
  serviceMonitor:
    enable: true
    selector: main # optional. Indicates the prometheus installation that will be using this service monitor.
```

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

`Cosmopilot` creates a single main data `PVC` which will be mounted at `/home/app/data`. However, some applications might need to persist more data outside the `data` diretory. When possible, it is advisable to [Override TOML config files](#overriding-toml-config-files) to store additional data in the volume (in `/home/app/data`). However, this is not always possible. In those cases, additional volumes can be created to store that data as follows:

```yaml
config:
  volumes:
  - name: wasm
    size: 1Gi
    path: /home/app/wasm
    deleteWithNode: true
  - name: ibc-08-wasm
    size: 1Gi
    path: /home/app/ibc_08-wasm
    deleteWithNode: true
```