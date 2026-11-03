package chainnode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
)

func TestEnsurePodDoesNotPatchReplacementBeforeTerminalHaltDelete(t *testing.T) {
	haltHeight := int64(100)
	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "chain-node"},
		Spec: appsv1.ChainNodeSpec{
			App:    appsv1.AppSpec{App: "appd", Image: "app:v1"},
			Config: &appsv1.Config{HaltHeight: &haltHeight},
		},
		Status: appsv1.ChainNodeStatus{LatestHeight: 98, Phase: appsv1.PhaseChainNodeRunning},
	}
	observedPod := terminalEvidencePod(t, 100, 100, 99, false, 0, "Completed")
	observedPod.Name = chainNode.Name
	observedPod.Namespace = chainNode.Namespace
	observedPod.Labels = map[string]string{"identity": "observed"}
	replacementPod := observedPod.DeepCopy()
	replacementPod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	replacementPod.UID = "replacement-pod"
	replacementPod.Labels = map[string]string{"identity": "replacement"}
	replacementPod.Status.ContainerStatuses[0].State.Terminated.ContainerID = "containerd://replacement"
	config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: chainNode.Name, Namespace: chainNode.Namespace}}

	var patches atomic.Int32
	var logRequests atomic.Int32
	var deletes atomic.Int32
	var requestedUID types.UID
	var latestHeightAtDelete int64
	var cache client.Client
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/namespaces/default/pods/node/log" {
			logRequests.Add(1)
			http.Error(w, "application logs must not be read", http.StatusInternalServerError)
			return
		}
		switch req.Method {
		case http.MethodPatch:
			patches.Add(1)
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(replacementPod))
		case http.MethodDelete:
			deletes.Add(1)
			stored := &appsv1.ChainNode{}
			require.NoError(t, cache.Get(t.Context(), client.ObjectKeyFromObject(chainNode), stored))
			latestHeightAtDelete = stored.Status.LatestHeight
			var options metav1.DeleteOptions
			require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
			if options.Preconditions != nil && options.Preconditions.UID != nil {
				requestedUID = *options.Preconditions.UID
			}
			writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
				Code: http.StatusConflict, Message: "pod UID precondition failed",
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cache = fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(chainNode).
		WithObjects(chainNode, observedPod, config).Build()
	r := &Reconciler{
		Client:    cache,
		ClientSet: clientSet,
		Scheme:    scheme,
		opts:      &controllers.ControllerRunOptions{NodeUtilsImage: "node-utils:test"},
	}

	err = r.ensurePod(t.Context(), nil, chainNode, "config-hash")
	require.Error(t, err)
	stored := &appsv1.ChainNode{}
	require.NoError(t, cache.Get(t.Context(), client.ObjectKeyFromObject(chainNode), stored))
	require.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])

	err = r.ensurePod(t.Context(), nil, stored, "config-hash")
	require.Error(t, err)
	assert.Zero(t, patches.Load(), "terminal recovery must not patch a same-name replacement")
	assert.Zero(t, logRequests.Load(), "terminal recovery must not read application logs")
	assert.Equal(t, int32(2), deletes.Load(), "the structured boundary hold must persist across reconciles")
	assert.Equal(t, observedPod.UID, requestedUID)
	assert.Equal(t, int64(99), latestHeightAtDelete)
	assert.Equal(t, map[string]string{"identity": "replacement"}, replacementPod.Labels)
}

func TestPersistTerminalHaltLatestHeightDoesNotRegressStatus(t *testing.T) {
	for _, tt := range []struct {
		name           string
		cachedHeight   int64
		evidenceHeight *int64
		wantHeight     int64
	}{
		{name: "missing height", cachedHeight: 97, wantHeight: 97},
		{name: "zero height", cachedHeight: 97, evidenceHeight: ptr.To[int64](0), wantHeight: 97},
		{name: "unchanged height", cachedHeight: 97, evidenceHeight: ptr.To[int64](97), wantHeight: 97},
		{name: "older height", cachedHeight: 100, evidenceHeight: ptr.To[int64](99), wantHeight: 100},
		{name: "newer height", cachedHeight: 97, evidenceHeight: ptr.To[int64](99), wantHeight: 99},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, node, pod := terminalHaltEnsurePodFixture(t, "https://kubernetes.invalid", tt.cachedHeight, tt.evidenceHeight)
			require.NoError(t, r.persistTerminalHaltLatestHeight(t.Context(), node, pod))
			assert.Equal(t, tt.wantHeight, node.Status.LatestHeight)
			stored := &appsv1.ChainNode{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, tt.wantHeight, stored.Status.LatestHeight)
		})
	}
}

func TestRefreshLatestHeightDoesNotRegressStatus(t *testing.T) {
	for _, tt := range []struct {
		name          string
		freshHeight   int64
		wantHeight    int64
		wantAvailable bool
		wantWrites    []string
	}{
		{name: "older height", freshHeight: 99, wantHeight: 100, wantAvailable: true},
		{name: "equal height", freshHeight: 100, wantHeight: 100, wantAvailable: true},
		{name: "newer height", freshHeight: 101, wantHeight: 101, wantAvailable: true, wantWrites: []string{"status"}},
		{name: "zero height", wantHeight: 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, appsv1.AddToScheme(scheme))
			node := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
				Status:     appsv1.ChainNodeStatus{LatestHeight: 100},
			}
			base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node).Build()
			tracking := &dataHeightResetPersistenceClient{Client: base}
			nodeClient := &fakeTerminalNodeUtilsClient{height: tt.freshHeight}
			r := &Reconciler{
				Client: tracking,
				nodeStatusClientFactory: func(string) nodeStatusClient {
					return nodeClient
				},
			}

			freshHeight, available, err := r.refreshLatestHeight(t.Context(), node)
			require.NoError(t, err)
			assert.Equal(t, tt.freshHeight, freshHeight)
			assert.Equal(t, tt.wantAvailable, available)
			assert.Equal(t, tt.wantHeight, node.Status.LatestHeight)
			assert.Equal(t, tt.wantWrites, tracking.writes)
			stored := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, tt.wantHeight, stored.Status.LatestHeight)
		})
	}
}

