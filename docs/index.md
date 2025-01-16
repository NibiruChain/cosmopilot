---
layout: home

hero:
  name: "Cosmopilot"
  text: "Documentation"
  tagline: Kubernetes operator designed to simplify the deployment and management of Cosmos-based blockchain nodes. It automates tasks like node deployment, upgrades, disk-resize, API exposure, backup (with data integrity validation), ensuring a seamless experience for Cosmos node operators.
  image:
    src: /logo.png
    alt: Cosmopilot
  actions:
    - theme: brand
      text: Getting Started
      link: /01-getting-started/01-prerequisites
    - theme: alt
      text: Usage
      link: /02-usage/01-deploy-node

features:
  - title: Upgrades
    details: Monitors the chain for governance upgrades and automatically upgrades the nodes without manual intervention.
    icon:
      src: /features/upgrades.png
      width: 75
      height: 75
  - title: Peering
    details: Automatically peers nodes from the same network if they are on the same namespace.
    icon:
      src: /features/peering.png
      width: 75
      height: 75
  - title: PVC Resize
    details: Automatically increases PVC size when usage exceeds a configurable threshold.
    icon:
      src: /features/pvc-resize.png
      width: 75
      height: 75
  - title: API Endpoints Exposure
    details: Allows to publicly expose node's API endpoints with fine-grained access control and caching for increased performance.
    icon:
      src: /features/api.png
      width: 75
      height: 75
  - title: Volume Snapshots
    details: Periodically takes volume snapshots based on policies, verifies their integrity and optionally exports them as tarballs to external storage.
    icon:
      src: /features/snapshot.png
      width: 75
      height: 75
  - title: Genesis Download
    details: Retrieve genesis from a URL, ConfigMap, RPC endpoint, or generate a new one (useful for launching testnets).
    icon:
      src: /features/genesis.png
      width: 75
      height: 75
  - title: State-sync
    details: Automatically configures state-sync between nodes, simplifying node recovery when needed.
    icon:
      src: /features/state-sync.png
      width: 75
      height: 75
  - title: TmKMS Integration
    details: Securely manages private keys with TmKMS (with support for HashiCorp Vault as the key provider).
    icon:
      src: /features/tmkms.png
      width: 75
      height: 75
---

