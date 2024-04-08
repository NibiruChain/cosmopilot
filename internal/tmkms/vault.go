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

func NewVaultProvider(chainID, address, key string, token, ca *corev1.SecretKeySelector, autoRenewToken bool) Provider {
	vault := &VaultProvider{
		ChainID:           chainID,
		Address:           address,
		Key:               key,
		CertificateSecret: ca,
		TokenSecret:       token,
		TokenFile:         hashicorpMountDir + token.Key,
		AutoRenewToken:    autoRenewToken,
	}
	if ca == nil {
		vault.CaCert = ""
	} else {
		vault.CaCert = hashicorpMountDir + ca.Key
	}

	return vault
}

func (v VaultProvider) getVolumes() []corev1.Volume {
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

func (v VaultProvider) getVolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "vault-token",
			ReadOnly:  true,
			MountPath: hashicorpMountDir + v.TokenSecret.Key,
			SubPath:   v.TokenSecret.Key,
		},
	}

	if v.CertificateSecret != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "vault-ca-cert",
			ReadOnly:  true,
			MountPath: hashicorpMountDir + v.CertificateSecret.Key,
			SubPath:   v.CertificateSecret.Key,
		})
	}
	return volumeMounts
}

func (v VaultProvider) getContainers() []corev1.Container {
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
					Value: hashicorpMountDir + v.TokenSecret.Key,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vault-token",
					ReadOnly:  true,
					MountPath: hashicorpMountDir + v.TokenSecret.Key,
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
				Value: hashicorpMountDir + v.CertificateSecret.Key,
			})
			spec.VolumeMounts = append(spec.VolumeMounts, corev1.VolumeMount{
				Name:      "vault-ca-cert",
				ReadOnly:  true,
				MountPath: hashicorpMountDir + v.CertificateSecret.Key,
				SubPath:   v.CertificateSecret.Key,
			})
		}
		containers = append(containers, spec)
	}

	return containers
}

func (v VaultProvider) UploadKey(ctx context.Context, kms *KMS, key string) error {
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
							SecretName: v.Key,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            tmkmsAppName,
					Image:           kms.Config.Image,
					ImagePullPolicy: corev1.PullAlways,
					Args:            []string{"hashicorp", "upload", v.Key, "--payload", key},
					Env: []corev1.EnvVar{
						{
							Name:  "VAULT_ADDR",
							Value: v.Address,
						},
						{
							Name: "VAULT_TOKEN",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: v.TokenSecret,
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "vault-token",
							ReadOnly:  true,
							MountPath: hashicorpMountDir + v.TokenSecret.Key,
							SubPath:   v.TokenSecret.Key,
						},
					},
				},
			},
		},
	}

	if v.CertificateSecret != nil {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "vault-ca-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: v.CertificateSecret.Name,
				},
			},
		})
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "VAULT_CACERT",
			Value: hashicorpMountDir + v.CertificateSecret.Key,
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "vault-ca-cert",
			ReadOnly:  true,
			MountPath: hashicorpMountDir + v.CertificateSecret.Key,
			SubPath:   v.CertificateSecret.Key,
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