func TestEnsurePodDoesNotDeleteWhenTerminalHaltHeightWriteFails(t *testing.T) {
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodDelete || req.Method == http.MethodPost {
			mutations.Add(1)
		}
		http.NotFound(w, req)
	}))
	defer server.Close()

	height := int64(99)
	r, node, _ := terminalHaltEnsurePodFixture(t, server.URL, 97, &height)
	tracking := &dataHeightResetPersistenceClient{Client: r.Client, failStatusHeight: &height}
	r.Client = tracking

	err := r.ensurePod(t.Context(), nil, node, "config-hash")
	require.ErrorContains(t, err, "status persistence failed")
	assert.Zero(t, mutations.Load())
	assert.Equal(t, []string{"metadata", "status"}, tracking.writes)
}

func TestRecreatePodRejectsReplacementAfterStructuredHaltEvidence(t *testing.T) {
	observedPod := terminalEvidencePod(t, 100, 100, 99, false, 0, "Completed")
	observedPod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	observedPod.Name = "node"
	observedPod.Namespace = "default"
	replacementPod := observedPod.DeepCopy()
	replacementPod.UID = "replacement-pod"
	replacementPod.Status.ContainerStatuses[0].State.Terminated.ContainerID = "containerd://replacement"
	desiredPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"}}

	replacementPresent := true
	var requestedUID types.UID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			var options metav1.DeleteOptions
			require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
			if options.Preconditions != nil && options.Preconditions.UID != nil {
				requestedUID = *options.Preconditions.UID
			}
			if requestedUID != observedPod.UID {
				replacementPresent = false
				writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
				return
			}
			writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
				Code: http.StatusConflict, Message: "pod UID precondition failed",
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	r := &Reconciler{ClientSet: clientSet}
	chainNodeWithHalt := &appsv1.ChainNode{
		Spec: appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{HaltHeight: ptr.To[int64](100)}},
	}
	require.Equal(t, terminalPodHold, terminalPodRecoveryFor(chainNodeWithHalt, observedPod))
	require.NotEqual(t, observedPod.UID, replacementPod.UID)
	chainNode := &appsv1.ChainNode{Status: appsv1.ChainNodeStatus{Phase: appsv1.PhaseChainNodeRestarting}}
	err = r.recreatePod(t.Context(), chainNode, observedPod, desiredPod, false)
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err))
	assert.Equal(t, observedPod.UID, requestedUID)
	assert.True(t, replacementPresent, "replacement Pod must not be deleted")
}

func TestUpgradePodSelectsReplacementAfterDeletingPodWithLongGracePeriod(t *testing.T) {
	graceSeconds := int64(600)
	require.Greater(t, graceSeconds, int64(timeoutPodDeleted.Seconds()))
	currentPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "observed-pod"},
		Spec:       corev1.PodSpec{TerminationGracePeriodSeconds: &graceSeconds},
	}
	desiredPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "appd", Image: "app:old",
		}}},
	}
	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
		Spec:       appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}},
		Status:     appsv1.ChainNodeStatus{Phase: appsv1.PhaseChainNodeUpgrading},
	}

	var requestedUID types.UID
	var requestedGrace int64
	var selectedImage string
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			var options metav1.DeleteOptions
			require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
			require.NotNil(t, options.Preconditions)
			require.NotNil(t, options.Preconditions.UID)
			requestedUID = *options.Preconditions.UID
			require.NotNil(t, options.GracePeriodSeconds)
			requestedGrace = *options.GracePeriodSeconds
			writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			writeChainNodeTestJSON(t, w, http.StatusNotFound, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound,
				Code: http.StatusNotFound, Message: "observed Pod deleted",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/namespaces/default/pods":
			var pod corev1.Pod
			require.NoError(t, json.NewDecoder(req.Body).Decode(&pod))
			selectedImage = pod.Spec.Containers[0].Image
			writeChainNodeTestJSON(t, w, http.StatusInternalServerError, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonInternalError,
				Code: http.StatusInternalServerError, Message: "stop after selecting replacement",
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	r := &Reconciler{ClientSet: clientSet}

	targetCommitted, err := r.upgradePod(t.Context(), chainNode, currentPod, desiredPod, "app:new")
	require.Error(t, err)
	assert.True(t, targetCommitted, "an accepted delete commits the upgrade target")
	assert.Equal(t, currentPod.UID, requestedUID)
	assert.Equal(t, int64((timeoutPodDeleted / 2).Seconds()), requestedGrace)
	assert.Equal(t, "app:new", selectedImage)
	assert.Equal(t, []string{
		"DELETE /api/v1/namespaces/default/pods/node",
		"GET /api/v1/namespaces/default/pods/node",
		"POST /api/v1/namespaces/default/pods",
	}, requests)
}

func TestUpgradePodCommitsTargetWhenDeleteWaitFails(t *testing.T) {
	graceSeconds := int64(600)
	currentPod := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "observed-pod"},
		Spec:       corev1.PodSpec{TerminationGracePeriodSeconds: &graceSeconds},
	}
	desiredPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "appd", Image: "app:old",
		}}},
	}
	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
		Spec:       appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}},
		Status:     appsv1.ChainNodeStatus{Phase: appsv1.PhaseChainNodeUpgrading},
	}

	getStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
		case req.Method == http.MethodGet:
			getStarted <- struct{}{}
			<-req.Context().Done()
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	r := &Reconciler{ClientSet: clientSet}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan struct {
		committed bool
		err       error
	}, 1)
	go func() {
		committed, err := r.upgradePod(ctx, chainNode, currentPod, desiredPod, "app:new")
		result <- struct {
			committed bool
			err       error
		}{committed: committed, err: err}
	}()
	select {
	case <-getStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deletion poll")
	}
	cancel()
	var got struct {
		committed bool
		err       error
	}
	select {
	case got = <-result:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upgradePod to return")
	}

	require.Error(t, got.err)
	assert.True(t, got.committed, "accepted deletion must preserve the selected upgrade target")
	assert.Equal(t, "app:new", desiredPod.Spec.Containers[0].Image)
}

