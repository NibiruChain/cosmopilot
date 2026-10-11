package chainnode

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

// mockStatsClient implements nodeutils.StatsClient for testing
type mockStatsClient struct {
	cpuUsage    float64 // CPU usage in cores (e.g., 0.5 = 500m)
	memoryUsage uint64  // Memory usage in bytes
	cpuErr      error
	memErr      error
}

func (m *mockStatsClient) GetCPUStats(ctx context.Context, since time.Duration) (float64, error) {
	if m.cpuErr != nil {
		return 0, m.cpuErr
	}
	return m.cpuUsage, nil
}

func (m *mockStatsClient) GetMemoryStats(ctx context.Context, since time.Duration) (uint64, error) {
	if m.memErr != nil {
		return 0, m.memErr
	}
	return m.memoryUsage, nil
}

// newTestReconciler creates a minimal reconciler for testing VPA logic
func newTestReconciler() *Reconciler {
	return &Reconciler{
		recorder: record.NewFakeRecorder(100),
	}
}

// TestScenario_CPUScaleUp tests that CPU scales up when usage exceeds threshold
func TestScenario_CPUScaleUp(t *testing.T) {
	tests := []struct {
		name           string
		currentCPU     string
		cpuUsage       float64 // in cores
		usageThreshold int
		stepPercent    int
		minCPU         string
		maxCPU         string
		wantScale      bool
		wantNewCPU     string
	}{
		{
			name:           "85% usage triggers scale-up at 80% threshold",
			currentCPU:     "1000m",
			cpuUsage:       0.85, // 850m = 85% of 1000m
			usageThreshold: 80,
			stepPercent:    20,
			minCPU:         "500m",
			maxCPU:         "4000m",
			wantScale:      true,
			wantNewCPU:     "1200m", // 1000m * 1.20
		},
		{
			name:           "exactly 80% usage triggers scale-up at 80% threshold",
			currentCPU:     "1000m",
			cpuUsage:       0.80, // 800m = 80% of 1000m
			usageThreshold: 80,
			stepPercent:    20,
			minCPU:         "500m",
			maxCPU:         "4000m",
			wantScale:      true,
			wantNewCPU:     "1200m",
		},
		{
			name:           "79% usage does NOT trigger scale-up at 80% threshold",
			currentCPU:     "1000m",
			cpuUsage:       0.79, // 790m = 79% of 1000m
			usageThreshold: 80,
			stepPercent:    20,
			minCPU:         "500m",
			maxCPU:         "4000m",
			wantScale:      false,
		},
		{
			name:           "scale-up clamped to max",
			currentCPU:     "3500m",
			cpuUsage:       3.0, // 3000m = 86% of 3500m
			usageThreshold: 80,
			stepPercent:    50, // Would go to 5250m without clamp
			minCPU:         "500m",
			maxCPU:         "4000m",
			wantScale:      true,
			wantNewCPU:     "4", // Clamped to max (4000m = "4" in canonical form)
		},
		{
			name:           "50% step doubles CPU",
			currentCPU:     "500m",
			cpuUsage:       0.45, // 450m = 90% of 500m
			usageThreshold: 80,
			stepPercent:    50,
			minCPU:         "100m",
			maxCPU:         "4000m",
			wantScale:      true,
			wantNewCPU:     "750m", // 500m * 1.50
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			ctx := context.Background()

			chainNode := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}

			client := &mockStatsClient{cpuUsage: tt.cpuUsage}

			current := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse(tt.currentCPU),
				},
			}

			cfg := &appsv1.VerticalAutoscalingMetricConfig{
				Min: resource.MustParse(tt.minCPU),
				Max: resource.MustParse(tt.maxCPU),
			}

			rule := &appsv1.VerticalAutoscalingRule{
				Direction:    appsv1.ScaleUp,
				UsagePercent: tt.usageThreshold,
				StepPercent:  tt.stepPercent,
			}

			shouldScale, newCPU, err := r.evaluateCpuRule(ctx, chainNode, client, current, cfg, rule)
			if err != nil {
				t.Fatalf("evaluateCpuRule() error = %v", err)
			}

			if shouldScale != tt.wantScale {
				t.Errorf("shouldScale = %v, want %v", shouldScale, tt.wantScale)
			}

			if tt.wantScale && newCPU.String() != tt.wantNewCPU {
				t.Errorf("newCPU = %s, want %s", newCPU.String(), tt.wantNewCPU)
			}
		})
	}
}

