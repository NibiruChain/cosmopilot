package chainnodeset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCompatiblePatchGroupReconciliation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	r := &Reconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme, recorder: record.NewFakeRecorder(10)}
	owner := &appsv1.ChainNodeSet{ObjectMeta: metav1.ObjectMeta{Name: "chain", Namespace: "test", UID: "owner"}, Spec: appsv1.ChainNodeSetSpec{App: appsv1.AppSpec{Image: "example/nibid", Version: ptr.To("2.21.0")}, Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")}}}
	group := appsv1.NodeGroupSpec{Name: "snapshot", CompatiblePatch: &appsv1.CompatiblePatchSpec{Version: "2.21.2", UpgradeHeight: 100}}
	for _, version := range []string{"2.21.2", "2.21.3", ""} {
		if version == "" {
			group.CompatiblePatch = nil
		} else {
			group.CompatiblePatch.Version = version
		}
		node, err := r.getNodeSpec(owner, group, 0)
		require.NoError(t, err)
		require.Equal(t, group.CompatiblePatch, node.Spec.CompatiblePatch)
		if group.CompatiblePatch != nil {
			require.NotSame(t, group.CompatiblePatch, node.Spec.CompatiblePatch)
		}
		require.NoError(t, r.ensureNode(context.Background(), owner, node, false))
		stored := &appsv1.ChainNode{}
		require.NoError(t, r.Get(context.Background(), client.ObjectKeyFromObject(node), stored))
		require.Equal(t, group.CompatiblePatch, stored.Spec.CompatiblePatch)
	}
	other, err := r.getNodeSpec(owner, appsv1.NodeGroupSpec{Name: "other"}, 0)
	require.NoError(t, err)
	require.Nil(t, other.Spec.CompatiblePatch)
	owner.Spec.Validator = &appsv1.NodeSetValidatorConfig{CompatiblePatch: &appsv1.CompatiblePatchSpec{Version: "2.21.2", UpgradeHeight: 100}}
	validator, err := r.getValidatorSpec(owner)
	require.NoError(t, err)
	require.Equal(t, owner.Spec.Validator.CompatiblePatch, validator.Spec.CompatiblePatch)
	require.NotSame(t, owner.Spec.Validator.CompatiblePatch, validator.Spec.CompatiblePatch)
}