func TestUpgradePodUIDConflictDoesNotCommitTarget(t *testing.T) {
	graceSeconds := int64(600)
	currentPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "observed-pod"},
		Spec:       corev1.PodSpec{TerminationGracePeriodSeconds: &graceSeconds},
	}
	desiredPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "appd", Image: "app:old",
		}}},
	}
	chainNode := &appsv1.ChainNode{Status: appsv1.ChainNodeStatus{Phase: appsv1.PhaseChainNodeUpgrading}}
	replacementPresent := true
	var requestedUID types.UID
	var createRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodDelete:
			var options metav1.DeleteOptions
			require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
			require.NotNil(t, options.Preconditions)
			require.NotNil(t, options.Preconditions.UID)
			requestedUID = *options.Preconditions.UID
			writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
				Code: http.StatusConflict, Message: "replacement Pod has a different UID",
			})
		case http.MethodPost:
			createRequests.Add(1)
			replacementPresent = false
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	r := &Reconciler{ClientSet: clientSet}

	committed, err := r.upgradePod(t.Context(), chainNode, currentPod, desiredPod, "app:new")
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err))
	assert.False(t, committed)
	assert.Equal(t, currentPod.UID, requestedUID)
	assert.True(t, replacementPresent)
	assert.Zero(t, createRequests.Load())
}

func TestEnsurePodUsesRunningNodeUtilsToUpgradeTerminalApp(t *testing.T) {
	var requestedUID types.UID
	var requestedGrace int64
	var selectedImage string
	var livePod *corev1.Pod
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			var options metav1.DeleteOptions
			require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
			require.NotNil(t, options.Preconditions)
			require.NotNil(t, options.Preconditions.UID)
			requestedUID = *options.Preconditions.UID
			require.NotNil(t, options.GracePeriodSeconds)
			requestedGrace = *options.GracePeriodSeconds
			deleted = true
			writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			if !deleted {
				writeChainNodeTestJSON(t, w, http.StatusOK, livePod)
				return
			}
			writeChainNodeTestJSON(t, w, http.StatusNotFound, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound,
				Code: http.StatusNotFound, Message: "old Pod deleted",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/namespaces/default/pods":
			var pod corev1.Pod
			require.NoError(t, json.NewDecoder(req.Body).Decode(&pod))
			selectedImage = pod.Spec.Containers[0].Image
			writeChainNodeTestJSON(t, w, http.StatusInternalServerError, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonInternalError,
				Code: http.StatusInternalServerError, Message: "stop after selecting target",
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	nodeClient := &fakeTerminalNodeUtilsClient{
		height: 100, requiresUpgrade: true,
	}
	r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, nodeClient)
	livePod = observedPod.DeepCopy()
	livePod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}

	require.NoError(t, r.ensurePod(t.Context(), nil, node, "config-hash"))
	assert.Equal(t, []string{"upgrade-fresh", "latest"}, nodeClient.calls)
	assert.Equal(t, observedPod.UID, requestedUID)
	assert.Equal(t, int64((timeoutPodDeleted / 2).Seconds()), requestedGrace)
	assert.Equal(t, "app:v2", selectedImage)
	stored := &appsv1.ChainNode{}
	require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
	assert.Equal(t, int64(100), stored.Status.LatestHeight)
	assert.Equal(t, "v2", stored.Status.AppVersion)
	assert.Equal(t, appsv1.UpgradeCompleted, stored.Status.Upgrades[0].Status)
}

