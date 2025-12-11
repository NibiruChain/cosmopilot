# Nibiru Multi Validator Devnet

```yaml
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNodeSet
metadata:
  name: nibiru-devnet
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 1.5.0
    app: nibid

  validator:
    info:
      moniker: nibiru-0
      website: https://nibiru.fi

    config:
      override:
        app.toml:
          minimum-gas-prices: 0.025unibi
          mempool:
            max-txs: 100000
        config.toml:
          mempool:
            size: 100000
            cache_size: 200000

      sidecars:
        - name: pricefeeder
          image: ghcr.io/nibiruchain/pricefeeder:1.0.3
          env:
            - name: FEEDER_MNEMONIC
              valueFrom:
                secretKeyRef:
                  name: nibiru-devnet-validator-account
                  key: mnemonic
            - name: CHAIN_ID
              value: nibiru-devnet-0
            - name: GRPC_ENDPOINT
              value: localhost:9090
            - name: WEBSOCKET_ENDPOINT
              value: ws://localhost:26657/websocket

    init:
      chainID: nibiru-devnet-0
      assets: ["100000000000000unibi", "1000000000000000000unusd", "10000000000000000uusdt"]
      stakeAmount: 100000000unibi
      unbondingTime: 60s
      votingPeriod: 60s
      chainNodeAccounts:
        - chainNode: nibiru-devnet-validator-1
          assets: ["100000000000000unibi", "1000000000000000000unusd", "10000000000000000uusdt"]
        - chainNode: nibiru-devnet-validator-2
          assets: [ "100000000000000unibi", "1000000000000000000unusd", "10000000000000000uusdt" ]
        - chainNode: nibiru-devnet-validator-3
          assets: [ "100000000000000unibi", "1000000000000000000unusd", "10000000000000000uusdt" ]
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
---
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNode
metadata:
  name: nibiru-devnet-validator-1
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 1.5.0
    app: nibid

  genesis:
    fromNodeRPC:
      hostname: nibiru-devnet-validator.default.svc.cluster.local

  validator:
    info:
      moniker: nibiru-1
      website: https://nibiru.fi

    createValidator:
      stakeAmount: 100000000unibi
      gasPrices: 0.025unibi

  config:
    sidecars:
      - name: pricefeeder
        image: ghcr.io/nibiruchain/pricefeeder:1.0.3
        env:
          - name: FEEDER_MNEMONIC
            valueFrom:
              secretKeyRef:
                name: nibiru-devnet-validator-1-account
                key: mnemonic
          - name: CHAIN_ID
            value: nibiru-devnet-0
          - name: GRPC_ENDPOINT
            value: localhost:9090
          - name: WEBSOCKET_ENDPOINT
            value: ws://localhost:26657/websocket

    override:
      app.toml:
        minimum-gas-prices: 0.025unibi
        mempool:
          max-txs: 100000
      config.toml:
        mempool:
          size: 100000
          cache_size: 200000
---
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNode
metadata:
  name: nibiru-devnet-validator-2
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 1.5.0
    app: nibid

  genesis:
    fromNodeRPC:
      hostname: nibiru-devnet-validator.default.svc.cluster.local

  validator:
    info:
      moniker: nibiru-2
      website: https://nibiru.fi

    createValidator:
      stakeAmount: 100000000unibi
      gasPrices: 0.025unibi

  config:
    sidecars:
      - name: pricefeeder
        image: ghcr.io/nibiruchain/pricefeeder:1.0.3
        env:
          - name: FEEDER_MNEMONIC
            valueFrom:
              secretKeyRef:
                name: nibiru-devnet-validator-2-account
                key: mnemonic
          - name: CHAIN_ID
            value: nibiru-devnet-0
          - name: GRPC_ENDPOINT
            value: localhost:9090
          - name: WEBSOCKET_ENDPOINT
            value: ws://localhost:26657/websocket

    override:
      app.toml:
        minimum-gas-prices: 0.025unibi
        mempool:
          max-txs: 100000
      config.toml:
        mempool:
          size: 100000
          cache_size: 200000
---
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNode
metadata:
  name: nibiru-devnet-validator-3
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 1.5.0
    app: nibid

  genesis:
    fromNodeRPC:
      hostname: nibiru-devnet-validator.default.svc.cluster.local

  validator:
    info:
      moniker: nibiru-3
      website: https://nibiru.fi

    createValidator:
      stakeAmount: 100000000unibi
      gasPrices: 0.025unibi

  config:
    sidecars:
      - name: pricefeeder
        image: ghcr.io/nibiruchain/pricefeeder:1.0.3
        env:
          - name: FEEDER_MNEMONIC
            valueFrom:
              secretKeyRef:
                name: nibiru-devnet-validator-3-account
                key: mnemonic
          - name: CHAIN_ID
            value: nibiru-devnet-0
          - name: GRPC_ENDPOINT
            value: localhost:9090
          - name: WEBSOCKET_ENDPOINT
            value: ws://localhost:26657/websocket

    override:
      app.toml:
        minimum-gas-prices: 0.025unibi
        mempool:
          max-txs: 100000
      config.toml:
        mempool:
          size: 100000
          cache_size: 200000
```
