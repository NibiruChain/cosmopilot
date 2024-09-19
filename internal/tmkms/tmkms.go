package tmkms

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/cosmopilot/internal/k8s"
	"github.com/NibiruChain/cosmopilot/internal/utils"
)

const (
	tmkmsCpu     = "100m"
	tmkmsMemory  = "64Mi"
	tmkmsPvcSize = "1Gi"
)

var (
	tmkmsCpuResources    = resource.MustParse(tmkmsCpu)
	tmkmsMemoryResources = resource.MustParse(tmkmsMemory)
)

type KMS struct {
	Name   string
	Owner  metav1.Object
	Client *kubernetes.Clientset
	Scheme *runtime.Scheme
	Config *Config
}

func New(client *kubernetes.Clientset, scheme *runtime.Scheme, name string, owner metav1.Object, opts ...Option) *KMS {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &KMS{
		Name:   name,
		Owner:  owner,
		Client: client,
		Scheme: scheme,
		Config: cfg,
	}
}

func (kms *KMS) DeployConfig(ctx context.Context) error {
	if err := kms.ensureIdentityKey(ctx); err != nil {
		return err
	}

	if err := kms.ensureConfigMap(ctx); err != nil {
		return err
	}

	if kms.Config.PersistState {
		return kms.ensurePVC(ctx)
	}
	return nil
}

func (kms *KMS) UndeployConfig(ctx context.Context) error {
	// Delete config map
	if err := kms.Client.CoreV1().ConfigMaps(kms.Owner.GetNamespace()).Delete(ctx, kms.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete Secret
	if err := kms.Client.CoreV1().Secrets(kms.Owner.GetNamespace()).Delete(ctx, kms.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (kms *KMS) getConfigToml() (string, error) {
	return utils.TomlEncode(kms.Config)
}

func (kms *KMS) getConfigHash() string {
	cfg, _ := kms.getConfigToml()
	return utils.Sha256(cfg)
}

func (kms *KMS) ensureIdentityKey(ctx context.Context) error {
	_, err := kms.Client.CoreV1().Secrets(kms.Owner.GetNamespace()).Get(ctx, kms.Name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		var key string
		key, err = kms.generateKmsIdentityKey(ctx)
		if err != nil {
			return err
		}

		spec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kms.Name,
				Namespace: kms.Owner.GetNamespace(),
			},
			Immutable: pointer.Bool(true),
			Data:      map[string][]byte{identityKeyName: []byte(key)},
		}
		_, err = kms.Client.CoreV1().Secrets(kms.Owner.GetNamespace()).Create(ctx, spec, metav1.CreateOptions{})
	}
	return err
}

func (kms *KMS) generateKmsIdentityKey(ctx context.Context) (string, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-generate-identity", kms.Name),
			Namespace: kms.Owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            tmkmsAppName,
					Image:           kms.Config.Image,
					ImagePullPolicy: corev1.PullAlways,
					Command:         []string{"/bin/sh", "-c", "tmkms init /root/tmkms"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/root/tmkms",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           kms.Config.Image,
					ImagePullPolicy: corev1.PullAlways,
					Command:         []string{"cat", "/root/tmkms/secrets/" + identityKeyName},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/root/tmkms",
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(kms.Owner, pod, kms.Scheme); err != nil {
		return "", err
	}

	ph := k8s.NewPodHelper(kms.Client, nil, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer ph.Delete(ctx)

	if err := ph.Create(ctx); err != nil {
		return "", err
	}

	// Wait for the pod to finish
	if err := ph.WaitForPodSucceeded(ctx, time.Minute); err != nil {
		return "", err
	}

	// Grab identity key file content
	out, err := ph.GetLogs(ctx, "busybox")
	return strings.TrimSpace(out), err
}

func (kms *KMS) ensureConfigMap(ctx context.Context) error {
	config, err := kms.getConfigToml()
	if err != nil {
		return err
	}

	spec := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kms.Name,
			Namespace: kms.Owner.GetNamespace(),
		},
		Data: map[string]string{configFileName: config},
	}
	if err := controllerutil.SetControllerReference(kms.Owner, spec, kms.Scheme); err != nil {
		return err
	}

	cm, err := kms.Client.CoreV1().ConfigMaps(kms.Owner.GetNamespace()).Get(ctx, kms.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = kms.Client.CoreV1().ConfigMaps(kms.Owner.GetNamespace()).Create(ctx, spec, metav1.CreateOptions{})
		}
		return err
	}

	// Update when config changes
	if cm.Data[configFileName] != config {
		cm.Data[configFileName] = config
		_, err = kms.Client.CoreV1().ConfigMaps(kms.Owner.GetNamespace()).Update(ctx, cm, metav1.UpdateOptions{})
	}
	return err
}

