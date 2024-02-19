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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/pkg/nodeutils"
)

func (r *Reconciler) ensurePersistence(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	pvc, err := r.ensurePvc(ctx, chainNode)
	if err != nil {
		return err
	}

	if pvc.Annotations[annotationDataInitialized] != "true" {
		return r.initializeData(ctx, app, chainNode, pvc)
	}

	return nil
}

func (r *Reconciler) initializeData(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode, pvc *corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	logger.Info("initializing data", "pvc", pvc.GetName())
	if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeInitData); err != nil {
		return err
	}

	initCommands := make([]*chainutils.InitCommand, len(chainNode.GetPersistenceInitCommands()))
	for i, c := range chainNode.GetPersistenceInitCommands() {
		initCommands[i] = &chainutils.InitCommand{Args: c.Args, Command: c.Command}
		if c.Image != nil {
			initCommands[i].Image = *c.Image
		} else {
			initCommands[i].Image = chainNode.GetAppImage()
		}
	}

	if err := app.InitPvcData(ctx, pvc, initCommands...); err != nil {
		return err
	}
	// Get the updated PVC for updating annotation
	if err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc); err != nil {
		return err
	}
	pvc.Annotations[annotationDataInitialized] = strconv.FormatBool(true)
	if err := r.Update(ctx, pvc); err != nil {
		return err
	}
	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonDataInitialized,
		"Data volume was successfully initialized",
	)
	chainNode.Status.PvcSize = pvc.Spec.Resources.Requests.Storage().String()
	return r.Status().Update(ctx, chainNode)
}

func (r *Reconciler) ensurePvc(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.PersistentVolumeClaim, error) {
	logger := log.FromContext(ctx)

	storageSize, err := r.getStorageSize(ctx, chainNode)
	if err != nil {
		return nil, err
	}

	if chainNode.ShouldRestoreFromSnapshot() {
		snapshot := &snapshotv1.VolumeSnapshot{}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: chainNode.GetNamespace(),
			Name:      chainNode.Spec.Persistence.RestoreFromSnapshot.Name,
		}, snapshot)
		if err != nil {
			return nil, err
		}
		if snapshot.Status.RestoreSize != nil {
			storageSize = *snapshot.Status.RestoreSize
		} else {
			logger.Info("could not grab restore size from snapshot. Falling back to .persistence.size", "size", storageSize)
		}

		// Get height from the snapshot so that operator knows which version to run in case there were upgrades
		if hs, ok := snapshot.Annotations[annotationDataHeight]; ok {
			height, err := strconv.ParseInt(hs, 10, 64)
			if err != nil {
				return nil, err
			}
			chainNode.Status.LatestHeight = height
			if err := r.Status().Update(ctx, chainNode); err != nil {
				return nil, err
			}
		}
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pvc", "pvc", pvc.GetName(), "size", storageSize)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.GetName(),
					Namespace: chainNode.GetNamespace(),
					Labels:    WithChainNodeLabels(chainNode),
					Annotations: map[string]string{
						annotationDataInitialized: strconv.FormatBool(chainNode.ShouldRestoreFromSnapshot()),
						annotationDataHeight:      strconv.FormatInt(chainNode.Status.LatestHeight, 10),
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: storageSize,
						},
					},
					StorageClassName: chainNode.GetPersistenceStorageClass(),
				},
			}
			if chainNode.ShouldRestoreFromSnapshot() {
				pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{
					APIGroup: pointer.String(VolumeSnapshotDataSourceApiGroup),
					Kind:     VolumeSnapshotDataSourceKind,
					Name:     chainNode.Spec.Persistence.RestoreFromSnapshot.Name,
				}
			}
			if err := r.Create(ctx, pvc); err != nil {
				return nil, err
			}
			chainNode.Status.PvcSize = storageSize.String()
			return pvc, r.Status().Update(ctx, chainNode)
		}
		return nil, err
	}

	// This happens when a chainnode is created but the volume for it already exists. We try to get the
	// block height for the data on that volume, so that operator will know which version to run this
	// node with.
	if chainNode.Status.PvcSize == "" {
		if dataHeight, ok := pvc.Annotations[annotationDataHeight]; ok {
			height, err := strconv.ParseInt(dataHeight, 10, 64)
			if err != nil {
				return nil, err
			}
			if chainNode.Status.LatestHeight != height {
				chainNode.Status.LatestHeight = height
				if err := r.Status().Update(ctx, chainNode); err != nil {
					return nil, err
				}
			}
		}
	}

	if chainNode.Status.Phase == appsv1.PhaseChainNodeRunning || chainNode.Status.Phase == appsv1.PhaseChainNodeSyncing {
		if err := r.updateLatestHeight(ctx, chainNode); err != nil {
			return nil, err
		}
		dataHeight := strconv.FormatInt(chainNode.Status.LatestHeight, 10)
		if pvc.Annotations[annotationDataHeight] != dataHeight {
			pvc.Annotations[annotationDataHeight] = dataHeight
			if err := r.Update(ctx, pvc); err != nil {
				return nil, err
			}
		}
	}

	switch pvc.Spec.Resources.Requests.Storage().Cmp(storageSize) {
	case -1:
		logger.Info("resizing pvc", "pvc", pvc.GetName(), "old-size", pvc.Spec.Resources.Requests.Storage(), "new-size", storageSize)
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: storageSize,
		}
		if err := r.Update(ctx, pvc); err != nil {
			return nil, err
		}
		chainNode.Status.PvcSize = storageSize.String()
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonPvcResized,
			"Data volume was resized to %v", chainNode.Status.PvcSize,
		)
		return pvc, r.Status().Update(ctx, chainNode)

	case 1:
		logger.Info("skipping pvc resize: new-size < old-size", "pvc", pvc.GetName(), "old-size", pvc.Spec.Resources.Requests.Storage(), "new-size", storageSize)
		return pvc, nil

	default:
		if chainNode.Status.PvcSize != storageSize.String() {
			chainNode.Status.PvcSize = storageSize.String()
			return pvc, r.Status().Update(ctx, chainNode)
		}
		return pvc, nil
	}
}

func (r *Reconciler) getStorageSize(ctx context.Context, chainNode *appsv1.ChainNode) (resource.Quantity, error) {
	logger := log.FromContext(ctx)

	specSize, err := resource.ParseQuantity(chainNode.GetPersistenceSize())
	if err != nil {
		return resource.Quantity{}, err
	}

	// No PVC is available yet. Let's create it with .spec.persistence.size
	if chainNode.Status.PvcSize == "" {
		return specSize, nil
	}

	// Get current size of data
	dataSizeBytes, err := nodeutils.NewClient(chainNode.GetNodeFQDN()).GetDataSize()
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
			if err := r.Status().Update(ctx, chainNode); err != nil {
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
		if err := r.Status().Update(ctx, chainNode); err != nil {
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
