# Nibiru distribution

This repository tracks upstream Cosmopilot. The v2.2.1 baseline is upstream
commit `fb46820`, including its original history. Keep the Go module path and
`cosmopilot.voluzi.com/v1` Kubernetes API so upstream patches remain portable
and existing mainnet resources continue to work.

The operator image publishes to `ghcr.io/nibiruchain/cosmopilot`. The Helm
chart publishes to `oci://ghcr.io/nibiruchain/helm/cosmopilot`. Companion images
retain the upstream versions declared in the chart; they have their own
release cycles and are not part of the operator patch.

## Releases

Use distinct Nibiru release tags, such as `v2.2.1-nibiru.1` and
`chart/v2.2.1-nibiru.1`, on the same reviewed commit. The first tag builds the
AMD64 and ARM64 operator images; the second packages the chart with the
matching image version. Wait for the image publication to succeed before
publishing the chart. Verify both artifacts before merging the IaC switch.

Historical upstream tags identify upstream commits. Do not move those tags
to Nibiru patch commits. Importing release tags triggers publishing workflows,
so import them deliberately after reviewing the org publishing configuration.

## Mainnet adoption

The IaC module selects the Nibiru chart repository, Nibiru CRD source, and
Nibiru operator image. Retain the existing CRD names and Kubernetes API group.
The first distribution change should preserve the v2.2.1 operator behavior.
The snapshot `overrideVersion` remains until a separate patch implements a
scoped compatible release and a controlled migration of existing overrides.

Keep functional changes in separate PRs from upstream synchronization and
publishing changes. Submit portable fixes upstream, then remove local patches
once a later upstream release includes them.
