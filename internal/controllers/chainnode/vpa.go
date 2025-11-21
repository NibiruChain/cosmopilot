package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/pkg/nodeutils"
)

const OneMiB = int64(1024 * 1024)

func (r *Reconciler) maybeGetVpaResources(ctx context.Context, chainNode *appsv1.ChainNode) (corev1.ResourceRequirements, error) {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	if !chainNode.Spec.VPA.IsEnabled() {
		return chainNode.GetResources(), r.clearVpaLastAppliedResources(ctx, chainNode)
	}

	if chainNode.Status.Phase == appsv1.PhaseChainNodeStateSyncing {
		logger.Info("Skipping VPA while node is state-syncing")
		return chainNode.GetResources(), nil
	}

	// Clone the last applied or fallback
	updated := getVpaLastAppliedResourcesOrFallback(chainNode)
	var cpuScaleTs, memScaleTs time.Time

	client := nodeutils.NewClient(chainNode.GetNodeFQDN())

	if chainNode.Spec.VPA.CPU != nil {
		metricCooldown := chainNode.Spec.VPA.CPU.GetCooldownDuration()
		lastCpuScaleTime := getLastCpuScaleTime(chainNode)

		for _, rule := range chainNode.Spec.VPA.CPU.Rules {
			ruleCooldown := rule.GetCooldownDuration(metricCooldown)
			if withinCooldown(lastCpuScaleTime, ruleCooldown) {
				logger.V(1).Info("vpa rule is within cooldown period for cpu scaling", "rule", rule.Direction, "cooldown", ruleCooldown)
				continue
			}

			shouldScale, newCpuRequest, err := r.evaluateCpuRule(ctx, chainNode, client, updated, chainNode.Spec.VPA.CPU, rule)
			if err != nil {
				return getVpaLastAppliedResourcesOrFallback(chainNode), err
			}

			if shouldScale {
				oldCpuRequest := updated.Requests[corev1.ResourceCPU]
				var oldCpuLimit *resource.Quantity
				if v, ok := updated.Limits[corev1.ResourceCPU]; ok {
					oldCpuLimit = &v
				}

				updated.Requests[corev1.ResourceCPU] = newCpuRequest
				cpuScaleTs = time.Now()

				newCpuLimit := calculateLimitFromRequest(chainNode, newCpuRequest, chainNode.Spec.VPA.CPU, corev1.ResourceCPU)
				if newCpuLimit != nil {
					if updated.Limits == nil {
						updated.Limits = corev1.ResourceList{}
					}
					updated.Limits[corev1.ResourceCPU] = *newCpuLimit
				} else {
					delete(updated.Limits, corev1.ResourceCPU)
				}

				logger.Info("scaling cpu",
					"old-request", oldCpuRequest,
					"new-request", newCpuRequest,
					"old-limit", oldCpuLimit,
					"new-limit", newCpuLimit,
				)
				break
			}
		}
	}

	if chainNode.Spec.VPA.Memory != nil {
		metricCooldown := chainNode.Spec.VPA.Memory.GetCooldownDuration()
		lastMemoryScaleTime := getLastMemoryScaleTime(chainNode)

		for _, rule := range chainNode.Spec.VPA.Memory.Rules {
			ruleCooldown := rule.GetCooldownDuration(metricCooldown)
			if withinCooldown(lastMemoryScaleTime, ruleCooldown) {
				logger.V(1).Info("vpa rule is within cooldown period for memory scaling", "rule", rule.Direction, "cooldown", ruleCooldown)
				continue
			}

			shouldScale, newMemRequest, err := r.evaluateMemoryRule(ctx, chainNode, client, updated, chainNode.Spec.VPA.Memory, rule)
			if err != nil {
				return getVpaLastAppliedResourcesOrFallback(chainNode), err
			}

			if shouldScale {
				oldMemRequest := updated.Requests[corev1.ResourceMemory]
				var oldMemLimit *resource.Quantity
				if v, ok := updated.Limits[corev1.ResourceMemory]; ok {
					oldMemLimit = &v
				}

				updated.Requests[corev1.ResourceMemory] = newMemRequest
				memScaleTs = time.Now()

				newMemLimit := calculateLimitFromRequest(chainNode, newMemRequest, chainNode.Spec.VPA.Memory, corev1.ResourceMemory)
				if newMemLimit != nil {
					if updated.Limits == nil {
						updated.Limits = corev1.ResourceList{}
					}
					updated.Limits[corev1.ResourceMemory] = *newMemLimit
				} else {
					delete(updated.Limits, corev1.ResourceMemory)
				}

				logger.Info("scaling memory",
					"old-request", oldMemRequest,
					"new-request", newMemRequest,
					"old-limit", oldMemLimit,
					"new-limit", newMemLimit,
				)
				break
			}
		}
	}

	return updated, r.storeVpaLastAppliedResources(ctx, chainNode, updated, cpuScaleTs, memScaleTs)
}

