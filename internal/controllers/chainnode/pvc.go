package chainnode

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/pkg/nodeutils"
)

func (r *Reconciler) ensurePersistence(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	pvc, err := r.ensurePvc(ctx, chainNode)
	if err != nil {
		return err
	}

	if pvc.Annotations[annotationDataInitialized] != "true" {
		logger.Info("initializing data on pvc and updating status")
		if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeInitData); err != nil {
			return err
		}

		initCommands := make([]*chainutils.InitCommand, len(chainNode.GetPersistenceInitCommands()))
		for i, c := range chainNode.GetPersistenceInitCommands() {
			initCommands[i] = &chainutils.InitCommand{Args: c.Args, Command: c.Command}
			if c.Image != nil {
				initCommands[i].Image = *c.Image
			} else {
				initCommands[i].Image = chainNode.Spec.App.GetImage()
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

	if chainNode.Status.PvcSize != pvc.Spec.Resources.Requests.Storage().String() {
		chainNode.Status.PvcSize = pvc.Spec.Resources.Requests.Storage().String()
		if err = r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonPvcResized,
			"Data volume was resized to %v", chainNode.Status.PvcSize,
		)
	}

	return nil
}

func (r *Reconciler) ensurePvc(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.PersistentVolumeClaim, error) {
	logger := log.FromContext(ctx)

	storageSize, err := r.getStorageSize(ctx, chainNode)
	if err != nil {
		return nil, err
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pvc", "size", storageSize)
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.GetName(),
					Namespace: chainNode.GetNamespace(),
					Annotations: map[string]string{
						annotationDataInitialized: strconv.FormatBool(false),
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
			return pvc, r.Create(ctx, pvc)
		}
		return nil, err
	}

	if pvc.Spec.Resources.Requests.Storage().Equal(storageSize) {
		return pvc, nil
	}

	pvc.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceStorage: storageSize,
	}

	logger.Info("updating pvc size", "size", storageSize)
	return pvc, r.Update(ctx, pvc)
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

	max, err := resource.ParseQuantity(chainNode.GetPersistenceAutoResizeMaxSize())
	if err != nil {
		return resource.Quantity{}, err
	}

	newSize := currentSize.DeepCopy()
	newSize.Add(increment)

	if newSize.Cmp(max) == 1 {
		logger.Info("pvc reached maximum size")
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonPvcMaxReached,
			"Data volume reached maximum allowed size (%v)", max.String(),
		)
		return max, nil
	}

	return newSize, nil
}
