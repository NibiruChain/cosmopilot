package chainnode

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/internal/datasnapshot"
	"github.com/NibiruChain/cosmopilot/internal/k8s"
	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

type SnapshotIntegrityStatus string

const (
	timeLayout = "20060102150405"

	snapshotIntegrityChecking  SnapshotIntegrityStatus = "checking"
	snapshotIntegrityOk        SnapshotIntegrityStatus = "ok"
	snapshotIntegrityCorrupted SnapshotIntegrityStatus = "corrupted"
)

func (r *Reconciler) ensureVolumeSnapshots(ctx context.Context, chainNode *appsv1.ChainNode, nodePodReady bool) error {
	logger := log.FromContext(ctx)

	if !chainNode.SnapshotsEnabled() || chainNode.Status.PvcSize == "" || chainNode.Status.LatestHeight == 0 {
		return nil
	}

	// Get list of snapshots
	snapshots, err := r.listNodeSnapshots(ctx, chainNode)
	if err != nil {
		return err
	}

	// Fix snapshotting status in case there are no snapshots for this node
	if len(snapshots) == 0 && volumeSnapshotInProgress(chainNode) {
		setSnapshotInProgress(chainNode, false)
		if err = r.Update(ctx, chainNode); err != nil {
			return err
		}
	}

	// Grab list of possible tarball names to make sure we delete any possible dangling jobs
	tarballNames := make([]string, 0)

	for _, snapshot := range snapshots {
		if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() {
			tarballNames = append(tarballNames, getTarballName(chainNode, &snapshot))
		}

		switch {

		// If the snapshot does not have the ready annotation, we haven't processed it yet. So we check if it's ready
		// and if it is, we mark it as ready and register its timestamp in chainnode.
		// In case tarball export is enabled, we also start the export right away, unless integrity checks are enabled,
		// in that case integrity check starts first
		case snapshot.Annotations[controllers.AnnotationPvcSnapshotReady] == strconv.FormatBool(false) && isSnapshotReady(&snapshot):
			logger.Info("pvc snapshot has finished", "snapshot", snapshot.GetName())
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonFinishedSnapshot,
				"Finished PVC snapshot %s", snapshot.GetName(),
			)

			// Update snapshot ready annotation
			snapshot.ObjectMeta.Annotations[controllers.AnnotationPvcSnapshotReady] = strconv.FormatBool(true)
			if err = r.Update(ctx, &snapshot); err != nil {
				return err
			}

			// Update ChainNode status
			setSnapshotInProgress(chainNode, false)
			setSnapshotTime(chainNode, snapshot.CreationTimestamp.Time)
			if err = r.Update(ctx, chainNode); err != nil {
				return err
			}

			// If verify is enabled, lets start it now. If not, let's start tarball export if its enabled
			if chainNode.Spec.Persistence.Snapshots.ShouldVerify() {
				logger.Info("starting data integrity check", "snapshot", snapshot.GetName())
				if err = r.startSnapshotIntegrityCheck(ctx, chainNode, &snapshot); err != nil {
					return err
				}
				snapshot.ObjectMeta.Annotations[controllers.AnnotationSnapshotIntegrityStatus] = string(snapshotIntegrityChecking)
				if err = r.Update(ctx, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonSnapshotIntegrityStart,
					"Starting snapshot %s integrity check", snapshot.GetName(),
				)
			} else if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() {
				logger.Info("starting tarball export", "snapshot", snapshot.GetName())
				if err = r.exportTarball(ctx, chainNode, &snapshot); err != nil {
					return err
				}
				snapshot.ObjectMeta.Annotations[controllers.AnnotationExportingTarball] = strconv.FormatBool(true)
				if err = r.Update(ctx, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonTarballExportStart,
					"Exporting tarball %s from snapshot", getTarballName(chainNode, &snapshot),
				)
			}

		// Let's start the verification job if not started yet, and check if it has completed otherwise.
		case chainNode.Spec.Persistence.Snapshots.ShouldVerify() &&
			snapshot.Annotations[controllers.AnnotationSnapshotIntegrityStatus] != string(snapshotIntegrityOk) &&
			snapshot.Annotations[controllers.AnnotationSnapshotIntegrityStatus] != string(snapshotIntegrityCorrupted):

			status, err := r.getSnapshotIntegrityCheckStatus(ctx, chainNode, &snapshot)
			if err != nil {
				return err
			}

			switch status {
			case snapshotIntegrityChecking:
				logger.Info("data integrity check in progress", "snapshot", snapshot.GetName())

			case snapshotIntegrityOk:
				logger.Info("data integrity check finished successfully. Data is ok.", "snapshot", snapshot.GetName())
				snapshot.ObjectMeta.Annotations[controllers.AnnotationSnapshotIntegrityStatus] = string(snapshotIntegrityOk)
				if err = r.Update(ctx, &snapshot); err != nil {
					return err
				}

				// Let's start the tarball export right now if it is enabled
				if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() && snapshot.Annotations[controllers.AnnotationExportingTarball] == "" {
					logger.Info("starting tarball export", "snapshot", snapshot.GetName())
					if err = r.exportTarball(ctx, chainNode, &snapshot); err != nil {
						return err
					}
					snapshot.ObjectMeta.Annotations[controllers.AnnotationExportingTarball] = strconv.FormatBool(true)
					if err = r.Update(ctx, &snapshot); err != nil {
						return err
					}
					r.recorder.Eventf(chainNode,
						corev1.EventTypeNormal,
						appsv1.ReasonTarballExportStart,
						"Exporting tarball %s from snapshot", getTarballName(chainNode, &snapshot),
					)
				}

			case snapshotIntegrityCorrupted:
				logger.Info("data integrity check finished. Data is corrupted.", "snapshot", snapshot.GetName())
				snapshot.ObjectMeta.Annotations[controllers.AnnotationSnapshotIntegrityStatus] = string(snapshotIntegrityCorrupted)
				if err = r.Update(ctx, &snapshot); err != nil {
					return err
				}
				logger.Info("re-creating snapshot")
				if err = r.Delete(ctx, &snapshot); err != nil {
					return err
				}
				return r.startNewSnapshot(ctx, chainNode)

			default:
				// Integrity check job was not started yet. Let's start it.
				logger.Info("starting data integrity check", "snapshot", snapshot.GetName())

				if err = r.startSnapshotIntegrityCheck(ctx, chainNode, &snapshot); err != nil {
					return err
				}
				snapshot.ObjectMeta.Annotations[controllers.AnnotationSnapshotIntegrityStatus] = string(snapshotIntegrityChecking)
				if err = r.Update(ctx, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonSnapshotIntegrityStart,
					"Starting snapshot %s integrity check", snapshot.GetName(),
				)
			}

		// If for some reason, there is an error starting the tarball export, it is never retried. So we do it here.
		case chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() &&
			(!chainNode.Spec.Persistence.Snapshots.ShouldVerify() || snapshot.Annotations[controllers.AnnotationSnapshotIntegrityStatus] == string(snapshotIntegrityOk)) &&
			snapshot.Annotations[controllers.AnnotationPvcSnapshotReady] == strconv.FormatBool(true) &&
			snapshot.Annotations[controllers.AnnotationExportingTarball] == "":
			logger.Info("starting tarball export", "snapshot", snapshot.GetName())
			if err = r.exportTarball(ctx, chainNode, &snapshot); err != nil {
				return err
			}
			snapshot.ObjectMeta.Annotations[controllers.AnnotationExportingTarball] = strconv.FormatBool(true)
			if err = r.Update(ctx, &snapshot); err != nil {
				return err
			}
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonTarballExportStart,
				"Exporting tarball %s from snapshot", getTarballName(chainNode, &snapshot),
			)

		// If the exporting-tarball annotation is set to true, then the export was started. We need to check if it
		// has finished, and if it is, we set the export-tarballl annotation to finished so that it won't be processed
		// again
		case chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() &&
			snapshot.Annotations[controllers.AnnotationPvcSnapshotReady] == strconv.FormatBool(true) &&
			snapshot.Annotations[controllers.AnnotationExportingTarball] == strconv.FormatBool(true):
			ready, err := r.isTarballReady(ctx, chainNode, &snapshot)
			if err != nil {
				r.recorder.Eventf(chainNode,
					corev1.EventTypeWarning,
					appsv1.ReasonTarballExportError,
					"Error on tarball export %v", err,
				)
				return err
			}
			if ready {
				logger.Info("finished tarball export", "snapshot", snapshot.GetName())
				snapshot.Annotations[controllers.AnnotationExportingTarball] = tarballFinished
				if err = r.Update(ctx, &snapshot); err != nil {
					return err
				}
			}

		// Default case is checking if snapshot has expired. If tarball is also set for deletion on expire it is also
		// taken care here.
		default:
			expired, err := isSnapshotExpired(&snapshot)
			if err != nil {
				return err
			}
			if expired {
				logger.Info("deleting expired pvc snapshot", "snapshot", snapshot.GetName(), "retention", snapshot.Annotations[controllers.AnnotationSnapshotRetention])
				if err = r.Delete(ctx, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonDeletedSnapshot,
					"Deleted expired PVC snapshot %s", snapshot.GetName(),
				)
				if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() && chainNode.Spec.Persistence.Snapshots.ExportTarball.DeleteWhenExpired() {
					logger.Info("deleting expired snapshot tarball", "snapshot", snapshot.GetName(), "retention", snapshot.Annotations[controllers.AnnotationSnapshotRetention])
					if err = r.deleteTarball(ctx, chainNode, &snapshot); err != nil {
						return err
					}
					r.recorder.Eventf(chainNode,
						corev1.EventTypeNormal,
						appsv1.ReasonTarballDeleted,
						"Deleted expired tarball %s", getTarballName(chainNode, &snapshot),
					)
				}
			}
		}
	}

	// Remove any dangling jobs whose volumesnapshot does not exist anymore
	if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() {
		exporter, err := r.getTarballExportProvider(chainNode)
		if err != nil {
			return err
		}
		tarballSnapshots, err := exporter.ListSnapshots(ctx)
		if err != nil {
			return err
		}
		for _, snapshot := range tarballSnapshots {
			if !utils.SliceContains[string](tarballNames, snapshot) {
				logger.Info("deleting orphaned tarball upload job as volumesnapshot does not exist anymore", "snapshot", snapshot)
				if err = exporter.DeleteSnapshot(ctx, snapshot); err != nil {
					return err
				}
			}
		}
	}

	// We don't want to have more than one snapshot being taken at the same time
	if volumeSnapshotInProgress(chainNode) {
		return nil
	}

	// Create a snapshot if it's time for that
	if shouldSnapshot(chainNode, nodePodReady) {
		logger.Info("creating new pvc snapshot")
		return r.startNewSnapshot(ctx, chainNode)
	}

	return nil
}

