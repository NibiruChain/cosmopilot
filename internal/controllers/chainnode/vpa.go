package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
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

	// Check for OOM and handle emergency scale-up for memory
	if chainNode.Spec.VPA.Memory != nil {
		currentPod, err := r.getChainNodePod(ctx, chainNode)
		if err == nil && currentPod != nil {
			if isOOMKilled(currentPod, chainNode.Spec.App.App) {
				logger.Info("OOM kill detected, checking emergency scale-up")
				emergencyScaled, newMemRequest, err := r.handleOOMRecovery(ctx, chainNode, updated)
				if err != nil {
					logger.Error(err, "failed to handle OOM recovery")
				} else if emergencyScaled {
					updated.Requests[corev1.ResourceMemory] = newMemRequest
					memScaleTs = time.Now()

					// Calculate new limit based on strategy
					newMemLimit := calculateLimitFromRequest(chainNode, newMemRequest, chainNode.Spec.VPA.Memory, corev1.ResourceMemory)
					if newMemLimit != nil {
						if updated.Limits == nil {
							updated.Limits = corev1.ResourceList{}
						}
						updated.Limits[corev1.ResourceMemory] = *newMemLimit
					} else {
						delete(updated.Limits, corev1.ResourceMemory)
					}

					// Store and return immediately - skip normal VPA evaluation for memory
					return updated, r.storeVpaLastAppliedResources(ctx, chainNode, updated, cpuScaleTs, memScaleTs)
				}
			}
		}
	}

	client := r.statsClientFactory(chainNode.GetNodeFQDN())

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

func (r *Reconciler) evaluateCpuRule(ctx context.Context, chainNode *appsv1.ChainNode, client nodeutils.StatsClient, current corev1.ResourceRequirements, cfg *appsv1.VerticalAutoscalingMetricConfig, rule *appsv1.VerticalAutoscalingRule) (bool, resource.Quantity, error) {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	avg, err := client.GetCPUStats(ctx, rule.GetDuration())
	if err != nil {
		return false, resource.Quantity{}, err
	}

	if avg == 0 {
		return false, resource.Quantity{}, nil
	}

	// Always use current requests for usage calculation and step calculation
	currentRequest, ok := current.Requests[corev1.ResourceCPU]
	if !ok {
		return false, resource.Quantity{}, fmt.Errorf("no CPU request found")
	}

	requestMillicores := currentRequest.MilliValue()
	if requestMillicores == 0 {
		return false, resource.Quantity{}, fmt.Errorf("CPU request is zero, cannot evaluate")
	}

	usedPercent := int((avg * 1000 / float64(requestMillicores)) * 100)
	logger.V(1).Info("got cpu usage", "percentage", usedPercent, "duration", rule.GetDuration())

	// Apply hysteresis for scale-down rules
	effectiveThreshold := rule.UsagePercent
	if rule.Direction == appsv1.ScaleDown {
		hysteresis := cfg.GetHysteresisPercent()
		effectiveThreshold = rule.UsagePercent - hysteresis
		if effectiveThreshold < 0 {
			effectiveThreshold = 0
		}
	}

	shouldScale := (rule.Direction == appsv1.ScaleUp && usedPercent >= effectiveThreshold) ||
		(rule.Direction == appsv1.ScaleDown && usedPercent <= effectiveThreshold)

	if shouldScale {
		// Calculate step from current request (not from source/limit)
		step := requestMillicores * int64(rule.StepPercent) / 100
		var newVal int64
		if rule.Direction == appsv1.ScaleUp {
			newVal = requestMillicores + step
		} else {
			newVal = requestMillicores - step

			// Apply safety margin for scale-down: don't go below usage + margin
			safetyMargin := cfg.GetSafetyMarginPercent()
			avgMillicores := int64(avg * 1000)
			minSafeVal := avgMillicores * int64(100+safetyMargin) / 100
			if newVal < minSafeVal {
				newVal = minSafeVal
				logger.V(1).Info("safety margin applied", "minSafeVal", minSafeVal, "avgMillicores", avgMillicores)
			}
		}

		// Clamp to configured min/max
		newVal = clamp(newVal, cfg.Min.MilliValue(), cfg.Max.MilliValue())
		newQuantity := resource.NewMilliQuantity(newVal, resource.DecimalSI)

		r.recorder.Eventf(chainNode, corev1.EventTypeNormal, getScaleReason(rule.Direction),
			"Scaling CPU requests %s from %s to %s (CPU usage was at %d%% for the past %s)",
			rule.Direction, currentRequest.String(), newQuantity.String(), usedPercent, rule.GetDuration().String(),
		)

		return true, *newQuantity, nil
	}

	return false, resource.Quantity{}, nil
}

