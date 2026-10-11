package chainnode

import (
	"context"
	"fmt"
	"strconv"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/chainutils"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
)

// initializeData manages the data initialization process in a non-blocking way.
// It monitors the init pod status and returns a Result indicating whether to requeue.
// This approach preserves progress if the controller restarts during initialization.
func (r *Reconciler) initializeData(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode, pvc *corev1.PersistentVolumeClaim) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	initPodName := fmt.Sprintf("%s-init-data", chainNode.GetName())

	// Check if an init pod already exists
	existingPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      initPodName,
		Namespace: chainNode.GetNamespace(),
	}, existingPod)

	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get init pod: %w", err)
	}

	podExists := err == nil

	if podExists {
		switch existingPod.Status.Phase {
		case corev1.PodSucceeded:
			// Init completed successfully - mark PVC as initialized and clean up
			logger.Info("init pod completed successfully", "pod", initPodName)
			return r.markDataInitialized(ctx, chainNode, pvc, existingPod)

		case corev1.PodFailed:
			// Init failed - delete pod and retry
			logger.Info("init pod failed, will retry", "pod", initPodName, "reason", getPodFailureReason(existingPod))
			r.recorder.Eventf(chainNode,
				corev1.EventTypeWarning,
				appsv1.ReasonDataInitFailed,
				"Data initialization pod failed: %s", getPodFailureReason(existingPod),
			)
			if err := r.Delete(ctx, existingPod); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to delete failed init pod: %w", err)
			}
			// Requeue to create a new pod after short delay
			return ctrl.Result{RequeueAfter: initDataRetryPeriod}, nil

		case corev1.PodRunning, corev1.PodPending:
			// Init is in progress - update phase and requeue to check later
			logger.Info("init pod still running", "pod", initPodName, "phase", existingPod.Status.Phase)
			if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeInitData); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: initDataCheckPeriod}, nil

		default:
			// Unknown phase, log and requeue
			logger.Info("init pod in unknown phase", "pod", initPodName, "phase", existingPod.Status.Phase)
			return ctrl.Result{RequeueAfter: initDataUnknownPhasePeriod}, nil
		}
	}

	// No pod exists (or was deleted due to failure) - create one
	logger.Info("creating init pod", "pvc", pvc.GetName())
	if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeInitData); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update phase to InitData: %w", err)
	}

	initCommands := r.buildInitCommands(chainNode)
	additionalVolumes := r.buildAdditionalVolumes(chainNode)
	initTimeout := chainNode.GetPersistenceInitTimeout()

	if err := app.CreateInitPod(ctx, pvc, initTimeout, additionalVolumes, initCommands...); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create init pod: %w", err)
	}

	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonDataInitStarted,
		"Data initialization started",
	)

	// Requeue to monitor the pod
	return ctrl.Result{RequeueAfter: initDataCheckPeriod}, nil
}

// markDataInitialized marks the PVC as initialized and cleans up the init pod.
func (r *Reconciler) markDataInitialized(ctx context.Context, chainNode *appsv1.ChainNode, pvc *corev1.PersistentVolumeClaim, initPod *corev1.Pod) (ctrl.Result, error) {
	// Clean up the init pod
	if err := r.Delete(ctx, initPod); err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to delete completed init pod: %w", err)
	}

	// Get the updated PVC for updating annotation
	if err := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get PVC %s after initialization: %w", pvc.GetName(), err)
	}

	// Mark PVC as initialized
	pvc.Annotations[controllers.AnnotationDataInitialized] = controllers.StringValueTrue
	if err := r.Update(ctx, pvc); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update PVC %s with initialized annotation: %w", pvc.GetName(), err)
	}

	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonDataInitialized,
		"Data volume was successfully initialized",
	)

	chainNode.Status.PvcSize = pvc.Spec.Resources.Requests.Storage().String()
	if err := r.Status().Update(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue immediately to continue reconciliation
	return ctrl.Result{Requeue: true}, nil
}

// buildInitCommands constructs the init commands from the ChainNode spec.
func (r *Reconciler) buildInitCommands(chainNode *appsv1.ChainNode) []*chainutils.InitCommand {
	initCommands := make([]*chainutils.InitCommand, len(chainNode.GetPersistenceInitCommands()))
	for i, c := range chainNode.GetPersistenceInitCommands() {
		initCommands[i] = &chainutils.InitCommand{
			Args:      c.Args,
			Command:   c.Command,
			Resources: c.Resources,
			Env:       c.Env,
		}
		if c.Image != nil {
			initCommands[i].Image = *c.Image
		} else {
			initCommands[i].Image = chainNode.GetAppImage()
		}
	}
	return initCommands
}