func TestEnsurePodDiscardsRunningNodeUtilsEvidenceFromReplacement(t *testing.T) {
	for _, tt := range []struct {
		name            string
		requiresUpgrade bool
		authoritative   string
		wantErr         string
	}{
		{name: "same-name replacement before identity check", authoritative: "replacement"},
		{name: "upgrade evidence from same-name replacement", requiresUpgrade: true, authoritative: "replacement"},
		{name: "pod disappears before identity check", authoritative: "missing"},
		{name: "identity check error is propagated", authoritative: "error", wantErr: "failed to verify terminal pod"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var authoritativeState atomic.Value
			authoritativeState.Store("old")
			var mutations atomic.Int32
			var livePod *corev1.Pod
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/v1/namespaces/default/pods/node":
					switch authoritativeState.Load().(string) {
					case "old":
						writeChainNodeTestJSON(t, w, http.StatusOK, livePod)
					case "replacement":
						replacement := livePod.DeepCopy()
						replacement.UID = "replacement-pod"
						writeChainNodeTestJSON(t, w, http.StatusOK, replacement)
					case "missing":
						writeChainNodeTestJSON(t, w, http.StatusNotFound, &metav1.Status{
							Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound,
							Code: http.StatusNotFound, Message: "observed Pod disappeared",
						})
					case "error":
						writeChainNodeTestJSON(t, w, http.StatusInternalServerError, &metav1.Status{
							Status: metav1.StatusFailure, Reason: metav1.StatusReasonInternalError,
							Code: http.StatusInternalServerError, Message: "identity lookup failed",
						})
					}
				case req.Method == http.MethodPatch || req.Method == http.MethodDelete || req.Method == http.MethodPost:
					mutations.Add(1)
					writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
						Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
						Code: http.StatusConflict, Message: "unexpected Pod mutation",
					})
				default:
					http.NotFound(w, req)
				}
			}))
			defer server.Close()

			freshHeight := int64(99)
			if tt.requiresUpgrade {
				freshHeight = 100
			}
			nodeClient := &fakeTerminalNodeUtilsClient{height: freshHeight, requiresUpgrade: tt.requiresUpgrade}
			r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, nodeClient)
			livePod = observedPod.DeepCopy()
			livePod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
			nodeClient.onLatest = func() { authoritativeState.Store(tt.authoritative) }
			haltHeight := int64(100)
			node.Spec.Config.HaltHeight = &haltHeight
			observedPod.Spec.InitContainers[0].Env = []corev1.EnvVar{{Name: "HALT_HEIGHT", Value: "100"}}
			require.NoError(t, r.Update(t.Context(), node))
			node.Status.AppVersion = "v1"
			require.NoError(t, r.Status().Update(t.Context(), node))
			require.NoError(t, r.Update(t.Context(), observedPod))

			err := r.ensurePod(t.Context(), nil, node, "config-hash")
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			assert.Zero(t, mutations.Load())
			assert.Equal(t, []string{"upgrade-fresh", "latest"}, nodeClient.calls)
			stored := &appsv1.ChainNode{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.NotContains(t, stored.Annotations, appsv1.AnnotationHaltHeightHold)
			assert.Equal(t, appsv1.UpgradeScheduled, stored.Status.Upgrades[0].Status)
			assert.Equal(t, "v1", stored.Status.AppVersion)
		})
	}
}

func TestEnsurePodEnforcesExistingHaltHoldOnRunningReplacement(t *testing.T) {
	for _, tt := range []struct {
		name          string
		conflictFirst bool
		wantErrFirst  bool
		wantUIDs      []types.UID
	}{
		{name: "deletes running replacement", wantUIDs: []types.UID{"replacement-pod"}},
		{name: "retries UID conflict with refreshed replacement", conflictFirst: true, wantErrFirst: true, wantUIDs: []types.UID{"replacement-pod", "newer-replacement-pod"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var deletes atomic.Int32
			var creates atomic.Int32
			var patches atomic.Int32
			var requestedUIDs []types.UID
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
					deletes.Add(1)
					var options metav1.DeleteOptions
					require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
					require.NotNil(t, options.Preconditions)
					require.NotNil(t, options.Preconditions.UID)
					requestedUIDs = append(requestedUIDs, *options.Preconditions.UID)
					if tt.conflictFirst && deletes.Load() == 1 {
						writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
							Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
							Code: http.StatusConflict, Message: "pod UID precondition failed",
						})
						return
					}
					writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
				case req.Method == http.MethodPost:
					creates.Add(1)
					http.NotFound(w, req)
				case req.Method == http.MethodPatch:
					patches.Add(1)
					http.NotFound(w, req)
				default:
					http.NotFound(w, req)
				}
			}))
			defer server.Close()

			nodeClient := &fakeTerminalNodeUtilsClient{heightErr: errors.New("node-utils polling must not run")}
			r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, nodeClient)
			haltHeight := int64(100)
			node.Spec.Config.HaltHeight = &haltHeight
			node.Annotations = map[string]string{appsv1.AnnotationHaltHeightHold: "100"}
			require.NoError(t, r.Update(t.Context(), node))

			desiredPod, err := r.getPodSpec(t.Context(), node, "config-hash")
			require.NoError(t, err)
			replacementPod := desiredPod.DeepCopy()
			replacementPod.UID = "replacement-pod"
			replacementPod.Status.Phase = corev1.PodRunning
			replacementPod.Status.ContainerStatuses = []corev1.ContainerStatus{{
				Name: node.Spec.App.App, Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}}
			replacementPod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
				Name: nodeUtilsContainerName, Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}}
			require.NoError(t, r.Delete(t.Context(), observedPod))
			require.NoError(t, r.Create(t.Context(), replacementPod))

			err = r.ensurePod(t.Context(), nil, node, "config-hash")
			if tt.wantErrFirst {
				require.Error(t, err)
				assert.True(t, apierrors.IsConflict(err))
				require.NoError(t, r.Delete(t.Context(), replacementPod))
				replacementPod = replacementPod.DeepCopy()
				replacementPod.ResourceVersion = ""
				replacementPod.UID = "newer-replacement-pod"
				require.NoError(t, r.Create(t.Context(), replacementPod))
				stored := &appsv1.ChainNode{}
				require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
				err = r.ensurePod(t.Context(), nil, stored, "config-hash")
			}
			require.NoError(t, err)
			assert.Equal(t, int32(len(tt.wantUIDs)), deletes.Load())
			assert.Equal(t, tt.wantUIDs, requestedUIDs)
			assert.Zero(t, creates.Load())
			assert.Zero(t, patches.Load())
			assert.Empty(t, nodeClient.calls)

			require.NoError(t, r.Delete(t.Context(), replacementPod))
			stored := &appsv1.ChainNode{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			require.NoError(t, r.ensurePod(t.Context(), nil, stored, "config-hash"))
			assert.Equal(t, int32(len(tt.wantUIDs)), deletes.Load())
			assert.Zero(t, creates.Load())
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])
			assert.Equal(t, appsv1.PhaseChainNodeStopped, stored.Status.Phase)
		})
	}
}