func (r *Reconciler) evaluateMemoryRule(ctx context.Context, chainNode *appsv1.ChainNode, client nodeutils.StatsClient, current corev1.ResourceRequirements, cfg *appsv1.VerticalAutoscalingMetricConfig, rule *appsv1.VerticalAutoscalingRule) (bool, resource.Quantity, error) {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	avg, err := client.GetMemoryStats(ctx, rule.GetDuration())
	if err != nil {
		return false, resource.Quantity{}, err
	}

	if avg == 0 {
		return false, resource.Quantity{}, nil
	}

	// Always use current requests for usage calculation and step calculation
	currentRequest, ok := current.Requests[corev1.ResourceMemory]
	if !ok {
		return false, resource.Quantity{}, fmt.Errorf("no memory request found")
	}

	requestBytes := currentRequest.Value()
	if requestBytes == 0 {
		return false, resource.Quantity{}, fmt.Errorf("memory request is zero, cannot evaluate")
	}

	usedPercent := int((float64(avg) / float64(requestBytes)) * 100)
	logger.V(1).Info("got memory usage", "percentage", usedPercent, "duration", rule.GetDuration())

	// Apply hysteresis for scale-down rules
	effectiveThreshold := rule.UsagePercent
	if rule.Direction == appsv1.ScaleDown {
		hysteresis := cfg.GetHysteresisPercent()
		effectiveThreshold = rule.UsagePercent - hysteresis
		if effectiveThreshold < 0 {
			effectiveThreshold = 0
		}
	}

	shouldScale := (rule.Direction == appsv1.ScaleUp && usedPercent >= effectiveThreshold) ||
		(rule.Direction == appsv1.ScaleDown && usedPercent <= effectiveThreshold)

	if shouldScale {
		// Calculate step from current request (not from source/limit)
		step := int64(float64(requestBytes) * float64(rule.StepPercent) / 100.0)
		var newVal int64
		if rule.Direction == appsv1.ScaleUp {
			newVal = requestBytes + step
		} else {
			newVal = requestBytes - step

			// Apply safety margin for scale-down: don't go below usage + margin
			safetyMargin := cfg.GetSafetyMarginPercent()
			minSafeVal := int64(float64(avg) * float64(100+safetyMargin) / 100.0)
			if newVal < minSafeVal {
				newVal = minSafeVal
				logger.V(1).Info("safety margin applied", "minSafeVal", minSafeVal, "avgBytes", avg)
			}
		}

		// Clamp within configured min/max
		newVal = clamp(newVal, cfg.Min.Value(), cfg.Max.Value())

		// Round up to nearest MiB
		rounded := ((newVal + OneMiB - 1) / OneMiB) * OneMiB
		newQuantity := resource.NewQuantity(rounded, resource.BinarySI)

		r.recorder.Eventf(chainNode, corev1.EventTypeNormal, getScaleReason(rule.Direction),
			"Scaling memory requests %s from %s to %s (Memory usage was at %d%% for the past %s)",
			rule.Direction, currentRequest.String(), newQuantity.String(), usedPercent, rule.GetDuration().String(),
		)

		return true, *newQuantity, nil
	}

	return false, resource.Quantity{}, nil
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

func (r *Reconciler) resetVpaAfterUpgrade(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx).WithValues("module", "vpa")

	if !chainNode.Spec.VPA.IsEnabled() {
		return nil
	}

	if !chainNode.Spec.VPA.ResetVpaAfterNodeUpgrade {
		return nil
	}

	logger.Info("resetting vpa after node upgrade")

	annotations := chainNode.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	changed := false

	// Clear VPA resources to revert to user-specified values
	if _, ok := annotations[controllers.AnnotationVPAResources]; ok {
		delete(annotations, controllers.AnnotationVPAResources)
		changed = true
	}

	// Set cooldown timestamps to prevent immediate VPA action after upgrade
	now := time.Now().UTC().Format(timeLayout)
	if annotations[controllers.AnnotationVPALastCPUScale] != now {
		annotations[controllers.AnnotationVPALastCPUScale] = now
		changed = true
	}
	if annotations[controllers.AnnotationVPALastMemoryScale] != now {
		annotations[controllers.AnnotationVPALastMemoryScale] = now
		changed = true
	}

	if changed {
		chainNode.Annotations = annotations
		return r.Update(ctx, chainNode)
	}
	return nil
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

// isOOMKilled checks if the specified container was OOM killed in its last termination
func isOOMKilled(pod *corev1.Pod, containerName string) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == containerName {
			if cs.LastTerminationState.Terminated != nil &&
				cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return true
			}
		}
	}
	return false
}

