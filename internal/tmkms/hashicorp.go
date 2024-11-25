package tmkms

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/cosmopilot/internal/k8s"
)

const (
	hashicorpProviderName = "hashicorp"
	hashicorpMountDir     = "/vault/"

	tokenRenewerCpu    = "100m"
	tokenRenewerMemory = "64Mi"
)

var (
	tokenRenewerCpuResources    = resource.MustParse(tokenRenewerCpu)
	tokenRenewerMemoryResources = resource.MustParse(tokenRenewerMemory)
)

type HashicorpProvider struct {
	Keys    []*HashicorpKey   `toml:"keys"`
	Adapter *HashicorpAdapter `toml:"adapter"`

	CertificateSecret *corev1.SecretKeySelector `toml:"-"`
	TokenSecret       *corev1.SecretKeySelector `toml:"-"`
	AutoRenewToken    bool                      `toml:"-"`
}

type HashicorpKey struct {
	ChainID string         `toml:"chain_id"`
	Key     string         `toml:"key"`
	KeyType string         `toml:"key_type,omitempty"`
	Auth    *HashicorpAuth `toml:"auth"`
}

type HashicorpAuth struct {
	AccessToken     string `toml:"access_token,omitempty"`
	AccessTokenFile string `toml:"access_token_file,omitempty"`
}

type HashicorpAdapter struct {
	VaultAddress    string                    `toml:"vault_addr"`
	VaultCaCert     string                    `toml:"vault_cacert,omitempty"`
	VaultSkipVerify bool                      `toml:"vault_skip_verify,omitempty"`
	CachePublicKey  bool                      `toml:"cache_pk,omitempty"`
	Endpoints       *HashicorpEndpointsConfig `toml:"endpoints,omitempty"`
}

type HashicorpEndpointsConfig struct {
	Keys        string `toml:"keys"`
	HandShake   string `toml:"hand_shake"`
	WrappingKey string `toml:"wrapping_key"`
	Sign        string `toml:"sign"`
}

func NewHashicorpProvider(chainID, address, key string, token, ca *corev1.SecretKeySelector, autoRenewToken, skipVerify bool) Provider {
	hashicorp := &HashicorpProvider{
		Keys: []*HashicorpKey{
			{
				ChainID: chainID,
				Key:     key,
				Auth: &HashicorpAuth{
					AccessTokenFile: hashicorpMountDir + token.Key,
				},
			},
		},
		Adapter: &HashicorpAdapter{
			VaultAddress:    address,
			VaultCaCert:     "",
			VaultSkipVerify: skipVerify,
		},
		CertificateSecret: ca,
		TokenSecret:       token,
		AutoRenewToken:    autoRenewToken,
	}

	if ca != nil {
		hashicorp.Adapter.VaultCaCert = hashicorpMountDir + ca.Key
	}

	return hashicorp
}

func (v HashicorpProvider) getVolumes() []corev1.Volume {
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

func (v HashicorpProvider) getVolumeMounts() []corev1.VolumeMount {
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

func (v HashicorpProvider) getContainers() []corev1.Container {
	var containers []corev1.Container

	if v.AutoRenewToken {
		spec := corev1.Container{
			Name:  "vault-token-renewer",
			Image: "ghcr.io/nibiruchain/vault-token-renewer",
			Env: []corev1.EnvVar{
				{
					Name:  "VAULT_ADDR",
					Value: v.Adapter.VaultAddress,
				},
				{
					Name:  "VAULT_TOKEN_PATH",
					Value: hashicorpMountDir + v.TokenSecret.Key,
				},
				{
					Name:  "VAULT_SKIP_VERIFY",
					Value: strconv.FormatBool(v.Adapter.VaultSkipVerify),
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

func (v HashicorpProvider) UploadKey(ctx context.Context, kms *KMS, key string) error {
	if len(v.Keys) != 1 {
		return fmt.Errorf("config has no keys configured. this is not supposed to happen")
	}
	hashicorpKey := v.Keys[0]

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
							SecretName: v.TokenSecret.Name,
						},
					},
				},
				{
					Name: "tmkms-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: kms.Name,
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            tmkmsAppName,
					Image:           kms.Config.Image,
					ImagePullPolicy: corev1.PullAlways,
					Args: []string{"hashicorp", "upload", hashicorpKey.Key,
						"--payload", key,
						"--payload-format", "base64",
						"-c", "/data/" + configFileName,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "tmkms-config",
							MountPath: "/data/" + configFileName,
							SubPath:   configFileName,
						},
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