func TestEnsurePodRetriesWhenRunningNodeUtilsStatusIsUnavailable(t *testing.T) {
	for _, tt := range []struct {
		name    string
		client  *fakeTerminalNodeUtilsClient
		wantErr string
	}{
		{name: "latest height error after fresh negative", client: &fakeTerminalNodeUtilsClient{heightErr: errors.New("latest unavailable")}, wantErr: "latest unavailable"},
		{name: "upgrade query error", client: &fakeTerminalNodeUtilsClient{height: 100, upgradeErr: errors.New("upgrade unavailable")}, wantErr: "upgrade unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var mutations atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodDelete || req.Method == http.MethodPost {
					mutations.Add(1)
				}
				http.NotFound(w, req)
			}))
			defer server.Close()
			r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, tt.client)

			err := r.ensurePod(t.Context(), nil, node, "config-hash")
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			assert.Zero(t, mutations.Load())
			wantCalls := []string{"upgrade-fresh"}
			if tt.client.heightErr != nil {
				wantCalls = []string{"upgrade-fresh", "latest"}
			}
			assert.Equal(t, wantCalls, tt.client.calls)
			storedPod := &corev1.Pod{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(observedPod), storedPod))
			assert.Equal(t, map[string]string{"identity": "observed"}, storedPod.Labels)
		})
	}
}

func TestEnsurePodDoesNotMutatePodWhenFreshHeightStatusWriteFails(t *testing.T) {
	var mutations atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodDelete || req.Method == http.MethodPost {
			mutations.Add(1)
		}
		http.NotFound(w, req)
	}))
	defer server.Close()
	nodeClient := &fakeTerminalNodeUtilsClient{height: 100}
	r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, nodeClient)
	r.Client = &dataHeightResetPersistenceClient{Client: r.Client, failStatus: true}

	err := r.ensurePod(t.Context(), nil, node, "config-hash")
	require.ErrorContains(t, err, "status persistence failed")
	assert.Zero(t, mutations.Load())
	assert.Equal(t, []string{"upgrade-fresh", "latest"}, nodeClient.calls)
	storedPod := &corev1.Pod{}
	require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(observedPod), storedPod))
	assert.Equal(t, observedPod.UID, storedPod.UID)
}

