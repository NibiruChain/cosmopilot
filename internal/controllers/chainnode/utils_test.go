package chainnode

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
)

func TestWithChainNodeLabels(t *testing.T) {
	tests := []struct {
		name       string
		chainNode  *appsv1.ChainNode
		additional []map[string]string
		wantKeys   []string
		excludeKey string
	}{
		{
			name: "basic labels without worker-name",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "myapp",
						"version": "v1",
					},
				},
			},
			additional: nil,
			wantKeys:   []string{"app", "version"},
			excludeKey: controllers.LabelWorkerName,
		},
		{
			name: "excludes worker-name label",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                       "myapp",
						controllers.LabelWorkerName: "worker-1",
						"version":                   "v1",
					},
				},
			},
			additional: nil,
			wantKeys:   []string{"app", "version"},
			excludeKey: controllers.LabelWorkerName,
		},
		{
			name: "merges additional labels",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			additional: []map[string]string{
				{"environment": "prod"},
				{"region": "us-west"},
			},
			wantKeys:   []string{"app", "environment", "region"},
			excludeKey: controllers.LabelWorkerName,
		},
		{
			name: "empty chainnode labels",
			chainNode: &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			additional: []map[string]string{
				{"custom": "label"},
			},
			wantKeys:   []string{"custom"},
			excludeKey: controllers.LabelWorkerName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WithChainNodeLabels(tt.chainNode, tt.additional...)

			// Verify worker-name is excluded
			if _, exists := result[tt.excludeKey]; exists {
				t.Errorf("WithChainNodeLabels() should exclude %s label", tt.excludeKey)
			}

			// Verify expected keys are present
			for _, key := range tt.wantKeys {
				if _, exists := result[key]; !exists {
					t.Errorf("WithChainNodeLabels() missing expected key: %s", key)
				}
			}
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "pod is ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod is not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "no ready condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "empty conditions",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			want: false,
		},
		{
			name: "multiple conditions, ready is true",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodInitialized,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "multiple conditions, ready is false",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "ready condition unknown",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionUnknown,
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodReady(tt.pod); got != tt.want {
				t.Errorf("IsPodReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
