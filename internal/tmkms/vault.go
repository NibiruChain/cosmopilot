package tmkms

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

const (
	vaultProviderName = "hashicorp"
)

type VaultProvider struct {
	ChainID           string                    `toml:"chain_id"`
	Address           string                    `toml:"api_endpoint"`
	Key               string                    `toml:"pk_name"`
	CertificateSecret *corev1.SecretKeySelector `toml:"-"`
	TokenSecret       *corev1.SecretKeySelector `toml:"-"`
	Token             string                    `toml:"access_token"`
	CaCert            string                    `toml:"ca_cert"`
	AutoRenewToken    bool                      `toml:"-"`
}

func WithVaultProvider(chainID, address, key string, token, ca *corev1.SecretKeySelector, autoRenewToken bool) Option {
	vault := &VaultProvider{
		ChainID:           chainID,
		Address:           address,
		Key:               key,
		CertificateSecret: ca,
		TokenSecret:       token,
		AutoRenewToken:    autoRenewToken,
	}
	if ca == nil {
		vault.CaCert = ""
	} else {
		vault.CaCert = "/vault/" + ca.Key
	}

	return func(cfg *Config) {
		if _, ok := cfg.Providers[vaultProviderName]; !ok {
			cfg.Providers[vaultProviderName] = make([]Provider, 0)
		}
		cfg.Providers[vaultProviderName] = append(cfg.Providers[vaultProviderName], vault)
	}
}

func (v *VaultProvider) process(kms *KMS, ctx context.Context) error {
	secret, err := kms.Client.CoreV1().Secrets(kms.Owner.GetNamespace()).Get(ctx, v.TokenSecret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	token, ok := secret.Data[v.TokenSecret.Key]
	if !ok {
		return fmt.Errorf("key %q is not present in secret %q", v.TokenSecret.Key, v.TokenSecret.Name)
	}

	v.Token = string(token)
	return nil
}

func (v *VaultProvider) getVolumes() []corev1.Volume {
	var volumes []corev1.Volume

	if v.CertificateSecret != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "vault-ca-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: v.CertificateSecret.Name,
				},
			},
		})
	}

	if v.AutoRenewToken {
		volumes = append(volumes, corev1.Volume{
			Name: "vault-token",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: v.TokenSecret.Name,
				},
			},
		})
	}

	return volumes
}

func (v *VaultProvider) getVolumeMounts() []corev1.VolumeMount {
	if v.CertificateSecret != nil {
		return []corev1.VolumeMount{
			{
				Name:      "vault-ca-cert",
				ReadOnly:  true,
				MountPath: "/vault/" + v.CertificateSecret.Key,
				SubPath:   v.CertificateSecret.Key,
			},
		}
	}
	return []corev1.VolumeMount{}
}

func (v *VaultProvider) getContainers() []corev1.Container {
	var containers []corev1.Container
	if v.AutoRenewToken {
		spec := corev1.Container{
			Name:  "vault-token-renewer",
			Image: "ghcr.io/nibiruchain/vault-token-renewer",
			Env: []corev1.EnvVar{
				{
					Name:  "VAULT_ADDR",
					Value: v.Address,
				},
				{
					Name:  "VAULT_TOKEN_PATH",
					Value: "/vault/" + v.TokenSecret.Key,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vault-token",
					ReadOnly:  true,
					MountPath: "/vault/" + v.TokenSecret.Key,
					SubPath:   v.TokenSecret.Key,
				},
			},
		}

		if v.CertificateSecret != nil {
			spec.Env = append(spec.Env, corev1.EnvVar{
				Name:  "VAULT_CACERT",
				Value: "/vault/" + v.CertificateSecret.Key,
			})
			spec.VolumeMounts = append(spec.VolumeMounts, corev1.VolumeMount{
				Name:      "vault-ca-cert",
				ReadOnly:  true,
				MountPath: "/vault/" + v.CertificateSecret.Key,
				SubPath:   v.CertificateSecret.Key,
			})
		}
		containers = append(containers, spec)
	}

	return containers
}

func (kms *KMS) UploadKeyToVault(ctx context.Context, name, key string, token *corev1.SecretKeySelector) error {
	if err := kms.processAllProviders(ctx); err != nil {
		return err
	}

	if err := kms.ensureConfigMap(ctx); err != nil {
		return err
	}

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: kms.Name,
					},
				},
			},
		},
	}

	for provider := range kms.Config.Providers {
		for _, p := range kms.Config.Providers[provider] {
			volumes = append(volumes, p.getVolumes()...)
		}
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/data/" + configFileName,
			SubPath:   configFileName,
		},
	}

	for provider := range kms.Config.Providers {
		for _, p := range kms.Config.Providers[provider] {
			volumeMounts = append(volumeMounts, p.getVolumeMounts()...)
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-vault-upload", kms.Name),
			Namespace: kms.Owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes:       volumes,
			Containers: []corev1.Container{
				{
					Name:            tmkmsAppName,
					Image:           kms.Config.Image,
					ImagePullPolicy: corev1.PullAlways,
					Args:            []string{"hashicorp", "upload", name, "--payload", key, "-c", "/data/" + configFileName},
					Env: []corev1.EnvVar{
						{
							Name: "VAULT_TOKEN",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: token,
							},
						},
					},
					VolumeMounts: volumeMounts,
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(kms.Owner, pod, kms.Scheme); err != nil {
		return err
	}

	ph := k8s.NewPodHelper(kms.Client, nil, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer ph.Delete(ctx)

	if err := ph.Create(ctx); err != nil {
		return err
	}

	// TODO: handle key already existing error
	if err := ph.WaitForPodSucceeded(ctx, time.Minute); err != nil {
		return err
	}
	return nil
}