func TestEnsurePodRecoversTerminalAppWhileNodeUtilsIsRunning(t *testing.T) {
	for _, tt := range []struct {
		name          string
		haltHeight    *int64
		podHaltHeight int64
		cachedHeight  int64
		freshHeight   int64
		wantHeight    int64
		appExitCode   int32
		appReason     string
		appSignal     int32
		addRenewer    bool
		wantHold      string
		deleteStatus  int
	}{
		{
			name: "OOM restarts without waiting for ordinary sidecar", cachedHeight: 98, freshHeight: 100, wantHeight: 100,
			appExitCode: 137, appReason: "OOMKilled", addRenewer: true, deleteStatus: http.StatusOK,
		},
		{
			name:       "clean configured halt at H minus one persists hold before delete",
			haltHeight: ptr.To[int64](100), podHaltHeight: 100, cachedHeight: 98, freshHeight: 99, wantHeight: 99,
			wantHold: "100", deleteStatus: http.StatusConflict,
		},
		{
			name:       "signal-style configured halt at H persists hold before delete",
			haltHeight: ptr.To[int64](100), podHaltHeight: 100, cachedHeight: 98, freshHeight: 100, wantHeight: 100,
			appExitCode: 143, appReason: "Error", wantHold: "100", deleteStatus: http.StatusConflict,
		},
		{
			name:       "eligible nonzero configured halt at H persists hold before delete",
			haltHeight: ptr.To[int64](100), podHaltHeight: 100, cachedHeight: 98, freshHeight: 100, wantHeight: 100,
			appExitCode: 1, appReason: "Error", wantHold: "100", deleteStatus: http.StatusConflict,
		},
		{
			name:       "clean configured exit at H minus two restarts",
			haltHeight: ptr.To[int64](100), podHaltHeight: 100, cachedHeight: 97, freshHeight: 98, wantHeight: 98, deleteStatus: http.StatusOK,
		},
		{
			name:       "older fresh height controls recovery without regressing durable progress",
			haltHeight: ptr.To[int64](100), podHaltHeight: 100, cachedHeight: 100, freshHeight: 98, wantHeight: 100, deleteStatus: http.StatusOK,
		},
		{
			name:       "clean configured exit past H restarts",
			haltHeight: ptr.To[int64](100), podHaltHeight: 100, cachedHeight: 98, freshHeight: 101, wantHeight: 101, deleteStatus: http.StatusOK,
		},
		{
			name:       "configured target mismatch restarts",
			haltHeight: ptr.To[int64](100), podHaltHeight: 99, cachedHeight: 98, freshHeight: 99, wantHeight: 99, deleteStatus: http.StatusOK,
		},
		{
			name: "fresh zero does not use cached H minus one", haltHeight: ptr.To[int64](100), podHaltHeight: 100,
			cachedHeight: 99, wantHeight: 99, deleteStatus: http.StatusOK,
		},
		{
			name: "fresh zero does not use cached H", haltHeight: ptr.To[int64](100), podHaltHeight: 100,
			cachedHeight: 100, wantHeight: 100, deleteStatus: http.StatusOK,
		},
		{
			name: "fresh zero does not hold halt height one from default status", haltHeight: ptr.To[int64](1), podHaltHeight: 1,
			cachedHeight: 0, wantHeight: 0, deleteStatus: http.StatusOK,
		},
		{
			name: "clean exit without configured halt restarts", cachedHeight: 98, freshHeight: 100, wantHeight: 100, deleteStatus: http.StatusOK,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var deletes atomic.Int32
			var logRequests atomic.Int32
			var requestedUID types.UID
			var holdAtDelete string
			var cache client.Client
			var livePod *corev1.Pod
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.URL.Path == "/api/v1/namespaces/default/pods/node/log":
					logRequests.Add(1)
					http.Error(w, "application logs unavailable", http.StatusInternalServerError)
				case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
					deletes.Add(1)
					var options metav1.DeleteOptions
					require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
					if options.Preconditions != nil && options.Preconditions.UID != nil {
						requestedUID = *options.Preconditions.UID
					}
					stored := &appsv1.ChainNode{}
					require.NoError(t, cache.Get(t.Context(), client.ObjectKey{Name: "node", Namespace: "default"}, stored))
					holdAtDelete = stored.Annotations[appsv1.AnnotationHaltHeightHold]
					if tt.deleteStatus == http.StatusConflict {
						writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
							Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict,
							Code: http.StatusConflict, Message: "pod UID precondition failed",
						})
						return
					}
					deleted = true
					writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
				case req.Method == http.MethodGet && req.URL.Path == "/api/v1/namespaces/default/pods/node":
					if !deleted {
						writeChainNodeTestJSON(t, w, http.StatusOK, livePod)
						return
					}
					writeChainNodeTestJSON(t, w, http.StatusNotFound, &metav1.Status{
						Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound,
						Code: http.StatusNotFound, Message: "old Pod deleted",
					})
				case req.Method == http.MethodPost && req.URL.Path == "/api/v1/namespaces/default/pods":
					writeChainNodeTestJSON(t, w, http.StatusInternalServerError, &metav1.Status{
						Status: metav1.StatusFailure, Reason: metav1.StatusReasonInternalError,
						Code: http.StatusInternalServerError, Message: "stop after selecting replacement",
					})
				default:
					http.NotFound(w, req)
				}
			}))
			defer server.Close()
			nodeClient := &fakeTerminalNodeUtilsClient{height: tt.freshHeight}
			r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, nodeClient)
			livePod = observedPod.DeepCopy()
			livePod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
			cache = r.Client
			node.Spec.Config.HaltHeight = tt.haltHeight
			if tt.haltHeight != nil {
				observedPod.Spec.InitContainers[0].Env = []corev1.EnvVar{{Name: "HALT_HEIGHT", Value: strconv.FormatInt(tt.podHaltHeight, 10)}}
			}
			terminated := observedPod.Status.ContainerStatuses[0].State.Terminated
			terminated.ExitCode = tt.appExitCode
			terminated.Reason = tt.appReason
			terminated.Signal = tt.appSignal
			if tt.addRenewer {
				observedPod.Spec.InitContainers = append(observedPod.Spec.InitContainers, corev1.Container{Name: "vault-token-renewer"})
				observedPod.Status.InitContainerStatuses = append(observedPod.Status.InitContainerStatuses, corev1.ContainerStatus{
					Name: "vault-token-renewer", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				})
			}
			require.NoError(t, r.Update(t.Context(), node))
			node.Status.LatestHeight = tt.cachedHeight
			require.NoError(t, r.Status().Update(t.Context(), node))
			require.NoError(t, r.Update(t.Context(), observedPod))
			require.NoError(t, r.Status().Update(t.Context(), observedPod))

			err := r.ensurePod(t.Context(), nil, node, "config-hash")
			require.Error(t, err)
			assert.Equal(t, int32(1), deletes.Load())
			assert.Zero(t, logRequests.Load())
			assert.Equal(t, observedPod.UID, requestedUID)
			assert.Equal(t, tt.wantHold, holdAtDelete)
			stored := &appsv1.ChainNode{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, tt.wantHold, stored.Annotations[appsv1.AnnotationHaltHeightHold])
			assert.Equal(t, tt.wantHeight, stored.Status.LatestHeight)
			assert.Equal(t, []string{"upgrade-fresh", "latest"}, nodeClient.calls)
		})
	}
}

