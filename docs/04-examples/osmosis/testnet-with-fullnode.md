# Osmosis Testnet With Fullnode

```yaml
apiVersion: cosmopilot.voluzi.com/v1
kind: ChainNodeSet
metadata:
  name: osmosis-devnet
spec:
  app:
    image: osmolabs/osmosis
    version: 31.0.0
    app: osmosisd
    sdkVersion: v0.53
    sdkOptions:
      genesisSubcommand: false

  validator:
    accountPrefix: osmo
    valPrefix: osmovaloper

    info:
      moniker: cosmopilot

    persistence:
      additionalVolumes:
        - name: wasm
          size: 1Gi
          path: /home/app/wasm
          deleteWithNode: true
        - name: ibc-08-wasm
          size: 1Gi
          path: /home/app/ibc_08-wasm
          deleteWithNode: true

    config:
      runFlags: [ "--reject-config-defaults=true" ]
      override:
        app.toml:
          minimum-gas-prices: 0.025uosmo

    init:
      chainID: osmosis-testnet-0
      assets: [ "100000000000000000000uosmo" ]
      stakeAmount: 100000000uosmo
      unbondingTime: 60s
      votingPeriod: 60s
      expeditedVotingPeriod: 30s
      additionalInitCommands:
        # Use uOSMO as default denom
        - image: busybox
          command: [ "sh", "-c" ]
          args:
            - sed -i 's/"stake"/"uosmo"/g' /home/app/config/genesis.json

  nodes:
    - name: fullnodes
      instances: 1

      persistence:
        additionalVolumes:
          - name: wasm
            size: 1Gi
            path: /home/app/wasm
            deleteWithNode: true
          - name: ibc-08-wasm
            size: 1Gi
            path: /home/app/ibc_08-wasm
            deleteWithNode: true

      config:
        runFlags: [ "--reject-config-defaults=true" ]
        override:
          app.toml:
            minimum-gas-prices: 0.025uosmo

```
