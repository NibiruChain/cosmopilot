# Cosmopilot

[![Test](https://github.com/voluzi/cosmopilot/actions/workflows/test.yaml/badge.svg)](https://github.com/voluzi/cosmopilot/actions/workflows/test.yaml)
[![E2E](https://github.com/voluzi/cosmopilot/actions/workflows/e2e.yaml/badge.svg)](https://github.com/voluzi/cosmopilot/actions/workflows/e2e.yaml)
[![Builds](https://github.com/voluzi/cosmopilot/actions/workflows/release.yaml/badge.svg)](https://github.com/voluzi/cosmopilot/actions/workflows/release.yaml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/voluzi/cosmopilot/blob/main/LICENSE.md)

**Cosmopilot** is a Kubernetes operator designed to simplify the deployment and management of Cosmos-based blockchain nodes.
It automates tasks like node deployment, upgrades, disk-resize, api exposure, backup (with data integrity validation), ensuring a seamless experience for blockchain operators.

---

## Key Features

- **Node Deployment**:
    - Deploy nodes based on CRDs: `ChainNode` (single node) and `ChainNodeSet` (group of nodes).
    - Automatically peers nodes from the same network if they are on the same namespace.
    - Automatically generates consensus keys and account private keys when needed, and keeps them in secrets.

- **Persistent Volume Management**:
    - Creates (or imports) and manages Persistent Volume Claims (PVCs) for node data.
    - Automatically increases PVC size when usage exceeds a configurable threshold.

- **API Endpoints Exposure**:
    - Integrates with NGINX and cert-manager to optionally expose API endpoints publicly (RPC, LCD, gRPC and EVM).
    - Integrates with [cosmoguard](https://github.com/voluzi/cosmoguard) for fine-grained control over API access and caching.

- **Backup and Snapshot Management**:
    - Configurable snapshot frequency and retention policies (with optional node shutdown during snapshot creation).
    - Export snapshots to Google Cloud Storage (GCS) as tarball files.
    - Verifies snapshot integrity.

- **Genesis Management**:
    - Retrieve genesis from a URL, ConfigMap, RPC endpoint, or generate a new one (useful for launching testnets).

- **State-Sync Automation**:
    - Automatically configures state-sync between nodes.
    - Simplifies restoring nodes from state-sync snapshots.

- **TMKMS Integration**:
    - Securely manages private keys using TMKMS.
    - Supports HashiCorp Vault as the key provider.

- **Governance Upgrade Automation**:
    - Monitors for governance upgrades with detailed Docker image info.
    - Automatically upgrades nodes without manual intervention.
    - Manual upgrades are also schedulable.

---

## Documentation

Full documentation, including installation guides, configuration examples, and advanced features, is available at:

📖 [Cosmopilot Documentation](https://cosmopilot.voluzi.com)

---

## License

Cosmopilot is open source and available under the [MIT License](LICENSE.md).

---

## Contact

Have questions or need help? Feel free to open an issue or reach out to us via [dev@voluzi.com](mailto:dev@voluzi.com).