func (r *Reconciler) listNodeSnapshots(ctx context.Context, chainNode *appsv1.ChainNode) ([]snapshotv1.VolumeSnapshot, error) {
	listOption := client.MatchingLabels{controllers.LabelChainNode: chainNode.GetName()}
	list := &snapshotv1.VolumeSnapshotList{}
	if err := r.List(ctx, list, listOption); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *Reconciler) startNewSnapshot(ctx context.Context, chainNode *appsv1.ChainNode) error {
	snapshot, err := r.createSnapshot(ctx, chainNode)
	if err != nil {
		return err
	}

	// If snapshot is nil then it was not started
	if snapshot == nil {
		return nil
	}

	r.recorder.Eventf(chainNode,
		corev1.EventTypeNormal,
		appsv1.ReasonStartedSnapshot,
		"Started PVC snapshot %s", snapshot.GetName(),
	)

	setSnapshotInProgress(chainNode, true)
	if err := r.Update(ctx, chainNode); err != nil {
		return err
	}
	return r.updatePhase(ctx, chainNode, appsv1.PhaseChainNodeSnapshotting)
}

func (r *Reconciler) createSnapshot(ctx context.Context, chainNode *appsv1.ChainNode) (*snapshotv1.VolumeSnapshot, error) {
	logger := log.FromContext(ctx)

	if chainNode.Spec.Persistence.Snapshots.ShouldStopNode() {
		pod, err := r.getPodSpec(ctx, chainNode, "")
		if err != nil {
			return nil, err
		}

		ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
		if err := ph.Delete(ctx); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			if err := ph.WaitForPodDeleted(ctx, timeoutPodDeleted); err != nil {
				return nil, err
			}
		}
	}
	if err := r.updateLatestHeight(ctx, chainNode); err != nil {
		// When this error happens, the most likely scenario is that pod is not running. So lets not throw the error and
		// let the rest of the reconcile loop handle the missing pod.
		logger.Error(err, "error getting latest height (pod is probably missing)")
		return nil, nil
	}
	snapshot := getVolumeSnapshotSpec(chainNode)
	return snapshot, r.Create(ctx, snapshot)
}

