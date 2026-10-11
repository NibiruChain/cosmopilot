# Nibiru Mainnet Fullnode

```yaml
apiVersion: cosmopilot.voluzi.com/v1
kind: ChainNodeSet
metadata:
  name: nibiru
spec:
  app:
    image: ghcr.io/nibiruchain/nibiru
    version: 2.9.0
    app: nibid
    sdkVersion: v0.47

  genesis:
    url: https://raw.githubusercontent.com/NibiruChain/Networks/refs/heads/main/Mainnet/cataclysm-1/genesis.json

  nodes:
    - name: fullnodes
      instances: 1

      peers:
        # Nibiru
        - id: 418e1b5d5872e9d6acd5341101aa5ae298a1a9a7
          address: 35.241.151.181
          port: 26656

      persistence:
        size: 100Gi
        initTimeout: 1h
        additionalInitCommands:
          - image: ghcr.io/voluzi/node-tools
            command: [ "sh" ]
            args:
              - "-c"
              - |
                SNAPSHOT_URL=$(wget -qO- https://networks.nibiru.fi/cataclysm-1/snapshots | grep -o 'https://[^"]*pruned-pebbledb\.tar\.gz' | tail -1)
                echo "Downloading snapshot: $SNAPSHOT_URL" && \
                wget -T 0 -qO- "$SNAPSHOT_URL" | tar -xzvf - -C /home/app/data

      config:
        override:
          app.toml:
            minimum-gas-prices: 0.025unibi
```
