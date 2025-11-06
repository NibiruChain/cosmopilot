package chainnode

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateLockKey(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name: "single label",
			labels: map[string]string{
				"app": "test",
			},
			expected: "app=test,",
		},
		{
			name: "multiple labels sorted",
			labels: map[string]string{
				"zebra":   "last",
				"app":     "first",
				"version": "middle",
			},
			expected: "app=first,version=middle,zebra=last,",
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: "",
		},
		{
			name:     "nil labels",
			labels:   nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateLockKey(tt.labels)
			if result != tt.expected {
				t.Errorf("generateLockKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateLockKey_ConsistentForSameLabels(t *testing.T) {
	labels1 := map[string]string{"app": "test", "env": "prod"}
	labels2 := map[string]string{"env": "prod", "app": "test"} // Different order

	key1 := generateLockKey(labels1)
	key2 := generateLockKey(labels2)

	if key1 != key2 {
		t.Errorf("generateLockKey() should be consistent regardless of map order, got %q vs %q", key1, key2)
	}
}

func TestIsPodRunningAndReady(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "pod running and ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
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
			name: "pod running but not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
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
			name: "pod pending",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
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
			name: "pod failed",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			want: false,
		},
		{
			name: "pod succeeded",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			want: false,
		},
		{
			name: "pod with no ready condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      corev1.PodRunning,
					Conditions: []corev1.PodCondition{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodRunningAndReady(tt.pod); got != tt.want {
				t.Errorf("isPodRunningAndReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnavailablePodCount(t *testing.T) {
	tests := []struct {
		name    string
		podList *corev1.PodList
		want    int
	}{
		{
			name: "all pods running and ready",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{Type: corev1.PodReady, Status: corev1.ConditionTrue},
							},
						},
					},
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{Type: corev1.PodReady, Status: corev1.ConditionTrue},
							},
						},
					},
				},
			},
			want: 0,
		},
		{
			name: "one pod not ready",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{Type: corev1.PodReady, Status: corev1.ConditionTrue},
							},
						},
					},
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{Type: corev1.PodReady, Status: corev1.ConditionFalse},
							},
						},
					},
				},
			},
			want: 1,
		},
		{
			name: "all pods unavailable",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
						},
					},
					{
						Status: corev1.PodStatus{
							Phase: corev1.PodFailed,
						},
					},
				},
			},
			want: 2,
		},
		{
			name: "pod terminating but still ready is available",
			podList: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &metav1.Time{},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{Type: corev1.PodReady, Status: corev1.ConditionTrue},
							},
						},
					},
				},
			},
			want: 0, // Still counts as available since it's running and ready
		},
		{
			name:    "empty pod list",
			podList: &corev1.PodList{Items: []corev1.Pod{}},
			want:    0,
		},
		{
			name:    "nil pod list",
			podList: &corev1.PodList{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unavailablePodCount(tt.podList); got != tt.want {
				t.Errorf("unavailablePodCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewLockManager(t *testing.T) {
	lm := newLockManager()
	if lm == nil {
		t.Fatal("newLockManager() returned nil")
	}
	if lm.locks == nil {
		t.Error("newLockManager() did not initialize locks map")
	}
}

func TestLockManager_GetLockForLabels(t *testing.T) {
	lm := newLockManager()

	labels1 := map[string]string{"app": "test"}
	labels2 := map[string]string{"app": "test"}
	labels3 := map[string]string{"app": "other"}

	// Get lock for labels1
	lock1 := lm.getLockForLabels(labels1)
	if lock1 == nil {
		t.Fatal("getLockForLabels() returned nil for labels1")
	}

	// Get lock for labels2 (same labels as labels1)
	lock2 := lm.getLockForLabels(labels2)
	if lock2 == nil {
		t.Fatal("getLockForLabels() returned nil for labels2")
	}

	// Should return same lock instance for same labels
	if lock1 != lock2 {
		t.Error("getLockForLabels() returned different locks for same labels")
	}

	// Get lock for different labels
	lock3 := lm.getLockForLabels(labels3)
	if lock3 == nil {
		t.Fatal("getLockForLabels() returned nil for labels3")
	}

	// Should return different lock for different labels
	if lock1 == lock3 {
		t.Error("getLockForLabels() returned same lock for different labels")
	}
}
