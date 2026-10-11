# Using TmKMS

`TmKMS` (Tendermint Key Management System) allows you to secure your consensus private key by keeping it off-chain and using an external signing mechanism. `Cosmopilot` integrates with `TmKMS` to ensure your validator’s private key is securely managed.

Currently, `Cosmopilot` supports the **HashiCorp Vault** provider for secure key storage and signing.

::: tip Important
HashiCorp Vault support is not yet officially available on the main TmKMS repository (see [iqlusioninc/tmkms#840](https://github.com/iqlusioninc/tmkms/pull/840)). To address this, `Cosmopilot` relies on a custom fork of `TmKMS` (`v0.14.0`) with HashiCorp support. The docker image for this fork is available at `ghcr.io/voluzi/tmkms:0.14.0-vault`. This configuration has been successfully used in production environments for over a year, but please proceed with caution.
:::

## Prepare Vault and Token

### 1. Enable Transit secrets

Make sure Transit secrets are enabled in your vault cluster:

```bash
$ vault secrets enable transit
```

### 2. Create Vault policy

```bash
$ export KEY=my-consensus-key
$ cat <<EOF | vault policy write $KEY -
path "auth/token/lookup-self" {
  capabilities = ["read"]
}

path "auth/token/renew-self" {
  capabilities = ["update"]
}

path "transit/wrapping_key" {
	capabilities = ["read"]
}

path "transit/keys/$KEY/import" {
  capabilities = ["update"]
}

path "transit/keys/$KEY" {
  capabilities = ["read"]
}

path "transit/sign/$KEY" {
  capabilities = ["update"]
}
EOF
```

### 3. Create Vault token

Finally, to create the token with the above policy:

```bash
$ export KEY=my-consensus-key
$ vault token create \
 -policy=$KEY \
 -no-default-policy  \
 -non-interactive \
 -period=10d
```

Put it in a Kubernetes secret with:

```bash
$ export VAULT_TOKEN=<your-token-here>
$ kubectl create secret generic vault --from-literal=token=$VAULT_TOKEN 
```

## Uploading Key to Vault

### Using TmKMS (Recomended)

#### 1. Install TmKMS

Install TmKMS from [voluzi](https://voluzi.com) fork [github.com/voluzi/tmkms](https://github.com/voluzi/tmkms) (make sure you use tag `v0.14.0-vault`):

```bash
$ git clone --branch v0.14.0-vault https://github.com/voluzi/tmkms
$ cd tmkms
$ cargo build --release --features hashicorp,softsign
```

#### 2. Upload Key from priv_validator_key.json

```bash
$ export VAULT_ADDR='http://0.0.0.0:8200'
$ export VAULT_TOKEN=<your-token-here>
$ export KEY=my-consensus-key
$ export KEY_PATH=~/.nibid/config/priv_validator_key.json
$ ./target/release/tmkms hashicorp upload $KEY --payload-file $KEY_PATH
```

### Using Cosmopilot

::: warning
Do not use this in production.
:::

`Cosmopilot` is also able to upload the consensus key to `Vault`. For that, ensure the `Vault` token used has proper permissions for uploading they key, and add:

```yaml{13}
validator:
  tmKMS:
    provider:
      hashicorp:
        ...
        uploadGenerated: true // [!code focus]
        ...
```

::: tip NOTE
On networks with `.spec.validator.init` configure, `Cosmopilot` assumes its a testnet and sets `uploadGenerated` to `true` by default.
:::

## Basic Configuration

To configure `TmKMS` with a `ChainNode` or `ChainNodeSet`, set up the following in the `tmKMS` section under `.spec.validator`:

```yaml
validator:
  tmKMS:
    provider:
      hashicorp:
        address: https://vault.example.com:8200
        key: my-consensus-key
        tokenSecret:
          name: vault
          key: token
        autoRenewToken: true # Optional. Defaults to false. Use for tokens with expirity (non-root tokens)
```

::: tip NOTE
Unless you are using root token, you should enable `autoRenewToken` to have it renewed by `Cosmopilot` using a sidecar container.
:::

## CA Certificate

If your Vault cluster uses a CA certificate you can also include it in a Kubernetes secret and configure it, or just skip its verification:

```yaml{10-13}
validator:
  tmKMS:
    provider:
      hashicorp:
        address: https://vault.example.com:8200
        key: my-consensus-key
        tokenSecret:
          name: vault
          key: token
        certificateSecret:
          name: vault-ca-cert
          key: tls.crt
        skipCertificateVerify: false # Optional. Defaults to false.
```

## Persist State

By default, `Cosmpilot` does not persist `TmKMS` state. If you need to enable it, use:

```yaml{3}
validator:
  tmKMS:
    persistState: true # Default is false.
```

`Cosmopilot` will create an additional `1Gi` `PVC` to store `priv_validator_state.json`.

## Resource Configuration

You can configure resource requests and limits for the `TmKMS` container to ensure it runs optimally:

```yaml{3-9}
validator:
  tmKMS:
    resources:
      requests:
        cpu: "200m"
        memory: "256Mi"
      limits:
        cpu: "500m"
        memory: "512Mi"
```
