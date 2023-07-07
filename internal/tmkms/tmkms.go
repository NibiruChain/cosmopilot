package tmkms

import (
	"context"
	"time"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/nibiru-operator/internal/k8s"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
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

func (kms *KMS) Deploy(ctx context.Context) error {
	if err := kms.processAllProviders(ctx); err != nil {
		return err
	}

	if err := kms.ensurePVC(ctx); err != nil {
		return err
	}

	if err := kms.ensureConfigMap(ctx); err != nil {
		return err
	}

	return kms.ensureDeployment(ctx)
}

func (kms *KMS) Undeploy(ctx context.Context) error {
	// Delete deployment
	if err := kms.Client.AppsV1().Deployments(kms.Owner.GetNamespace()).Delete(ctx, kms.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete config map
	if err := kms.Client.CoreV1().ConfigMaps(kms.Owner.GetNamespace()).Delete(ctx, kms.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete PVC
	if err := kms.Client.CoreV1().PersistentVolumeClaims(kms.Owner.GetNamespace()).Delete(ctx, kms.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (kms *KMS) processAllProviders(ctx context.Context) error {
	for provider := range kms.Config.Providers {
		for _, p := range kms.Config.Providers[provider] {
			if err := p.process(kms, ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (kms *KMS) getConfigToml() (string, error) {
	return utils.TomlEncode(kms.Config)
}

func (kms *KMS) ensurePVC(ctx context.Context) error {
	size, err := resource.ParseQuantity(DefaultPvcSize)
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
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(kms.Owner, spec, kms.Scheme); err != nil {
		return err
	}

	_, err = kms.Client.CoreV1().PersistentVolumeClaims(kms.Owner.GetNamespace()).Get(ctx, kms.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = kms.Client.CoreV1().PersistentVolumeClaims(kms.Owner.GetNamespace()).Create(ctx, spec, metav1.CreateOptions{})
		}
		return err
	}
	// PVC does not require any updates
	return nil
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

func (kms *KMS) ensureDeployment(ctx context.Context) error {
	// Build volumes list with providers volumes
	volumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: kms.Name,
				},
			},
		},
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

	// Build volume mounts list with provider volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
		},
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

	spec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kms.Name,
			Namespace: kms.Owner.GetNamespace(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelApp:   tmkmsAppName,
					labelOwner: kms.Owner.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelApp:   tmkmsAppName,
						labelOwner: kms.Owner.GetName(),
					},
				},
				Spec: corev1.PodSpec{
					Volumes: volumes,
					InitContainers: []corev1.Container{
						{
							Name:            "tmkms-init",
							Image:           kms.Config.Image,
							ImagePullPolicy: corev1.PullAlways,
							Command: []string{
								"/bin/sh",
								"-c",
								"ls /data/kms-identity.key || (tmkms init /tmp && mv /tmp/secrets/kms-identity.key /data/kms-identity.key)",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            tmkmsAppName,
							Image:           kms.Config.Image,
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"start", "-c", "/data/" + configFileName},
							VolumeMounts:    volumeMounts,
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(kms.Owner, spec, kms.Scheme); err != nil {
		return err
	}

	deployment, err := kms.Client.AppsV1().Deployments(kms.Owner.GetNamespace()).Get(ctx, kms.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			deployment, err = kms.Client.AppsV1().Deployments(kms.Owner.GetNamespace()).Create(ctx, spec, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			return kms.waitDeploymentReady(ctx, deployment)
		}
		return err
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(deployment, spec)
	if err != nil {
		return err
	}

	if !patchResult.IsEmpty() {
		spec.ObjectMeta.ResourceVersion = deployment.ObjectMeta.ResourceVersion
		spec, err = kms.Client.AppsV1().Deployments(kms.Owner.GetNamespace()).Update(ctx, spec, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		deployment = spec
	}

	return kms.waitDeploymentReady(ctx, deployment)
}

func (kms *KMS) waitDeploymentReady(ctx context.Context, deployment *appsv1.Deployment) error {
	dh := k8s.NewDeploymentHelper(kms.Client, nil, deployment)
	return dh.WaitForCondition(ctx, func(deployment *appsv1.Deployment) (bool, error) {
		return deployment.Status.ReadyReplicas == 1, nil
	}, time.Minute)
}