func getScaleReason(direction appsv1.ScalingDirection) string {
	if direction == appsv1.ScaleUp {
		return appsv1.ReasonVPAScaleUp
	}
	return appsv1.ReasonVPAScaleDown
}

func (r *Reconciler) evaluateCpuRule(ctx context.Context, chainNode *appsv1.ChainNode, client *nodeutils.Client, current corev1.ResourceRequirements, cfg *appsv1.VerticalAutoscalingMetricConfig, rule *appsv1.VerticalAutoscalingRule) (bool, resource.Quantity, error) {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	avg, err := client.GetCPUStats(ctx, rule.GetDuration())
	if err != nil {
		return false, resource.Quantity{}, err
	}

	if avg == 0 {
		return false, resource.Quantity{}, nil
	}

	limit, err := getSourceCpuQuantity(current, cfg.GetSource())
	if err != nil {
		return false, resource.Quantity{}, err
	}

	limitMillicores := limit.MilliValue()
	usedPercent := int((avg * 1000 / float64(limitMillicores)) * 100)
	logger.V(1).Info("got cpu usage", "percentage", usedPercent, "duration", rule.GetDuration())

	if (rule.Direction == appsv1.ScaleUp && usedPercent >= rule.UsagePercent) || (rule.Direction == appsv1.ScaleDown && usedPercent <= rule.UsagePercent) {
		step := limitMillicores * int64(rule.StepPercent) / 100
		var newVal int64
		if rule.Direction == appsv1.ScaleUp {
			newVal = limitMillicores + step
		} else {
			newVal = limitMillicores - step
		}
		// Clamp
		newVal = clamp(newVal, cfg.Min.MilliValue(), cfg.Max.MilliValue())
		newQuantity := resource.NewMilliQuantity(newVal, resource.DecimalSI)

		r.recorder.Eventf(chainNode, corev1.EventTypeNormal, getScaleReason(rule.Direction),
			"Scaling CPU requests %s from %s to %s (CPU usage was at %d%% for the past %s)",
			rule.Direction, limit.String(), newQuantity.String(), usedPercent, rule.GetDuration().String(),
		)

		return true, *newQuantity, nil
	}

	return false, resource.Quantity{}, nil
}

func (r *Reconciler) evaluateMemoryRule(ctx context.Context, chainNode *appsv1.ChainNode, client *nodeutils.Client, current corev1.ResourceRequirements, cfg *appsv1.VerticalAutoscalingMetricConfig, rule *appsv1.VerticalAutoscalingRule) (bool, resource.Quantity, error) {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	avg, err := client.GetMemoryStats(ctx, rule.GetDuration())
	if err != nil {
		return false, resource.Quantity{}, err
	}

	if avg == 0 {
		return false, resource.Quantity{}, nil
	}

	limit, err := getSourceMemoryQuantity(current, cfg.GetSource())
	if err != nil {
		return false, resource.Quantity{}, err
	}

	limitBytes := limit.Value()
	if limitBytes == 0 {
		return false, resource.Quantity{}, fmt.Errorf("memory limit is zero, cannot evaluate")
	}

	usedPercent := int((float64(avg) / float64(limitBytes)) * 100)
	logger.V(1).Info("got memory usage", "percentage", usedPercent, "duration", rule.GetDuration())

	if (rule.Direction == appsv1.ScaleUp && usedPercent >= rule.UsagePercent) ||
		(rule.Direction == appsv1.ScaleDown && usedPercent <= rule.UsagePercent) {
		step := int64(float64(limitBytes) * float64(rule.StepPercent) / 100.0)
		var newVal int64
		if rule.Direction == appsv1.ScaleUp {
			newVal = limitBytes + step
		} else {
			newVal = limitBytes - step
		}
		// Clamp within configured min/max
		newVal = clamp(newVal, cfg.Min.Value(), cfg.Max.Value())

		// Round up to nearest MiB
		rounded := ((newVal + OneMiB - 1) / OneMiB) * OneMiB
		newQuantity := resource.NewQuantity(rounded, resource.BinarySI)

		r.recorder.Eventf(chainNode, corev1.EventTypeNormal, getScaleReason(rule.Direction),
			"Scaling memory requests %s from %s to %s (Memory usage was at %d%% for the past %s)",
			rule.Direction, limit.String(), newQuantity.String(), usedPercent, rule.GetDuration().String(),
		)

		return true, *newQuantity, nil
	}

	return false, resource.Quantity{}, nil
}

