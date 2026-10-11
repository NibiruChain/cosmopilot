package chainnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCompatiblePatchPodUpgradeAndRecreation(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	node := &appsv1.ChainNode{ObjectMeta: metav1.ObjectMeta{Name: "snapshot", Namespace: "test", UID: "node"}, Spec: appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "nibid", Image: "example/nibid", Version: ptr.To("1.0.0")}, Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")}, CompatiblePatch: &appsv1.CompatiblePatchSpec{Version: "2.21.2", UpgradeHeight: 100}}, Status: appsv1.ChainNodeStatus{LatestHeight: 150, Upgrades: []appsv1.Upgrade{{Height: 100, Image: "example/nibid:2.21.0", Status: appsv1.UpgradeCompleted}, {Height: 200, Image: "example/nibid:2.22.0", Status: appsv1.UpgradeScheduled}}}}
	r := &Reconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: node.Name, Namespace: node.Namespace}, Data: map[string]string{"config.toml": ""}}).Build(), Scheme: scheme, opts: &controllers.ControllerRunOptions{}}
	pod, err := r.getPodSpec(ctx, node, "config")
	require.NoError(t, err)
	require.Equal(t, "example/nibid:2.21.2", pod.Spec.Containers[0].Image)
	require.NoError(t, r.Create(ctx, pod))
	node.Status.LatestHeight = 200
	node.Status.Upgrades[1].Status = appsv1.UpgradeOnGoing
	upgrading, err := r.getPodSpec(ctx, node, "config")
	require.NoError(t, err)
	require.Equal(t, "example/nibid:2.22.0", upgrading.Spec.Containers[0].Image)
	node.Status.Upgrades[1].Status = appsv1.UpgradeCompleted
	require.NoError(t, r.Status().Update(ctx, node))
	// Recreate from persisted state, retaining the obsolete patch configuration.
	require.NoError(t, r.Delete(ctx, pod))
	restored := &appsv1.ChainNode{}
	require.NoError(t, r.Get(ctx, client.ObjectKeyFromObject(node), restored))
	recreated, err := r.getPodSpec(ctx, restored, "config")
	require.NoError(t, err)
	require.Equal(t, "example/nibid:2.22.0", recreated.Spec.Containers[0].Image)
	require.Equal(t, "2.21.2", restored.Spec.CompatiblePatch.Version)
	require.NoError(t, r.Create(ctx, recreated))
}
