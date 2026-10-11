# Osmosis Mainnet Fullnode

```yaml
apiVersion: cosmopilot.voluzi.com/v1
kind: ChainNodeSet
metadata:
  name: osmosis
spec:
  app:
    image: osmolabs/osmosis
    version: 31.0.0
    app: osmosisd
    sdkVersion: v0.53
    sdkOptions:
      genesisSubcommand: false

  genesis:
    url: https://github.com/osmosis-labs/networks/raw/main/osmosis-1/genesis.json
    chainID: osmosis-1
    useDataVolume: true

  nodes:
    - name: fullnodes
      instances: 1

      peers:
        # Polkachu Nodes
        - id: ade4d8bc8cbe014af6ebdf3cb7b1e9ad36f412c0
          address: seeds.polkachu.com
          port: 12556

      persistence:
        size: 100Gi
        initTimeout: 1h
        additionalVolumes:
          - name: wasm
            size: 1Gi
            path: /home/app/wasm
          - name: ibc-08-wasm
            size: 1Gi
            path: /home/app/ibc_08-wasm
        additionalInitCommands:
          - image: ghcr.io/voluzi/node-tools
            command: [ "sh" ]
            args:
              - "-c"
              - |
                SNAPSHOT_URL=$(wget -qO- https://snapshots.osmosis.zone/index.html | sed -n 's/.*href="\(https:\/\/hel1\.your-objectstorage\.com\/osmosis\/osmosis-1\/snapshots\/v31\/[^"]*\.tar\.lz4\)".*/\1/p' | tail -1) && \
                echo "Downloading snapshot: $SNAPSHOT_URL" && \
                wget -T 0 -qO- "$SNAPSHOT_URL" | lz4 -d | tar -C /home/app -xvf -

      config:
        runFlags: [ "--reject-config-defaults=true" ]
        override:
          app.toml:
            minimum-gas-prices: 0.025uosmo
```