// TestScenario_CPUScaleDown tests that CPU scales down when usage drops below threshold
func TestScenario_CPUScaleDown(t *testing.T) {
	tests := []struct {
		name              string
		currentCPU        string
		cpuUsage          float64
		usageThreshold    int
		stepPercent       int
		hysteresisPercent int
		safetyMargin      int
		minCPU            string
		maxCPU            string
		wantScale         bool
		wantNewCPU        string
	}{
		{
			name:              "25% usage triggers scale-down at 40% threshold with 10% hysteresis",
			currentCPU:        "2000m",
			cpuUsage:          0.5, // 500m = 25% of 2000m
			usageThreshold:    40,  // Effective: 40% - 10% = 30%
			stepPercent:       20,
			hysteresisPercent: 10,
			safetyMargin:      15,
			minCPU:            "500m",
			maxCPU:            "4000m",
			wantScale:         true,
			wantNewCPU:        "1600m", // 2000m * 0.80
		},
		{
			name:              "35% usage does NOT trigger scale-down (above effective threshold)",
			currentCPU:        "2000m",
			cpuUsage:          0.7, // 700m = 35% of 2000m
			usageThreshold:    40,  // Effective: 40% - 10% = 30%
			stepPercent:       20,
			hysteresisPercent: 10,
			safetyMargin:      15,
			minCPU:            "500m",
			maxCPU:            "4000m",
			wantScale:         false,
		},
		{
			name:              "scale-down clamped to min",
			currentCPU:        "600m",
			cpuUsage:          0.1, // 100m = 17% of 600m
			usageThreshold:    40,
			stepPercent:       50, // Would go to 300m without clamp
			hysteresisPercent: 0,
			safetyMargin:      0,
			minCPU:            "500m",
			maxCPU:            "4000m",
			wantScale:         true,
			wantNewCPU:        "500m", // Clamped to min
		},
		{
			name:              "safety margin prevents aggressive scale-down",
			currentCPU:        "2000m",
			cpuUsage:          0.6, // 600m = 30% of 2000m
			usageThreshold:    40,
			stepPercent:       80, // Would go to 400m (2000 * 0.2)
			hysteresisPercent: 0,
			safetyMargin:      20, // Usage + 20% = 600m * 1.2 = 720m
			minCPU:            "500m",
			maxCPU:            "4000m",
			wantScale:         true,
			wantNewCPU:        "720m", // Clamped to safety margin (600 * 1.2), not 400m
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			ctx := context.Background()

			chainNode := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}

			client := &mockStatsClient{cpuUsage: tt.cpuUsage}

			current := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse(tt.currentCPU),
				},
			}

			cfg := &appsv1.VerticalAutoscalingMetricConfig{
				Min:                 resource.MustParse(tt.minCPU),
				Max:                 resource.MustParse(tt.maxCPU),
				HysteresisPercent:   ptr.To(tt.hysteresisPercent),
				SafetyMarginPercent: ptr.To(tt.safetyMargin),
			}

			rule := &appsv1.VerticalAutoscalingRule{
				Direction:    appsv1.ScaleDown,
				UsagePercent: tt.usageThreshold,
				StepPercent:  tt.stepPercent,
			}

			shouldScale, newCPU, err := r.evaluateCpuRule(ctx, chainNode, client, current, cfg, rule)
			if err != nil {
				t.Fatalf("evaluateCpuRule() error = %v", err)
			}

			if shouldScale != tt.wantScale {
				t.Errorf("shouldScale = %v, want %v", shouldScale, tt.wantScale)
			}

			if tt.wantScale && newCPU.String() != tt.wantNewCPU {
				t.Errorf("newCPU = %s, want %s", newCPU.String(), tt.wantNewCPU)
			}
		})
	}
}

