package chainnode

import (
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name string
		val  int64
		min  int64
		max  int64
		want int64
	}{
		{
			name: "value within range",
			val:  5,
			min:  1,
			max:  10,
			want: 5,
		},
		{
			name: "value below min",
			val:  0,
			min:  1,
			max:  10,
			want: 1,
		},
		{
			name: "value above max",
			val:  15,
			min:  1,
			max:  10,
			want: 10,
		},
		{
			name: "value equals min",
			val:  1,
			min:  1,
			max:  10,
			want: 1,
		},
		{
			name: "value equals max",
			val:  10,
			min:  1,
			max:  10,
			want: 10,
		},
		{
			name: "negative values",
			val:  -5,
			min:  -10,
			max:  -1,
			want: -5,
		},
		{
			name: "negative value below min",
			val:  -15,
			min:  -10,
			max:  -1,
			want: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clamp(tt.val, tt.min, tt.max); got != tt.want {
				t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestWithinCooldown(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		last     time.Time
		cooldown time.Duration
		want     bool
	}{
		{
			name:     "within cooldown period",
			last:     now.Add(-1 * time.Minute),
			cooldown: 5 * time.Minute,
			want:     true,
		},
		{
			name:     "outside cooldown period",
			last:     now.Add(-10 * time.Minute),
			cooldown: 5 * time.Minute,
			want:     false,
		},
		{
			name:     "exactly at cooldown boundary",
			last:     now.Add(-5 * time.Minute),
			cooldown: 5 * time.Minute,
			want:     false,
		},
		{
			name:     "zero cooldown",
			last:     now.Add(-1 * time.Second),
			cooldown: 0,
			want:     false,
		},
		{
			name:     "very old timestamp",
			last:     now.Add(-24 * time.Hour),
			cooldown: 1 * time.Hour,
			want:     false,
		},
		{
			name:     "just inside cooldown",
			last:     now.Add(-4*time.Minute - 59*time.Second),
			cooldown: 5 * time.Minute,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withinCooldown(tt.last, tt.cooldown); got != tt.want {
				t.Errorf("withinCooldown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetScaleReason(t *testing.T) {
	tests := []struct {
		name      string
		direction appsv1.ScalingDirection
		want      string
	}{
		{
			name:      "scale up",
			direction: appsv1.ScaleUp,
			want:      appsv1.ReasonVPAScaleUp,
		},
		{
			name:      "scale down",
			direction: appsv1.ScaleDown,
			want:      appsv1.ReasonVPAScaleDown,
		},
		{
			name:      "unknown direction defaults to scale down",
			direction: appsv1.ScalingDirection("unknown"),
			want:      appsv1.ReasonVPAScaleDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getScaleReason(tt.direction); got != tt.want {
				t.Errorf("getScaleReason() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOOMKilled(t *testing.T) {
	tests := []struct {
		name          string
		pod           *corev1.Pod
		containerName string
		want          bool
	}{
		{
			name: "container was OOM killed",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
						},
					},
				},
			},
			containerName: "app",
			want:          true,
		},
		{
			name: "container was not OOM killed",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "Error",
								},
							},
						},
					},
				},
			},
			containerName: "app",
			want:          false,
		},
		{
			name: "container has no termination state",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
						},
					},
				},
			},
			containerName: "app",
			want:          false,
		},
		{
			name: "different container was OOM killed",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "sidecar",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
						},
					},
				},
			},
			containerName: "app",
			want:          false,
		},
		{
			name:          "empty pod",
			pod:           &corev1.Pod{},
			containerName: "app",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOOMKilled(tt.pod, tt.containerName); got != tt.want {
				t.Errorf("isOOMKilled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOOMRecoveryHistory(t *testing.T) {
	now := time.Now().UTC()
	ts1 := now.Add(-30 * time.Minute).Format(timeLayout)
	ts2 := now.Add(-15 * time.Minute).Format(timeLayout)

	tests := []struct {
		name      string
		chainNode *appsv1.ChainNode
		wantCount int
	}{
		{
			name: "no history annotation",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			wantCount: 0,
		},
		{
			name: "empty history",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: "[]",
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "history with two entries",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: mustMarshalJSON([]string{ts1, ts2}),
					},
				},
			},
			wantCount: 2,
		},
		{
			name: "invalid JSON returns empty",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: "invalid-json",
					},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getOOMRecoveryHistory(tt.chainNode)
			if len(got) != tt.wantCount {
				t.Errorf("getOOMRecoveryHistory() returned %d entries, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestCalculateLimitFromRequest(t *testing.T) {
	// Helper to create pointer to LimitUpdateStrategy
	strategyPtr := func(s appsv1.LimitUpdateStrategy) *appsv1.LimitUpdateStrategy {
		return &s
	}
	intPtr := func(i int) *int {
		return &i
	}

	tests := []struct {
		name         string
		chainNode    *appsv1.ChainNode
		request      resource.Quantity
		cfg          *appsv1.VerticalAutoscalingMetricConfig
		resourceName corev1.ResourceName
		wantNil      bool
		wantValue    string
	}{
		{
			name: "LimitEqual strategy - CPU",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("500m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitEqual),
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      false,
			wantValue:    "500m",
		},
		{
			name: "LimitEqual strategy - Memory",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("1Gi"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitEqual),
			},
			resourceName: corev1.ResourceMemory,
			wantNil:      false,
			wantValue:    "1Gi",
		},
		{
			name: "LimitVpaMax strategy - CPU",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("500m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitVpaMax),
				Max:           resource.MustParse("4000m"),
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      false,
			wantValue:    "4",
		},
		{
			name: "LimitVpaMax strategy - Memory",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("1Gi"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitVpaMax),
				Max:           resource.MustParse("8Gi"),
			},
			resourceName: corev1.ResourceMemory,
			wantNil:      false,
			wantValue:    "8Gi",
		},
		{
			name: "LimitPercentage strategy - CPU 150%",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("1000m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy:   strategyPtr(appsv1.LimitPercentage),
				LimitPercentage: intPtr(150),
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      false,
			wantValue:    "1500m",
		},
		{
			name: "LimitPercentage strategy - Memory 200%",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("512Mi"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy:   strategyPtr(appsv1.LimitPercentage),
				LimitPercentage: intPtr(200),
			},
			resourceName: corev1.ResourceMemory,
			wantNil:      false,
			wantValue:    "1Gi",
		},
		{
			name: "LimitPercentage strategy - uses default 150% when not specified",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("1000m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitPercentage),
				// LimitPercentage not set, should use default 150
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      false,
			wantValue:    "1500m",
		},
		{
			name: "LimitUnset strategy returns nil",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{},
			},
			request: resource.MustParse("500m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitUnset),
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      true,
		},
		{
			name: "LimitRetain strategy - has limits in spec",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2000m"),
						},
					},
				},
			},
			request: resource.MustParse("500m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitRetain),
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      false,
			wantValue:    "2",
		},
		{
			name: "LimitRetain strategy - no limits in spec returns nil",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
				},
			},
			request: resource.MustParse("500m"),
			cfg: &appsv1.VerticalAutoscalingMetricConfig{
				LimitStrategy: strategyPtr(appsv1.LimitRetain),
			},
			resourceName: corev1.ResourceCPU,
			wantNil:      true,
		},
		{
			name: "Default strategy (nil) uses LimitRetain",
			chainNode: &appsv1.ChainNode{
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			},
			request: resource.MustParse("1Gi"),
			cfg:     &appsv1.VerticalAutoscalingMetricConfig{
				// LimitStrategy not set, should default to LimitRetain
			},
			resourceName: corev1.ResourceMemory,
			wantNil:      false,
			wantValue:    "4Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateLimitFromRequest(tt.chainNode, tt.request, tt.cfg, tt.resourceName)
			if tt.wantNil {
				if got != nil {
					t.Errorf("calculateLimitFromRequest() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("calculateLimitFromRequest() = nil, want %s", tt.wantValue)
				return
			}
			if got.String() != tt.wantValue {
				t.Errorf("calculateLimitFromRequest() = %s, want %s", got.String(), tt.wantValue)
			}
		})
	}
}