func shouldSnapshot(chainNode *appsv1.ChainNode, nodePodReady bool) bool {
	switch {
	case chainNode.Spec.Persistence.Snapshots.ShouldDisableWhileSyncing() && chainNode.Status.Phase == appsv1.PhaseChainNodeSyncing:
		return false
	case chainNode.Spec.Persistence.Snapshots.ShouldDisableWhileUnhealthy() && !nodePodReady:
		return false
	case chainNode.Spec.Persistence.Snapshots.ShouldDisableWhileUnhealthy() && chainNode.Status.Phase != appsv1.PhaseChainNodeRunning:
		return false
	}

	period, err := strfmt.ParseDuration(chainNode.Spec.Persistence.Snapshots.Frequency)
	if err != nil {
		return false
	}
	lastSnapshotTime := getLastSnapshotTime(chainNode)
	if lastSnapshotTime.IsZero() {
		return chainNode.CreationTimestamp.UTC().Add(minimumTimeBeforeFirstSnapshot).Before(time.Now().UTC())
	}
	return lastSnapshotTime.Add(period).Before(time.Now().UTC())
}

func isSnapshotReady(snapshot *snapshotv1.VolumeSnapshot) bool {
	return snapshot != nil && snapshot.Status != nil && snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse
}

func isSnapshotExpired(snapshot *snapshotv1.VolumeSnapshot) (bool, error) {
	retention, ok := snapshot.Annotations[controllers.AnnotationSnapshotRetention]
	if !ok {
		return false, nil
	}

	expiration, err := strfmt.ParseDuration(retention)
	if err != nil {
		return false, err
	}

	return snapshot.CreationTimestamp.UTC().Add(expiration).Before(time.Now().UTC()), nil
}

