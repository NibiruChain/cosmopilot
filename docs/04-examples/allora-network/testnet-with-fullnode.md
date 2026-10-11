# Allora-network Testnet With Fullnode

```yaml
apiVersion: cosmopilot.voluzi.com/v1
kind: ChainNodeSet
metadata:
  name: allora-testnet
spec:
  app:
    image: alloranetwork/allora-chain
    version: v0.14.0
    app: allorad
    sdkVersion: v0.50

  validator:
    accountPrefix: allo
    valPrefix: allovaloper

    info:
      moniker: cosmopilot

    config:
      override:
        app.toml:
          minimum-gas-prices: 10uallo

    init:
      chainID: allora-testnet-0
      assets: ["1000000000allo"]
      stakeAmount: 10allo
      unbondingTime: 60s
      votingPeriod: 60s
      expeditedVotingPeriod: 30s
      additionalInitCommands:
        # Use uALLO as default denom
        - image: busybox
          command: [ "sh", "-c" ]
          args:
            - sed -i 's/"stake"/"uallo"/g' /home/app/config/genesis.json;

  nodes:
    - name: fullnodes
      instances: 1

      config:
        override:
          app.toml:
            minimum-gas-prices: 10uallo
            pruning: custom
            pruning-keep-recent: "100"
            pruning-interval: "10"

```