func TestGetVpaLastAppliedResourcesOrFallback(t *testing.T) {
	tests := []struct {
		name            string
		chainNode       *appsv1.ChainNode
		wantCPURequest  string
		wantMemRequest  string
		wantCPULimit    string
		wantMemLimit    string
		wantHasCPULimit bool
		wantHasMemLimit bool
	}{
		{
			name: "no VPA annotation - returns spec resources",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
			wantCPURequest:  "500m",
			wantMemRequest:  "1Gi",
			wantCPULimit:    "1",
			wantMemLimit:    "2Gi",
			wantHasCPULimit: true,
			wantHasMemLimit: true,
		},
		{
			name: "has VPA annotation - returns VPA resources merged with spec",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAResources: `{"requests":{"cpu":"750m","memory":"1536Mi"},"limits":{"cpu":"1500m","memory":"3Gi"}}`,
					},
				},
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			wantCPURequest:  "750m",
			wantMemRequest:  "1536Mi",
			wantCPULimit:    "1500m",
			wantMemLimit:    "3Gi",
			wantHasCPULimit: true,
			wantHasMemLimit: true,
		},
		{
			name: "invalid VPA annotation JSON - falls back to spec",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAResources: "invalid-json",
					},
				},
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			wantCPURequest:  "500m",
			wantMemRequest:  "1Gi",
			wantHasCPULimit: false,
			wantHasMemLimit: false,
		},
		{
			name: "VPA annotation with only CPU - preserves spec memory",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAResources: `{"requests":{"cpu":"750m"}}`,
					},
				},
				Spec: appsv1.ChainNodeSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			wantCPURequest:  "750m",
			wantMemRequest:  "1Gi",
			wantHasCPULimit: false,
			wantHasMemLimit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVpaLastAppliedResourcesOrFallback(tt.chainNode)

			// Check CPU request
			if cpu, ok := got.Requests[corev1.ResourceCPU]; ok {
				if cpu.String() != tt.wantCPURequest {
					t.Errorf("CPU request = %s, want %s", cpu.String(), tt.wantCPURequest)
				}
			} else if tt.wantCPURequest != "" {
				t.Errorf("CPU request not found, want %s", tt.wantCPURequest)
			}

			// Check Memory request
			if mem, ok := got.Requests[corev1.ResourceMemory]; ok {
				if mem.String() != tt.wantMemRequest {
					t.Errorf("Memory request = %s, want %s", mem.String(), tt.wantMemRequest)
				}
			} else if tt.wantMemRequest != "" {
				t.Errorf("Memory request not found, want %s", tt.wantMemRequest)
			}

			// Check CPU limit
			if cpu, ok := got.Limits[corev1.ResourceCPU]; ok {
				if !tt.wantHasCPULimit {
					t.Errorf("CPU limit found but not expected")
				} else if cpu.String() != tt.wantCPULimit {
					t.Errorf("CPU limit = %s, want %s", cpu.String(), tt.wantCPULimit)
				}
			} else if tt.wantHasCPULimit {
				t.Errorf("CPU limit not found, want %s", tt.wantCPULimit)
			}

			// Check Memory limit
			if mem, ok := got.Limits[corev1.ResourceMemory]; ok {
				if !tt.wantHasMemLimit {
					t.Errorf("Memory limit found but not expected")
				} else if mem.String() != tt.wantMemLimit {
					t.Errorf("Memory limit = %s, want %s", mem.String(), tt.wantMemLimit)
				}
			} else if tt.wantHasMemLimit {
				t.Errorf("Memory limit not found, want %s", tt.wantMemLimit)
			}
		})
	}
}

func TestClampEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		val  int64
		min  int64
		max  int64
		want int64
	}{
		{
			name: "zero value within range",
			val:  0,
			min:  -10,
			max:  10,
			want: 0,
		},
		{
			name: "min equals max",
			val:  5,
			min:  10,
			max:  10,
			want: 10,
		},
		{
			name: "very large values",
			val:  1 << 40,
			min:  1 << 30,
			max:  1 << 50,
			want: 1 << 40,
		},
		{
			name: "max less than min returns min (edge case)",
			val:  5,
			min:  20,
			max:  10,
			want: 20, // val < min, returns min
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clamp(tt.val, tt.min, tt.max); got != tt.want {
				t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestWithinCooldownEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		last     time.Time
		cooldown time.Duration
		want     bool
	}{
		{
			name:     "zero time (epoch) - outside cooldown",
			last:     time.Time{},
			cooldown: 5 * time.Minute,
			want:     false,
		},
		{
			name:     "future timestamp - within cooldown",
			last:     time.Now().Add(1 * time.Hour),
			cooldown: 5 * time.Minute,
			want:     true,
		},
		{
			name:     "negative cooldown",
			last:     time.Now().Add(-1 * time.Second),
			cooldown: -5 * time.Minute,
			want:     false,
		},
		{
			name:     "very long cooldown",
			last:     time.Now().Add(-23 * time.Hour),
			cooldown: 24 * time.Hour,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withinCooldown(tt.last, tt.cooldown); got != tt.want {
				t.Errorf("withinCooldown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOOMKilledEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		pod           *corev1.Pod
		containerName string
		want          bool
	}{
		{
			name: "multiple containers - only target is OOM",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "sidecar1",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "Error",
								},
							},
						},
						{
							Name: "app",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
						},
						{
							Name: "sidecar2",
						},
					},
				},
			},
			containerName: "app",
			want:          true,
		},
		{
			name: "container has current terminated state but not last",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
							// LastTerminationState is empty
						},
					},
				},
			},
			containerName: "app",
			want:          false, // We only check LastTerminationState
		},
		{
			name: "OOMKilled with different case - should not match",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "oomkilled", // lowercase
								},
							},
						},
					},
				},
			},
			containerName: "app",
			want:          false,
		},
		{
			name: "container name with special characters",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "my-app-container",
							LastTerminationState: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "OOMKilled",
								},
							},
						},
					},
				},
			},
			containerName: "my-app-container",
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOOMKilled(tt.pod, tt.containerName); got != tt.want {
				t.Errorf("isOOMKilled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOOMRecoveryHistoryEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		chainNode  *appsv1.ChainNode
		wantCount  int
		wantRecent bool // whether the most recent entry is within the last hour
	}{
		{
			name: "nil annotations",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			wantCount: 0,
		},
		{
			name: "empty string annotation",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: "",
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "array with invalid timestamps",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: `["invalid-timestamp", "also-invalid"]`,
					},
				},
			},
			wantCount: 0, // Invalid timestamps are skipped
		},
		{
			name: "mixed valid and invalid timestamps",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: mustMarshalJSON([]string{
							time.Now().UTC().Format(timeLayout),
							"invalid",
							time.Now().Add(-30 * time.Minute).UTC().Format(timeLayout),
						}),
					},
				},
			},
			wantCount: 2, // Only valid timestamps are returned
		},
		{
			name: "timestamps from different timezones",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						controllers.AnnotationVPAOOMRecoveryHistory: mustMarshalJSON([]string{
							time.Now().UTC().Format(timeLayout),
						}),
					},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getOOMRecoveryHistory(tt.chainNode)
			if len(got) != tt.wantCount {
				t.Errorf("getOOMRecoveryHistory() returned %d entries, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestOneMiBConstant(t *testing.T) {
	// Verify OneMiB is correctly defined
	expectedMiB := int64(1024 * 1024)
	if OneMiB != expectedMiB {
		t.Errorf("OneMiB = %d, want %d", OneMiB, expectedMiB)
	}
}

func TestMemoryRoundingToMiB(t *testing.T) {
	tests := []struct {
		name      string
		bytes     int64
		wantBytes int64
	}{
		{
			name:      "exactly 1 MiB",
			bytes:     1024 * 1024,
			wantBytes: 1024 * 1024,
		},
		{
			name:      "1 byte over MiB boundary rounds up",
			bytes:     1024*1024 + 1,
			wantBytes: 2 * 1024 * 1024,
		},
		{
			name:      "1 byte under MiB boundary rounds up",
			bytes:     1024*1024 - 1,
			wantBytes: 1024 * 1024,
		},
		{
			name:      "exactly 512 MiB",
			bytes:     512 * 1024 * 1024,
			wantBytes: 512 * 1024 * 1024,
		},
		{
			name:      "1.5 MiB rounds up to 2 MiB",
			bytes:     int64(1.5 * 1024 * 1024),
			wantBytes: 2 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the rounding formula used in VPA
			rounded := ((tt.bytes + OneMiB - 1) / OneMiB) * OneMiB
			if rounded != tt.wantBytes {
				t.Errorf("rounded %d bytes = %d, want %d", tt.bytes, rounded, tt.wantBytes)
			}
		})
	}
}

