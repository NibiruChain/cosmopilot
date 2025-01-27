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

## Restoring from a Volume Snapshot

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

### Notes
- The application’s home directory is located at `/home/app`.
- The data directory is located at `/home/app/data`.
- A temporary shared volume is available for all initialization containers at `/temp`.
