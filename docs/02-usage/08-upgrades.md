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