func TestHysteresisThresholdCalculation(t *testing.T) {
	tests := []struct {
		name                string
		usagePercent        int
		hysteresisPercent   int
		wantEffectiveThresh int
	}{
		{
			name:                "no hysteresis",
			usagePercent:        50,
			hysteresisPercent:   0,
			wantEffectiveThresh: 50,
		},
		{
			name:                "5% hysteresis on 50% threshold",
			usagePercent:        50,
			hysteresisPercent:   5,
			wantEffectiveThresh: 45,
		},
		{
			name:                "10% hysteresis on 50% threshold",
			usagePercent:        50,
			hysteresisPercent:   10,
			wantEffectiveThresh: 40,
		},
		{
			name:                "hysteresis larger than threshold - clamps to 0",
			usagePercent:        10,
			hysteresisPercent:   20,
			wantEffectiveThresh: 0,
		},
		{
			name:                "hysteresis equal to threshold",
			usagePercent:        15,
			hysteresisPercent:   15,
			wantEffectiveThresh: 0,
		},
		{
			name:                "large threshold with small hysteresis",
			usagePercent:        80,
			hysteresisPercent:   5,
			wantEffectiveThresh: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate hysteresis calculation from evaluateCpuRule/evaluateMemoryRule
			effectiveThreshold := tt.usagePercent - tt.hysteresisPercent
			if effectiveThreshold < 0 {
				effectiveThreshold = 0
			}
			if effectiveThreshold != tt.wantEffectiveThresh {
				t.Errorf("effective threshold = %d, want %d", effectiveThreshold, tt.wantEffectiveThresh)
			}
		})
	}
}

func TestSafetyMarginCalculation(t *testing.T) {
	tests := []struct {
		name                string
		currentUsageBytes   int64
		safetyMarginPercent int
		wantMinSafeBytes    int64
	}{
		{
			name:                "15% safety margin on 1GB usage",
			currentUsageBytes:   1024 * 1024 * 1024, // 1GB
			safetyMarginPercent: 15,
			wantMinSafeBytes:    1073741824 * 115 / 100, // 1GB * 1.15 = 1234803097
		},
		{
			name:                "0% safety margin",
			currentUsageBytes:   1024 * 1024 * 1024,
			safetyMarginPercent: 0,
			wantMinSafeBytes:    1024 * 1024 * 1024,
		},
		{
			name:                "100% safety margin doubles the value",
			currentUsageBytes:   512 * 1024 * 1024, // 512MB
			safetyMarginPercent: 100,
			wantMinSafeBytes:    1024 * 1024 * 1024, // 1GB
		},
		{
			name:                "25% safety margin",
			currentUsageBytes:   800 * 1024 * 1024, // 800MB
			safetyMarginPercent: 25,
			wantMinSafeBytes:    1000 * 1024 * 1024, // 1000MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate safety margin calculation from evaluateMemoryRule
			minSafeVal := int64(float64(tt.currentUsageBytes) * float64(100+tt.safetyMarginPercent) / 100.0)
			if minSafeVal != tt.wantMinSafeBytes {
				t.Errorf("minSafeVal = %d, want %d", minSafeVal, tt.wantMinSafeBytes)
			}
		})
	}
}