func (kms *KMS) ensurePVC(ctx context.Context) error {
	storageSize, err := resource.ParseQuantity(tmkmsPvcSize)
	if err != nil {
		return err
	}

	spec := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kms.Name,
			Namespace: kms.Owner.GetNamespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	_, err = kms.Client.CoreV1().PersistentVolumeClaims(kms.Owner.GetNamespace()).Get(ctx, kms.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = kms.Client.CoreV1().PersistentVolumeClaims(kms.Owner.GetNamespace()).Create(ctx, spec, metav1.CreateOptions{})
		}
		return err
	}
	return nil
}

func (kms *KMS) GetVolumes() []corev1.Volume {
	// Build volumes list with providers volumes
	volumes := []corev1.Volume{
		{
			Name: "tmkms-identity",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kms.Name,
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
	}

	if kms.Config.PersistState {
		volumes = append(volumes, corev1.Volume{
			Name: "tmkms-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: kms.Name,
				},
			},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: "tmkms-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	for provider := range kms.Config.Providers {
		for _, p := range kms.Config.Providers[provider] {
			volumes = append(volumes, p.getVolumes()...)
		}
	}

	return volumes
}

func (kms *KMS) GetContainersSpec() []corev1.Container {
	// Build volume mounts list with provider volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "tmkms-data",
			MountPath: "/data",
		},
		{
			Name:      "tmkms-identity",
			MountPath: "/data/" + identityKeyName,
			SubPath:   identityKeyName,
		},
		{
			Name:      "tmkms-config",
			MountPath: "/data/" + configFileName,
			SubPath:   configFileName,
		},
	}
	for provider := range kms.Config.Providers {
		for _, p := range kms.Config.Providers[provider] {
			volumeMounts = append(volumeMounts, p.getVolumeMounts()...)
		}
	}

	containers := []corev1.Container{
		{
			Name:            tmkmsAppName,
			Image:           kms.Config.Image,
			ImagePullPolicy: corev1.PullAlways,
			Args:            []string{"start", "-c", "/data/" + configFileName},
			VolumeMounts:    volumeMounts,
			Env: []corev1.EnvVar{
				{
					Name:  "ROLLME",
					Value: kms.getConfigHash(),
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    tmkmsCpuResources,
					corev1.ResourceMemory: tmkmsMemoryResources,
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    tmkmsCpuResources,
					corev1.ResourceMemory: tmkmsMemoryResources,
				},
			},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/tmkms_active",
						Port: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 8000,
						},
						Scheme: "HTTP",
					},
				},
				// Startup failure after 1h (not likely to happen)
				FailureThreshold: 720,
				PeriodSeconds:    5,
				TimeoutSeconds:   5,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/tmkms_active",
						Port: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 8000,
						},
						Scheme: "HTTP",
					},
				},
				FailureThreshold: 2,
				PeriodSeconds:    2,
				TimeoutSeconds:   5,
			},
		},
	}

	for provider := range kms.Config.Providers {
		for _, p := range kms.Config.Providers[provider] {
			containers = append(containers, p.getContainers()...)
		}
	}

	return containers
}
