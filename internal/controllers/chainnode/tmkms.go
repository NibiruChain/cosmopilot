package chainnode

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/internal/cometbft"
	"github.com/NibiruChain/nibiru-operator/internal/tmkms"
)

func (r *Reconciler) ensureTmKMSConfig(ctx context.Context, chainNode *appsv1.ChainNode) error {
	if !chainNode.UsesTmKms() {
		// Configuration not specified or removed. Let's try to delete it anyway.
		_ = tmkms.New(r.ClientSet,
			r.Scheme,
			fmt.Sprintf("%s-tmkms", chainNode.GetName()),
			chainNode).
			UndeployConfig(ctx)
		return nil
	}

	kms, err := r.getTmkms(ctx, chainNode)
	if err != nil {
		return err
	}

	return kms.DeployConfig(ctx)
}

func (r *Reconciler) getTmkms(ctx context.Context, chainNode *appsv1.ChainNode) (*tmkms.KMS, error) {
	if !chainNode.UsesTmKms() {
		return nil, fmt.Errorf("no tmkms configuration available in chainnode")
	}

	var providerConfig tmkms.Option
	switch provider := chainNode.Spec.Validator.TmKMS.Provider; {
	case provider.Vault != nil:
		providerConfig = tmkms.WithVaultProvider(
			chainNode.Status.ChainID,
			provider.Vault.Address,
			provider.Vault.Key,
			provider.Vault.TokenSecret,
			provider.Vault.CertificateSecret,
			provider.Vault.AutoRenewToken,
		)
		if chainNode.ShouldUploadVaultKey() {
			if err := r.ensureTmkmsVaultUploadKey(ctx, chainNode); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("no supported provider configured")
	}

	chainConfig := tmkms.WithChain(
		chainNode.Status.ChainID,
		tmkms.WithKeyFormat(
			chainNode.Spec.Validator.TmKMS.GetKeyFormat().Type,
			chainNode.Spec.Validator.TmKMS.GetKeyFormat().AccountKeyPrefix,
			chainNode.Spec.Validator.TmKMS.GetKeyFormat().ConsensusKeyPrefix,
		),
	)

	validatorConfig := tmkms.WithValidator(
		chainNode.Status.ChainID,
		fmt.Sprintf("tcp://localhost:%d", chainutils.PrivValPort),
		tmkms.WithProtocolVersion(chainNode.Spec.Validator.TmKMS.GetProtocolVersion()),
	)

	return tmkms.New(
		r.ClientSet,
		r.Scheme,
		fmt.Sprintf("%s-tmkms", chainNode.GetName()),
		chainNode,
		tmkms.PersistState(chainNode.Spec.Validator.TmKMS.ShouldPersistState()),
		chainConfig,
		validatorConfig,
		providerConfig), nil
}

func (r *Reconciler) ensureTmkmsVaultUploadKey(ctx context.Context, chainNode *appsv1.ChainNode) error {
	uploaded, ok := chainNode.Annotations[annotationVaultKeyUploaded]
	if ok && uploaded == strconv.FormatBool(true) {
		return nil
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: chainNode.GetNamespace(),
		Name:      chainNode.Spec.Validator.GetPrivKeySecretName(chainNode),
	}, secret)
	if err != nil {
		return err
	}

	data, ok := secret.Data[PrivKeyFilename]
	if !ok {
		return fmt.Errorf("%s is not present in the secret", PrivKeyFilename)
	}

	privKey, err := cometbft.LoadPrivKey(data)
	if err != nil {
		return err
	}

	err = tmkms.New(r.ClientSet, r.Scheme, fmt.Sprintf("%s-tmkms", chainNode.GetName()), chainNode,
		tmkms.PersistState(chainNode.Spec.Validator.TmKMS.ShouldPersistState()),
		tmkms.WithChain(chainNode.Status.ChainID),
		tmkms.WithValidator(chainNode.Status.ChainID,
			fmt.Sprintf("tcp://%s:%d", chainNode.GetNodeFQDN(), chainutils.PrivValPort),
		),
		tmkms.WithVaultProvider(
			chainNode.Status.ChainID,
			chainNode.Spec.Validator.TmKMS.Provider.Vault.Address,
			chainNode.Spec.Validator.TmKMS.Provider.Vault.Key,
			chainNode.Spec.Validator.TmKMS.Provider.Vault.TokenSecret,
			chainNode.Spec.Validator.TmKMS.Provider.Vault.CertificateSecret,
			chainNode.Spec.Validator.TmKMS.Provider.Vault.AutoRenewToken,
		)).UploadKeyToVault(ctx,
		chainNode.Spec.Validator.TmKMS.Provider.Vault.Key,
		privKey.PrivKey.Value,
		chainNode.Spec.Validator.TmKMS.Provider.Vault.Address,
		chainNode.Spec.Validator.TmKMS.Provider.Vault.TokenSecret,
		chainNode.Spec.Validator.TmKMS.Provider.Vault.CertificateSecret,
	)
	if err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonUploadFailure,
			"failed to upload key: %v", err,
		)
		return err
	}

	if chainNode.Annotations == nil {
		chainNode.Annotations = make(map[string]string)
	}

	chainNode.Annotations[annotationVaultKeyUploaded] = strconv.FormatBool(true)
	return r.Update(ctx, chainNode)
}