func getVolumeSnapshotSpec(chainNode *appsv1.ChainNode) *snapshotv1.VolumeSnapshot {
	spec := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getSnapshotName(chainNode),
			Namespace: chainNode.GetNamespace(),
			Annotations: map[string]string{
				controllers.AnnotationPvcSnapshotReady: strconv.FormatBool(false),
				controllers.AnnotationDataHeight:       strconv.FormatInt(chainNode.Status.LatestHeight, 10),
			},
			Labels: WithChainNodeLabels(chainNode, map[string]string{
				controllers.LabelChainNode: chainNode.GetName(),
			}),
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: pointer.String(chainNode.GetName()),
			},
			VolumeSnapshotClassName: chainNode.Spec.Persistence.Snapshots.SnapshotClassName,
		},
	}

	if chainNode.Spec.Persistence.Snapshots.Retention != nil {
		spec.ObjectMeta.Annotations[controllers.AnnotationSnapshotRetention] = *chainNode.Spec.Persistence.Snapshots.Retention
	}

	return spec
}

func volumeSnapshotInProgress(chainNode *appsv1.ChainNode) bool {
	if chainNode.ObjectMeta.Annotations == nil {
		return false
	}
	v, ok := chainNode.ObjectMeta.Annotations[controllers.AnnotationPvcSnapshotInProgress]
	if !ok {
		return false
	}
	return v == strconv.FormatBool(true)
}

