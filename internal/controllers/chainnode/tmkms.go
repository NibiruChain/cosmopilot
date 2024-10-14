package chainnode

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/cometbft"
	"github.com/NibiruChain/cosmopilot/internal/tmkms"
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

	provider, kms, err := r.getTmkms(chainNode)
	if err != nil {
		return err
	}

	if err = kms.DeployConfig(ctx); err != nil {
		return err
	}

	// Provider specific operations
	switch p := provider.(type) {
	case *tmkms.HashicorpProvider:
		if chainNode.ShouldUploadVaultKey() {
			key, err := r.loadPrivKey(ctx, chainNode)
			if err != nil {
				return err
			}

			// If the key is empty, then it was uploaded already
			if key == "" {
				return nil
			}

			if err = p.UploadKey(ctx, kms, key); err != nil {
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
	}

	return nil
}

func (r *Reconciler) getTmkms(chainNode *appsv1.ChainNode) (tmkms.Provider, *tmkms.KMS, error) {
	if !chainNode.UsesTmKms() {
		return nil, nil, fmt.Errorf("no tmkms configuration available in chainnode")
	}

	var tmkmsOptions []tmkms.Option
	tmkmsOptions = append(tmkmsOptions, tmkms.WithResources(chainNode.Spec.Validator.TmKMS.GetResources()))

	var provider tmkms.Provider
	switch providerCfg := chainNode.Spec.Validator.TmKMS.Provider; {

	case providerCfg.Hashicorp != nil:
		provider = tmkms.NewHashicorpProvider(
			chainNode.Status.ChainID,
			providerCfg.Hashicorp.Address,
			providerCfg.Hashicorp.Key,
			providerCfg.Hashicorp.TokenSecret,
			providerCfg.Hashicorp.CertificateSecret,
			providerCfg.Hashicorp.AutoRenewToken,
			providerCfg.Hashicorp.SkipCertificateVerify,
		)
		tmkmsOptions = append(tmkmsOptions, tmkms.WithProvider(provider))

		// TODO: remove this when we have official release of tmkms (see https://github.com/iqlusioninc/tmkms/pull/843)
		tmkmsOptions = append(tmkmsOptions, tmkms.WithImage("ghcr.io/nibiruchain/tmkms:new-vault"))

	default:
		return nil, nil, fmt.Errorf("no supported provider configured")
	}

	tmkmsOptions = append(tmkmsOptions, tmkms.WithChain(
		chainNode.Status.ChainID,
		tmkms.WithKeyFormat(
			chainNode.Spec.Validator.TmKMS.GetKeyFormat().Type,
			chainNode.Spec.Validator.TmKMS.GetKeyFormat().AccountKeyPrefix,
			chainNode.Spec.Validator.TmKMS.GetKeyFormat().ConsensusKeyPrefix,
		),
	))

	tmkmsOptions = append(tmkmsOptions, tmkms.WithValidator(
		chainNode.Status.ChainID,
		fmt.Sprintf("tcp://localhost:%d", chainutils.PrivValPort),
		tmkms.WithProtocolVersion(chainNode.Spec.Validator.TmKMS.GetProtocolVersion()),
	))

	tmkmsOptions = append(tmkmsOptions, tmkms.PersistState(chainNode.Spec.Validator.TmKMS.ShouldPersistState()))

	return provider, tmkms.New(
		r.ClientSet,
		r.Scheme,
		fmt.Sprintf("%s-tmkms", chainNode.GetName()),
		chainNode,
		tmkmsOptions...), nil
}

func (r *Reconciler) loadPrivKey(ctx context.Context, chainNode *appsv1.ChainNode) (string, error) {
	uploaded, ok := chainNode.Annotations[annotationVaultKeyUploaded]
	if ok && uploaded == strconv.FormatBool(true) {
		return "", nil
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: chainNode.GetNamespace(),
		Name:      chainNode.Spec.Validator.GetPrivKeySecretName(chainNode),
	}, secret)
	if err != nil {
		return "", err
	}

	data, ok := secret.Data[PrivKeyFilename]
	if !ok {
		return "", fmt.Errorf("%s is not present in the secret", PrivKeyFilename)
	}

	privKey, err := cometbft.LoadPrivKey(data)
	if err != nil {
		return "", err
	}

	return privKey.PrivKey.Value, nil
}
