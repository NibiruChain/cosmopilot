# Vertical Pod Autoscaling

Vertical Pod Autoscaling (VPA) automatically adjusts CPU and memory requests based on actual usage. This helps keep pods right-sized without manual tuning.

`Cosmopilot` can configure VPA for a `ChainNode` or the validator within a `ChainNodeSet`.

## Example

```yaml
spec:
  vpa:
    enabled: true
    cpu:
      cooldown: 30m
      min: 750m
      max: 8000m
      rules:
        - direction: up
          usagePercent: 90
          duration: 5m
          stepPercent: 50
        - direction: down
          usagePercent: 40
          duration: 30m
          stepPercent: 50
    memory:
      cooldown: 30m
      min: 4Gi
      max: 32Gi
      rules:
        - direction: up
          usagePercent: 90
          duration: 5m
          stepPercent: 75
        - direction: down
          usagePercent: 40
          duration: 30m
          stepPercent: 50
```

In this example the `ChainNode` defines CPU and memory scaling rules for its pods.

## Per-Rule Cooldown

You can override the metric-level cooldown for individual rules by specifying a `cooldown` field in the rule itself:

```yaml
spec:
  vpa:
    enabled: true
    cpu:
      cooldown: 30m  # Default cooldown for all CPU rules
      min: 750m
      max: 8000m
      rules:
        - direction: up
          usagePercent: 90
          duration: 5m
          stepPercent: 50
          cooldown: 5m  # Override: scale up more frequently
        - direction: down
          usagePercent: 40
          duration: 30m
          stepPercent: 50
          # No cooldown specified - uses the 30m default
```

In this example, the scale-up rule can trigger every 5 minutes, while the scale-down rule respects the default 30-minute cooldown. This allows for responsive scaling up during high load while preventing frequent scale-down actions.

## Notes

- VPA requires a Vertical Pod Autoscaler controller running in the cluster.
- If `enabled` is set to `false`, pods keep their configured CPU and memory requests without vertical autoscaling.
- VPA is automatically disabled for a `ChainNode` that has the `StateSyncing` status to prevent it from restarting. Once state synchronization is complete, VPA is re-enabled.
