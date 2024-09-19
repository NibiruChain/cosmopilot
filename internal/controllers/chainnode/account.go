package chainnode

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
)

func (r *Reconciler) ensureAccount(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	// We probably want the user to delete the secret with mnemonic when we dont need it anymore.
	// And we only need it for gentx.
	if chainNode.Status.ValidatorAddress != "" {
		return nil
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: chainNode.GetNamespace(),
		Name:      chainNode.Spec.Validator.GetAccountSecretName(chainNode),
	}, secret)

	mustCreate := false
	if err != nil {
		if errors.IsNotFound(err) {
			mustCreate = true
			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.Spec.Validator.GetAccountSecretName(chainNode),
					Namespace: chainNode.GetNamespace(),
					Labels:    WithChainNodeLabels(chainNode),
				},
				Data: make(map[string][]byte),
			}
		} else {
			return err
		}
	}

	// Ensure private key
	var validatorAddress, accountAddress string
	mustUpdate := false
	if _, ok := secret.Data[MnemonicKey]; !ok {
		if !mustCreate {
			mustUpdate = true
		}
		account, err := chainutils.CreateAccount(
			chainNode.Spec.Validator.GetAccountPrefix(),
			chainNode.Spec.Validator.GetValPrefix(),
			chainNode.Spec.Validator.GetAccountHDPath(),
		)
		if err != nil {
			return err
		}
		secret.Data[MnemonicKey] = []byte(account.Mnemonic)
		validatorAddress = account.ValidatorAddress
		accountAddress = account.Address
	} else {
		account, err := chainutils.AccountFromMnemonic(
			string(secret.Data[MnemonicKey]),
			chainNode.Spec.Validator.GetAccountPrefix(),
			chainNode.Spec.Validator.GetValPrefix(),
			chainNode.Spec.Validator.GetAccountHDPath(),
		)
		if err != nil {
			return err
		}
		validatorAddress = account.ValidatorAddress
		accountAddress = account.Address
		logger.Info("account imported from secret", "secret", secret.GetName())
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonAccountImported,
			"Validator account imported from Secret",
		)
	}

	if mustCreate {
		logger.Info("creating secret with account mnemonic", "secret", secret.GetName())
		if err := r.Create(ctx, secret); err != nil {
			return err
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonAccountCreated,
			"Validator account created",
		)
	} else if mustUpdate {
		logger.Info("updating secret with account mnemonic", "secret", secret.GetName())
		if err := r.Update(ctx, secret); err != nil {
			return err
		}
	}

	// update status
	if chainNode.Status.ValidatorAddress != validatorAddress || chainNode.Status.AccountAddress != accountAddress {
		logger.Info("updating .status.validatorAddress and .status.accountAddress", "val", validatorAddress, "acc", accountAddress)
		chainNode.Status.ValidatorAddress = validatorAddress
		chainNode.Status.AccountAddress = accountAddress
		return r.Status().Update(ctx, chainNode)
	}
	return nil
}