// TestScenario_MemoryScaleUp tests that memory scales up when usage exceeds threshold
func TestScenario_MemoryScaleUp(t *testing.T) {
	tests := []struct {
		name           string
		currentMem     string
		memUsageBytes  uint64
		usageThreshold int
		stepPercent    int
		minMem         string
		maxMem         string
		wantScale      bool
		wantMinMem     int64 // minimum expected memory in bytes
	}{
		{
			name:           "90% memory usage triggers scale-up at 80% threshold",
			currentMem:     "1Gi",
			memUsageBytes:  922 * 1024 * 1024, // ~90% of 1Gi (922 MiB)
			usageThreshold: 80,
			stepPercent:    25,
			minMem:         "512Mi",
			maxMem:         "8Gi",
			wantScale:      true,
			wantMinMem:     1280 * 1024 * 1024, // 1Gi * 1.25 = 1.25Gi (1280 MiB)
		},
		{
			name:           "75% memory usage does NOT trigger scale-up at 80% threshold",
			currentMem:     "1Gi",
			memUsageBytes:  768 * 1024 * 1024, // ~75% of 1Gi (768 MiB)
			usageThreshold: 80,
			stepPercent:    25,
			minMem:         "512Mi",
			maxMem:         "8Gi",
			wantScale:      false,
		},
		{
			name:           "scale-up clamped to max",
			currentMem:     "7Gi",
			memUsageBytes:  6656 * 1024 * 1024, // ~93% of 7Gi (6.5 GiB = 6656 MiB)
			usageThreshold: 80,
			stepPercent:    50, // Would go to 10.5Gi without clamp
			minMem:         "512Mi",
			maxMem:         "8Gi",
			wantScale:      true,
			wantMinMem:     8 * 1024 * 1024 * 1024, // Clamped to 8Gi
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			ctx := context.Background()

			chainNode := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}

			client := &mockStatsClient{memoryUsage: tt.memUsageBytes}

			current := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(tt.currentMem),
				},
			}

			cfg := &appsv1.VerticalAutoscalingMetricConfig{
				Min: resource.MustParse(tt.minMem),
				Max: resource.MustParse(tt.maxMem),
			}

			rule := &appsv1.VerticalAutoscalingRule{
				Direction:    appsv1.ScaleUp,
				UsagePercent: tt.usageThreshold,
				StepPercent:  tt.stepPercent,
			}

			shouldScale, newMem, err := r.evaluateMemoryRule(ctx, chainNode, client, current, cfg, rule)
			if err != nil {
				t.Fatalf("evaluateMemoryRule() error = %v", err)
			}

			if shouldScale != tt.wantScale {
				t.Errorf("shouldScale = %v, want %v", shouldScale, tt.wantScale)
			}

			if tt.wantScale && newMem.Value() < tt.wantMinMem {
				t.Errorf("newMem = %d bytes, want at least %d bytes", newMem.Value(), tt.wantMinMem)
			}
		})
	}
}

// TestScenario_MemoryScaleDown tests that memory scales down with safety margin
func TestScenario_MemoryScaleDown(t *testing.T) {
	tests := []struct {
		name              string
		currentMem        string
		memUsageBytes     uint64
		usageThreshold    int
		stepPercent       int
		hysteresisPercent int
		safetyMargin      int
		minMem            string
		maxMem            string
		wantScale         bool
		wantMinMem        int64 // minimum expected memory in bytes (due to safety margin)
		wantMaxMem        int64 // maximum expected memory in bytes
	}{
		{
			name:              "scale-down respects safety margin",
			currentMem:        "2Gi",
			memUsageBytes:     800 * 1024 * 1024, // 800Mi = 40% of 2Gi
			usageThreshold:    50,
			stepPercent:       50, // Would go to 1Gi without safety
			hysteresisPercent: 0,
			safetyMargin:      20, // min = 800Mi * 1.2 = 960Mi
			minMem:            "512Mi",
			maxMem:            "8Gi",
			wantScale:         true,
			wantMinMem:        960 * 1024 * 1024,  // Safety margin: 800Mi * 1.2
			wantMaxMem:        1024 * 1024 * 1024, // Should not exceed 1Gi (step result)
		},
		{
			name:              "scale-down clamped to min",
			currentMem:        "1Gi",
			memUsageBytes:     100 * 1024 * 1024, // 100Mi = 10% of 1Gi
			usageThreshold:    50,
			stepPercent:       80, // Would go to 200Mi
			hysteresisPercent: 0,
			safetyMargin:      0,
			minMem:            "512Mi", // Clamp to 512Mi
			maxMem:            "8Gi",
			wantScale:         true,
			wantMinMem:        512 * 1024 * 1024, // Clamped to min
			wantMaxMem:        512 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			ctx := context.Background()

			chainNode := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}

			client := &mockStatsClient{memoryUsage: tt.memUsageBytes}

			current := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(tt.currentMem),
				},
			}

			cfg := &appsv1.VerticalAutoscalingMetricConfig{
				Min:                 resource.MustParse(tt.minMem),
				Max:                 resource.MustParse(tt.maxMem),
				HysteresisPercent:   ptr.To(tt.hysteresisPercent),
				SafetyMarginPercent: ptr.To(tt.safetyMargin),
			}

			rule := &appsv1.VerticalAutoscalingRule{
				Direction:    appsv1.ScaleDown,
				UsagePercent: tt.usageThreshold,
				StepPercent:  tt.stepPercent,
			}

			shouldScale, newMem, err := r.evaluateMemoryRule(ctx, chainNode, client, current, cfg, rule)
			if err != nil {
				t.Fatalf("evaluateMemoryRule() error = %v", err)
			}

			if shouldScale != tt.wantScale {
				t.Errorf("shouldScale = %v, want %v", shouldScale, tt.wantScale)
			}

			if tt.wantScale {
				if newMem.Value() < tt.wantMinMem {
					t.Errorf("newMem = %d bytes, want at least %d bytes", newMem.Value(), tt.wantMinMem)
				}
				if newMem.Value() > tt.wantMaxMem {
					t.Errorf("newMem = %d bytes, want at most %d bytes", newMem.Value(), tt.wantMaxMem)
				}
			}
		})
	}
}

