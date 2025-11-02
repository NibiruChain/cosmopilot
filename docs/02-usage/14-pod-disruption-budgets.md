# Pod Disruption Budgets

Pod Disruption Budgets (PDBs) ensure that a minimum number of pods remain available during voluntary disruptions such as node upgrades or evictions. `Cosmopilot` allows you to configure PDBs for validator and node groups within a `ChainNodeSet`.

## Example

```yaml
spec:
  nodes:
    - name: fullnode
      instances: 3
      pdb:
        enabled: true
        minAvailable: 2   # optional, defaults to instances - 1
  validator:
    pdb:
      enabled: true
      minAvailable: 3    # meaningful only when other validators exist
```

With this configuration, Kubernetes will ensure that at least two fullnode pods remain running and, when multiple validators exist in the namespace, at least three validator pods stay available during maintenance operations.

## Notes

- PDBs are currently supported only on `ChainNodeSet` resources.
- `minAvailable` defaults to the number of instances minus one for node groups.
- A validator PDB only has an effect when multiple validators run in the same namespace; otherwise the default `minAvailable: 0` leaves it ineffective.
- During [upgrades](08-upgrades), PDBs are automatically disabled for `ChainNodes` with the `Upgrading` status.