func TestStepCalculation(t *testing.T) {
	tests := []struct {
		name           string
		currentRequest int64 // millicores for CPU
		stepPercent    int
		direction      appsv1.ScalingDirection
		wantNewValue   int64
	}{
		{
			name:           "scale up 20% from 1000m",
			currentRequest: 1000,
			stepPercent:    20,
			direction:      appsv1.ScaleUp,
			wantNewValue:   1200,
		},
		{
			name:           "scale down 20% from 1000m",
			currentRequest: 1000,
			stepPercent:    20,
			direction:      appsv1.ScaleDown,
			wantNewValue:   800,
		},
		{
			name:           "scale up 50% from 500m",
			currentRequest: 500,
			stepPercent:    50,
			direction:      appsv1.ScaleUp,
			wantNewValue:   750,
		},
		{
			name:           "scale down 10% from 2000m",
			currentRequest: 2000,
			stepPercent:    10,
			direction:      appsv1.ScaleDown,
			wantNewValue:   1800,
		},
		{
			name:           "scale up 100% doubles the value",
			currentRequest: 1000,
			stepPercent:    100,
			direction:      appsv1.ScaleUp,
			wantNewValue:   2000,
		},
		{
			name:           "small step from small request",
			currentRequest: 100,
			stepPercent:    5,
			direction:      appsv1.ScaleUp,
			wantNewValue:   105,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate step calculation from evaluateCpuRule
			step := tt.currentRequest * int64(tt.stepPercent) / 100
			var newVal int64
			if tt.direction == appsv1.ScaleUp {
				newVal = tt.currentRequest + step
			} else {
				newVal = tt.currentRequest - step
			}
			if newVal != tt.wantNewValue {
				t.Errorf("newVal = %d, want %d", newVal, tt.wantNewValue)
			}
		})
	}
}

func TestOOMRecoveryRateLimiting(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name           string
		history        []time.Time
		recoveryWindow time.Duration
		maxRecoveries  int
		wantUnderLimit bool
	}{
		{
			name:           "empty history - under limit",
			history:        []time.Time{},
			recoveryWindow: 1 * time.Hour,
			maxRecoveries:  3,
			wantUnderLimit: true,
		},
		{
			name: "2 recoveries in window - under limit of 3",
			history: []time.Time{
				now.Add(-30 * time.Minute),
				now.Add(-15 * time.Minute),
			},
			recoveryWindow: 1 * time.Hour,
			maxRecoveries:  3,
			wantUnderLimit: true,
		},
		{
			name: "3 recoveries in window - at limit",
			history: []time.Time{
				now.Add(-45 * time.Minute),
				now.Add(-30 * time.Minute),
				now.Add(-15 * time.Minute),
			},
			recoveryWindow: 1 * time.Hour,
			maxRecoveries:  3,
			wantUnderLimit: false,
		},
		{
			name: "old recoveries outside window - under limit",
			history: []time.Time{
				now.Add(-2 * time.Hour),
				now.Add(-90 * time.Minute),
				now.Add(-75 * time.Minute),
			},
			recoveryWindow: 1 * time.Hour,
			maxRecoveries:  3,
			wantUnderLimit: true, // All outside window
		},
		{
			name: "mixed old and recent - counts only recent",
			history: []time.Time{
				now.Add(-2 * time.Hour),    // outside
				now.Add(-30 * time.Minute), // inside
				now.Add(-15 * time.Minute), // inside
			},
			recoveryWindow: 1 * time.Hour,
			maxRecoveries:  3,
			wantUnderLimit: true, // Only 2 inside window
		},
		{
			name: "exactly at window boundary - outside",
			history: []time.Time{
				now.Add(-1 * time.Hour), // exactly at boundary
			},
			recoveryWindow: 1 * time.Hour,
			maxRecoveries:  1,
			wantUnderLimit: true, // At boundary is considered outside
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate rate limiting logic from handleOOMRecovery
			cutoff := now.Add(-tt.recoveryWindow)
			recentRecoveries := 0
			for _, ts := range tt.history {
				if ts.After(cutoff) {
					recentRecoveries++
				}
			}
			underLimit := recentRecoveries < tt.maxRecoveries
			if underLimit != tt.wantUnderLimit {
				t.Errorf("underLimit = %v (recent=%d), want %v", underLimit, recentRecoveries, tt.wantUnderLimit)
			}
		})
	}
}

func TestEmergencyScaleUpCalculation(t *testing.T) {
	tests := []struct {
		name                    string
		currentRequestBytes     int64
		emergencyScaleUpPercent int
		maxBytes                int64
		wantNewBytes            int64
	}{
		{
			name:                    "25% emergency scale up from 1GB",
			currentRequestBytes:     1024 * 1024 * 1024,
			emergencyScaleUpPercent: 25,
			maxBytes:                8 * 1024 * 1024 * 1024,
			wantNewBytes:            int64(1.25 * 1024 * 1024 * 1024),
		},
		{
			name:                    "50% emergency scale up",
			currentRequestBytes:     512 * 1024 * 1024,
			emergencyScaleUpPercent: 50,
			maxBytes:                8 * 1024 * 1024 * 1024,
			wantNewBytes:            768 * 1024 * 1024,
		},
		{
			name:                    "scale up clamped to max",
			currentRequestBytes:     7 * 1024 * 1024 * 1024,
			emergencyScaleUpPercent: 50,
			maxBytes:                8 * 1024 * 1024 * 1024,
			wantNewBytes:            8 * 1024 * 1024 * 1024, // Clamped to max
		},
		{
			name:                    "already at max - stays at max",
			currentRequestBytes:     8 * 1024 * 1024 * 1024,
			emergencyScaleUpPercent: 25,
			maxBytes:                8 * 1024 * 1024 * 1024,
			wantNewBytes:            8 * 1024 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate emergency scale up from handleOOMRecovery
			step := int64(float64(tt.currentRequestBytes) * float64(tt.emergencyScaleUpPercent) / 100.0)
			newVal := tt.currentRequestBytes + step
			// Clamp to max
			if newVal > tt.maxBytes {
				newVal = tt.maxBytes
			}
			if newVal != tt.wantNewBytes {
				t.Errorf("newVal = %d, want %d", newVal, tt.wantNewBytes)
			}
		})
	}
}

