package chainnode

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// maxLocks defines the maximum number of locks to maintain in memory.
	// This prevents unbounded growth in long-running operators.
	// With typical Kubernetes deployments, this should be more than sufficient.
	maxLocks = 500
)

// lockManager manages locks for disruption control based on label sets.
// It implements a capacity-limited lock cache to prevent memory leaks.
type lockManager struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}

// newLockManager creates a new lock manager instance.
func newLockManager() *lockManager {
	return &lockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

func generateLockKey(l map[string]string) string {
	var keys []string
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort the keys to ensure deterministic order

	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteByte('=')
		builder.WriteString(l[k])
		builder.WriteByte(',')
	}
	return builder.String()
}

// getLockForLabels returns a mutex for the given label set.
// Creates a new mutex if one doesn't exist for this label set.
// If the maximum number of locks is reached, it returns an existing lock
// to prevent unbounded memory growth.
func (lm *lockManager) getLockForLabels(l map[string]string) *sync.Mutex {
	lockKey := generateLockKey(l)

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lock, exists := lm.locks[lockKey]; exists {
		return lock
	}

	// Enforce capacity limit to prevent unbounded growth
	if len(lm.locks) >= maxLocks {
		// Return any existing lock when at capacity
		// This maintains concurrency control while preventing memory leaks
		for _, existingLock := range lm.locks {
			return existingLock
		}
	}

	newLock := &sync.Mutex{}
	lm.locks[lockKey] = newLock
	return newLock
}

func (r *Reconciler) checkDisruptionAllowance(ctx context.Context, l map[string]string) error {
	logger := log.FromContext(ctx)

	podsList, err := r.listPodsWithLabels(ctx, l)
	if err != nil {
		return err
	}
	unavailable := unavailablePodCount(podsList)

	logger.V(1).Info("disruption check", "unavailable", unavailable, "labels", l)
	if unavailable >= r.opts.DisruptionMaxUnavailable {
		return fmt.Errorf("%d pods are unavailable", unavailable)
	}
	return nil
}

func (r *Reconciler) listPodsWithLabels(ctx context.Context, l map[string]string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	return podList, r.List(ctx, podList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(l),
	})
}

func unavailablePodCount(podList *corev1.PodList) int {
	unavailable := 0
	for _, pod := range podList.Items {
		if !isPodRunningAndReady(&pod) {
			unavailable++
		}
	}

	return unavailable
}

func isPodRunningAndReady(pod *corev1.Pod) bool {
	// Check if the pod's phase is "Running"
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Check the pod's "Ready" condition is "True"
	ready := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	if !ready {
		return false
	}

	// Check if all containers are ready
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready || containerStatus.State.Running == nil {
			return false
		}
	}

	// Check if node-utils and cosmoguard containers are ready
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		if containerStatus.Name == nodeUtilsContainerName || containerStatus.Name == cosmoGuardContainerName {
			if !containerStatus.Ready || containerStatus.State.Running == nil {
				return false
			}
		}
	}

	return true
}