func getSourceCpuQuantity(current corev1.ResourceRequirements, source appsv1.LimitSource) (resource.Quantity, error) {
	return getSourceQuantity(current, source, corev1.ResourceCPU)
}

func getSourceMemoryQuantity(current corev1.ResourceRequirements, source appsv1.LimitSource) (resource.Quantity, error) {
	return getSourceQuantity(current, source, corev1.ResourceMemory)
}

func getSourceQuantity(current corev1.ResourceRequirements, source appsv1.LimitSource, name corev1.ResourceName) (resource.Quantity, error) {
	switch source {
	case appsv1.Limits:
		if cpu, ok := current.Limits[name]; ok {
			return cpu, nil
		}
		return resource.Quantity{}, fmt.Errorf("no %s %s found", name, source)

	case appsv1.Requests:
		if cpu, ok := current.Requests[name]; ok {
			return cpu, nil
		}
		return resource.Quantity{}, fmt.Errorf("no %s %s found", name, source)

	case appsv1.EffectiveLimit:
		if cpu, ok := current.Limits[name]; ok {
			return cpu, nil
		}
		// Fallback to requests
		if q, ok := current.Requests[name]; ok {
			return q, nil
		}
		return resource.Quantity{}, fmt.Errorf("no %s limits or requests found", name)

	default:
		return resource.Quantity{}, fmt.Errorf("invalid limit source")
	}
}

func withinCooldown(last time.Time, cooldown time.Duration) bool {
	return time.Since(last) < cooldown
}

func getLastCpuScaleTime(chainNode *appsv1.ChainNode) time.Time {
	if s, ok := chainNode.ObjectMeta.Annotations[controllers.AnnotationVPALastCPUScale]; ok {
		if ts, err := time.Parse(timeLayout, s); err == nil {
			return ts.UTC()
		}
	}
	return chainNode.ObjectMeta.CreationTimestamp.Time
}

func getLastMemoryScaleTime(chainNode *appsv1.ChainNode) time.Time {
	if s, ok := chainNode.ObjectMeta.Annotations[controllers.AnnotationVPALastMemoryScale]; ok {
		if ts, err := time.Parse(timeLayout, s); err == nil {
			return ts.UTC()
		}
	}
	return chainNode.ObjectMeta.CreationTimestamp.Time
}

func (r *Reconciler) storeVpaLastAppliedResources(
	ctx context.Context,
	chainNode *appsv1.ChainNode,
	resources corev1.ResourceRequirements,
	cpuScaleTs, memScaleTs time.Time,
) error {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	// Serialize current resources
	b, err := json.Marshal(resources)
	if err != nil {
		return err
	}
	newResourcesJSON := string(b)

	// Get current annotations
	annotations := chainNode.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	changed := false

	// Compare resources
	if annotations[controllers.AnnotationVPAResources] != newResourcesJSON {
		annotations[controllers.AnnotationVPAResources] = newResourcesJSON
		changed = true
	}

	// Compare CPU timestamp
	if !cpuScaleTs.IsZero() {
		newCPUTime := cpuScaleTs.UTC().Format(timeLayout)
		if annotations[controllers.AnnotationVPALastCPUScale] != newCPUTime {
			annotations[controllers.AnnotationVPALastCPUScale] = newCPUTime
			changed = true
		}
	}

	// Compare Memory timestamp
	if !memScaleTs.IsZero() {
		newMemTime := memScaleTs.UTC().Format(timeLayout)
		if annotations[controllers.AnnotationVPALastMemoryScale] != newMemTime {
			annotations[controllers.AnnotationVPALastMemoryScale] = newMemTime
			changed = true
		}
	}

	if changed {
		logger.Info("updating annotations")
		chainNode.Annotations = annotations
		return r.Update(ctx, chainNode)
	}
	return nil
}

