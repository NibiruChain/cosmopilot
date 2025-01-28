# Validator

This page explains how to configure a `ChainNode` or `ChainNodeSet` to run a validator. `Cosmopilot` provides multiple options to suit different use cases.

## Existing Consensus Key

If a consensus key is already defined in the genesis file as a validator, you can configure the node as follows:

1. Create a Kubernetes secret containing the private key.
2. Specify the secret name in `.spec.validator.privateKeySecret`.

Example configuration:
```yaml
validator:
  privateKeySecret: my-validator-key
```

## Manual Create-Validator

For nodes that are not validators by default, you can:
1. Configure `.spec.validator.info.moniker` with the validator's moniker.
2. Manually submit a `create-validator` transaction to the blockchain to make the node a validator.

Example configuration:
```yaml
validator:
  info:
    moniker: my-validator
```

## Automated Create-Validator

`Cosmopilot` can also submit a `create-validator` transaction for you. For that you need to:
1. Either create a Kubernetes secret containing the mnemonic of an account that has funds (the secret name must follow the pattern `<chainnode-name>-account`), or wait for `Cosmopilot` to create an account and you can send funds to it later.
2. Configure `.spec.validator.info` and `.spec.validator.createValidator`.

Example configuration:
```yaml
validator:
  info:
    moniker: my-validator
    website: https://validator.example.com
    details: "A reliable validator"
    identity: "ABCD1234"

  createValidator:
    stakeAmount: "1000000unibi"
    commissionRate: "0.1"
    commissionMaxRate: "0.2"
    commissionMaxChangeRate: "0.01"
    gasPrices: "0.025unibi"
    minSelfDelegation: "1"
```

## Initializing a New Network

Please refer to [Initializing a New Network](10-initializing-new-network) page for information about setting up new networks.