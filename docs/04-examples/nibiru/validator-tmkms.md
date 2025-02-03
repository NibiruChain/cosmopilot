# Nibiru Validator with TmKMS

```yaml
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNode
metadata:
  name: nibiru-validator
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 1.5.0
    app: nibid

  validator:
    info:
      moniker: nibiru
      website: https://nibiru.fi
    tmKMS:
      provider:
        vault:
          address: https://vault.default.svc.cluster.local:8200
          key: nibiru-devnet-0-validator-key
          tokenSecret:
            key: token
            name: vault
          certificateSecret:
            key: vault.ca
            name: vault-server-tls

    init:
      chainID: nibiru-devnet-0
      assets: [ "1000000000000000unibi" ]
      stakeAmount: 100000000unibi
      unbondingTime: 86400s
      votingPeriod: 7200s
      additionalInitCommands:
        - command: [ "sh", "-c" ]
          args:
            - >
              nibid genesis add-sudo-root-account \
                $(nibid keys show account -a --home=/home/app --keyring-backend test) \
                --home=/home/app
  config:
    override:
      app.toml:
        minimum-gas-prices: 0.025unibi
```