func TestGetLastScaleTime(t *testing.T) {
	now := time.Now().UTC()
	creationTime := now.Add(-1 * time.Hour)
	scaleTime := now.Add(-30 * time.Minute)

	tests := []struct {
		name      string
		chainNode *appsv1.ChainNode
		getFunc   func(*appsv1.ChainNode) time.Time
		wantTime  time.Time
	}{
		{
			name: "CPU scale time from annotation",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: creationTime},
					Annotations: map[string]string{
						controllers.AnnotationVPALastCPUScale: scaleTime.Format(timeLayout),
					},
				},
			},
			getFunc:  getLastCpuScaleTime,
			wantTime: scaleTime,
		},
		{
			name: "Memory scale time from annotation",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: creationTime},
					Annotations: map[string]string{
						controllers.AnnotationVPALastMemoryScale: scaleTime.Format(timeLayout),
					},
				},
			},
			getFunc:  getLastMemoryScaleTime,
			wantTime: scaleTime,
		},
		{
			name: "no annotation - falls back to creation time",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: creationTime},
					Annotations:       map[string]string{},
				},
			},
			getFunc:  getLastCpuScaleTime,
			wantTime: creationTime,
		},
		{
			name: "invalid annotation format - falls back to creation time",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: creationTime},
					Annotations: map[string]string{
						controllers.AnnotationVPALastCPUScale: "invalid-time-format",
					},
				},
			},
			getFunc:  getLastCpuScaleTime,
			wantTime: creationTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.getFunc(tt.chainNode)
			// Compare within 1 second tolerance due to time formatting
			if got.Sub(tt.wantTime).Abs() > time.Second {
				t.Errorf("getLastScaleTime() = %v, want %v", got, tt.wantTime)
			}
		})
	}
}

func TestUsagePercentageCalculation(t *testing.T) {
	tests := []struct {
		name              string
		actualUsage       float64 // in cores for CPU
		requestMillicores int64
		wantPercent       int
	}{
		{
			name:              "50% usage",
			actualUsage:       0.5,
			requestMillicores: 1000,
			wantPercent:       50,
		},
		{
			name:              "100% usage",
			actualUsage:       1.0,
			requestMillicores: 1000,
			wantPercent:       100,
		},
		{
			name:              "150% usage (over request)",
			actualUsage:       1.5,
			requestMillicores: 1000,
			wantPercent:       150,
		},
		{
			name:              "10% usage",
			actualUsage:       0.1,
			requestMillicores: 1000,
			wantPercent:       10,
		},
		{
			name:              "0% usage",
			actualUsage:       0,
			requestMillicores: 1000,
			wantPercent:       0,
		},
		{
			name:              "small request with proportional usage",
			actualUsage:       0.05,
			requestMillicores: 100,
			wantPercent:       50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate usage percentage calculation from evaluateCpuRule
			usedPercent := int((tt.actualUsage * 1000 / float64(tt.requestMillicores)) * 100)
			if usedPercent != tt.wantPercent {
				t.Errorf("usedPercent = %d, want %d", usedPercent, tt.wantPercent)
			}
		})
	}
}