// TestScenario_HysteresisPreventOscillation tests that hysteresis creates stable zones
func TestScenario_HysteresisPreventOscillation(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	// Scenario: scale-up at 80%, scale-down at 50%, hysteresis 10%
	// This creates a stable zone from 40% (50-10) to 80%

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1000m"),
		},
	}

	cfgWithHysteresis := &appsv1.VerticalAutoscalingMetricConfig{
		Min:               resource.MustParse("500m"),
		Max:               resource.MustParse("4000m"),
		HysteresisPercent: ptr.To(10),
	}

	scaleUpRule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleUp,
		UsagePercent: 80,
		StepPercent:  20,
	}

	scaleDownRule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleDown,
		UsagePercent: 50,
		StepPercent:  20,
	}

	// Test usage in the "stable zone" (45% - between 40% and 80%)
	stableClient := &mockStatsClient{cpuUsage: 0.45} // 450m = 45% of 1000m

	// Should NOT scale up (45% < 80%)
	shouldScaleUp, _, err := r.evaluateCpuRule(ctx, chainNode, stableClient, current, cfgWithHysteresis, scaleUpRule)
	if err != nil {
		t.Fatalf("evaluateCpuRule() error = %v", err)
	}
	if shouldScaleUp {
		t.Error("should NOT scale up at 45% usage (threshold 80%)")
	}

	// Should NOT scale down (45% > 40% effective threshold)
	shouldScaleDown, _, err := r.evaluateCpuRule(ctx, chainNode, stableClient, current, cfgWithHysteresis, scaleDownRule)
	if err != nil {
		t.Fatalf("evaluateCpuRule() error = %v", err)
	}
	if shouldScaleDown {
		t.Error("should NOT scale down at 45% usage (effective threshold 40% with hysteresis)")
	}

	// Test usage below effective threshold (35%)
	lowClient := &mockStatsClient{cpuUsage: 0.35} // 350m = 35% of 1000m

	// Should scale down (35% <= 40% effective threshold)
	shouldScaleDown, _, err = r.evaluateCpuRule(ctx, chainNode, lowClient, current, cfgWithHysteresis, scaleDownRule)
	if err != nil {
		t.Fatalf("evaluateCpuRule() error = %v", err)
	}
	if !shouldScaleDown {
		t.Error("should scale down at 35% usage (effective threshold 40% with hysteresis)")
	}
}

// TestScenario_ZeroUsageNoScale tests that zero usage doesn't trigger scaling
func TestScenario_ZeroUsageNoScale(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	cfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("500m"),
		Max: resource.MustParse("4000m"),
	}

	rule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleDown,
		UsagePercent: 50,
		StepPercent:  20,
	}

	// Zero CPU usage - should not scale
	zeroClient := &mockStatsClient{cpuUsage: 0, memoryUsage: 0}

	shouldScale, _, err := r.evaluateCpuRule(ctx, chainNode, zeroClient, current, cfg, rule)
	if err != nil {
		t.Fatalf("evaluateCpuRule() error = %v", err)
	}
	if shouldScale {
		t.Error("should NOT scale when CPU usage is zero")
	}

	shouldScale, _, err = r.evaluateMemoryRule(ctx, chainNode, zeroClient, current, cfg, rule)
	if err != nil {
		t.Fatalf("evaluateMemoryRule() error = %v", err)
	}
	if shouldScale {
		t.Error("should NOT scale when memory usage is zero")
	}
}