func setSnapshotInProgress(chainNode *appsv1.ChainNode, snapshotting bool) {
	if chainNode.ObjectMeta.Annotations == nil {
		chainNode.ObjectMeta.Annotations = make(map[string]string)
	}
	chainNode.ObjectMeta.Annotations[controllers.AnnotationPvcSnapshotInProgress] = strconv.FormatBool(snapshotting)
	if snapshotting {
		chainNode.Status.Phase = appsv1.PhaseChainNodeSnapshotting
	} else {
		chainNode.Status.Phase = appsv1.PhaseChainNodeRunning
	}
}

func setSnapshotTime(chainNode *appsv1.ChainNode, ts time.Time) {
	if chainNode.ObjectMeta.Annotations == nil {
		chainNode.ObjectMeta.Annotations = make(map[string]string)
	}
	chainNode.ObjectMeta.Annotations[controllers.AnnotationLastPvcSnapshot] = ts.UTC().Format(timeLayout)
}

func getLastSnapshotTime(chainNode *appsv1.ChainNode) time.Time {
	if s, ok := chainNode.ObjectMeta.Annotations[controllers.AnnotationLastPvcSnapshot]; ok {
		if ts, err := time.Parse(timeLayout, s); err == nil {
			return ts.UTC()
		}
	}
	return time.Time{}
}

func getSnapshotName(chainNode *appsv1.ChainNode) string {
	name := chainNode.GetName()

	// When taking snapshots from a chainnode that belongs to chainnodeset group, we will only snapshot
	// from one of the group nodes, so we give it the group name instead.
	if group, ok := chainNode.Labels[controllers.LabelChainNodeSetGroup]; ok && group != "" {
		if nodeset, ok := chainNode.Labels[controllers.LabelChainNodeSet]; ok && nodeset != "" {
			name = fmt.Sprintf("%s-%s", nodeset, group)
		}

	}
	return fmt.Sprintf("%s-%s", name, time.Now().UTC().Format(timeLayout))
}

func (r *Reconciler) getTarballExportProvider(chainNode *appsv1.ChainNode) (datasnapshot.SnapshotProvider, error) {
	switch {
	case chainNode.Spec.Persistence.Snapshots.ExportTarball.GCS != nil:
		return datasnapshot.NewGcsSnapshotProvider(
			r.ClientSet,
			r.Scheme,
			chainNode,
			r.opts.GetDefaultPriorityClassName(),
			chainNode.Spec.Persistence.Snapshots.ExportTarball.GCS,
		), nil

	default:
		return nil, fmt.Errorf("no upload target defined")
	}
}

func (r *Reconciler) exportTarball(ctx context.Context, chainNode *appsv1.ChainNode, snapshot *snapshotv1.VolumeSnapshot) error {
	exporter, err := r.getTarballExportProvider(chainNode)
	if err != nil {
		return err
	}
	return exporter.CreateSnapshot(ctx, getTarballName(chainNode, snapshot), snapshot)
}

func (r *Reconciler) isTarballReady(ctx context.Context, chainNode *appsv1.ChainNode, snapshot *snapshotv1.VolumeSnapshot) (bool, error) {
	exporter, err := r.getTarballExportProvider(chainNode)
	if err != nil {
		return false, err
	}

	status, err := exporter.GetSnapshotStatus(ctx, getTarballName(chainNode, snapshot))
	if err != nil {
		return false, err
	}

	switch status {
	case datasnapshot.SnapshotNotFound:
		r.recorder.Eventf(chainNode,
			corev1.EventTypeWarning,
			appsv1.ReasonTarballExportError,
			"Tarball %s export job not found", getTarballName(chainNode, snapshot),
		)
		return true, nil

	case datasnapshot.SnapshotSucceeded:
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonTarballExportFinish,
			"Finished exporting tarball %s", getTarballName(chainNode, snapshot),
		)
		return true, nil

	default:
		return false, nil
	}
}

