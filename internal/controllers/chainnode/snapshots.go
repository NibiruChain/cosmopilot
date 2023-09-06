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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/controllers/chainnodeset"
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

const (
	timeLayout = "20060102150405"
)

func (r *Reconciler) ensureVolumeSnapshots(ctx context.Context, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	if !chainNode.SnapshotsEnabled() || chainNode.Status.PvcSize == "" {
		return nil
	}

	// Get list of snapshots
	listOption := client.MatchingLabels{LabelChainNode: chainNode.GetName()}
	list := &snapshotv1.VolumeSnapshotList{}
	if err := r.List(ctx, list, listOption); err != nil {
		return err
	}

	for _, snapshot := range list.Items {
		if snapshot.Annotations[annotationPvcSnapshotReady] == strconv.FormatBool(false) {
			if isSnapshotReady(&snapshot) {
				logger.Info("snapshot is ready", "snapshot", snapshot.GetName())
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
			}
		} else {
			expired, err := isSnapshotExpired(&snapshot)
			if err != nil {
				return err
			}
			if expired {
				logger.Info("deleting expired pvc snapshot", "retention", snapshot.Annotations[annotationSnapshotRetention])
				if err := r.Delete(ctx, &snapshot); err != nil {
					return err
				}
				r.recorder.Eventf(chainNode,
					corev1.EventTypeNormal,
					appsv1.ReasonDeletedSnapshot,
					"Deleted expired PVC snapshot %s", snapshot.GetName(),
				)
			}
		}
	}

	// We don't want to have more than one snapshot being taken at the same time
	if volumeSnapshotInProgress(chainNode) {
		return nil
	}

	// Create a snapshot if it's time for that
	if shouldSnapshot(chainNode) {
		logger.Info("creating pvc snapshot")
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
	period, err := time.ParseDuration(chainNode.Spec.Persistence.Snapshots.Frequency)
	if err != nil {
		return false
	}
	lastSnapshotTime := getLastSnapshotTime(chainNode)
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

	expiration, err := time.ParseDuration(retention)
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
	if group, ok := chainNode.Labels[chainnodeset.LabelChainNodeSetGroup]; ok {
		if nodeset, ok := chainNode.Labels[chainnodeset.LabelChainNodeSet]; ok {
			name = fmt.Sprintf("%s-%s", nodeset, group)
		}

	}
	return fmt.Sprintf("%s-%s", name, time.Now().UTC().Format(timeLayout))
}
