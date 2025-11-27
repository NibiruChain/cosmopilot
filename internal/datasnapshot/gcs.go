package datasnapshot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

const (
	gcsExporter = "gcs-exporter"
)

type GCS struct {
	Client        *kubernetes.Clientset
	Scheme        *runtime.Scheme
	Owner         metav1.Object
	priorityClass string
	Config        *appsv1.GcsExportConfig
}

func NewGcsSnapshotProvider(client *kubernetes.Clientset, scheme *runtime.Scheme, owner metav1.Object, priorityClass string, cfg *appsv1.GcsExportConfig) SnapshotProvider {
	return &GCS{
		Client:        client,
		Config:        cfg,
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
			TTLSecondsAfterFinished: ptr.To[int32](180),
			BackoffLimit:            ptr.To[int32](0),
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
									SecretName: gcs.Config.CredentialsSecret.Name,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "dataexporter",
							Image:           "ghcr.io/nibiruchain/dataexporter:latest",
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"gcs", "upload", "data", gcs.Config.Bucket, name},
							WorkingDir:      "/home/app",
							Env: []corev1.EnvVar{
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: fmt.Sprintf("/creds/%s", gcs.Config.CredentialsSecret.Key),
								},
								{
									Name:  "SIZE_LIMIT",
									Value: gcs.Config.GetSizeLimit(),
								},
								{
									Name:  "PART_SIZE",
									Value: gcs.Config.GetPartSize(),
								},
								{
									Name:  "CHUNK_SIZE",
									Value: gcs.Config.GetChunkSize(),
								},
								{
									Name:  "BUFFER_SIZE",
									Value: gcs.Config.GetBufferSize(),
								},
								{
									Name:  "CONCURRENT_JOBS",
									Value: strconv.Itoa(gcs.Config.GetConcurrentJobs()),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "credentials",
									MountPath: "/creds",
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

	_, err = gcs.Client.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Create(ctx, pvc, metav1.CreateOptions{})
	return err
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
			TTLSecondsAfterFinished: ptr.To[int32](60),
			BackoffLimit:            ptr.To[int32](5),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:     corev1.RestartPolicyNever,
					PriorityClassName: gcs.priorityClass,
					Volumes: []corev1.Volume{
						{
							Name: "credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: gcs.Config.CredentialsSecret.Name,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "dataexporter",
							Image:           "ghcr.io/nibiruchain/dataexporter:latest",
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"gcs", "delete", gcs.Config.Bucket, name},
							WorkingDir:      "/home/app",
							Env: []corev1.EnvVar{
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: fmt.Sprintf("/creds/%s", gcs.Config.CredentialsSecret.Key),
								},
								{
									Name:  "CONCURRENT_JOBS",
									Value: strconv.Itoa(gcs.Config.GetConcurrentJobs()),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "credentials",
									MountPath: "/creds",
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