// handleOOMRecovery handles emergency scale-up on OOM detection with rate limiting
func (r *Reconciler) handleOOMRecovery(ctx context.Context, chainNode *appsv1.ChainNode, current corev1.ResourceRequirements) (bool, resource.Quantity, error) {
	logger := log.FromContext(ctx).WithValues("module", "vpa")
	cfg := chainNode.Spec.VPA.Memory

	// Get current memory request
	currentRequest, ok := current.Requests[corev1.ResourceMemory]
	if !ok {
		return false, resource.Quantity{}, fmt.Errorf("no memory request found for OOM recovery")
	}

	// Check if we're within the OOM recovery limit
	oomHistory := getOOMRecoveryHistory(chainNode)
	recoveryWindow := cfg.GetOOMRecoveryWindow()
	maxRecoveries := cfg.GetMaxOOMRecoveries()

	// Filter history to only include entries within the window
	cutoff := time.Now().Add(-recoveryWindow)
	recentRecoveries := 0
	for _, ts := range oomHistory {
		if ts.After(cutoff) {
			recentRecoveries++
		}
	}

	if recentRecoveries >= maxRecoveries {
		logger.Info("OOM recovery limit reached, possible memory leak",
			"recentRecoveries", recentRecoveries,
			"maxRecoveries", maxRecoveries,
			"window", recoveryWindow)
		r.recorder.Eventf(chainNode, corev1.EventTypeWarning, appsv1.ReasonVPAOOMRecoveryLimitReached,
			"OOM recovery limit reached (%d/%d in %s). Possible memory leak - manual intervention required.",
			recentRecoveries, maxRecoveries, recoveryWindow)
		return false, resource.Quantity{}, nil
	}

	// Calculate emergency scale-up
	emergencyPercent := cfg.GetEmergencyScaleUpPercent()
	currentBytes := currentRequest.Value()
	step := int64(float64(currentBytes) * float64(emergencyPercent) / 100.0)
	newVal := currentBytes + step

	// Clamp to max
	newVal = clamp(newVal, cfg.Min.Value(), cfg.Max.Value())

	// Round up to nearest MiB
	rounded := ((newVal + OneMiB - 1) / OneMiB) * OneMiB
	newQuantity := resource.NewQuantity(rounded, resource.BinarySI)

	// Record event
	r.recorder.Eventf(chainNode, corev1.EventTypeWarning, appsv1.ReasonVPAEmergencyScaleUp,
		"Emergency memory scale-up due to OOM: %s -> %s (recovery %d/%d)",
		currentRequest.String(), newQuantity.String(), recentRecoveries+1, maxRecoveries)

	logger.Info("performing emergency memory scale-up",
		"old-request", currentRequest.String(),
		"new-request", newQuantity.String(),
		"emergencyPercent", emergencyPercent,
		"recovery", recentRecoveries+1)

	// Append to OOM recovery history
	if err := r.appendOOMRecoveryHistory(ctx, chainNode); err != nil {
		return false, resource.Quantity{}, err
	}

	return true, *newQuantity, nil
}

// getOOMRecoveryHistory retrieves the OOM recovery history from annotations
func getOOMRecoveryHistory(chainNode *appsv1.ChainNode) []time.Time {
	data, ok := chainNode.Annotations[controllers.AnnotationVPAOOMRecoveryHistory]
	if !ok || data == "" {
		return nil
	}

	var timestamps []string
	if err := json.Unmarshal([]byte(data), &timestamps); err != nil {
		return nil
	}

	var history []time.Time
	for _, ts := range timestamps {
		if t, err := time.Parse(timeLayout, ts); err == nil {
			history = append(history, t.UTC())
		}
	}
	return history
}

// appendOOMRecoveryHistory adds the current timestamp to OOM recovery history
func (r *Reconciler) appendOOMRecoveryHistory(ctx context.Context, chainNode *appsv1.ChainNode) error {
	cfg := chainNode.Spec.VPA.Memory
	recoveryWindow := cfg.GetOOMRecoveryWindow()
	cutoff := time.Now().Add(-recoveryWindow)

	// Get existing history and filter to keep only recent entries
	history := getOOMRecoveryHistory(chainNode)
	var recentHistory []string
	for _, ts := range history {
		if ts.After(cutoff) {
			recentHistory = append(recentHistory, ts.UTC().Format(timeLayout))
		}
	}

	// Add current timestamp
	recentHistory = append(recentHistory, time.Now().UTC().Format(timeLayout))

	// Serialize and store
	data, err := json.Marshal(recentHistory)
	if err != nil {
		return err
	}

	annotations := chainNode.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[controllers.AnnotationVPAOOMRecoveryHistory] = string(data)
	chainNode.Annotations = annotations

	return r.Update(ctx, chainNode)
}
