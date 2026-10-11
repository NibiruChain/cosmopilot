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

Historical upstream tags identify upstream commits. The v2.1 and v2.2 release
and chart tags are preserved in this org under `upstream/`, such as
`upstream/v2.2.1` and `upstream/chart/v2.2.1`. These names do not trigger release
publishing. Do not move those tags to Nibiru patch commits. Unprefixed release
tags trigger publishing workflows and are reserved for reviewed org releases.

## Mainnet adoption

The IaC module selects the Nibiru chart repository, Nibiru CRD source, and
Nibiru operator image. Retain the existing CRD names and Kubernetes API group.
The first distribution change should preserve the v2.2.1 operator behavior.
The snapshot `overrideVersion` remains until a separate patch implements a
scoped compatible release and a controlled migration of existing overrides.

Keep functional changes in separate PRs from upstream synchronization and
publishing changes. Submit portable fixes upstream, then remove local patches
once a later upstream release includes them.