func TestHandleOOMRecoveryLogic(t *testing.T) {
	tests := []struct {
		name             string
		currentMemory    string
		minMemory        string
		maxMemory        string
		emergencyPercent int
		oomHistory       []time.Time
		maxRecoveries    int
		recoveryWindow   time.Duration
		wantScaled       bool
		wantNewMemoryMin int64 // minimum expected (0 if no scaling)
		wantRateLimited  bool
	}{
		{
			name:             "successful emergency scale-up 25%",
			currentMemory:    "1Gi",
			minMemory:        "512Mi",
			maxMemory:        "4Gi",
			emergencyPercent: 25,
			oomHistory:       nil,
			maxRecoveries:    3,
			recoveryWindow:   1 * time.Hour,
			wantScaled:       true,
			wantNewMemoryMin: int64(1.25 * 1024 * 1024 * 1024), // 1.25Gi
		},
		{
			name:             "scale-up clamped to max",
			currentMemory:    "3Gi",
			minMemory:        "512Mi",
			maxMemory:        "4Gi",
			emergencyPercent: 50, // Would be 4.5Gi without clamp
			oomHistory:       nil,
			maxRecoveries:    3,
			recoveryWindow:   1 * time.Hour,
			wantScaled:       true,
			wantNewMemoryMin: 4 * 1024 * 1024 * 1024, // clamped to 4Gi
		},
		{
			name:             "rate limited - max recoveries reached",
			currentMemory:    "1Gi",
			minMemory:        "512Mi",
			maxMemory:        "4Gi",
			emergencyPercent: 25,
			oomHistory: []time.Time{
				time.Now().Add(-10 * time.Minute),
				time.Now().Add(-20 * time.Minute),
				time.Now().Add(-30 * time.Minute),
			},
			maxRecoveries:   3,
			recoveryWindow:  1 * time.Hour,
			wantScaled:      false,
			wantRateLimited: true,
		},
		{
			name:             "old recoveries outside window don't count",
			currentMemory:    "1Gi",
			minMemory:        "512Mi",
			maxMemory:        "4Gi",
			emergencyPercent: 25,
			oomHistory: []time.Time{
				time.Now().Add(-2 * time.Hour),    // outside 1h window
				time.Now().Add(-3 * time.Hour),    // outside 1h window
				time.Now().Add(-10 * time.Minute), // inside window
			},
			maxRecoveries:    3,
			recoveryWindow:   1 * time.Hour,
			wantScaled:       true,
			wantNewMemoryMin: int64(1.25 * 1024 * 1024 * 1024),
		},
		{
			name:             "already at max - no scaling possible",
			currentMemory:    "4Gi",
			minMemory:        "512Mi",
			maxMemory:        "4Gi",
			emergencyPercent: 25,
			oomHistory:       nil,
			maxRecoveries:    3,
			recoveryWindow:   1 * time.Hour,
			wantScaled:       true, // Will scale but clamp to max (same value)
			wantNewMemoryMin: 4 * 1024 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the handleOOMRecovery logic
			currentRequest := resource.MustParse(tt.currentMemory)
			minVal := resource.MustParse(tt.minMemory)
			maxVal := resource.MustParse(tt.maxMemory)

			// Count recent recoveries
			cutoff := time.Now().Add(-tt.recoveryWindow)
			recentRecoveries := 0
			for _, ts := range tt.oomHistory {
				if ts.After(cutoff) {
					recentRecoveries++
				}
			}

			// Check rate limit
			if recentRecoveries >= tt.maxRecoveries {
				if !tt.wantRateLimited {
					t.Errorf("unexpected rate limiting: recentRecoveries=%d, maxRecoveries=%d", recentRecoveries, tt.maxRecoveries)
				}
				return
			}

			if tt.wantRateLimited {
				t.Errorf("expected rate limiting but got none: recentRecoveries=%d, maxRecoveries=%d", recentRecoveries, tt.maxRecoveries)
				return
			}

			// Calculate emergency scale-up
			currentBytes := currentRequest.Value()
			step := int64(float64(currentBytes) * float64(tt.emergencyPercent) / 100.0)
			newVal := currentBytes + step

			// Clamp
			newVal = clamp(newVal, minVal.Value(), maxVal.Value())

			// Round up to nearest MiB
			rounded := ((newVal + OneMiB - 1) / OneMiB) * OneMiB

			if !tt.wantScaled {
				t.Errorf("expected no scaling but calculation proceeded")
				return
			}

			if rounded < tt.wantNewMemoryMin {
				t.Errorf("newMemory = %d, want >= %d", rounded, tt.wantNewMemoryMin)
			}
		})
	}
}

func TestOOMHistoryPruning(t *testing.T) {
	tests := []struct {
		name           string
		history        []time.Time
		recoveryWindow time.Duration
		wantKept       int
	}{
		{
			name:           "all within window",
			history:        []time.Time{time.Now().Add(-10 * time.Minute), time.Now().Add(-20 * time.Minute)},
			recoveryWindow: 1 * time.Hour,
			wantKept:       2,
		},
		{
			name:           "all outside window",
			history:        []time.Time{time.Now().Add(-2 * time.Hour), time.Now().Add(-3 * time.Hour)},
			recoveryWindow: 1 * time.Hour,
			wantKept:       0,
		},
		{
			name: "mixed",
			history: []time.Time{
				time.Now().Add(-10 * time.Minute), // kept
				time.Now().Add(-2 * time.Hour),    // pruned
				time.Now().Add(-30 * time.Minute), // kept
			},
			recoveryWindow: 1 * time.Hour,
			wantKept:       2,
		},
		{
			name:           "empty history",
			history:        nil,
			recoveryWindow: 1 * time.Hour,
			wantKept:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cutoff := time.Now().Add(-tt.recoveryWindow)
			kept := 0
			for _, ts := range tt.history {
				if ts.After(cutoff) {
					kept++
				}
			}
			if kept != tt.wantKept {
				t.Errorf("kept = %d, want %d", kept, tt.wantKept)
			}
		})
	}
}

func TestOOMRecoveryHistoryAnnotationFormat(t *testing.T) {
	// Test that the annotation format is correct JSON array of RFC3339 timestamps
	timestamps := []time.Time{
		time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		time.Date(2024, 1, 15, 11, 45, 0, 0, time.UTC),
	}

	var formatted []string
	for _, ts := range timestamps {
		formatted = append(formatted, ts.UTC().Format(timeLayout))
	}

	data, err := json.Marshal(formatted)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify it's valid JSON
	var parsed []string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed) != len(timestamps) {
		t.Errorf("parsed length = %d, want %d", len(parsed), len(timestamps))
	}

	// Verify timestamps can be parsed back
	for i, ts := range parsed {
		parsedTime, err := time.Parse(timeLayout, ts)
		if err != nil {
			t.Errorf("failed to parse timestamp %d: %v", i, err)
		}
		if !parsedTime.Equal(timestamps[i]) {
			t.Errorf("timestamp %d: got %v, want %v", i, parsedTime, timestamps[i])
		}
	}
}

