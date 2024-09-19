package chainnode

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/k8s"
)

func (r *Reconciler) ensureGenesis(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	// Return if we have a chain ID already
	if chainNode.Status.ChainID != "" {
		return nil
	}

	if chainNode.ShouldInitGenesis() {
		if err := r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeInitGenesis); err != nil {
			return err
		}
		if err := r.initGenesis(ctx, app, chainNode); err != nil {
			return err
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisInitialized,
			"Genesis was successfully initialized",
		)
		return nil
	}
	return r.getGenesis(ctx, app, chainNode)
}

func (r *Reconciler) getGenesis(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	var genesis string
	var err error

	switch {
	case chainNode.Spec.Genesis.Url != nil:
		if chainNode.Spec.Genesis.ShouldDownloadUsingContainer() {
			pvc := &corev1.PersistentVolumeClaim{}
			if err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc); err != nil {
				return err
			}
			logger.Info("downloading genesis to data volume using container",
				"url", *chainNode.Spec.Genesis.Url,
				"pvc", pvc.GetName(),
			)
			if err = k8s.NewPvcHelper(r.ClientSet, r.RestConfig, pvc).
				DownloadGenesis(ctx, *chainNode.Spec.Genesis.Url, chainutils.GenesisFilename); err != nil {
				return err
			}

			chainNode.Status.ChainID = *chainNode.Spec.Genesis.ChainID
			return r.Status().Update(ctx, chainNode)
		}

		logger.Info("retrieving genesis from url", "url", *chainNode.Spec.Genesis.Url)
		genesis, err = chainutils.RetrieveGenesisFromURL(*chainNode.Spec.Genesis.Url, chainNode.Spec.Genesis.GenesisSHA)
		if err != nil {
			r.recorder.Eventf(chainNode, corev1.EventTypeWarning, appsv1.ReasonGenesisError, err.Error())
			return err
		}

		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisImported,
			"Genesis downloaded using specified URL",
		)

	case chainNode.Spec.Genesis.FromNodeRPC != nil:
		genesisUrl := chainNode.Spec.Genesis.FromNodeRPC.GetGenesisFromRPCUrl()
		logger.Info("retrieving genesis from node RPC", "endpoint", genesisUrl)

		genesis, err = chainutils.RetrieveGenesisFromNodeRPC(genesisUrl, chainNode.Spec.Genesis.GenesisSHA)
		if err != nil {
			r.recorder.Eventf(chainNode, corev1.EventTypeWarning, appsv1.ReasonGenesisError, err.Error())
			return err
		}

		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisImported,
			"Genesis downloaded using specified RPC node",
		)

	case chainNode.Spec.Genesis.ConfigMap != nil:
		logger.Info("loading genesis from configmap", "configmap", *chainNode.Spec.Genesis.ConfigMap)
		genesis, err = app.LoadGenesisFromConfigMap(ctx, *chainNode.Spec.Genesis.ConfigMap)
		if err != nil {
			r.recorder.Eventf(chainNode, corev1.EventTypeWarning, appsv1.ReasonGenesisError, err.Error())
			return err
		}

		chainID, err := chainutils.ExtractChainIdFromGenesis(genesis)
		if err != nil {
			return err
		}

		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonGenesisImported,
			"Genesis imported from ConfigMap",
		)

		// update chainID in status
		logger.Info("updating .status.chainID", "chainID", chainID)
		chainNode.Status.ChainID = chainID
		return r.Status().Update(ctx, chainNode)

	default:
		return fmt.Errorf("genesis could not be retrived using any of the available methods")
	}

	chainID, err := chainutils.ExtractChainIdFromGenesis(genesis)
	if err != nil {
		return err
	}

	if chainNode.Spec.Genesis.ShouldUseDataVolume() {
		pvc := &corev1.PersistentVolumeClaim{}
		if err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), pvc); err != nil {
			return err
		}
		logger.Info("writing genesis to data volume", "pvc", pvc.GetName())
		if err = k8s.NewPvcHelper(r.ClientSet, r.RestConfig, pvc).
			WriteToFile(ctx, genesis, chainutils.GenesisFilename); err != nil {
			return err
		}

	} else {
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      chainNode.Spec.Genesis.GetConfigMapName(chainID),
				Namespace: chainNode.Namespace,
				Labels:    WithChainNodeLabels(chainNode),
			},
			Data: map[string]string{chainutils.GenesisFilename: genesis},
		}
		if err = controllerutil.SetControllerReference(chainNode, cm, r.Scheme); err != nil {
			return err
		}

		logger.Info("creating genesis configmap", "configmap", cm.GetName())
		if err = r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// update chainID in status
	logger.Info("updating .status.chainID", "chainID", chainID)
	chainNode.Status.ChainID = chainID
	return r.Status().Update(ctx, chainNode)
}

