# Upgrades

This page explains how `Cosmopilot` handles upgrades for `ChainNode` and `ChainNodeSet` resources, including both **Governance** and **Manual** upgrades.

## Initial Version

The `.spec.app.version` field specifies the initial version of the application for a `ChainNode` or all nodes in a `ChainNodeSet`. If no upgrades are configured, changing this field will cause the node(s) to restart with the new version.

However, once upgrades are configured, this field is ignored for the remainder of the node(s)' lifetime. At this point, all version changes are managed through the upgrade process.

## Governance Upgrades

By default, `Cosmopilot` monitors the blockchain for on-chain governance upgrades. This feature can be disabled by setting `.spec.app.checkGovUpgrades` to `false`. 

Cosmos-SDK based chains often use governance proposals to manage upgrades, which include all necessary information, such as:
- **Upgrade Height**: The block height at which the upgrade should occur.
- **Binaries**: Links to the binaries for the new version.

For full automation, ensure that the governance proposal includes the **container image** (with the proper tag) under the key `docker`. When the image is provided, `Cosmopilot` performs the upgrade automatically without requiring manual intervention.

### Governance Upgrade Workflow
1. When an upgrade proposal passes, `Cosmopilot` adds the upgrade to `.status.upgrades` as `scheduled`.
2. If the container image is not included in the proposal, the upgrade is marked as `missing image`. In this case, you must manually add the upgrade to `.spec.app.upgrades` (see [Manual Upgrades](#manual-upgrades)).

## Manual Upgrades

Manual upgrades allow you to define upgrades directly in `.spec.app.upgrades`. These upgrades result in a straightforward binary swap, and `Cosmopilot` does not wait for the node to panic and halt, as is typical with governance upgrades.

### Adding a Manual Upgrade
Example configuration:
```yaml
app:
  upgrades:
  - height: 3000
    image: yourimage:yourtag
```

### Handling Governance Upgrades Without Images

If a governance upgrade does not include the required container image, you can manually add the upgrade and ensure it aligns with the governance proposal. In this case, set the `forceOnChain` field to `true`. This instructs `Cosmopilot` to treat the manual entry as part of the governance process.

Example configuration for missing image:
```yaml
app:
  upgrades:
  - height: 3000
    image: yourimage:yourtag
    forceOnChain: true # Optional. Use only for governance upgrades.
```

::: tip Summary of Key Points
- `.spec.app.version` controls the initial version, but it is ignored once upgrades are configured.
- Governance upgrades are automatic if the proposal includes the necessary container image under the `docker` key.
- Manual upgrades provide a flexible way to apply updates directly through `.spec.app.upgrades`.
- Use the `forceOnChain` field to handle governance upgrades that lack required images.
:::
## Compatible patches

A compatible patch replaces the binary without an on-chain upgrade. Configure
`compatiblePatch` on a ChainNode, a ChainNodeSet node group, or its validator.
For example, on a node group:

```yaml
nodes:
- name: snapshot
  compatiblePatch:
    version: "2.21.2"
    upgradeHeight: 12345678 # Replace with this chain's current upgrade height.
```

The version is an image tag in the existing application image repository.
`upgradeHeight` identifies the upgrade period to patch, not a new height at
which to schedule an upgrade. Use the height of the currently active upgrade
from the node's status. Use `0` for the initial genesis version.

Cosmopilot applies the patch only when the selected upgrade has that exact
height. A scheduled future governance upgrade leaves the patch active. When
the next upgrade becomes active, Cosmopilot uses its image and continues to
use it after restarts, even while the old patch setting remains in IaC.
State-sync restores use the latest completed or skipped upgrade history.

You must verify that the patch is compatible with the chain's state machine
and consensus. Cosmopilot does not compare version numbers or establish binary
compatibility. Changing this field can restart affected nodes, and removing an
active patch returns them to the image recorded for that upgrade period.
Clearing a group patch also clears it from that group's individual nodes.

### Migrating a permanent override

`overrideVersion` retains its existing precedence and behavior. Adding
`compatiblePatch` does not remove or bypass it. To migrate existing pins:

1. Install the updated CRDs and operator before setting the new field.
2. Set the compatible patch against the verified active upgrade height, keeping
   its version equal to the existing override version.
3. Remove the group override, then clear each individual node override through
   a controlled deployment, checking node health between changes. Group removal
   alone still preserves existing individual overrides.
4. Confirm that the selected image remains the patch image and that a later
   active upgrade supersedes it without clearing the patch setting.

Encode this one-time migration in the deployment process. It is not a cleanup
step required for subsequent governance upgrades.
