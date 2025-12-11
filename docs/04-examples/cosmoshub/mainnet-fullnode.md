# Cosmoshub Mainnet Fullnode

```yaml
apiVersion: apps.k8s.nibiru.org/v1
kind: ChainNodeSet
metadata:
  name: cosmos
spec:
  app:
    image: ghcr.io/cosmos/gaia
    version: v25.2.0
    app: gaiad

  genesis:
    url: https://github.com/osmosis-labs/networks/raw/main/osmosis-1/genesis.json
    chainID: cosmoshub-4
    useDataVolume: true

  nodes:
    - name: fullnodes
      instances: 1

      persistence:
        size: 100Gi
        initTimeout: 30m
        additionalInitCommands:
          - image: ghcr.io/nibiruchain/node-tools
            command: [ "sh" ]
            args:
              - "-c"
              - |
                SNAPSHOT_URL="https://storage1.quicksync.io/cosmos/mainnet/daily/latest.tar.zst" && \
                echo "Downloading snapshot: $SNAPSHOT_URL" && \
                wget -qO- "$SNAPSHOT_URL" | zstd -d | tar -C /home/app -xvf -

      config:
        volumes:
          - name: wasm
            size: 1Gi
            path: /home/app/wasm
            deleteWithNode: true
        override:
          app.toml:
            minimum-gas-prices: 0.025uatom
            pruning: custom
            pruning-keep-recent: "100"
            pruning-interval: "10"

```
