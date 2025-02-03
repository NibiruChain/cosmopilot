# Osmosis Devnet Validator + Fullnode

```yaml
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNodeSet
metadata:
  name: osmosis-devnet
spec:
  app:
    image: osmolabs/osmosis
    version: 25.2.0
    app: osmosisd
    sdkVersion: v0.45 # Old version of genesis commands

  validator:
    info:
      moniker: nibiru-0
      website: https://nibiru.fi

    config:
      runFlags: ["--reject-config-defaults=true"]
      volumes:
        - name: wasm
          size: 1Gi
          path: /home/app/wasm
          deleteWithNode: true
        - name: ibc-08-wasm
          size: 1Gi
          path: /home/app/ibc_08-wasm
          deleteWithNode: true
      override:
        app.toml:
          minimum-gas-prices: 0.025uosmo
          mempool:
            max-txs: 100000
        config.toml:
          mempool:
            size: 100000
            cache_size: 200000

    init:
      chainID: osmosis-devnet-0
      accountPrefix: osmo
      valPrefix: osmovaloper
      assets: [ "100000000000000000000uosmo" ]
      stakeAmount: 100000000uosmo
      unbondingTime: 60s
      votingPeriod: 60s
      additionalInitCommands:
        # Use uOSMO as default denom
        - image: busybox
          command: [ "sh", "-c" ]
          args:
            - >
              sed -i 's/stake/uosmo/g' /home/app/config/genesis.json;
              sed -i 's/uosmors/stakers/g' /home/app/config/genesis.json;
              sed -i 's/uosmod/staked/g' /home/app/config/genesis.json;

  nodes:
    - name: fullnodes
      instances: 1

      config:
        runFlags: ["--reject-config-defaults=true"]
        volumes:
          - name: wasm
            size: 1Gi
            path: /home/app/wasm
            deleteWithNode: true
          - name: ibc-08-wasm
            size: 1Gi
            path: /home/app/ibc_08-wasm
            deleteWithNode: true
        override:
          app.toml:
            minimum-gas-prices: 0.025uosmo
            pruning: custom
            pruning-keep-recent: "100"
            pruning-interval: "10"
```