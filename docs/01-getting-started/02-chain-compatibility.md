# Chain Compatibility

This page describes the CLI commands and features a blockchain must support to be compatible with Cosmopilot.

## Overview

Cosmopilot is designed to work with chains built on the [Cosmos SDK](https://docs.cosmos.network/). While most Cosmos SDK chains are compatible out of the box, this document outlines the exact requirements your chain binary must meet.

## Supported SDK Versions

Cosmopilot adapts its behavior based on the SDK version, as different versions have different CLI command structures and behaviors.

| SDK Version | Status |
|-------------|--------|
| `v0.47` | Default |
| `v0.45` | Supported |
| `v0.50` | Coming soon |

You can specify the SDK version in your `ChainNode` or `ChainNodeSet` manifest:

```yaml
spec:
  app:
    sdkVersion: v0.47  # or v0.45
```

## Required CLI Commands

The following CLI commands must be supported by your chain binary for full Cosmopilot compatibility. Note that some commands differ between SDK versions.

### Node Initialization

```bash
<binary> init <moniker> --chain-id <chain-id> --home <path>
```

This command initializes the chain home directory and generates default configuration files (`config.toml`, `app.toml`, etc.).

### Node Operation

```bash
<binary> start --home <path> [additional-flags]
```

Starts the blockchain node. Must support the following flags:
- `--home` - Specify the home directory path
- `--trace-store` - Path for trace output (FIFO)

For snapshot integrity verification, the following is also required:

```bash
<binary> start --grpc-only --home <path>
```

### Key Management

```bash
<binary> keys add <account-name> --recover --keyring-backend test --home <path>
```

Recovers an account from a mnemonic. The `--keyring-backend test` flag must be supported for automated key management.

### Genesis Commands

These commands are only required when [initializing a new network](/02-usage/10-initializing-new-network).

<details>
<summary><b>SDK v0.47+ (Default)</b></summary>

```bash
# Add account to genesis
<binary> genesis add-genesis-account <address> <coins> --home <path>

# Generate genesis transaction
<binary> genesis gentx <account> <stake-amount> \
  --moniker <moniker> \
  --chain-id <chain-id> \
  --keyring-backend test \
  --yes \
  [--commission-max-change-rate <rate>] \
  [--commission-max-rate <rate>] \
  [--commission-rate <rate>] \
  [--min-self-delegation <amount>] \
  [--details <details>] \
  [--website <website>] \
  [--identity <identity>] \
  --home <path>

# Collect genesis transactions
<binary> genesis collect-gentxs --home <path>
```

</details>

<details>
<summary><b>SDK v0.45 and Earlier</b></summary>

```bash
# Add account to genesis
<binary> add-genesis-account <address> <coins> --home <path>

# Generate genesis transaction
<binary> gentx <account> <stake-amount> \
  --moniker <moniker> \
  --chain-id <chain-id> \
  --keyring-backend test \
  --yes \
  [--commission-max-change-rate <rate>] \
  [--commission-max-rate <rate>] \
  [--commission-rate <rate>] \
  [--min-self-delegation <amount>] \
  [--details <details>] \
  [--website <website>] \
  [--identity <identity>] \
  --home <path>

# Collect genesis transactions
<binary> collect-gentxs --home <path>
```

</details>

### Validator Creation

This command is only required when running a [validator node](/02-usage/09-validator).

```bash
<binary> tx staking create-validator \
  --amount <stake-amount> \
  --moniker <moniker> \
  --chain-id <chain-id> \
  --pubkey <consensus-pubkey> \
  --gas-prices <gas-prices> \
  --from <account> \
  --keyring-backend test \
  --yes \
  [--commission-max-change-rate <rate>] \
  [--commission-max-rate <rate>] \
  [--commission-rate <rate>] \
  [--min-self-delegation <amount>] \
  [--details <details>] \
  [--website <website>] \
  [--identity <identity>] \
  --node <rpc-endpoint> \
  --home <path>
```

## Required API Endpoints

Cosmopilot interacts with your chain through standard gRPC and RPC endpoints. The following must be exposed and functional:

### gRPC Endpoints

| Service | Method | Purpose |
|---------|--------|---------|
| `cosmos.staking.v1beta1.Query` | `Validator` | Query individual validator info |
| `cosmos.staking.v1beta1.Query` | `Validators` | List all validators |
| `cosmos.base.tendermint.v1beta1.Service` | `GetLatestBlock` | Get latest block info |
| `cosmos.base.tendermint.v1beta1.Service` | `GetBlockByHeight` | Get block by height |
| `cosmos.base.tendermint.v1beta1.Service` | `GetSyncing` | Check sync status |
| `cosmos.base.tendermint.v1beta1.Service` | `GetNodeInfo` | Get node information |
| `cosmos.upgrade.v1beta1.Query` | `CurrentPlan` | Get current upgrade plan |

### RPC Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/status` | Node status and sync info |
| `/abci_info` | ABCI application info |

## Configuration Files

The `init` command must generate standard Cosmos SDK configuration files in the `config/` directory:

- `config.toml` - CometBFT/Tendermint configuration
- `app.toml` - Application configuration

Cosmopilot runs the `init` command to generate these files and persists them in a ConfigMap. If your chain generates additional configuration files, they will also be persisted, though Cosmopilot only applies default configurations to the standard files above. Users can customize any configuration using the `.spec.config.override` field.

::: warning Configuration Key Format
Some chains use dashes instead of underscores in configuration keys (e.g., `addr-book-strict` vs `addr_book_strict`). Cosmopilot does **not** auto-detect this. If your chain uses dashes, you must set `.spec.config.dashedConfigToml: true` in your manifest.
:::

## Upgrade Module

For automatic upgrade handling, your chain should implement the standard Cosmos SDK upgrade module (`x/upgrade`). Cosmopilot queries the `CurrentPlan` endpoint to detect pending upgrades and can automatically handle version changes.

## Compatible Chains

The following table lists chains that have been tested with Cosmopilot:

| Chain | Version | SDK Version | Image                              | Join Network |                           New Network                            | Notes                                           |
|-------|---------|-------------|------------------------------------|:------------:|:----------------------------------------------------------------:|-------------------------------------------------|
| [Cosmos Hub](https://cosmos.network/) | v25.2.0 | v0.50       | `ghcr.io/cosmos/gaia:v25.2.0`      | ![Verified](https://img.shields.io/badge/-Verified-brightgreen) | ![Verified](https://img.shields.io/badge/-Verified-brightgreen)  |                                                 |
| [Osmosis](https://osmosis.zone/) | v31.0.0 | v0.50       | `osmolabs/osmosis:31.0.0`          | ![Verified](https://img.shields.io/badge/-Verified-brightgreen) | ![Verified](https://img.shields.io/badge/-Verified-brightgreen)* | Use sdkVersion `v0.45` for old genesis commands |
| [Nibiru](https://nibiru.fi/) | v2.9.0  | v0.47       | `ghcr.io/nibiruchain/nibiru:2.9.0` | ![Verified](https://img.shields.io/badge/-Verified-brightgreen) | ![Verified](https://img.shields.io/badge/-Verified-brightgreen)  | All previous versions work                      |

**Status Legend:**
- ![Verified](https://img.shields.io/badge/-Verified-brightgreen) - Fully tested and working
- ![Partial](https://img.shields.io/badge/-Partial-yellow) - Works with limitations (see notes)
- ![Unsupported](https://img.shields.io/badge/-Unsupported-red) - Known issues, not recommended

**Column Descriptions:**
- **Join Network** - Joining an existing network with a provided genesis file
- **New Network** - Initializing a new network from scratch (requires genesis commands)
- **Notes** - Version-specific information or known limitations

::: tip Adding Your Chain
If you have tested your chain with Cosmopilot and would like to add it to this list, please open a pull request updating this table.
:::

If your chain is built on Cosmos SDK and follows standard conventions, it should work with Cosmopilot without modifications.