func (r *Reconciler) deleteTarball(ctx context.Context, chainNode *appsv1.ChainNode, snapshot *snapshotv1.VolumeSnapshot) error {
	exporter, err := r.getTarballExportProvider(chainNode)
	if err != nil {
		return err
	}
	return exporter.DeleteSnapshot(ctx, getTarballName(chainNode, snapshot))
}

func getTarballName(chainNode *appsv1.ChainNode, snapshot *snapshotv1.VolumeSnapshot) string {
	name := fmt.Sprintf("%s-%s", chainNode.Status.ChainID, snapshot.CreationTimestamp.UTC().Format(timeLayout))
	if chainNode.Spec.Persistence.Snapshots.ExportTarball.Suffix != nil {
		name += "-" + *chainNode.Spec.Persistence.Snapshots.ExportTarball.Suffix
	}
	return name
}

func (r *Reconciler) startSnapshotIntegrityCheck(ctx context.Context, chainNode *appsv1.ChainNode, snapshot *snapshotv1.VolumeSnapshot) error {
	if snapshot.Status.RestoreSize == nil {
		return fmt.Errorf("restore size is not available yet")
	}

	apiVersion := strings.Split(snapshot.APIVersion, "/")
	if len(apiVersion) == 0 {
		return fmt.Errorf("unsupported api version")
	}

	var sidecarRestartAlways = corev1.ContainerRestartPolicyAlways

	// Create job to verify data integrity
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ichk", snapshot.GetName()),
			Namespace: chainNode.GetNamespace(),
			Labels: map[string]string{
				volumeSnapshot: snapshot.GetName(),
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(60),
			BackoffLimit:            pointer.Int32(1),
			PodFailurePolicy: &batchv1.PodFailurePolicy{
				Rules: []batchv1.PodFailurePolicyRule{
					// 1) Count real checker failures (anything except 137/143)
					{
						Action: batchv1.PodFailurePolicyActionCount,
						OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{
							ContainerName: pointer.String("start-checker"),
							Operator:      batchv1.PodFailurePolicyOnExitCodesOpNotIn,
							Values:        []int32{137, 143},
						},
					},
					// 2) Ignore kube-initiated disruptions (eviction/drain/preemption)
					{
						Action: batchv1.PodFailurePolicyActionIgnore,
						OnPodConditions: []batchv1.PodFailurePolicyOnPodConditionsPattern{
							{
								Type:   corev1.DisruptionTarget,
								Status: corev1.ConditionTrue,
							},
						},
					},
					// 3) Ignore external kill paths (OOM/SIGTERM)
					{
						Action: batchv1.PodFailurePolicyActionIgnore,
						OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{
							Operator: batchv1.PodFailurePolicyOnExitCodesOpIn,
							Values:   []int32{137, 143},
						},
					},
				},
			},
			Completions: pointer.Int32(1),
			Parallelism: pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:         corev1.RestartPolicyNever,
					PriorityClassName:     r.opts.GetDefaultPriorityClassName(),
					ShareProcessNamespace: pointer.Bool(true),
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: fmt.Sprintf("%s-ichk", snapshot.GetName()),
								},
							},
						},
						{
							Name: "config-empty-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: chainNode.GetName(),
									},
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:    "init-config",
							Image:   "busybox",
							Command: []string{"sh"},
							Args: []string{
								"-c",
								fmt.Sprintf(
									"cp -rL /node-config/* /home/app/config/;"+
										"sed -i 's/iavl-lazy-loading =.*/iavl-lazy-loading = false/g' /home/app/config/app.toml;"+
										"echo '{\"chain_id\":%q}' > /home/app/config/genesis.json",
									chainNode.Status.ChainID,
								),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-empty-dir",
									MountPath: "/home/app/config",
								},
								{
									Name:      "config",
									MountPath: "/node-config",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    lightContainerCpuResources,
									corev1.ResourceMemory: lightContainerMemoryResources,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    lightContainerCpuResources,
									corev1.ResourceMemory: lightContainerMemoryResources,
								},
							},
						},
						{
							Name:            chainNode.Spec.App.App,
							Image:           chainNode.GetAppImage(),
							ImagePullPolicy: chainNode.Spec.App.GetImagePullPolicy(),
							RestartPolicy:   &sidecarRestartAlways,
							Command:         []string{chainNode.Spec.App.App},
							Args:            []string{"start", "--grpc-only", "--home", "/home/app"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/home/app/data",
								},
								{
									Name:      "config-empty-dir",
									MountPath: "/home/app/config",
								},
							},
							Resources: chainNode.Spec.Persistence.Snapshots.Resources,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "start-checker",
							Image:           "busybox",
							ImagePullPolicy: chainNode.Spec.App.GetImagePullPolicy(),
							Command:         []string{"sh"},
							Args: []string{
								"-c",
								"if ! pidof " + chainNode.Spec.App.App + " > /dev/null; then " +
									"echo '" + chainNode.Spec.App.App + " not running'; exit 1; " +
									"fi; " +
									"APP_PID=$(pidof " + chainNode.Spec.App.App + "); " +
									"echo 'Initial " + chainNode.Spec.App.App + " PID: '$APP_PID; " +
									"while true; do " +
									"if nc -z localhost 9090; then " +
									"echo 'Data is ok'; exit 0; " +
									"fi; " +
									"if ! pidof " + chainNode.Spec.App.App + " > /dev/null || [ $(pidof " + chainNode.Spec.App.App + ") -ne $APP_PID ]; then " +
									"echo '" + chainNode.Spec.App.App + " failed or restarted'; exit 1; " +
									"fi; " +
									"sleep 2; " +
									"done",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    lightContainerCpuResources,
									corev1.ResourceMemory: lightContainerMemoryResources,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    lightContainerCpuResources,
									corev1.ResourceMemory: lightContainerMemoryResources,
								},
							},
						},
					},
					NodeSelector: chainNode.Spec.Persistence.Snapshots.NodeSelector,
					Affinity:     chainNode.Spec.Persistence.Snapshots.Affinity,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(snapshot, job, r.Scheme); err != nil {
		return err
	}

	job, err := r.ClientSet.BatchV1().Jobs(chainNode.GetNamespace()).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Create PVC from Snapshot
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ichk", snapshot.GetName()),
			Namespace: snapshot.GetNamespace(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *snapshot.Status.RestoreSize,
				},
			},
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: &apiVersion[0],
				Kind:     snapshot.Kind,
				Name:     snapshot.Name,
			},
		},
	}

	if err = controllerutil.SetControllerReference(job, pvc, r.Scheme); err != nil {
		return err
	}

	pvc, err = r.ClientSet.CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	snapshot.ObjectMeta.Annotations[controllers.AnnotationSnapshotIntegrityStatus] = string(snapshotIntegrityChecking)
	return r.Update(ctx, snapshot)
}

func (r *Reconciler) getSnapshotIntegrityCheckStatus(ctx context.Context, chainNode *appsv1.ChainNode, snapshot *snapshotv1.VolumeSnapshot) (SnapshotIntegrityStatus, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			volumeSnapshot: snapshot.GetName(),
		}).String(),
	}
	list, err := r.ClientSet.BatchV1().Jobs(snapshot.GetNamespace()).List(ctx, listOptions)
	if err != nil {
		return "", err
	}

	if len(list.Items) == 0 {
		// No job is running
		return "", nil
	}
	job := list.Items[0]
	if job.Status.Failed > 0 {
		return snapshotIntegrityCorrupted, nil
	}
	if job.Status.Succeeded > 0 {
		return snapshotIntegrityOk, nil
	}
	return snapshotIntegrityChecking, nil
}