// TestScenario_ErrorHandling tests that errors are properly propagated
func TestScenario_ErrorHandling(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1000m"),
		},
	}

	cfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("500m"),
		Max: resource.MustParse("4000m"),
	}

	rule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleUp,
		UsagePercent: 80,
		StepPercent:  20,
	}

	// Test with error from stats client
	errClient := &mockStatsClient{cpuErr: context.DeadlineExceeded}

	_, _, err := r.evaluateCpuRule(ctx, chainNode, errClient, current, cfg, rule)
	if err == nil {
		t.Error("expected error from stats client to be propagated")
	}
}

// TestScenario_MissingResourceRequest tests handling of missing resource requests
func TestScenario_MissingResourceRequest(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	client := &mockStatsClient{cpuUsage: 0.5}

	// Empty requests
	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
	}

	cfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("500m"),
		Max: resource.MustParse("4000m"),
	}

	rule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleUp,
		UsagePercent: 80,
		StepPercent:  20,
	}

	_, _, err := r.evaluateCpuRule(ctx, chainNode, client, current, cfg, rule)
	if err == nil {
		t.Error("expected error when CPU request is missing")
	}
}

// TestScenario_SimultaneousCPUAndMemoryScaling tests that both CPU and memory can scale
func TestScenario_SimultaneousCPUAndMemoryScaling(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	// High usage for both CPU and memory
	client := &mockStatsClient{
		cpuUsage:    0.9,               // 900m = 90% of 1000m
		memoryUsage: 900 * 1024 * 1024, // 900Mi = ~88% of 1Gi
	}

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	cpuCfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("500m"),
		Max: resource.MustParse("4000m"),
	}

	memCfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("512Mi"),
		Max: resource.MustParse("4Gi"),
	}

	cpuRule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleUp,
		UsagePercent: 80,
		StepPercent:  25,
	}

	memRule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleUp,
		UsagePercent: 80,
		StepPercent:  25,
	}

	// Both should scale up
	cpuShouldScale, newCPU, err := r.evaluateCpuRule(ctx, chainNode, client, current, cpuCfg, cpuRule)
	if err != nil {
		t.Fatalf("evaluateCpuRule() error = %v", err)
	}
	if !cpuShouldScale {
		t.Error("CPU should scale up")
	}
	if newCPU.MilliValue() <= 1000 {
		t.Errorf("new CPU should be > 1000m, got %s", newCPU.String())
	}

	memShouldScale, newMem, err := r.evaluateMemoryRule(ctx, chainNode, client, current, memCfg, memRule)
	if err != nil {
		t.Fatalf("evaluateMemoryRule() error = %v", err)
	}
	if !memShouldScale {
		t.Error("Memory should scale up")
	}
	if newMem.Value() <= 1*1024*1024*1024 {
		t.Errorf("new memory should be > 1Gi, got %s", newMem.String())
	}
}

// TestScenario_LowUsageNoScaleUp tests that low usage doesn't trigger scale-up
func TestScenario_LowUsageNoScaleUp(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	// Low usage - well below threshold
	client := &mockStatsClient{
		cpuUsage:    0.3,               // 300m = 30% of 1000m
		memoryUsage: 300 * 1024 * 1024, // 300Mi = ~29% of 1Gi
	}

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	cfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("100m"),
		Max: resource.MustParse("4000m"),
	}

	scaleUpRule := &appsv1.VerticalAutoscalingRule{
		Direction:    appsv1.ScaleUp,
		UsagePercent: 80,
		StepPercent:  25,
	}

	// Should NOT scale up
	shouldScale, _, err := r.evaluateCpuRule(ctx, chainNode, client, current, cfg, scaleUpRule)
	if err != nil {
		t.Fatalf("evaluateCpuRule() error = %v", err)
	}
	if shouldScale {
		t.Error("CPU should NOT scale up at 30% usage with 80% threshold")
	}
}

