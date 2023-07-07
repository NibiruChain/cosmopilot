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

func (r *Reconciler) ensureTmKMS(ctx context.Context, chainNode *appsv1.ChainNode) error {
	if chainNode.Spec.Validator == nil || chainNode.Spec.Validator.TmKMS == nil {
		// Configuration not specified or removed. Let's try to delete it anyway.
		_ = tmkms.New(r.ClientSet, r.Scheme, fmt.Sprintf("%s-tmkms", chainNode.GetName()), chainNode).Undeploy(ctx)
		return nil
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
		)
		if chainNode.ShouldUploadVaultKey() {
			if err := r.ensureTmkmsVaultUploadKey(ctx, chainNode); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("no supported provider configured")
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
		fmt.Sprintf("tcp://%s:%d", chainNode.GetNodeFQDN(), chainutils.PrivValPort),
		tmkms.WithProtocolVersion(chainNode.Spec.Validator.TmKMS.GetProtocolVersion()),
	)

	return tmkms.New(r.ClientSet, r.Scheme, fmt.Sprintf("%s-tmkms", chainNode.GetName()), chainNode, chainConfig, validatorConfig, providerConfig).Deploy(ctx)
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

	data, ok := secret.Data[privKeyFilename]
	if !ok {
		return fmt.Errorf("%s is not present in the secret", privKeyFilename)
	}

	privKey, err := cometbft.LoadPrivKey(data)
	if err != nil {
		return err
	}

	err = tmkms.New(r.ClientSet, r.Scheme, fmt.Sprintf("%s-tmkms", chainNode.GetName()), chainNode,
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
		)).UploadKeyToVault(ctx,
		chainNode.Spec.Validator.TmKMS.Provider.Vault.Key,
		privKey.PrivKey.Value,
		chainNode.Spec.Validator.TmKMS.Provider.Vault.TokenSecret,
	)
	if err != nil {
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonUploadFailure,
			"failed to upload key: %v", err,
		)
		return err
	}

	chainNode.Annotations[annotationVaultKeyUploaded] = strconv.FormatBool(true)
	return r.Update(ctx, chainNode)
}