func getVpaLastAppliedResourcesOrFallback(chainNode *appsv1.ChainNode) corev1.ResourceRequirements {
	data, ok := chainNode.Annotations[controllers.AnnotationVPAResources]
	if !ok {
		return chainNode.GetResources()
	}

	vpaResources := corev1.ResourceRequirements{}
	if err := json.Unmarshal([]byte(data), &vpaResources); err != nil {
		return chainNode.GetResources()
	}

	// Merge VPA resources with fallback, preserving non-CPU/Memory resources
	fallback := chainNode.GetResources()

	// Start with fallback as base
	merged := fallback

	// Override CPU and Memory from VPA
	if merged.Requests == nil {
		merged.Requests = corev1.ResourceList{}
	}
	if merged.Limits == nil {
		merged.Limits = corev1.ResourceList{}
	}

	// Copy VPA CPU/Memory to requests
	if q, ok := vpaResources.Requests[corev1.ResourceCPU]; ok {
		merged.Requests[corev1.ResourceCPU] = q
	}
	if q, ok := vpaResources.Requests[corev1.ResourceMemory]; ok {
		merged.Requests[corev1.ResourceMemory] = q
	}

	// Copy VPA CPU/Memory to limits
	if q, ok := vpaResources.Limits[corev1.ResourceCPU]; ok {
		merged.Limits[corev1.ResourceCPU] = q
	}
	if q, ok := vpaResources.Limits[corev1.ResourceMemory]; ok {
		merged.Limits[corev1.ResourceMemory] = q
	}

	return merged
}

func (r *Reconciler) clearVpaLastAppliedResources(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	if _, ok := chainNode.ObjectMeta.Annotations[controllers.AnnotationVPAResources]; !ok {
		return nil
	}

	logger.Info("clearing vpa annotations")
	delete(chainNode.ObjectMeta.Annotations, controllers.AnnotationVPAResources)
	return r.Update(ctx, chainNode)
}

func clamp(val, min, max int64) int64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func calculateLimitFromRequest(chainNode *appsv1.ChainNode, request resource.Quantity, cfg *appsv1.VerticalAutoscalingMetricConfig, resourceName corev1.ResourceName) *resource.Quantity {
	switch cfg.GetLimitUpdateStrategy() {
	case appsv1.LimitEqual:
		// Match requests
		limit := request.DeepCopy()
		return &limit

	case appsv1.LimitVpaMax:
		// Use configured max
		limit := cfg.Max.DeepCopy()
		return &limit

	case appsv1.LimitPercentage:
		// Use % of requests
		percent := cfg.GetLimitPercentage()

		if resourceName == corev1.ResourceCPU {
			val := request.MilliValue()
			adjusted := val * int64(percent) / 100
			return resource.NewMilliQuantity(adjusted, resource.DecimalSI)
		}

		valBytes := request.Value() // memory in bytes
		adjustedBytes := valBytes * int64(percent) / 100

		// Round to MiB boundary to avoid KiB-level noise
		adjustedMiB := (adjustedBytes + OneMiB - 1) / OneMiB // round up
		adjusted := adjustedMiB * OneMiB
		return resource.NewQuantity(adjusted, resource.BinarySI)

	case appsv1.LimitUnset:
		// Clear limits entirely
		return nil

	case appsv1.LimitRetain:
		// Copy from ChainNode resources if it exists
		if chainNode.GetResources().Limits != nil {
			qtt, ok := chainNode.GetResources().Limits[resourceName]
			if ok {
				return &qtt
			}
		}
		return nil

	default:
		return nil
	}
}