// buildAdditionalVolumes constructs the additional volumes from the ChainNode spec.
func (r *Reconciler) buildAdditionalVolumes(chainNode *appsv1.ChainNode) []chainutils.AdditionalVolume {
	additionalVolumes := make([]chainutils.AdditionalVolume, len(chainNode.GetPersistenceAdditionalVolumes()))
	for i, vol := range chainNode.GetPersistenceAdditionalVolumes() {
		additionalVolumes[i] = chainutils.AdditionalVolume{
			Name:    vol.Name,
			PVCName: fmt.Sprintf("%s-%s", chainNode.GetName(), vol.Name),
			Path:    vol.Path,
		}
	}
	return additionalVolumes
}

// getPodFailureReason extracts a human-readable failure reason from a failed pod.
func getPodFailureReason(pod *corev1.Pod) string {
	// Check container statuses for termination reason
	for _, cs := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			if cs.State.Terminated.Reason != "" {
				return fmt.Sprintf("%s: %s", cs.Name, cs.State.Terminated.Reason)
			}
			return fmt.Sprintf("%s: exit code %d", cs.Name, cs.State.Terminated.ExitCode)
		}
	}
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	return "unknown"
}

func (r *Reconciler) ensureDataVolume(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) (*corev1.PersistentVolumeClaim, ctrl.Result, error) {
	logger := log.FromContext(ctx)

	pvc, err := r.getPVC(ctx, chainNode)
	if err != nil {
		return nil, ctrl.Result{}, err
	}

	// If PVC does not exist
	if pvc == nil {
		// Assume .spec size by default
		storageSize, err := resource.ParseQuantity(chainNode.GetPersistenceSize())
		if err != nil {
			return nil, ctrl.Result{}, err
		}

		if chainNode.ShouldRestoreFromSnapshot() {
			snapshot := &snapshotv1.VolumeSnapshot{}
			err = r.Get(ctx, types.NamespacedName{
				Namespace: chainNode.GetNamespace(),
				Name:      chainNode.Spec.Persistence.RestoreFromSnapshot.Name,
			}, snapshot)
			if err != nil {
				return nil, ctrl.Result{}, err
			}
			if snapshot.Status.RestoreSize != nil {
				storageSize = *snapshot.Status.RestoreSize
			} else {
				logger.Info("could not grab restore size from snapshot. Falling back to .persistence.size", "size", storageSize)
			}

			// Get height from the snapshot so that operator knows which version to run in case there were upgrades already.
			if hs, ok := snapshot.Annotations[controllers.AnnotationDataHeight]; ok {
				height, err := strconv.ParseInt(hs, 10, 64)
				if err != nil {
					return nil, ctrl.Result{}, err
				}
				chainNode.Status.LatestHeight = height
				if err = r.Status().Update(ctx, chainNode); err != nil {
					return nil, ctrl.Result{}, err
				}
			}
		} else {
			// In case the PVC was deleted on an existing node, lets set latest height to 0 to make sure state-sync
			// configuration can be applied if necessary.
			if chainNode.Status.LatestHeight != 0 {
				chainNode.Status.LatestHeight = 0
				if err = r.Status().Update(ctx, chainNode); err != nil {
					return nil, ctrl.Result{}, err
				}
			}
		}

		logger.Info("creating pvc", "pvc", chainNode.GetName(), "size", storageSize)

		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      chainNode.GetName(),
				Namespace: chainNode.GetNamespace(),
				Labels:    WithChainNodeLabels(chainNode),
				Annotations: map[string]string{
					controllers.AnnotationDataInitialized: strconv.FormatBool(chainNode.ShouldRestoreFromSnapshot()),
					controllers.AnnotationDataHeight:      strconv.FormatInt(chainNode.Status.LatestHeight, 10),
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageSize,
					},
				},
				StorageClassName: chainNode.GetPersistenceStorageClass(),
			},
		}

		if chainNode.ShouldRestoreFromSnapshot() {
			pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To(VolumeSnapshotDataSourceApiGroup),
				Kind:     VolumeSnapshotDataSourceKind,
				Name:     chainNode.Spec.Persistence.RestoreFromSnapshot.Name,
			}
		}

		if err = r.Create(ctx, pvc); err != nil {
			return nil, ctrl.Result{}, err
		}

		chainNode.Status.PvcSize = storageSize.String()
		if err = r.Status().Update(ctx, chainNode); err != nil {
			return nil, ctrl.Result{}, err
		}

	} else {
		// This happens when a chainnode is created but the volume for it already exists. We try to get the
		// block height for the data on that volume, so that operator will know which version to run this
		// node with.
		if chainNode.Status.PvcSize == "" {
			if dataHeight, ok := pvc.Annotations[controllers.AnnotationDataHeight]; ok {
				height, err := strconv.ParseInt(dataHeight, 10, 64)
				if err != nil {
					return nil, ctrl.Result{}, err
				}
				if chainNode.Status.LatestHeight != height {
					chainNode.Status.LatestHeight = height
					chainNode.Status.PvcSize = pvc.Spec.Resources.Requests.Storage().String()
					if err = r.Status().Update(ctx, chainNode); err != nil {
						return nil, ctrl.Result{}, err
					}
				}
			}
		}
	}

	if pvc.Annotations[controllers.AnnotationDataInitialized] != controllers.StringValueTrue {
		result, err := r.initializeData(ctx, app, chainNode, pvc)
		return pvc, result, err
	}
	return pvc, ctrl.Result{}, nil
}

