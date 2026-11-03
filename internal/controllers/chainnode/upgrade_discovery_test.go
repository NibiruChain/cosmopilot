package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/chainutils"
	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
)

type currentPlanServer struct {
	upgradetypes.UnimplementedQueryServer
	mu   sync.Mutex
	plan *upgradetypes.Plan
	err  error
}

func (s *currentPlanServer) CurrentPlan(context.Context, *upgradetypes.QueryCurrentPlanRequest) (*upgradetypes.QueryCurrentPlanResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &upgradetypes.QueryCurrentPlanResponse{Plan: s.plan}, s.err
}

func (s *currentPlanServer) set(plan *upgradetypes.Plan, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plan, s.err = plan, err
}

func upgradePlan(height int64, image string) *upgradetypes.Plan {
	info, _ := json.Marshal(map[string]any{"binaries": map[string]string{"docker": image}})
	return &upgradetypes.Plan{Name: "next", Height: height, Info: string(info)}
}

func discoveryHarness(t *testing.T, initial []appsv1.Upgrade) (*Reconciler, *appsv1.ChainNode, *currentPlanServer) {
	t.Helper()
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	node := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "upgrade-test", Namespace: "test", UID: "upgrade-test"},
		Spec:       appsv1.ChainNodeSpec{App: appsv1.AppSpec{Image: "example/node", App: "nibid"}},
		Status:     appsv1.ChainNodeStatus{Phase: appsv1.PhaseChainNodeRunning, LatestHeight: 50, Upgrades: initial},
	}
	kube := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node).Build()
	require.NoError(t, kube.Get(ctx, client.ObjectKeyFromObject(node), node))
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	query := &currentPlanServer{}
	upgradetypes.RegisterQueryServer(server, query)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	dns := manual.NewBuilderWithScheme("dns")
	dns.InitialState(resolver.State{Addresses: []resolver.Address{{Addr: "bufnet"}}})
	chain, err := chainutils.NewClient(node.GetNodeFQDN(), grpc.WithResolvers(dns), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = chain.Close() })
	cache := ttlcache.New[string, *chainutils.Client]()
	cache.Set(node.GetNodeFQDN(), chain, ttlcache.DefaultTTL)
	return &Reconciler{Client: kube, Scheme: scheme, nodeClients: cache}, node, query
}

func publishedUpgrades(t *testing.T, r *Reconciler, node *appsv1.ChainNode) []appsv1.Upgrade {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, r.ensureUpgrades(ctx, node, true))
	cm := &corev1.ConfigMap{}
	require.NoError(t, r.Get(ctx, client.ObjectKey{Name: node.Name + "-upgrades", Namespace: node.Namespace}, cm))
	var published struct {
		Upgrades []appsv1.Upgrade `json:"upgrades"`
	}
	require.NoError(t, json.Unmarshal([]byte(cm.Data[upgradesConfigFile]), &published))
	return published.Upgrades
}

func TestDiscoveryRetractsCancelledPlanFromNodeUtils(t *testing.T) {
	r, node, query := discoveryHarness(t, nil)
	query.set(upgradePlan(100, "example/node:next"), nil)
	require.Len(t, publishedUpgrades(t, r, node), 1)
	query.set(nil, nil)
	published := publishedUpgrades(t, r, node)
	require.Empty(t, published, "a cancelled pending plan must leave the published ConfigMap")
	config := filepath.Join(t.TempDir(), "upgrades.json")
	// Load exactly the data shape consumed by node-utils after ConfigMap delivery.
	bz, err := json.Marshal(map[string]any{"upgrades": published})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(config, bz, 0o600))
	checker, err := nodeutils.NewUpgradeChecker(config)
	require.NoError(t, err)
	require.False(t, checker.ShouldUpgrade(100), "cancellation must not trigger a stale upgrade-required flag")
}

func TestDiscoveryReplacesHeightAndImage(t *testing.T) {
	for _, height := range []int64{100, 200} {
		t.Run(fmt.Sprint(height), func(t *testing.T) {
			r, node, query := discoveryHarness(t, nil)
			query.set(upgradePlan(100, "example/node:old"), nil)
			require.Len(t, publishedUpgrades(t, r, node), 1)
			query.set(upgradePlan(height, "example/node:new"), nil)
			published := publishedUpgrades(t, r, node)
			require.Equal(t, []appsv1.Upgrade{{Height: height, Image: "example/node:new", Status: appsv1.UpgradeScheduled, Source: appsv1.OnChainUpgrade}}, published)
		})
	}
}

func TestDiscoveryQueryFailurePreservesCachedPlan(t *testing.T) {
	r, node, query := discoveryHarness(t, nil)
	query.set(upgradePlan(100, "example/node:next"), nil)
	before := publishedUpgrades(t, r, node)
	query.set(nil, status.Error(codes.Unavailable, "temporary query outage"))
	require.Equal(t, before, publishedUpgrades(t, r, node))
}

func TestDiscoveryRetainsCompletedOngoingSkippedAndManualUpgrades(t *testing.T) {
	for _, phase := range []appsv1.UpgradePhase{appsv1.UpgradeCompleted, appsv1.UpgradeOnGoing, appsv1.UpgradeSkipped} {
		t.Run(string(phase), func(t *testing.T) {
			// A successful upgrade removes CurrentPlan before controller status
			// necessarily records completion. Ongoing/terminal records also
			// retain the running image when the cached height is behind.
			existing := []appsv1.Upgrade{
				{Height: 100, Image: "example/node:running", Status: phase, Source: appsv1.OnChainUpgrade},
				{Height: 300, Image: "example/node:manual", Status: appsv1.UpgradeScheduled, Source: appsv1.ManualUpgrade},
			}
			r, node, query := discoveryHarness(t, existing)
			query.set(nil, nil)
			require.Equal(t, existing, publishedUpgrades(t, r, node))
		})
	}
}

func TestDiscoveryRetractsCancellationObservedAfterOldHeight(t *testing.T) {
	r, node, query := discoveryHarness(t, nil)
	query.set(upgradePlan(100, "example/node:cancelled"), nil)
	require.Len(t, publishedUpgrades(t, r, node), 1)
	// The next successful poll can arrive after the cancelled height.
	node.Status.LatestHeight = 101
	query.set(nil, nil)
	require.Empty(t, publishedUpgrades(t, r, node))
}

func TestDiscoveryInvalidHeightDoesNotReplaceCachedPlan(t *testing.T) {
	r, node, query := discoveryHarness(t, nil)
	query.set(upgradePlan(100, "example/node:next"), nil)
	before := publishedUpgrades(t, r, node)
	query.set(upgradePlan(0, "example/node:invalid"), nil)
	require.Equal(t, before, publishedUpgrades(t, r, node))
}

func TestDiscoveryReplacementCanRemoveImageReadiness(t *testing.T) {
	r, node, query := discoveryHarness(t, nil)
	query.set(upgradePlan(100, "example/node:old"), nil)
	require.Len(t, publishedUpgrades(t, r, node), 1)
	query.set(&upgradetypes.Plan{Name: "replacement", Height: 100, Info: "{}"}, nil)
	require.Equal(t, []appsv1.Upgrade{{Height: 100, Image: "", Status: appsv1.UpgradeImageMissing, Source: appsv1.OnChainUpgrade}}, publishedUpgrades(t, r, node))
}
