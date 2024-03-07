package tmkms

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

const (
	vaultProviderName = "hashicorp"
	vaultMountDir     = "/vault/"

	tokenRenewerCpu    = "100m"
	tokenRenewerMemory = "64Mi"
)

var (
	tokenRenewerCpuResources    = resource.MustParse(tokenRenewerCpu)
	tokenRenewerMemoryResources = resource.MustParse(tokenRenewerMemory)
)

type VaultProvider struct {
	ChainID           string                    `toml:"chain_id"`
	Address           string                    `toml:"api_endpoint"`
	Key               string                    `toml:"pk_name"`
	CertificateSecret *corev1.SecretKeySelector `toml:"-"`
	TokenSecret       *corev1.SecretKeySelector `toml:"-"`
	TokenFile         string                    `toml:"token_file"`
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
		TokenFile:         vaultMountDir + token.Key,
		AutoRenewToken:    autoRenewToken,
	}
	if ca == nil {
		vault.CaCert = ""
	} else {
		vault.CaCert = vaultMountDir + ca.Key
	}

	return func(cfg *Config) {
		if _, ok := cfg.Providers[vaultProviderName]; !ok {
			cfg.Providers[vaultProviderName] = make([]Provider, 0)
		}
		cfg.Providers[vaultProviderName] = append(cfg.Providers[vaultProviderName], vault)
	}
}

func (v *VaultProvider) getVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "vault-token",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: v.TokenSecret.Name,
				},
			},
		},
	}

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

	return volumes
}

func (v *VaultProvider) getVolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "vault-token",
			ReadOnly:  true,
			MountPath: vaultMountDir + v.TokenSecret.Key,
			SubPath:   v.TokenSecret.Key,
		},
	}

	if v.CertificateSecret != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "vault-ca-cert",
			ReadOnly:  true,
			MountPath: vaultMountDir + v.CertificateSecret.Key,
			SubPath:   v.CertificateSecret.Key,
		})
	}
	return volumeMounts
}

func (v *VaultProvider) getContainers() []corev1.Container {
	var containers []corev1.Container
	var sidecarRestartAlways = corev1.ContainerRestartPolicyAlways

	if v.AutoRenewToken {
		spec := corev1.Container{
			Name:          "vault-token-renewer",
			Image:         "ghcr.io/nibiruchain/vault-token-renewer",
			RestartPolicy: &sidecarRestartAlways,
			Env: []corev1.EnvVar{
				{
					Name:  "VAULT_ADDR",
					Value: v.Address,
				},
				{
					Name:  "VAULT_TOKEN_PATH",
					Value: vaultMountDir + v.TokenSecret.Key,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vault-token",
					ReadOnly:  true,
					MountPath: vaultMountDir + v.TokenSecret.Key,
					SubPath:   v.TokenSecret.Key,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    tokenRenewerCpuResources,
					corev1.ResourceMemory: tokenRenewerMemoryResources,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    tokenRenewerCpuResources,
					corev1.ResourceMemory: tokenRenewerMemoryResources,
				},
			},
		}

		if v.CertificateSecret != nil {
			spec.Env = append(spec.Env, corev1.EnvVar{
				Name:  "VAULT_CACERT",
				Value: vaultMountDir + v.CertificateSecret.Key,
			})
			spec.VolumeMounts = append(spec.VolumeMounts, corev1.VolumeMount{
				Name:      "vault-ca-cert",
				ReadOnly:  true,
				MountPath: vaultMountDir + v.CertificateSecret.Key,
				SubPath:   v.CertificateSecret.Key,
			})
		}
		containers = append(containers, spec)
	}

	return containers
}

func (kms *KMS) UploadKeyToVault(ctx context.Context, name, key, address string, token, ca *corev1.SecretKeySelector) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-vault-upload", kms.Name),
			Namespace: kms.Owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "vault-token",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: token.Name,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            tmkmsAppName,
					Image:           kms.Config.Image,
					ImagePullPolicy: corev1.PullAlways,
					Args:            []string{"hashicorp", "upload", name, "--payload", key},
					Env: []corev1.EnvVar{
						{
							Name:  "VAULT_ADDR",
							Value: address,
						},
						{
							Name: "VAULT_TOKEN",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: token,
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "vault-token",
							ReadOnly:  true,
							MountPath: vaultMountDir + token.Key,
							SubPath:   token.Key,
						},
					},
				},
			},
		},
	}

	if ca != nil {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "vault-ca-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: ca.Name,
				},
			},
		})
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "VAULT_CACERT",
			Value: vaultMountDir + ca.Key,
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "vault-ca-cert",
			ReadOnly:  true,
			MountPath: vaultMountDir + ca.Key,
			SubPath:   ca.Key,
		})
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