func TestEnsurePodRecreatesCleanExitAcrossReconcilesOffHaltBoundary(t *testing.T) {
	var deletes atomic.Int32
	var creates atomic.Int32
	var createdImages []string
	var livePod *corev1.Pod
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			deletes.Add(1)
			deleted = true
			writeChainNodeTestJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/namespaces/default/pods/node":
			if !deleted {
				writeChainNodeTestJSON(t, w, http.StatusOK, livePod)
				return
			}
			writeChainNodeTestJSON(t, w, http.StatusNotFound, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound,
				Code: http.StatusNotFound, Message: "old Pod deleted",
			})
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/namespaces/default/pods":
			var pod corev1.Pod
			require.NoError(t, json.NewDecoder(req.Body).Decode(&pod))
			creates.Add(1)
			createdImages = append(createdImages, pod.Spec.Containers[0].Image)
			writeChainNodeTestJSON(t, w, http.StatusInternalServerError, &metav1.Status{
				Status: metav1.StatusFailure, Reason: metav1.StatusReasonInternalError,
				Code: http.StatusInternalServerError, Message: "simulated create failure",
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	nodeClient := &fakeTerminalNodeUtilsClient{height: 98}
	r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, nodeClient)
	livePod = observedPod.DeepCopy()
	livePod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	haltHeight := int64(100)
	node.Spec.Config.HaltHeight = &haltHeight
	observedPod.Spec.InitContainers[0].Env = []corev1.EnvVar{{Name: "HALT_HEIGHT", Value: "100"}}
	require.NoError(t, r.Update(t.Context(), node))
	require.NoError(t, r.Update(t.Context(), observedPod))
	require.Error(t, r.ensurePod(t.Context(), nil, node, "config-hash"))
	require.Equal(t, int32(1), deletes.Load())
	require.Equal(t, int32(1), creates.Load())
	require.NoError(t, r.Client.Delete(t.Context(), observedPod))

	require.Error(t, r.ensurePod(t.Context(), nil, node, "config-hash"))
	assert.Equal(t, int32(2), creates.Load(), "an off-boundary exit must not suppress the replacement retry")
	assert.Equal(t, []string{"app:v1", "app:v1"}, createdImages)
	stored := &appsv1.ChainNode{}
	require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
	assert.NotContains(t, stored.Annotations, appsv1.AnnotationHaltHeightHold)
}

func TestEnsurePodKeepsVerifiedHaltStoppedWhenPodIsMissing(t *testing.T) {
	var creates atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			creates.Add(1)
		}
		http.NotFound(w, req)
	}))
	defer server.Close()
	r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, &fakeTerminalNodeUtilsClient{})
	haltHeight := int64(100)
	node.Spec.Config.HaltHeight = &haltHeight
	node.Annotations = map[string]string{appsv1.AnnotationHaltHeightHold: "100"}
	node.Status.LatestHeight = 98
	require.NoError(t, r.Update(t.Context(), node))
	require.NoError(t, r.Client.Delete(t.Context(), observedPod))

	require.NoError(t, r.ensurePod(t.Context(), nil, node, "config-hash"))
	assert.Zero(t, creates.Load())
	stored := &appsv1.ChainNode{}
	require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
	assert.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])
	assert.Equal(t, appsv1.PhaseChainNodeStopped, stored.Status.Phase)
}

func TestEnsurePodMigratesLegacyStoppedHaltWhenPodIsMissing(t *testing.T) {
	var creates atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			creates.Add(1)
		}
		http.NotFound(w, req)
	}))
	defer server.Close()
	r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, &fakeTerminalNodeUtilsClient{})
	haltHeight := int64(100)
	node.Spec.Config.HaltHeight = &haltHeight
	require.NoError(t, r.Update(t.Context(), node))
	node.Status.LatestHeight = haltHeight
	node.Status.Phase = appsv1.PhaseChainNodeStopped
	node.Annotations = nil
	require.NoError(t, r.Status().Update(t.Context(), node))
	require.NoError(t, r.Client.Delete(t.Context(), observedPod))

	require.NoError(t, r.ensurePod(t.Context(), nil, node, "config-hash"))
	assert.Zero(t, creates.Load())
	stored := &appsv1.ChainNode{}
	require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
	assert.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])
	assert.Equal(t, appsv1.PhaseChainNodeStopped, stored.Status.Phase)
}

func TestEnsurePodMigratesLegacyStoppedHaltOnlyAfterTerminatingPodIsGone(t *testing.T) {
	for _, tt := range []struct {
		name              string
		replacement       bool
		wantHold          string
		wantAuthoritative int32
	}{
		{name: "old pod is gone", wantHold: "100", wantAuthoritative: 2},
		{name: "same-name replacement exists", replacement: true, wantAuthoritative: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var creates atomic.Int32
			var gets atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/api/v1/namespaces/default/pods/node":
					gets.Add(1)
					if tt.replacement {
						writeChainNodeTestJSON(t, w, http.StatusOK, &corev1.Pod{
							TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
							ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "replacement-pod"},
						})
						return
					}
					writeChainNodeTestJSON(t, w, http.StatusNotFound, &metav1.Status{
						Status: metav1.StatusFailure, Reason: metav1.StatusReasonNotFound,
						Code: http.StatusNotFound, Message: "old Pod deleted",
					})
				case req.Method == http.MethodPost && req.URL.Path == "/api/v1/namespaces/default/pods":
					creates.Add(1)
					writeChainNodeTestJSON(t, w, http.StatusConflict, &metav1.Status{
						Status: metav1.StatusFailure, Reason: metav1.StatusReasonAlreadyExists,
						Code: http.StatusConflict, Message: "replacement already exists",
					})
				default:
					http.NotFound(w, req)
				}
			}))
			defer server.Close()
			r, node, observedPod := terminalWaitEnsurePodFixture(t, server.URL, &fakeTerminalNodeUtilsClient{})
			config := &corev1.ConfigMap{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), config))
			haltHeight := int64(100)
			node.Spec.Config.HaltHeight = &haltHeight
			node.Status.LatestHeight = haltHeight
			node.Status.Phase = appsv1.PhaseChainNodeStopped
			now := metav1.Now()
			observedPod.DeletionTimestamp = &now
			observedPod.Finalizers = []string{"test.cosmopilot.voluzi.com/terminating"}
			r.Client = fake.NewClientBuilder().WithScheme(r.Scheme).WithStatusSubresource(node).
				WithObjects(node, observedPod, config).Build()

			require.NoError(t, r.ensurePod(t.Context(), nil, node, "config-hash"))
			assert.Zero(t, creates.Load())
			assert.Equal(t, tt.wantAuthoritative, gets.Load())
			stored := &appsv1.ChainNode{}
			require.NoError(t, r.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, tt.wantHold, stored.Annotations[appsv1.AnnotationHaltHeightHold])
		})
	}
}

