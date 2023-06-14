package chainnode

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/cometbft"
)

func (r *Reconciler) ensureNodeKey(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), secret)
	mustCreate := false
	if err != nil {
		if errors.IsNotFound(err) {
			mustCreate = true
			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.Name,
					Namespace: chainNode.Namespace,
				},
				Data: make(map[string][]byte),
			}
			if err := controllerutil.SetControllerReference(chainNode, secret, r.Scheme); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Ensure node key
	var nodeID string
	mustUpdate := false
	if _, ok := secret.Data[nodeKeyFilename]; !ok {
		if !mustCreate {
			mustUpdate = true
		}
		var key []byte
		nodeID, key, err = cometbft.GenerateNodeKey()
		if err != nil {
			return err
		}
		secret.Data[nodeKeyFilename] = key
	} else {
		nodeID, err = cometbft.GetNodeID(secret.Data[nodeKeyFilename])
		if err != nil {
			return err
		}
		if chainNode.Status.NodeID == "" {
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonNodeKeyImported,
				"Node key imported from Secret",
			)
		}
	}

	if mustCreate {
		logger.Info("creating secret with node-key")
		if err := r.Create(ctx, secret); err != nil {
			return err
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonNodeKeyCreated,
			"Node key created",
		)
	} else if mustUpdate {
		logger.Info("updating secret with node-key")
		if err := r.Update(ctx, secret); err != nil {
			return err
		}
	}

	// update nodeID in status if required
	if chainNode.Status.NodeID != nodeID {
		chainNode.Status.NodeID = nodeID
		if err := r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) ensureSigningKey(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: chainNode.GetNamespace(),
		Name:      chainNode.GetValidatorPrivKeySecretName(),
	}, secret)

	mustCreate := false
	if err != nil {
		if errors.IsNotFound(err) {
			mustCreate = true
			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.GetValidatorPrivKeySecretName(),
					Namespace: chainNode.GetNamespace(),
				},
				Data: make(map[string][]byte),
			}
		} else {
			return err
		}
	}

	// Ensure private key
	mustUpdate := false
	if _, ok := secret.Data[privKeyFilename]; !ok {
		if !mustCreate {
			mustUpdate = true
		}
		key, err := cometbft.GeneratePrivKey()
		if err != nil {
			return err
		}
		secret.Data[privKeyFilename] = key
	} else if !chainNode.Status.Validator {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonPrivateKeyImported,
			"Private key imported from Secret",
		)
	}

	if mustCreate {
		logger.Info("creating secret with priv-key")
		if err := r.Create(ctx, secret); err != nil {
			return err
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonPrivateKeyCreated,
			"Private key created for validating",
		)
	} else if mustUpdate {
		logger.Info("updating secret with priv-key")
		if err := r.Update(ctx, secret); err != nil {
			return err
		}
	}

	// update validator in status if required
	if !chainNode.Status.Validator {
		chainNode.Status.Validator = true
		if err := r.Status().Update(ctx, chainNode); err != nil {
			return err
		}
	}
	return nil
}
