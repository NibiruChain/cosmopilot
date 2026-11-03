# Nibiru distribution

This repository tracks upstream Cosmopilot. Release 2.2.5 uses upstream v2.2.4,
commit `9b2b43f8`, including its original history. Keep the Go module path and
`cosmopilot.voluzi.com/v1` Kubernetes API so upstream patches remain portable
and existing mainnet resources continue to work.

The operator image publishes to `ghcr.io/nibiruchain/cosmopilot`. The Helm
chart publishes to `oci://ghcr.io/nibiruchain/helm/cosmopilot`. Companion images
retain the upstream versions declared in the chart; they have their own
release cycles and are not part of the operator patch.

## Releases

Use the next available patch tag, `v2.2.5`, for this Nibiru release.
The [changelog](../../CHANGELOG.md) records its differences from upstream.
An optional chart tag, such as `chart/v2.2.5`, must reference the same reviewed commit. The first tag builds the
AMD64 and ARM64 operator images; the second packages the chart with the
matching image version. Wait for the image publication to succeed before
publishing a chart that references it. A separate chart release is optional
when IaC uses the existing upstream chart with operator image overrides.

Historical upstream tags identify upstream commits. The v2.1 and v2.2 release
and chart tags are preserved in this org under `upstream/`, such as
`upstream/v2.2.1` and `upstream/chart/v2.2.1`. These names do not trigger release
publishing. Do not move those tags to Nibiru patch commits. Unprefixed release
tags trigger publishing workflows. The unprefixed `v2.2.1` tag also preserves
the original upstream commit and publishes our unchanged baseline image.

## Mainnet adoption

IaC can keep the existing upstream chart and set its `image` value to
`ghcr.io/nibiruchain/cosmopilot` and `imageTag` to the reviewed patch version.
The Terraform module must expose the chart's `imageTag` input before deploying
a different operator version with the same chart. Publishing the image alone
does not change the running operator.

If a patch adds CRD fields, deploy the matching updated CRD definitions before
configuring those fields. An org-owned chart is available when we need to ship
updated deployment configuration. Retain the existing CRD names and Kubernetes
API group.
The `compatiblePatch` setting is available. Existing snapshot `overrideVersion`
pins remain until a controlled migration verifies the active upgrade height
and clears both group and individual overrides.

Keep functional changes in separate PRs from upstream synchronization and
publishing changes. Submit portable fixes upstream, then remove local patches
once a later upstream release includes them.
