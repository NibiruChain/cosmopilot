package chainnode

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
)

func (r *Reconciler) ensurePersistence(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	pvc, err := r.ensurePvc(ctx, chainNode)
	if err != nil {
		return err
	}

	if chainNode.Status.PvcSize == "" {
		logger.Info("initializing data on pvc and updating status")
		if err := r.updatePhase(ctx, chainNode, appsv1.PhaseInitData); err != nil {
			return err
		}
		if err := app.InitPvcData(ctx, pvc); err != nil {
			return err
		}
		chainNode.Status.PvcSize = pvc.Status.Capacity.Storage().String()
		return r.Status().Update(ctx, chainNode)
	}

	return nil
}

func (r *Reconciler) ensurePvc(ctx context.Context, chainNode *appsv1.ChainNode) (*corev1.PersistentVolumeClaim, error) {
	logger := log.FromContext(ctx)

	storageSize, err := resource.ParseQuantity(chainNode.GetPersistenceSize())
	if err != nil {
		return nil, err
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating pvc", "size", storageSize)
			pvc = &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.GetName(),
					Namespace: chainNode.GetNamespace(),
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