func (r *Reconciler) ensurePvcUpdates(ctx context.Context, chainNode *appsv1.ChainNode, pvc *corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	expectedStorageSize, err := r.getStorageSize(ctx, chainNode)
	if err != nil {
		return fmt.Errorf("failed to get storage size for %s: %w", chainNode.GetName(), err)
	}

	if err = r.updateLatestHeight(ctx, chainNode); err != nil {
		return fmt.Errorf("failed to update latest height for %s: %w", chainNode.GetName(), err)
	}

	dataHeight := strconv.FormatInt(chainNode.Status.LatestHeight, 10)
	if pvc.Annotations[controllers.AnnotationDataHeight] != dataHeight {
		pvc.Annotations[controllers.AnnotationDataHeight] = dataHeight
		if err = r.Update(ctx, pvc); err != nil {
			return fmt.Errorf("failed to update PVC %s data height annotation: %w", pvc.GetName(), err)
		}
	}

	switch pvc.Spec.Resources.Requests.Storage().Cmp(expectedStorageSize) {
	case -1:
		logger.Info("resizing pvc", "pvc", pvc.GetName(), "old-size", pvc.Spec.Resources.Requests.Storage(), "new-size", expectedStorageSize)
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: expectedStorageSize,
		}
		if err = r.Update(ctx, pvc); err != nil {
			return fmt.Errorf("failed to resize PVC %s: %w", pvc.GetName(), err)
		}
		chainNode.Status.PvcSize = expectedStorageSize.String()
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonPvcResized,
			"Data volume was resized to %v", chainNode.Status.PvcSize,
		)
		return r.Status().Update(ctx, chainNode)

	case 1:
		logger.Info("skipping pvc resize: new-size < old-size", "pvc", pvc.GetName(), "old-size", pvc.Spec.Resources.Requests.Storage(), "new-size", expectedStorageSize)
		return nil

	default:
		if chainNode.Status.PvcSize != expectedStorageSize.String() {
			chainNode.Status.PvcSize = expectedStorageSize.String()
			return r.Status().Update(ctx, chainNode)
		}
		return nil
	}
}