func (r *Reconciler) initGenesis(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	logger.Info("initializing new genesis", "chainID", chainNode.Spec.Validator.Init.ChainID)
	genesisParams := &chainutils.Params{
		ChainID:                 chainNode.Spec.Validator.Init.ChainID,
		Assets:                  chainNode.Spec.Validator.Init.Assets,
		StakeAmount:             chainNode.Spec.Validator.Init.StakeAmount,
		Accounts:                []chainutils.AccountAssets{},
		CommissionMaxChangeRate: chainNode.Spec.Validator.GetCommissionMaxChangeRate(),
		CommissionMaxRate:       chainNode.Spec.Validator.GetCommissionMaxRate(),
		CommissionRate:          chainNode.Spec.Validator.GetCommissionRate(),
		MinSelfDelegation:       chainNode.Spec.Validator.GetMinSelfDelegation(),
		UnbondingTime:           chainNode.Spec.Validator.GetInitUnbondingTime(),
		VotingPeriod:            chainNode.Spec.Validator.GetInitVotingPeriod(),
	}

	for _, a := range chainNode.Spec.Validator.Init.ChainNodeAccounts {
		cn := &appsv1.ChainNode{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: chainNode.GetNamespace(), Name: a.ChainNode}, cn); err != nil {
			return fmt.Errorf("failed to get chain node account: %w", err)
		}
		if cn.Status.AccountAddress == "" {
			return fmt.Errorf("chain node has no account address")
		}
		genesisParams.Accounts = append(genesisParams.Accounts, chainutils.AccountAssets{
			Address: cn.Status.AccountAddress,
			Assets:  a.Assets,
		})
	}

	for _, a := range chainNode.Spec.Validator.Init.Accounts {
		genesisParams.Accounts = append(genesisParams.Accounts, chainutils.AccountAssets{
			Address: a.Address,
			Assets:  a.Assets,
		})
	}

	initCommands := make([]*chainutils.InitCommand, len(chainNode.Spec.Validator.Init.AdditionalInitCommands))
	for i, c := range chainNode.Spec.Validator.Init.AdditionalInitCommands {
		initCommands[i] = &chainutils.InitCommand{Args: c.Args, Command: c.Command}
		if c.Image != nil {
			initCommands[i].Image = *c.Image
		} else {
			initCommands[i].Image = chainNode.GetAppImage()
		}
	}

	accountSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: chainNode.GetNamespace(), Name: chainNode.Spec.Validator.GetAccountSecretName(chainNode)}, accountSecret); err != nil {
		return err
	}
	account, err := chainutils.AccountFromMnemonic(
		string(accountSecret.Data[MnemonicKey]),
		chainNode.Spec.Validator.GetAccountPrefix(),
		chainNode.Spec.Validator.GetValPrefix(),
		chainNode.Spec.Validator.GetAccountHDPath(),
	)
	if err != nil {
		return err
	}

	// Gather validator info
	nodeInfo := &chainutils.NodeInfo{}
	nodeInfo.Moniker = chainNode.GetMoniker()
	if chainNode.Spec.Validator.Info != nil {
		nodeInfo.Details = chainNode.Spec.Validator.Info.Details
		nodeInfo.Website = chainNode.Spec.Validator.Info.Website
		nodeInfo.Identity = chainNode.Spec.Validator.Info.Identity
	}

	genesis, err := app.NewGenesis(
		ctx,
		chainNode.Spec.Validator.GetPrivKeySecretName(chainNode),
		account,
		nodeInfo,
		genesisParams,
		initCommands...,
	)
	if err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonInitGenesisFailure,
			"failed to initialize genesis: %s", err.Error())
		return err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainNode.Spec.Genesis.GetConfigMapName(chainNode.Spec.Validator.Init.ChainID),
			Namespace: chainNode.Namespace,
		},
		Data: map[string]string{chainutils.GenesisFilename: genesis},
	}
	if err = controllerutil.SetControllerReference(chainNode, cm, r.Scheme); err != nil {
		return err
	}

	logger.Info("creating genesis configmap", "configmap", cm.GetName())
	if err = r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// update chainID in status
	logger.Info("updating .status.chainID", "chainID", chainNode.Spec.Validator.Init.ChainID)
	chainNode.Status.ChainID = chainNode.Spec.Validator.Init.ChainID
	return r.Status().Update(ctx, chainNode)
}
