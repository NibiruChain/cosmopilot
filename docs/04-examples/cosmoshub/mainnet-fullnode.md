# Cosmoshub Mainnet Fullnode

```yaml
apiVersion: cosmopilot.voluzi.com/v1
kind: ChainNodeSet
metadata:
  name: cosmoshub
spec:
  app:
    image: ghcr.io/cosmos/gaia
    version: v25.2.0
    app: gaiad
    sdkVersion: v0.53

  genesis:
    url: https://github.com/cosmos/mainnet/raw/master/genesis/genesis.cosmoshub-4.json.gz
    chainID: cosmoshub-4
    useDataVolume: true

  nodes:
    - name: fullnodes
      instances: 1

      peers:
        # Polkachu Nodes
        - id: ade4d8bc8cbe014af6ebdf3cb7b1e9ad36f412c0
          address: seeds.polkachu.com
          port: 14956
        # Lavender.Five Nodes
        - id: 20e1000e88125698264454a884812746c2eb4807
          address: seeds.lavenderfive.com
          port: 14956

      persistence:
        size: 100Gi
        initTimeout: 1h
        additionalVolumes:
          - name: wasm
            size: 1Gi
            path: /home/app/wasm
        additionalInitCommands:
          - image: ghcr.io/voluzi/node-tools
            command: [ "sh" ]
            args:
              - "-c"
              - |
                SNAPSHOT_URL="https://storage1.quicksync.io/cosmos/mainnet/daily/latest.tar.zst" && \
                echo "Downloading snapshot: $SNAPSHOT_URL" && \
                wget -T 0 -qO- "$SNAPSHOT_URL" | zstd -d | tar -C /home/app -xvf -

      config:
        override:
          app.toml:
            minimum-gas-prices: 0.025uatom
            pruning: custom
            pruning-keep-recent: "100"
            pruning-interval: "10"

```