type fakeTerminalNodeUtilsClient struct {
	height                int64
	heightErr             error
	onLatest              func()
	requiresUpgrade       bool
	cachedRequiresUpgrade bool
	upgradeErr            error
	calls                 []string
}

func terminalHaltEnsurePodFixture(
	t *testing.T,
	apiServer string,
	cachedHeight int64,
	evidenceHeight *int64,
) (*Reconciler, *appsv1.ChainNode, *corev1.Pod) {
	t.Helper()
	haltHeight := int64(100)
	node := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "node-uid"},
		Spec: appsv1.ChainNodeSpec{
			App:    appsv1.AppSpec{App: "appd", Image: "app", Version: ptr.To("v1")},
			Config: &appsv1.Config{HaltHeight: &haltHeight},
		},
		Status: appsv1.ChainNodeStatus{LatestHeight: cachedHeight, Phase: appsv1.PhaseChainNodeRestarting},
	}
	observedPod := terminalEvidencePod(t, haltHeight, haltHeight, 0, false, 0, "Completed")
	observedPod.Name = node.Name
	observedPod.Namespace = node.Namespace
	evidence, ok := nodeUtilsTerminationEvidence(observedPod)
	require.True(t, ok)
	evidence.LatestHeight = evidenceHeight
	body, err := json.Marshal(evidence)
	require.NoError(t, err)
	observedPod.Status.InitContainerStatuses[0].State.Terminated.Message = string(body)
	config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: node.Name, Namespace: node.Namespace}}

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cache := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).
		WithObjects(node, observedPod, config).Build()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: apiServer})
	require.NoError(t, err)
	r := &Reconciler{
		Client:    cache,
		ClientSet: clientSet,
		Scheme:    scheme,
		recorder:  record.NewFakeRecorder(10),
		opts:      &controllers.ControllerRunOptions{NodeUtilsImage: "node-utils:test"},
	}
	return r, node, observedPod
}

func (c *fakeTerminalNodeUtilsClient) GetLatestHeight(context.Context) (int64, error) {
	c.calls = append(c.calls, "latest")
	if c.onLatest != nil {
		c.onLatest()
	}
	return c.height, c.heightErr
}

func (c *fakeTerminalNodeUtilsClient) RequiresUpgrade(context.Context) (bool, error) {
	c.calls = append(c.calls, "upgrade")
	return c.cachedRequiresUpgrade, c.upgradeErr
}

func (c *fakeTerminalNodeUtilsClient) RequiresUpgradeFresh(context.Context) (bool, error) {
	c.calls = append(c.calls, "upgrade-fresh")
	return c.requiresUpgrade, c.upgradeErr
}

func terminalWaitEnsurePodFixture(
	t *testing.T,
	apiServer string,
	nodeUtilsClient *fakeTerminalNodeUtilsClient,
) (*Reconciler, *appsv1.ChainNode, *corev1.Pod) {
	t.Helper()
	graceSeconds := int64(600)
	node := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "node-uid"},
		Spec: appsv1.ChainNodeSpec{
			App: appsv1.AppSpec{App: "appd", Image: "app", Version: ptr.To("v1")},
			Config: &appsv1.Config{
				TerminationGracePeriodSeconds: &graceSeconds,
			},
		},
		Status: appsv1.ChainNodeStatus{
			LatestHeight: 98,
			Phase:        appsv1.PhaseChainNodeRunning,
			Upgrades: []appsv1.Upgrade{{
				Height: 100, Image: "app:v2", Status: appsv1.UpgradeScheduled,
			}},
		},
	}
	observedPod := terminalEvidencePod(t, 0, 0, 98, false, 0, "Completed")
	observedPod.Name = node.Name
	observedPod.Namespace = node.Namespace
	observedPod.Labels = map[string]string{"identity": "observed"}
	observedPod.Spec.TerminationGracePeriodSeconds = &graceSeconds
	observedPod.Status.InitContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	config := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: node.Name, Namespace: node.Namespace},
		Data: map[string]string{
			appTomlFilename:    "",
			configTomlFilename: "",
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	cache := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).
		WithObjects(node, observedPod, config).Build()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: apiServer})
	require.NoError(t, err)
	configCache := ttlcache.New[string, map[string]interface{}]()
	configCache.Set("app:v2", map[string]interface{}{
		appTomlFilename:    map[string]interface{}{},
		configTomlFilename: map[string]interface{}{},
	}, ttlcache.DefaultTTL)
	r := &Reconciler{
		Client:      cache,
		ClientSet:   clientSet,
		Scheme:      scheme,
		configCache: configCache,
		configLocks: newConfigLockManager(),
		recorder:    record.NewFakeRecorder(10),
		opts:        &controllers.ControllerRunOptions{NodeUtilsImage: "node-utils:test"},
		nodeStatusClientFactory: func(string) nodeStatusClient {
			return nodeUtilsClient
		},
	}
	return r, node, observedPod
}

func writeChainNodeTestJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(value))
}
