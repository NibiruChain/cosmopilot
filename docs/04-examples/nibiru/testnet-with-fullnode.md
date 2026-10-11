# Nibiru Testnet With Fullnode

```yaml
apiVersion: cosmopilot.voluzi.com/v1
kind: ChainNodeSet
metadata:
  name: nibiru-testnet
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 2.9.0
    app: nibid
    sdkVersion: v0.47

  validator:
    accountPrefix: nibi
    valPrefix: nibivaloper

    info:
      moniker: cosmopilot

    config:
      override:
        app.toml:
          minimum-gas-prices: 0.025unibi

      sidecars:
        - name: pricefeeder
          image: ghcr.io/nibiruchain/pricefeeder:1.1.1
          env:
            - name: FEEDER_MNEMONIC
              valueFrom:
                secretKeyRef:
                  name: nibiru-testnet-validator-account
                  key: mnemonic
            - name: CHAIN_ID
              value: nibiru-testnet-0
            - name: GRPC_ENDPOINT
              value: localhost:9090
            - name: WEBSOCKET_ENDPOINT
              value: ws://localhost:26657/websocket

    init:
      chainID: nibiru-testnet-0
      assets: ["100000000000000unibi", "1000000000000000000unusd", "10000000000000000uusdt"]
      stakeAmount: 100000000unibi
      unbondingTime: 60s
      votingPeriod: 60s
      additionalInitCommands:
        - command: [ "sh", "-c" ]
          args:
            - >
              nibid genesis add-sudo-root-account \
                $(nibid keys show account -a --home=/home/app --keyring-backend test) \
                --home=/home/app

  nodes:
    - name: fullnodes
      instances: 1

      config:
        override:
          app.toml:
            minimum-gas-prices: 0.025unibi
            pruning: custom
            pruning-keep-recent: "100"
            pruning-interval: "10"

```