// TestScenario_ExactlyAtThreshold tests behavior when usage is exactly at threshold
func TestScenario_ExactlyAtThreshold(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1000m"),
		},
	}

	cfg := &appsv1.VerticalAutoscalingMetricConfig{
		Min: resource.MustParse("100m"),
		Max: resource.MustParse("4000m"),
	}

	tests := []struct {
		name       string
		cpuUsage   float64
		direction  appsv1.ScalingDirection
		threshold  int
		hysteresis int
		wantScale  bool
	}{
		{
			name:       "exactly at scale-up threshold - should scale",
			cpuUsage:   0.8, // exactly 80%
			direction:  appsv1.ScaleUp,
			threshold:  80,
			hysteresis: 0,
			wantScale:  true,
		},
		{
			name:       "exactly at scale-down threshold - should scale",
			cpuUsage:   0.4, // exactly 40%
			direction:  appsv1.ScaleDown,
			threshold:  40,
			hysteresis: 0,
			wantScale:  true,
		},
		{
			name:       "exactly at scale-down threshold with hysteresis - should NOT scale",
			cpuUsage:   0.4, // 40%, but effective is 30% due to hysteresis
			direction:  appsv1.ScaleDown,
			threshold:  40,
			hysteresis: 10,
			wantScale:  false, // 40% > 30% effective threshold
		},
		{
			name:       "at effective threshold with hysteresis - should scale",
			cpuUsage:   0.3, // 30% = effective threshold (40 - 10)
			direction:  appsv1.ScaleDown,
			threshold:  40,
			hysteresis: 10,
			wantScale:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockStatsClient{cpuUsage: tt.cpuUsage}

			cfgWithHysteresis := &appsv1.VerticalAutoscalingMetricConfig{
				Min:               cfg.Min,
				Max:               cfg.Max,
				HysteresisPercent: ptr.To(tt.hysteresis),
			}

			rule := &appsv1.VerticalAutoscalingRule{
				Direction:    tt.direction,
				UsagePercent: tt.threshold,
				StepPercent:  20,
			}

			shouldScale, _, err := r.evaluateCpuRule(ctx, chainNode, client, current, cfgWithHysteresis, rule)
			if err != nil {
				t.Fatalf("evaluateCpuRule() error = %v", err)
			}

			if shouldScale != tt.wantScale {
				t.Errorf("shouldScale = %v, want %v", shouldScale, tt.wantScale)
			}
		})
	}
}

// TestScenario_SafetyMarginWithMinClamp tests interaction between safety margin and min clamp
func TestScenario_SafetyMarginWithMinClamp(t *testing.T) {
	r := newTestReconciler()
	ctx := context.Background()

	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	// Low usage that would trigger aggressive scale-down
	client := &mockStatsClient{
		cpuUsage: 0.15, // 150m = 15% of 1000m
	}

	current := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1000m"),
		},
	}

	tests := []struct {
		name         string
		minVal       string
		safetyMargin int
		stepPercent  int
		wantNewCPU   string // expected clamped value
		wantReason   string // "min" or "safety"
	}{
		{
			name:         "min value wins over safety margin",
			minVal:       "500m",
			safetyMargin: 20, // 150m * 1.2 = 180m, but min is 500m
			stepPercent:  80, // 1000m - 800m = 200m without clamp
			wantNewCPU:   "500m",
			wantReason:   "min",
		},
		{
			name:         "safety margin wins over calculated step",
			minVal:       "100m",
			safetyMargin: 50, // 150m * 1.5 = 225m
			stepPercent:  80, // 1000m - 800m = 200m
			wantNewCPU:   "225m",
			wantReason:   "safety",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &appsv1.VerticalAutoscalingMetricConfig{
				Min:                 resource.MustParse(tt.minVal),
				Max:                 resource.MustParse("4000m"),
				SafetyMarginPercent: ptr.To(tt.safetyMargin),
			}

			rule := &appsv1.VerticalAutoscalingRule{
				Direction:    appsv1.ScaleDown,
				UsagePercent: 40, // 15% < 40%, will trigger
				StepPercent:  tt.stepPercent,
			}

			shouldScale, newCPU, err := r.evaluateCpuRule(ctx, chainNode, client, current, cfg, rule)
			if err != nil {
				t.Fatalf("evaluateCpuRule() error = %v", err)
			}

			if !shouldScale {
				t.Error("should scale down")
			}

			wantVal := resource.MustParse(tt.wantNewCPU)
			if newCPU.MilliValue() != wantVal.MilliValue() {
				t.Errorf("newCPU = %s, want %s (reason: %s)", newCPU.String(), tt.wantNewCPU, tt.wantReason)
			}
		})
	}
}
