package chainnodeset

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

func (r *Reconciler) ensureGenesis(ctx context.Context, app *chainutils.App, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)
	// If we already have a chainID, then we have a genesis already. However, it might have been created
	// by ChainNode controller for validator node, which now owns the configmap containing the genesis.
	// Here, we will move that ownership to the ChainNodeSet instead.
	if nodeSet.Status.ChainID != "" {
		if nodeSet.Spec.Genesis.ShouldUseDataVolume() {
			return nil
		}

		cm := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      nodeSet.Spec.Genesis.GetConfigMapName(nodeSet.Status.ChainID),
			Namespace: nodeSet.GetNamespace(),
		}, cm); err != nil {
			return err
		}

		// If configmap has no owner, lets not touch it.
		if cm.OwnerReferences == nil {
			return nil
		}

		// If configmap is owned by ChainNodeSet already, there is nothing to do.
		if len(cm.OwnerReferences) > 0 && cm.OwnerReferences[0].UID == nodeSet.UID {
			return nil
		}

		// If configmap has owner, but it's not of ChainNode kind, lets not touch it.
		if len(cm.OwnerReferences) > 0 && cm.OwnerReferences[0].Kind != ChainNodeKind {
			return nil
		}

		logger.Info("changing genesis configmap ownership to chainnodeset")
		cm.OwnerReferences = nil
		if err := controllerutil.SetControllerReference(nodeSet, cm, r.Scheme); err != nil {
			return err
		}
		return r.Update(ctx, cm)
	}

	if nodeSet.Spec.Genesis.ShouldDownloadUsingContainer() {
		// Operator will download the genesis for each ChainNode directly into their volumes
		nodeSet.Status.ChainID = *nodeSet.Spec.Genesis.ChainID
		return r.Status().Update(ctx, nodeSet)
	}

	return r.getGenesis(ctx, app, nodeSet)
}

func (r *Reconciler) getGenesis(ctx context.Context, app *chainutils.App, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)

	var genesis string
	var err error

	switch {
	case nodeSet.Spec.Genesis.Url != nil:
		logger.Info("retrieving genesis from url", "url", *nodeSet.Spec.Genesis.Url)

		genesis, err = chainutils.RetrieveGenesisFromURL(*nodeSet.Spec.Genesis.Url, nodeSet.Spec.Genesis.GenesisSHA)
		if err != nil {
			r.recorder.Eventf(nodeSet, corev1.EventTypeWarning, appsv1.ReasonGenesisError, err.Error())
			return err
		}

		r.recorder.Eventf(nodeSet,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisImported,
			"Genesis downloaded using specified URL",
		)

	case nodeSet.Spec.Genesis.FromNodeRPC != nil:
		genesisUrl := nodeSet.Spec.Genesis.FromNodeRPC.GetGenesisFromRPCUrl()
		logger.Info("retrieving genesis from node RPC", "endpoint", genesisUrl)

		genesis, err = chainutils.RetrieveGenesisFromNodeRPC(genesisUrl, nodeSet.Spec.Genesis.GenesisSHA)
		if err != nil {
			r.recorder.Eventf(nodeSet, corev1.EventTypeWarning, appsv1.ReasonGenesisError, err.Error())
			return err
		}

		r.recorder.Eventf(nodeSet,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisImported,
			"Genesis downloaded using specified RPC node",
		)

	case nodeSet.Spec.Genesis.ConfigMap != nil:
		logger.Info("loading genesis from configmap", "configmap", *nodeSet.Spec.Genesis.ConfigMap)
		genesis, err = app.LoadGenesisFromConfigMap(ctx, *nodeSet.Spec.Genesis.ConfigMap)
		if err != nil {
			r.recorder.Eventf(nodeSet, corev1.EventTypeWarning, appsv1.ReasonGenesisError, err.Error())
			return err
		}

		chainID, err := chainutils.ExtractChainIdFromGenesis(genesis)
		if err != nil {
			return err
		}

		r.recorder.Eventf(nodeSet,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisImported,
			"Genesis imported from ConfigMap",
		)

		// update chainID in status
		logger.Info("updating .status.chainID", "chainID", chainID)
		nodeSet.Status.ChainID = chainID
		return r.Status().Update(ctx, nodeSet)

	default:
		return fmt.Errorf("genesis could not be retrived using any of the available methods")
	}

	chainID, err := chainutils.ExtractChainIdFromGenesis(genesis)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeSet.Spec.Genesis.GetConfigMapName(chainID),
			Namespace: nodeSet.Namespace,
			Labels:    utils.ExcludeMapKeys(nodeSet.ObjectMeta.Labels, controllers.LabelWorkerName),
		},
		Data: map[string]string{chainutils.GenesisFilename: genesis},
	}
	if err := controllerutil.SetControllerReference(nodeSet, cm, r.Scheme); err != nil {
		return err
	}

	logger.Info("creating genesis configmap", "configmap", cm.GetName())
	if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// update chainID in status
	logger.Info("updating .status.chainID", "chainID", chainID)
	nodeSet.Status.ChainID = chainID
	return r.Status().Update(ctx, nodeSet)
}