func (r *Reconciler) getStorageSize(ctx context.Context, chainNode *appsv1.ChainNode) (resource.Quantity, error) {
	logger := log.FromContext(ctx)

	specSize, err := resource.ParseQuantity(chainNode.GetPersistenceSize())
	if err != nil {
		return resource.Quantity{}, err
	}

	// Get current size of data
	dataSizeBytes, err := nodeutils.NewClient(chainNode.GetNodeFQDN()).GetDataSize(ctx)
	if err != nil {
		return resource.Quantity{}, err
	}

	// If auto-resize is disabled, we should also just return .spec.persistence.size, but we can also update data usage.
	if !chainNode.GetPersistenceAutoResizeEnabled() {
		sizeBytes, ok := specSize.AsInt64()
		if !ok {
			return resource.Quantity{}, fmt.Errorf("could not convert quantity to bytes")
		}

		dataUsage := int(float64(dataSizeBytes) / float64(sizeBytes) * 100.0)
		dataUsageStr := fmt.Sprintf("%d%%", dataUsage)
		if chainNode.Status.DataUsage != dataUsageStr {
			logger.Info("updating .status.dataUsage", "usage", dataUsageStr)
			chainNode.Status.DataUsage = dataUsageStr
			if err = r.Status().Update(ctx, chainNode); err != nil {
				return resource.Quantity{}, err
			}
		}
		return specSize, nil
	}

	// Get current size of PVC
	currentSize, err := resource.ParseQuantity(chainNode.Status.PvcSize)
	if err != nil {
		return resource.Quantity{}, err
	}
	currentSizeBytes, ok := currentSize.AsInt64()
	if !ok {
		return resource.Quantity{}, fmt.Errorf("could not convert quantity to bytes")
	}

	dataUsage := int(float64(dataSizeBytes) / float64(currentSizeBytes) * 100.0)
	dataUsageStr := fmt.Sprintf("%d%%", dataUsage)
	if chainNode.Status.DataUsage != dataUsageStr {
		logger.Info("updating .status.dataUsage", "usage", dataUsageStr)
		chainNode.Status.DataUsage = dataUsageStr
		if err = r.Status().Update(ctx, chainNode); err != nil {
			return resource.Quantity{}, err
		}
	}

	// If we are below threshold, lets just return current size
	if dataUsage <= chainNode.GetPersistenceAutoResizeThreshold() {
		return currentSize, nil
	}

	// We need to increase pvc size
	logger.Info("incrementing pvc size", "usage", dataUsageStr)

	increment, err := resource.ParseQuantity(chainNode.GetPersistenceAutoResizeIncrement())
	if err != nil {
		return resource.Quantity{}, err
	}

	maxSize, err := resource.ParseQuantity(chainNode.GetPersistenceAutoResizeMaxSize())
	if err != nil {
		return resource.Quantity{}, err
	}

	newSize := currentSize.DeepCopy()
	newSize.Add(increment)

	if newSize.Cmp(maxSize) == 1 {
		logger.Info("pvc reached maximum size", "current-size", currentSize, "max-size", maxSize)
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonPvcMaxReached,
			"Data volume reached maximum allowed size (%v)", maxSize.String(),
		)
		return maxSize, nil
	}

	return newSize, nil
}

func (r *Reconciler) getPVC(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pvc, nil
}

func (r *Reconciler) ensureAdditionalVolumes(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	additionalVolumes := chainNode.GetPersistenceAdditionalVolumes()
	if len(additionalVolumes) == 0 {
		return nil
	}

	for _, volume := range additionalVolumes {
		volumeName := fmt.Sprintf("%s-%s", chainNode.GetName(), volume.Name)
		specSize, err := resource.ParseQuantity(volume.Size)
		if err != nil {
			return fmt.Errorf("failed to parse volume size %s for volume %s: %w", volume.Size, volume.Name, err)
		}

		pvc := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{Namespace: chainNode.GetNamespace(), Name: volumeName}, pvc)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("creating pvc", "name", volumeName, "size", volume.Size)
				pvc = &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      volumeName,
						Namespace: chainNode.GetNamespace(),
						Labels:    WithChainNodeLabels(chainNode),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: specSize,
							},
						},
						StorageClassName: volume.GetStorageClass(chainNode.GetPersistenceStorageClass()),
					},
				}

				if volume.ShouldDeleteWithNode() {
					if err = controllerutil.SetControllerReference(chainNode, pvc, r.Scheme); err != nil {
						return fmt.Errorf("failed to set controller reference for PVC %s: %w", volumeName, err)
					}
				}

				if err = r.Create(ctx, pvc); err != nil {
					return fmt.Errorf("failed to create PVC %s: %w", volumeName, err)
				}
			} else {
				return fmt.Errorf("failed to get PVC %s: %w", volumeName, err)
			}
		} else {
			if pvc.Spec.Resources.Requests[corev1.ResourceStorage] != specSize {
				logger.Info("updating pvc", "name", volumeName, "old-size", pvc.Spec.Resources.Requests[corev1.ResourceStorage], "new-size", volume.Size)
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = specSize
				if err = r.Update(ctx, pvc); err != nil {
					return fmt.Errorf("failed to update PVC %s size: %w", volumeName, err)
				}
			}
		}
	}
	return nil
}
