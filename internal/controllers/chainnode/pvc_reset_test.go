package chainnode

import (
	"testing"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
)

func TestEnsureDataVolumeDoesNotCreatePVCWhenHaltHoldClearFails(t *testing.T) {
	for _, tt := range []struct {
		name            string
		restoreSnapshot bool
		wantHeight      int64
	}{
		{name: "fresh PVC", wantHeight: 0},
		{name: "snapshot restoration", restoreSnapshot: true, wantHeight: 50},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, snapshotv1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			node := dataHeightResetTestNode()
			node.Spec.Persistence = &appsv1.Persistence{}
			objects := []client.Object{node}
			if tt.restoreSnapshot {
				node.Spec.Persistence.RestoreFromSnapshot = &appsv1.PvcSnapshot{Name: "snapshot"}
				objects = append(objects, &snapshotv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name: "snapshot", Namespace: node.Namespace,
						Annotations: map[string]string{controllers.AnnotationDataHeight: "50"},
					},
					Status: &snapshotv1.VolumeSnapshotStatus{},
				})
			}
			base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).
				WithObjects(objects...).Build()
			tracking := &dataHeightResetPersistenceClient{Client: base, failMetadata: true}
			r := &Reconciler{Client: tracking}
			current := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), current))

			_, _, err := r.ensureDataVolume(t.Context(), nil, current)
			require.ErrorContains(t, err, "metadata persistence failed")
			assert.Equal(t, []string{"status", "metadata"}, tracking.writes)
			stored := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, tt.wantHeight, stored.Status.LatestHeight)
			assert.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])
			pvc := &corev1.PersistentVolumeClaim{}
			err = tracking.Get(t.Context(), client.ObjectKeyFromObject(node), pvc)
			assert.True(t, apierrors.IsNotFound(err), "PVC must not be created before hold removal persists")
		})
	}
}

func TestEnsureDataVolumeSnapshotWithoutHeightPreservesCommittedDataIdentity(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, snapshotv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	node := dataHeightResetTestNode()
	node.Spec.Persistence = &appsv1.Persistence{RestoreFromSnapshot: &appsv1.PvcSnapshot{Name: "snapshot"}}
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "snapshot", Namespace: node.Namespace},
		Status:     &snapshotv1.VolumeSnapshotStatus{},
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node, snapshot).Build()
	tracking := &dataHeightResetPersistenceClient{Client: base}
	r := &Reconciler{Client: tracking}
	current := &appsv1.ChainNode{}
	require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), current))

	pvc, _, err := r.ensureDataVolume(t.Context(), nil, current)
	require.NoError(t, err)
	require.NotNil(t, pvc)
	assert.Equal(t, []string{"metadata", "status"}, tracking.writes)
	stored := &appsv1.ChainNode{}
	require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
	assert.Equal(t, int64(100), stored.Status.LatestHeight)
	assert.Equal(t, "v2", stored.Status.AppVersion)
	assert.NotContains(t, stored.Annotations, appsv1.AnnotationHaltHeightHold)
	assert.Equal(t, "100", pvc.Annotations[controllers.AnnotationDataHeight])
}

func TestEnsureDataVolumeSnapshotWithoutHeightDoesNotCreatePVCWhenHoldClearFails(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, snapshotv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	node := dataHeightResetTestNode()
	node.Spec.Persistence = &appsv1.Persistence{RestoreFromSnapshot: &appsv1.PvcSnapshot{Name: "snapshot"}}
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "snapshot", Namespace: node.Namespace},
		Status:     &snapshotv1.VolumeSnapshotStatus{},
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node, snapshot).Build()
	tracking := &dataHeightResetPersistenceClient{Client: base, failMetadata: true}
	r := &Reconciler{Client: tracking}
	current := &appsv1.ChainNode{}
	require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), current))

	_, _, err := r.ensureDataVolume(t.Context(), nil, current)
	require.ErrorContains(t, err, "metadata persistence failed")
	assert.Equal(t, []string{"metadata"}, tracking.writes)
	stored := &appsv1.ChainNode{}
	require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
	assert.Equal(t, int64(100), stored.Status.LatestHeight)
	assert.Equal(t, "v2", stored.Status.AppVersion)
	assert.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])
	pvc := &corev1.PersistentVolumeClaim{}
	err = tracking.Get(t.Context(), client.ObjectKeyFromObject(node), pvc)
	assert.True(t, apierrors.IsNotFound(err))
}
