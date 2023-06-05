package chainnode

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

func (r *Reconciler) ensureGenesis(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	// Return if we have a chain ID already
	if chainNode.Status.ChainID != "" {
		return nil
	}

	if chainNode.ShouldInitGenesis() {
		return r.initGenesis(ctx, app, chainNode)
	}
	return r.getGenesis(ctx, chainNode)
}

func (r *Reconciler) getGenesis(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	genesis := ""
	chainID := ""
	var err error
	if chainNode.Spec.Genesis.Url != nil {
		logger.Info("retrieving genesis from url", "url", *chainNode.Spec.Genesis.Url)
		genesis, err = utils.FetchJson(*chainNode.Spec.Genesis.Url)
		if err != nil {
			return err
		}
		chainID, err = chainutils.ExtractChainIdFromGenesis(genesis)
		if err != nil {
			return err
		}
	}

	// TODO: add other methods for retrieving genesis

	if genesis == "" || chainID == "" {
		return fmt.Errorf("genesis could not be retrived using any of the available methods")
	}

	// We create the genesis once only.
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-genesis", chainID),
			Namespace: chainNode.Namespace,
		},
		Data: map[string]string{genesisFilename: genesis},
	}
	if err := controllerutil.SetControllerReference(chainNode, cm, r.Scheme); err != nil {
		return err
	}

	logger.Info("creating genesis configmap")
	if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// update chainID in status
	logger.Info("updating status with chain id")
	chainNode.Status.ChainID = chainID
	return r.Status().Update(ctx, chainNode)
}

func (r *Reconciler) initGenesis(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	genesisParams := &chainutils.GenesisParams{
		ChainID:       chainNode.Spec.Validator.Init.ChainID,
		Assets:        chainNode.Spec.Validator.Init.Assets,
		StakeAmount:   chainNode.Spec.Validator.Init.StakeAmount,
		Accounts:      make([]chainutils.AccountAssets, len(chainNode.Spec.Validator.Init.Accounts)),
		UnbondingTime: chainNode.GetInitUnbondingTime(),
		VotingPeriod:  chainNode.GetInitVotingPeriod(),
	}

	for i, a := range chainNode.Spec.Validator.Init.Accounts {
		genesisParams.Accounts[i] = chainutils.AccountAssets{
			Address: a.Address,
			Assets:  a.Assets,
		}
	}

	initCommands := make([]*chainutils.InitCommand, len(chainNode.Spec.Validator.Init.AdditionalInitCommands))
	for i, c := range chainNode.Spec.Validator.Init.AdditionalInitCommands {
		initCommands[i] = &chainutils.InitCommand{Args: c.Args, Command: c.Command}
		if c.Image != nil {
			initCommands[i].Image = *c.Image
		} else {
			initCommands[i].Image = chainNode.GetImage()
		}
	}

	accountSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: chainNode.GetNamespace(), Name: chainNode.GetValidatorAccountSecretName()}, accountSecret); err != nil {
		return err
	}
	account, err := chainutils.AccountFromMnemonic(
		string(accountSecret.Data[mnemonicKey]),
		chainNode.GetValidatorAccountPrefix(),
		chainNode.GetValidatorValPrefix(),
		chainNode.GetValidatorAccountHDPath(),
	)
	if err != nil {
		return err
	}

	// Gather validator info
	nodeInfo := &chainutils.NodeInfo{}
	if chainNode.Spec.Validator.Info == nil {
		nodeInfo.Moniker = chainNode.GetName()
	} else {
		if chainNode.Spec.Validator.Info.Moniker == nil {
			nodeInfo.Moniker = chainNode.GetName()
		} else {
			nodeInfo.Moniker = *chainNode.Spec.Validator.Info.Moniker
		}
		nodeInfo.Details = chainNode.Spec.Validator.Info.Details
		nodeInfo.Website = chainNode.Spec.Validator.Info.Website
		nodeInfo.Identity = chainNode.Spec.Validator.Info.Identity
	}

	genesis, err := app.NewGenesis(
		ctx,
		chainNode.GetValidatorPrivKeySecretName(),
		account,
		nodeInfo,
		genesisParams,
		initCommands...,
	)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-genesis", chainNode.Spec.Validator.Init.ChainID),
			Namespace: chainNode.Namespace,
		},
		Data: map[string]string{genesisFilename: genesis},
	}
	if err := controllerutil.SetControllerReference(chainNode, cm, r.Scheme); err != nil {
		return err
	}

	logger.Info("creating genesis configmap")
	if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// update chainID in status
	logger.Info("updating status with chain id")
	chainNode.Status.ChainID = chainNode.Spec.Validator.Init.ChainID
	return r.Status().Update(ctx, chainNode)
}
