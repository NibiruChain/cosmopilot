package chainnode

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
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

func TestGetSourceQuantity(t *testing.T) {
	tests := []struct {
		name      string
		current   corev1.ResourceRequirements
		source    appsv1.LimitSource
		resName   corev1.ResourceName
		want      string
		wantError bool
	}{
		{
			name: "get CPU request",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			source:    appsv1.Requests,
			resName:   corev1.ResourceCPU,
			want:      "100m",
			wantError: false,
		},
		{
			name: "get memory limit",
			current: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			source:    appsv1.Limits,
			resName:   corev1.ResourceMemory,
			want:      "256Mi",
			wantError: false,
		},
		{
			name: "resource not found in request",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
			},
			source:    appsv1.Requests,
			resName:   corev1.ResourceCPU,
			wantError: true,
		},
		{
			name: "resource not found in limit",
			current: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{},
			},
			source:    appsv1.Limits,
			resName:   corev1.ResourceMemory,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSourceQuantity(tt.current, tt.source, tt.resName)
			if (err != nil) != tt.wantError {
				t.Errorf("getSourceQuantity() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got.String() != tt.want {
				t.Errorf("getSourceQuantity() = %v, want %v", got.String(), tt.want)
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
