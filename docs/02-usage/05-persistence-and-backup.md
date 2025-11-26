# Persistence and Backups

This section explains how `Cosmopilot` handles persistence and backups for blockchain node data, ensuring data integrity and seamless recovery.

## Configuration Overview

Persistence settings can be configured in the following locations:
- For `ChainNode`: Configure under `.spec.persistence`.
- For `ChainNodeSet`: 
  - Configure persistence for specific groups under `.spec.nodes[].persistence`.
  - For the validator node (if present), configure under `.spec.validator.persistence`.

For example:
- **ChainNode**:
```yaml
  spec:
    persistence: {...}
```
- **ChainNodeSet**:
```yaml
spec:
  nodes:
    - name: fullnodes
      persistence: {...}
  validator:
    persistence: {...}
```

## PVC Size and Storage Class

Each node deployed by `Cosmopilot` requires a `PVC` for storing its data.

### Default PVC Size
By default, the `PVC` size is set to `50Gi`, with [auto-resize](#auto-resize) feature enabled. This can be changed in [persistence](/03-reference/crds/crds.html#persistence) spec:

```yaml
persistence:
  size: 100Gi
```

### Configuring Storage Class
By default, the `PVC` uses the default storage class configured in your Kubernetes cluster. You can specify a custom storage class in the node configuration:

```yaml
persistence:
  storageClass: custom-storage-class
```
 
## Auto-Resize

`Cosmopilot` includes an auto-resize feature that monitors PVC usage and increases the volume size when a configured threshold is exceeded. This feature is enabled by default.

You can configure auto-resize settings in the [persistence](/03-reference/crds/crds.html#persistence) configuration. Key options include:
- The **threshold** at which a resize event occurs.
- The **increment** added to the PVC size during each resize.
- The **maximum size** the PVC can reach.

### Example Configuration
```yaml
persistence:
  autoResize: true
  autoResizeThreshold: 70 # Default is 80%
  autoResizeIncrement: 20Gi # Default is 50Gi
  autoResizeMaxSize: 5Ti # Defaults to 2Ti
```


## Snapshots

`Cosmopilot` allows configuring periodic snapshots of volumes in order to back up node data. It is also possible to configure cleanup of old snapshots using either time-based retention or count-based retention (by default snapshots are kept forever).

::: warning NOTE
When configured on `ChainNodeSet`group, `snapshots` are only taken on one of the nodes of the group (the first node).
:::

### Example configuration

```yaml
persistence:
  snapshot:
    frequency: 24h # Take a snapshot every 24 hours
    retention: 72h # Retain snapshots of the last 3 days
```

### Retention Options

You can configure snapshot retention in two ways:

#### Time-based retention (`retention`)
Delete snapshots after a specified duration:

```yaml
persistence:
  snapshot:
    frequency: 24h
    retention: 72h # Delete snapshots older than 3 days
```

#### Count-based retention (`retain`)
Keep only the N most recent snapshots:

```yaml
persistence:
  snapshot:
    frequency: 24h
    retain: 5 # Keep only the 5 most recent snapshots
```

::: warning NOTE
The `retention` and `retain` fields are mutually exclusive. You can only use one of them at a time.
:::

### Snapshot Class

By default, `Cosmopilot` uses the default `VolumeSnapshotClass` configured in your cluster. You can also set a custom one:

```yaml{5}
persistence:
  snapshot:
    frequency: 24h # Take a snapshot every 24 hours
    retention: 72h # Retain snapshots of the last 3 days
    snapshotClass: my-custom-snapshot-class
```

### Stopping the Node for Snapshot

By default, `Cosmopilot` does not stop the node while taking a snapshot. This approach leverages the crash-consistent snapshot capabilities provided by most major cloud providers (e.g., GKE, AWS EKS, Azure AKS), which are generally reliable for blockchain nodes.

However, in environments where crash consistency cannot be guaranteed or when additional safety is required, you can configure `Cosmopilot` to stop the node during the snapshot process. This ensures the node is quiesced and no writes are occurring while the snapshot is taken.

You can enable this behavior with:

```yaml{3}
persistence:
  snapshot:
    stopNode: true
```

::: info NOTE
If you prefer to avoid downtime you can consider enabling [integrity verification](#integrity-checks) instead.
:::

#### When to Use
- **Unverified Providers**: If your cluster uses a storage backend without clear guarantees of crash consistency.
- **Heavy Write Workloads**: For nodes experiencing intensive writes, where the risk of snapshot inconsistency is higher.
- **Critical Data**: For validators or nodes with high availability requirements where snapshot integrity is paramount.

### Disable Snapshots While Node Is Syncing

In some cases, it may be beneficial to disable snapshots while the node is syncing blocks (still catching up to the network). This avoids creating snapshots of an outdated or incomplete state.

#### Configuration

To disable snapshots during sync, set the `.persistence.snapshot.disableWhileSyncing` option to `true`:

```yaml
persistence:
  snapshot:
    disableWhileSyncing: true
```

## **Integrity Checks**

`Cosmopilot` offers the ability to verify the integrity of snapshots by attempting to start a separate node from the snapshot data. This ensures that the snapshot is valid and can be successfully used to restore a node. Integrity checks add an additional layer of safety, especially when snapshots are taken without stopping the node.

#### Configuration

To enable integrity checks, set the `.persistence.snapshot.verify` option to `true`:

```yaml
persistence:
  snapshot:
    verify: true
```

#### How It Works
1. After a snapshot is created, `Cosmopilot` will:
   - Provision a temporary `PVC` from the snapshot.
   - Launch a test node using the `PVC`.
   - Monitor the test node to confirm it starts and runs successfully.
2. If the test node starts and syncs correctly, the snapshot is marked as valid.
3. If the integrity check fails, the snapshot is deleted, and `Cosmopilot` attempts to take a new snapshot.

## Exporting Tarball

`Cosmopilot` provides an option to export data from a volume snapshot as a tarball file and upload it to external storage. Currently, this feature supports uploading to Google Cloud Storage (GCS) buckets. 

To enable this functionality, configure the `.persistence.snapshot.exportTarball` field.

### Configuration

Here’s an example configuration for exporting tarballs:

```yaml
persistence:
  snapshot:
    exportTarball:
      suffix: "-archive" # Optional. Default: empty.
      deleteOnExpire: false # Optional. Default: false.
      gcs:
        bucket: my-backup-bucket
        credentialsSecret: gcs-credentials
```

### General Fields
- **`suffix`**: 
  - Optional. Adds a suffix to the tarball name. 
  - By default, the tarball name is `<chain-id>-<timestamp><suffix>`. If no suffix is provided, it will be empty.
  - **Use Cases**:
    - Add context about the tarball (e.g., `-archive` or `-pruned` to indicate the data type).
    - Specify database backend (e.g., `-goleveldb` or `-pebbledb`).
- **`deleteOnExpire`**:
  - Optional. Defaults to `false`.
  - Indicates whether the tarball should also be deleted when the associated volume snapshot is removed due to expiration (`.persistence.snapshot.retention`).

### Provider-Specific Fields
Currently, `Cosmopilot` supports exporting tarballs to **Google Cloud Storage (`GCS`)**. The following fields are required for `GCS` configuration:
- **`bucket`**:
  - The name of the `GCS` bucket where the tarball will be uploaded.
- **`credentialsSecret`**:
  - A Kubernetes secret containing the JSON credentials with permissions to upload and delete objects in the specified bucket.


## Restoring Data from Snapshot

For detailed instructions on restoring data from a snapshot, refer to the [Restore from Snapshot](06-restoring-from-snapshot) page.
