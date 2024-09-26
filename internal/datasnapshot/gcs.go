package datasnapshot

import (
	"context"
	"fmt"
	"strings"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	gcsExporter = "gcs-exporter"
)

type GCS struct {
	Client        *kubernetes.Clientset
	Scheme        *runtime.Scheme
	Bucket        string
	Credentials   *corev1.SecretKeySelector
	Owner         metav1.Object
	priorityClass string
}

func NewGcsSnapshotProvider(client *kubernetes.Clientset, scheme *runtime.Scheme, owner metav1.Object, bucket string, creds *corev1.SecretKeySelector, priorityClass string) SnapshotProvider {
	return &GCS{
		Client:        client,
		Bucket:        bucket,
		Credentials:   creds,
		Owner:         owner,
		Scheme:        scheme,
		priorityClass: priorityClass,
	}
}

func (gcs *GCS) CreateSnapshot(ctx context.Context, name string, vs *snapshotv1.VolumeSnapshot) error {
	if vs.Status.RestoreSize == nil {
		return fmt.Errorf("restore size is not available yet")
	}

	apiVersion := strings.Split(vs.APIVersion, "/")
	if len(apiVersion) == 0 {
		return fmt.Errorf("unsupported api version")
	}

	// Create job to compress and upload tarball
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-upload", name),
			Namespace: gcs.Owner.GetNamespace(),
			Labels: map[string]string{
				labelExporter: gcsExporter,
				labelOwner:    gcs.Owner.GetName(),
				labelType:     typeUpload,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(180),
			BackoffLimit:            pointer.Int32(0),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:     corev1.RestartPolicyNever,
					PriorityClassName: gcs.priorityClass,
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: fmt.Sprintf("%s-upload", name),
								},
							},
						},
						{
							Name: "credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: gcs.Credentials.Name,
								},
							},
						},
						{
							Name: "gcloud-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:            "gcloud-auth",
							Image:           "google/cloud-sdk:alpine",
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"gcloud"},
							Args:            []string{"auth", "activate-service-account", fmt.Sprintf("--key-file=/creds/%s", gcs.Credentials.Key)},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gcloud-config",
									MountPath: "/root/.config",
								},
								{
									Name:      "credentials",
									MountPath: "/creds",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "gcloud",
							Image:           "google/cloud-sdk:alpine",
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"sh", "-c"},
							Args: []string{
								fmt.Sprintf("tar czf - data | gsutil cp - gs://%s/%s.tar.gz", gcs.Bucket, name),
							},
							WorkingDir: "/home/app",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gcloud-config",
									MountPath: "/root/.config",
								},
								{
									Name:      "data",
									MountPath: "/home/app/data",
								},
							},
						},
					},
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(gcs.Owner, job, gcs.Scheme)
	if err != nil {
		return err
	}

	job, err = gcs.Client.BatchV1().Jobs(gcs.Owner.GetNamespace()).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Create PVC from Snapshot
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-upload", name),
			Namespace: gcs.Owner.GetNamespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *vs.Status.RestoreSize,
				},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: &apiVersion[0],
				Kind:     vs.Kind,
				Name:     vs.Name,
			},
		},
	}

	err = controllerutil.SetControllerReference(job, pvc, gcs.Scheme)
	if err != nil {
		return err
	}

	pvc, err = gcs.Client.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (gcs *GCS) GetSnapshotStatus(ctx context.Context, name string) (SnapshotStatus, error) {
	job, err := gcs.Client.BatchV1().Jobs(gcs.Owner.GetNamespace()).Get(ctx, fmt.Sprintf("%s-upload", name), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return SnapshotNotFound, nil
		}
		return "", err
	}

	switch {
	case job.Status.Active > 0:
		return SnapshotActive, nil

	case job.Status.Failed > 0:
		return SnapshotFailed, nil

	case job.Status.Succeeded >= 1:
		return SnapshotSucceeded, gcs.cleanUp(ctx, name)

	default:
		return "", fmt.Errorf("could not determine job status")
	}
}

func (gcs *GCS) cleanUp(ctx context.Context, name string) error {
	propagation := metav1.DeletePropagationBackground
	return gcs.Client.BatchV1().Jobs(gcs.Owner.GetNamespace()).Delete(ctx, fmt.Sprintf("%s-upload", name), metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}

func (gcs *GCS) DeleteSnapshot(ctx context.Context, name string) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-delete", name),
			Namespace: gcs.Owner.GetNamespace(),
			Labels: map[string]string{
				labelExporter: gcsExporter,
				labelOwner:    gcs.Owner.GetName(),
				labelType:     typeDelete,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(60),
			BackoffLimit:            pointer.Int32(5),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:     corev1.RestartPolicyNever,
					PriorityClassName: gcs.priorityClass,
					Volumes: []corev1.Volume{
						{
							Name: "credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: gcs.Credentials.Name,
								},
							},
						},
						{
							Name: "gcloud-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:            "gcloud-auth",
							Image:           "google/cloud-sdk:alpine",
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"gcloud"},
							Args:            []string{"auth", "activate-service-account", fmt.Sprintf("--key-file=/creds/%s", gcs.Credentials.Key)},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gcloud-config",
									MountPath: "/root/.config",
								},
								{
									Name:      "credentials",
									MountPath: "/creds",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "gcloud",
							Image:           "google/cloud-sdk:alpine",
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"sh", "-c"},
							Args: []string{
								fmt.Sprintf("gsutil rm gs://%s/%s.tar.gz", gcs.Bucket, name),
							},
							WorkingDir: "/home/app",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gcloud-config",
									MountPath: "/root/.config",
								},
							},
						},
					},
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(gcs.Owner, job, gcs.Scheme)
	if err != nil {
		return err
	}

	if _, err = gcs.Client.BatchV1().Jobs(gcs.Owner.GetNamespace()).Create(ctx, job, metav1.CreateOptions{}); err != nil {
		return err
	}
	return gcs.cleanUp(ctx, name)
}

func (gcs *GCS) ListSnapshots(ctx context.Context) ([]string, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			labelExporter: gcsExporter,
			labelOwner:    gcs.Owner.GetName(),
			labelType:     typeUpload, // We only list uploading jobs since the deleting jobs are auto deleted
		}).String(),
	}
	list, err := gcs.Client.BatchV1().Jobs(gcs.Owner.GetNamespace()).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	snapshotNames := make([]string, len(list.Items))
	for i, job := range list.Items {
		snapshotNames[i] = strings.TrimSuffix(job.GetName(), "-upload")
	}
	return snapshotNames, nil
}
