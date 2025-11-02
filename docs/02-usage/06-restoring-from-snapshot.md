# Restore from Snapshot

This page explains how to restore blockchain node data using `Cosmopilot`, including state-sync, restoring from volume snapshots, and custom snapshot restore methods.

## Using State-Sync

### From Another Node Managed by Cosmopilot

If there is a node configured to perform state-sync snapshots (as explained in the [Node Configurations](04-node-config#enabling-state-sync-snapshots) page), it is enough to enable:

```yaml
stateSyncRestore: true
```

`Cosmopilot` will take care of retrieving updated data such as trust height, trust hash, and `RPC` servers, and apply it to this ChainNode.

### From External Nodes

For external nodes, you can manually provide the necessary details by [overriding TOML configuration files](04-node-config#overriding-toml-config-files).

Example configuration:

```yaml
config:
  override:
    config.toml:
      statesync:
        enable: true
        rpc_servers: https://rpc.nibiru.fi:443,https://rpc.nibiru.fi:443
        trust_height: 17849562
        trust_hash: A8A55B09347E9BCC6A626D25EDEE2BA063812D2AC335B5EDCDB400239AD8CFE0
```

### Specifying State-Sync Resources

The state sync process may cause the node to consume more resources than during its usual operation. To avoid reconfiguring the node’s resource limits, you can define separate resource specifications that will be applied to the pod while the `ChainNode` is in the `StateSyncing` status.

```yaml{9-15}
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "1"
    memory: "2Gi"
  
stateSyncResources:
  requests:
    cpu: "1000m"
    memory: "2Gi"
  limits:
    cpu: "2"
    memory: "4Gi"
```

::: info NOTE
[Vertical pod autoscaling](15-vertical-pod-autoscaling) is automatically disabled for a `ChainNode` has the `StateSyncing` status to prevent it from restarting. Once state synchronization is complete, VPA is re-enabled.
:::

## Restoring from a Volume Snapshot

You can obtain the list of available volume snapshots by running

```bash
$ kubectl get volumesnapshots
```

To restore a node from a previously created volume snapshot, use the following configuration:

```yaml
persistence:
  restoreFromSnapshot:
    name: nibiru-testnet-1-fullnode-20241107112229
```

This will instruct `Cosmopilot` to create a Persistent Volume Claim (PVC) from the specified snapshot and attach it to the node.

## Custom Snapshot Restore

`Cosmopilot` allows you to specify additional commands (containers) to run during the initialization of the data volume. This can be used to, for example, download a tarball and extract it into the data directory.

Example configuration:

```yaml
persistence:
  additionalInitCommands:
  - image: alpine # Optional. Defaults to app image.
    command: ["sh"] # Optional. Defaults to image entrypoint.
    args: ["-c", "wget -qO- https://remote.tarball.here | tar xvf - -C /home/app/data"]
```

::: tip Important
Make sure to set the [initial PVC size](05-persistence-and-backup#default-pvc-size) large enough to store the extracted data.
Make sure to set the [initTimeout](/03-reference/crds/crds.html#persistence) long enough to allow init container have enough time to extract the tarball data.
:::

### Notes
- The application’s home directory is located at `/home/app`.
- The data directory is located at `/home/app/data`.
- A temporary shared volume is available for all initialization containers at `/temp`.