func TestEmergencyScaleUpRounding(t *testing.T) {
	// Verify that emergency scale-up results are rounded to MiB
	tests := []struct {
		name          string
		currentBytes  int64
		scalePercent  int
		wantRoundedUp bool // result should be >= calculated value and multiple of MiB
	}{
		{
			name:          "1Gi + 25% = 1.25Gi rounds to MiB",
			currentBytes:  1 * 1024 * 1024 * 1024,
			scalePercent:  25,
			wantRoundedUp: true,
		},
		{
			name:          "1.5Gi + 10% rounds to MiB",
			currentBytes:  int64(1.5 * 1024 * 1024 * 1024),
			scalePercent:  10,
			wantRoundedUp: true,
		},
		{
			name:          "exact MiB stays same",
			currentBytes:  512 * 1024 * 1024,
			scalePercent:  100, // doubles to 1Gi exactly
			wantRoundedUp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := int64(float64(tt.currentBytes) * float64(tt.scalePercent) / 100.0)
			newVal := tt.currentBytes + step

			// Round up to nearest MiB
			rounded := ((newVal + OneMiB - 1) / OneMiB) * OneMiB

			// Verify it's a multiple of MiB
			if rounded%OneMiB != 0 {
				t.Errorf("rounded value %d is not a multiple of MiB", rounded)
			}

			// Verify it's >= the unrounded value
			if rounded < newVal {
				t.Errorf("rounded %d < unrounded %d", rounded, newVal)
			}
		})
	}
}

func TestVPAScaleDownEdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		currentRequest    int64 // millicores
		usageMillicores   int64
		stepPercent       int
		safetyMargin      int
		minVal            int64
		wantNewVal        int64
		wantBlockedBySafe bool
	}{
		{
			name:            "normal scale-down",
			currentRequest:  2000,
			usageMillicores: 400, // 20%
			stepPercent:     25,
			safetyMargin:    15,
			minVal:          500,
			wantNewVal:      1500, // 2000 - 500
		},
		{
			name:              "blocked by safety margin",
			currentRequest:    1000,
			usageMillicores:   600, // 60%
			stepPercent:       50,  // would go to 500m
			safetyMargin:      20,  // min safe = 600 * 1.2 = 720m
			minVal:            200,
			wantNewVal:        720,
			wantBlockedBySafe: true,
		},
		{
			name:            "blocked by min value",
			currentRequest:  600,
			usageMillicores: 100,
			stepPercent:     50,  // would go to 300m
			safetyMargin:    10,  // min safe = 110m
			minVal:          500, // min is higher
			wantNewVal:      500,
		},
		{
			name:              "safety margin higher than min",
			currentRequest:    1000,
			usageMillicores:   700,
			stepPercent:       50, // would go to 500m
			safetyMargin:      50, // min safe = 700 * 1.5 = 1050m > current!
			minVal:            200,
			wantNewVal:        1000, // clamped to current (can't go higher on scale-down)
			wantBlockedBySafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := tt.currentRequest * int64(tt.stepPercent) / 100
			newVal := tt.currentRequest - step

			// Apply safety margin
			minSafeVal := tt.usageMillicores * int64(100+tt.safetyMargin) / 100
			blockedBySafety := false
			if newVal < minSafeVal {
				newVal = minSafeVal
				blockedBySafety = true
			}

			// Clamp to min (and max = current for scale-down)
			newVal = clamp(newVal, tt.minVal, tt.currentRequest)

			if newVal != tt.wantNewVal {
				t.Errorf("newVal = %d, want %d", newVal, tt.wantNewVal)
			}

			if blockedBySafety != tt.wantBlockedBySafe {
				t.Errorf("blockedBySafety = %v, want %v", blockedBySafety, tt.wantBlockedBySafe)
			}
		})
	}
}

func TestHysteresisEdgeCases(t *testing.T) {
	tests := []struct {
		name                string
		configuredThreshold int
		hysteresisPercent   int
		wantEffective       int
	}{
		{
			name:                "normal hysteresis",
			configuredThreshold: 50,
			hysteresisPercent:   10,
			wantEffective:       40,
		},
		{
			name:                "hysteresis would make negative - clamp to 0",
			configuredThreshold: 5,
			hysteresisPercent:   10,
			wantEffective:       0,
		},
		{
			name:                "zero hysteresis",
			configuredThreshold: 50,
			hysteresisPercent:   0,
			wantEffective:       50,
		},
		{
			name:                "100% threshold with hysteresis",
			configuredThreshold: 100,
			hysteresisPercent:   20,
			wantEffective:       80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effective := tt.configuredThreshold - tt.hysteresisPercent
			if effective < 0 {
				effective = 0
			}
			if effective != tt.wantEffective {
				t.Errorf("effective = %d, want %d", effective, tt.wantEffective)
			}
		})
	}
}
