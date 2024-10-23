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

var (
	locks      = make(map[string]*sync.Mutex)
	locksMutex sync.Mutex
)

func generateLockKey(l map[string]string) string {
	var keys []string
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort the keys to ensure deterministic order

	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(fmt.Sprintf("%s=%s,", k, l[k]))
	}
	return builder.String()
}

func getLockForLabels(l map[string]string) *sync.Mutex {
	lockKey := generateLockKey(l)

	locksMutex.Lock()
	defer locksMutex.Unlock()

	if lock, exists := locks[lockKey]; exists {
		return lock
	}

	newLock := &sync.Mutex{}
	locks[lockKey] = newLock
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
