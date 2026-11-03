# Nibiru Cosmopilot Changelog

This file records Nibiru releases and their differences from
[voluzi/cosmopilot](https://github.com/voluzi/cosmopilot). Upstream history and
the MIT license remain in this repository. The Go module path and Kubernetes
API group remain unchanged.

## 2.2.5

### Upstream Baseline

Includes upstream **v2.2.4**, commit
`9b2b43f8d183861c9f28387f6677a9683ecb040d`, and its preceding fixes since
v2.2.1. These include removal of the SDK trace-store dependency, upgrade
detection through ABCI and `upgrade-info.json`, halt/recovery handling, and
scoped data-size sampling. The default helper remains upstream
`ghcr.io/voluzi/node-utils:2.9.3`.

### Nibiru Differences

- Reconcile cached pending governance upgrades against successful
  `CurrentPlan` queries. Cancellation removes the obsolete record;
  replacement removes the former height and refreshes the image at the same
  height. Query errors preserve the previous configuration. Manual, ongoing,
  and terminal upgrades remain intact. See [PR #344](https://github.com/NibiruChain/cosmopilot/pull/344).
- Retain `compatiblePatch`, introduced in [PR #343](https://github.com/NibiruChain/cosmopilot/pull/343).
  An explicitly selected compatible binary applies only within its configured
  upgrade period. A later governance upgrade supersedes it, including after
  pod recreation. This setting does not certify binary compatibility.
- Publish the operator to `ghcr.io/nibiruchain/cosmopilot`. Preserve the
  independent chart/image selection and publication fixes introduced in
  [PR #342](https://github.com/NibiruChain/cosmopilot/pull/342).
- Use `dev@nibiru.org` as the contact for this distribution.

The cancellation regressions now exercise upstream's upgrade monitor rather
than preserving Nibiru's superseded trace-based sticky-flag implementation.
No additional helper-image release is required for this operator patch.

### Deployment

Operator image `ghcr.io/nibiruchain/cosmopilot:2.2.5` can be selected with the
existing upstream Helm chart **2.2.4** using `image` and `imageTag`. Use CRDs
from Nibiru tag `v2.2.5` to retain the `compatiblePatch` fields. Keep the
chart's upstream node-utils 2.9.3 selection. Publishing this release does not
apply Terraform or update running nodes.

## 2.2.1-nibiru.1

Based on upstream v2.2.1, commit
`fb46820bdae656d34e6d4f07db74f2225ba336e4`. Added Nibiru publication and
`compatiblePatch`. Superseded by the 2.2.5 baseline update.
