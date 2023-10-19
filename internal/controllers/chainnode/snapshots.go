package chainnode

import (
	"context"
	"fmt"
	"strconv"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/controllers/chainnodeset"
	"github.com/NibiruChain/nibiru-operator/internal/datasnapshot"
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

const (
	timeLayout = "20060102150405"
)

func (r *Reconciler) ensureVolumeSnapshots(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	if !chainNode.SnapshotsEnabled() || chainNode.Status.PvcSize == "" || chainNode.Status.LatestHeight == 0 {
		return nil
	}

	// Get list of snapshots
	listOption := client.MatchingLabels{LabelChainNode: chainNode.GetName()}
	list := &snapshotv1.VolumeSnapshotList{}
	if err := r.List(ctx, list, listOption); err != nil {
		return err
	}

	// Grab list of possible tarball names to make sure we delete any possible dangling jobs
	tarballNames := make([]string, 0)

	for _, snapshot := range list.Items {
		tarballNames = append(tarballNames, getTarballName(chainNode, &snapshot))
		switch {

		// If the snapshot does not have the ready annotation, we haven't processed it yet. So we check if it's ready
		// and if it is, we mark it as ready and register its timestamp in chainnode.
		// In case tarball export is enabled, we also start the export right away.
		case snapshot.Annotations[annotationPvcSnapshotReady] == strconv.FormatBool(false) && isSnapshotReady(&snapshot):
			logger.Info("pvc snapshot has finished", "snapshot", snapshot.GetName())
			snapshot.ObjectMeta.Annotations[annotationPvcSnapshotReady] = strconv.FormatBool(true)
			if err := r.Update(ctx, &snapshot); err != nil {
				return err
			}
			setSnapshotInProgress(chainNode, false)
			setSnapshotTime(chainNode, snapshot.CreationTimestamp.Time)
			if err := r.Update(ctx, chainNode); err != nil {
				return err
			}
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonFinishedSnapshot,
				"Finished PVC snapshot %s", snapshot.GetName(),
			)
			if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() {
				logger.Info("starting tarball export", "snapshot", snapshot.GetName())
				if err := r.exportTarball(ctx, chainNode, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonTarballExportStart,
					"Exporting tarball %s from snapshot", getTarballName(chainNode, &snapshot),
				)
			}

		// If for some reason, there is an error starting the export above, it is never retried. So we do it here.
		case chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() &&
			snapshot.Annotations[annotationPvcSnapshotReady] == strconv.FormatBool(true) &&
			snapshot.Annotations[annotationExportingTarball] == "":
			logger.Info("starting tarball export", "snapshot", snapshot.GetName())
			if err := r.exportTarball(ctx, chainNode, &snapshot); err != nil {
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
			snapshot.Annotations[annotationPvcSnapshotReady] == strconv.FormatBool(true) &&
			snapshot.Annotations[annotationExportingTarball] == strconv.FormatBool(true):
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
				snapshot.Annotations[annotationExportingTarball] = tarballFinished
				if err := r.Update(ctx, &snapshot); err != nil {
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
				logger.Info("deleting expired pvc snapshot", "snapshot", snapshot.GetName(), "retention", snapshot.Annotations[annotationSnapshotRetention])
				if err := r.Delete(ctx, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonDeletedSnapshot,
					"Deleted expired PVC snapshot %s", snapshot.GetName(),
				)
				if chainNode.Spec.Persistence.Snapshots.ShouldExportTarballs() && chainNode.Spec.Persistence.Snapshots.ExportTarball.DeleteWhenExpired() {
					logger.Info("deleting expired snapshot tarball", "snapshot", snapshot.GetName(), "retention", snapshot.Annotations[annotationSnapshotRetention])
					if err := r.deleteTarball(ctx, chainNode, &snapshot); err != nil {
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
				logger.Info("deleting orphaned tarball upload job as volumesnapshot does not exist anymore")
				if err := exporter.DeleteSnapshot(ctx, snapshot); err != nil {
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
	if shouldSnapshot(chainNode) {
		logger.Info("creating new pvc snapshot")
		return r.createSnapshot(ctx, chainNode)
	}

	return nil
}

func (r *Reconciler) createSnapshot(ctx context.Context, chainNode *appsv1.ChainNode) error {
	if chainNode.Spec.Persistence.Snapshots.ShouldStopNode() {
		pod, err := r.getPodSpec(ctx, chainNode, "")
		if err != nil {
			return err
		}

		ph := k8s.NewPodHelper(r.ClientSet, r.RestConfig, pod)
		if err := ph.Delete(ctx); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		} else {
			if err := ph.WaitForPodDeleted(ctx, timeoutPodDeleted); err != nil {
				return err
			}
		}
	}

	snapshot := getVolumeSnapshotSpec(chainNode)
	if err := r.Create(ctx, snapshot); err != nil {
		return err
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

func shouldSnapshot(chainNode *appsv1.ChainNode) bool {
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
	return snapshot.Status.ReadyToUse != nil && *snapshot.Status.ReadyToUse
}

func isSnapshotExpired(snapshot *snapshotv1.VolumeSnapshot) (bool, error) {
	retention, ok := snapshot.Annotations[annotationSnapshotRetention]
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
				annotationPvcSnapshotReady: strconv.FormatBool(false),
			},
			Labels: WithChainNodeLabels(chainNode, map[string]string{
				LabelChainNode: chainNode.GetName(),
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
		spec.ObjectMeta.Annotations[annotationSnapshotRetention] = *chainNode.Spec.Persistence.Snapshots.Retention
	}

	return spec
}

func volumeSnapshotInProgress(chainNode *appsv1.ChainNode) bool {
	return chainNode.ObjectMeta.Annotations[annotationPvcSnapshotInProgress] == strconv.FormatBool(true)
}

func setSnapshotInProgress(chainNode *appsv1.ChainNode, snapshotting bool) {
	if chainNode.ObjectMeta.Annotations == nil {
		chainNode.ObjectMeta.Annotations = make(map[string]string)
	}
	chainNode.ObjectMeta.Annotations[annotationPvcSnapshotInProgress] = strconv.FormatBool(snapshotting)
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
	chainNode.ObjectMeta.Annotations[annotationLastPvcSnapshot] = ts.UTC().Format(timeLayout)
}

func getLastSnapshotTime(chainNode *appsv1.ChainNode) time.Time {
	if s, ok := chainNode.ObjectMeta.Annotations[annotationLastPvcSnapshot]; ok {
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
	if group, ok := chainNode.Labels[chainnodeset.LabelChainNodeSetGroup]; ok && group != "" {
		if nodeset, ok := chainNode.Labels[chainnodeset.LabelChainNodeSet]; ok && nodeset != "" {
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
			chainNode.Spec.Persistence.Snapshots.ExportTarball.GCS.Bucket,
			chainNode.Spec.Persistence.Snapshots.ExportTarball.GCS.CredentialsSecret,
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

	if err := exporter.CreateSnapshot(ctx, getTarballName(chainNode, snapshot), snapshot); err != nil {
		return err
	}

	snapshot.ObjectMeta.Annotations[annotationExportingTarball] = strconv.FormatBool(true)
	return r.Update(ctx, snapshot)
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
