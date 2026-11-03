package chainnode

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
)

func TestTerminalPodRecoveryRequiresBoundHaltEvidence(t *testing.T) {
	for _, tt := range []struct {
		name              string
		cachedHeight      int64
		podHaltHeight     int64
		evidenceTarget    int64
		evidenceHeight    *int64
		forced            bool
		conflictingEnv    bool
		malformedEvidence bool
		appExitCode       int32
		appReason         string
		appSignal         int32
		clearIdentity     bool
		wantAction        terminalPodRecoveryAction
		wantHold          string
	}{
		{name: "clean exit at H minus one holds despite stale cache", cachedHeight: 98, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), wantAction: terminalPodHold, wantHold: "100"},
		{name: "clean exit at H holds despite stale cache", cachedHeight: 98, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](100), wantAction: terminalPodHold, wantHold: "100"},
		{name: "signal-style exit at boundary holds", cachedHeight: 98, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), appExitCode: 143, appReason: "Error", wantAction: terminalPodHold, wantHold: "100"},
		{name: "eligible nonzero exit at boundary holds", cachedHeight: 98, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](100), appExitCode: 1, appReason: "Error", wantAction: terminalPodHold, wantHold: "100"},
		{name: "container identity is not required", cachedHeight: 98, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), clearIdentity: true, wantAction: terminalPodHold, wantHold: "100"},
		{name: "evidence at H minus two restarts", cachedHeight: 98, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](98), wantAction: terminalPodRestart},
		{name: "evidence past H restarts", cachedHeight: 100, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](101), wantAction: terminalPodRestart},
		{name: "missing evidence height restarts", cachedHeight: 100, podHaltHeight: 100, evidenceTarget: 100, wantAction: terminalPodRestart},
		{name: "current evidence overrides cached boundary", cachedHeight: 100, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](98), wantAction: terminalPodRestart},
		{name: "OOM at boundary restarts", cachedHeight: 99, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), appExitCode: 137, appReason: "OOMKilled", wantAction: terminalPodRestart},
		{name: "SIGKILL at boundary restarts", cachedHeight: 99, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), appExitCode: 1, appReason: "Error", appSignal: 9, wantAction: terminalPodRestart},
		{name: "forced shutdown restarts", cachedHeight: 99, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), forced: true, wantAction: terminalPodRestart},
		{name: "different pod halt target restarts", cachedHeight: 99, podHaltHeight: 99, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), wantAction: terminalPodRestart},
		{name: "different evidence target restarts", cachedHeight: 99, podHaltHeight: 100, evidenceTarget: 99, evidenceHeight: ptr.To[int64](99), wantAction: terminalPodRestart},
		{name: "ambiguous pod halt target restarts", cachedHeight: 99, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), conflictingEnv: true, wantAction: terminalPodRestart},
		{name: "malformed current evidence restarts", cachedHeight: 99, podHaltHeight: 100, evidenceTarget: 100, evidenceHeight: ptr.To[int64](99), malformedEvidence: true, wantAction: terminalPodRestart},
	} {
		t.Run(tt.name, func(t *testing.T) {
			haltHeight := int64(100)
			node := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
				Spec: appsv1.ChainNodeSpec{
					App:    appsv1.AppSpec{App: "appd"},
					Config: &appsv1.Config{HaltHeight: &haltHeight},
				},
				Status: appsv1.ChainNodeStatus{LatestHeight: tt.cachedHeight},
			}
			pod := terminalEvidencePod(t, tt.podHaltHeight, tt.evidenceTarget, 0, tt.forced, tt.appExitCode, tt.appReason)
			evidence, ok := nodeUtilsTerminationEvidence(pod)
			require.True(t, ok)
			evidence.LatestHeight = tt.evidenceHeight
			body, err := json.Marshal(evidence)
			require.NoError(t, err)
			pod.Status.InitContainerStatuses[0].State.Terminated.Message = string(body)
			if tt.malformedEvidence {
				pod.Status.InitContainerStatuses[0].State.Terminated.Message = "not-json"
			}
			pod.Status.ContainerStatuses[0].State.Terminated.Signal = tt.appSignal
			if tt.clearIdentity {
				terminated := pod.Status.ContainerStatuses[0].State.Terminated
				terminated.ContainerID = ""
				terminated.StartedAt = metav1.Time{}
				terminated.FinishedAt = metav1.Time{}
			}
			if tt.conflictingEnv {
				pod.Spec.InitContainers[0].Env = append(pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{Name: "HALT_HEIGHT", Value: "101"})
			}

			assert.Equal(t, tt.wantAction, terminalPodRecoveryFor(node, pod))

			scheme := runtime.NewScheme()
			require.NoError(t, appsv1.AddToScheme(scheme))
			backing := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()
			r := &Reconciler{Client: backing}
			require.NoError(t, r.reconcileHaltHeightHold(t.Context(), node, pod))

			stored := &appsv1.ChainNode{}
			require.NoError(t, backing.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, tt.wantHold, stored.Annotations[appsv1.AnnotationHaltHeightHold])
		})
	}
}

func TestTerminalPodRecoveryUsesOnlyMatchingPendingUpgradeEvidence(t *testing.T) {
	for _, source := range []struct {
		name string
		app  appsv1.UpgradeSource
		node nodeutils.UpgradeSource
	}{
		{name: "on-chain", app: appsv1.OnChainUpgrade, node: nodeutils.OnChainUpgrade},
		{name: "manual", app: appsv1.ManualUpgrade, node: nodeutils.ManualUpgrade},
	} {
		t.Run(source.name, func(t *testing.T) {
			node := &appsv1.ChainNode{
				Spec:   appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{}},
				Status: appsv1.ChainNodeStatus{Upgrades: []appsv1.Upgrade{{Height: 100, Source: source.app, Status: appsv1.UpgradeScheduled}}},
			}
			pod := terminalEvidencePod(t, 0, 0, 98, false, 1, "Error")
			evidence, ok := nodeUtilsTerminationEvidence(pod)
			require.True(t, ok)
			evidence.RequiredUpgrade = &nodeutils.RequiredUpgrade{Height: 100, Source: source.node}
			body, err := json.Marshal(evidence)
			require.NoError(t, err)
			pod.Status.InitContainerStatuses[0].State.Terminated.Message = string(body)

			assert.Equal(t, terminalPodUpgrade, terminalPodRecoveryFor(node, pod))
			node.Status.Upgrades[0].Status = appsv1.UpgradeOnGoing
			assert.Equal(t, terminalPodUpgrade, terminalPodRecoveryFor(node, pod))
			node.Status.Upgrades[0].Status = appsv1.UpgradeScheduled
			if source.node == nodeutils.OnChainUpgrade {
				evidence.RequiredUpgrade.Source = nodeutils.ManualUpgrade
			} else {
				evidence.RequiredUpgrade.Source = nodeutils.OnChainUpgrade
			}
			body, err = json.Marshal(evidence)
			require.NoError(t, err)
			pod.Status.InitContainerStatuses[0].State.Terminated.Message = string(body)
			assert.Equal(t, terminalPodRestart, terminalPodRecoveryFor(node, pod))
			evidence.RequiredUpgrade.Source = source.node
			body, err = json.Marshal(evidence)
			require.NoError(t, err)
			pod.Status.InitContainerStatuses[0].State.Terminated.Message = string(body)
			node.Status.Upgrades[0].Status = appsv1.UpgradeCompleted
			assert.Equal(t, terminalPodRestart, terminalPodRecoveryFor(node, pod))
		})
	}
}

func TestTerminalPodRecoveryWaitsForCurrentSidecarEvidence(t *testing.T) {
	node := &appsv1.ChainNode{Spec: appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{}}}
	pod := terminalEvidencePod(t, 0, 0, 98, false, 137, "OOMKilled")
	pod.Status.InitContainerStatuses[0].State.Terminated = nil
	pod.Status.InitContainerStatuses[0].State.Running = &corev1.ContainerStateRunning{}

	assert.Equal(t, terminalPodWaitForEvidence, terminalPodRecoveryFor(node, pod))
}

func TestTerminalPodRecoveryRequiresCurrentAppTermination(t *testing.T) {
	haltHeight := int64(100)
	node := &appsv1.ChainNode{Spec: appsv1.ChainNodeSpec{
		App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{HaltHeight: &haltHeight},
	}}
	for _, tt := range []struct {
		name        string
		state       corev1.ContainerState
		phase       corev1.PodPhase
		terminating bool
		want        terminalPodRecoveryAction
	}{
		{name: "running application", state: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, want: terminalPodNotTerminated},
		{name: "waiting application", state: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}, want: terminalPodNotTerminated},
		{name: "failed pod without application termination", phase: corev1.PodFailed, want: terminalPodRestart},
		{name: "terminating pod", state: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}, terminating: true, want: terminalPodNotTerminated},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pod := terminalEvidencePod(t, 100, 100, 99, false, 0, "Completed")
			pod.Status.ContainerStatuses[0].State = tt.state
			pod.Status.Phase = tt.phase
			if tt.terminating {
				now := metav1.Now()
				pod.DeletionTimestamp = &now
				pod.Finalizers = []string{"test.cosmopilot.voluzi.com/terminating"}
			}
			assert.Equal(t, tt.want, terminalPodRecoveryFor(node, pod))
		})
	}
}

func TestTerminalPodRecoveryRejectsNonPositiveHaltTarget(t *testing.T) {
	for _, haltHeight := range []int64{0, -1} {
		node := &appsv1.ChainNode{Spec: appsv1.ChainNodeSpec{
			App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{HaltHeight: &haltHeight},
		}}
		pod := terminalEvidencePod(t, haltHeight, haltHeight, haltHeight, false, 0, "Completed")
		assert.Equal(t, terminalPodRestart, terminalPodRecoveryFor(node, pod))
	}
}

func TestTerminalPodRecoveryWithoutNodeUtilsEvidenceUsesStructuredBoundary(t *testing.T) {
	for _, tt := range []struct {
		name         string
		latestHeight int64
		podTarget    int64
		exitCode     int32
		reason       string
		signal       int32
		wantAction   terminalPodRecoveryAction
	}{
		{name: "clean exit at H minus one holds", latestHeight: 99, podTarget: 100, wantAction: terminalPodHold},
		{name: "clean exit at H holds", latestHeight: 100, podTarget: 100, wantAction: terminalPodHold},
		{name: "signal-style exit at boundary holds", latestHeight: 99, podTarget: 100, exitCode: 143, reason: "Error", wantAction: terminalPodHold},
		{name: "eligible nonzero exit at boundary holds", latestHeight: 100, podTarget: 100, exitCode: 1, reason: "Error", wantAction: terminalPodHold},
		{name: "H minus two restarts", latestHeight: 98, podTarget: 100, wantAction: terminalPodRestart},
		{name: "past H restarts", latestHeight: 101, podTarget: 100, wantAction: terminalPodRestart},
		{name: "target mismatch restarts", latestHeight: 99, podTarget: 99, wantAction: terminalPodRestart},
		{name: "OOM restarts", latestHeight: 99, podTarget: 100, exitCode: 137, reason: "OOMKilled", wantAction: terminalPodRestart},
		{name: "SIGKILL restarts", latestHeight: 99, podTarget: 100, exitCode: 1, reason: "Error", signal: 9, wantAction: terminalPodRestart},
	} {
		t.Run(tt.name, func(t *testing.T) {
			haltHeight := int64(100)
			node := &appsv1.ChainNode{
				Spec:   appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{HaltHeight: &haltHeight}},
				Status: appsv1.ChainNodeStatus{LatestHeight: tt.latestHeight},
			}
			pod := terminalEvidencePod(t, tt.podTarget, haltHeight, tt.latestHeight, false, tt.exitCode, tt.reason)
			pod.Status.ContainerStatuses[0].State.Terminated.Signal = tt.signal

			assert.Equal(t, tt.wantAction, terminalPodRecoveryWithoutNodeUtilsEvidence(node, pod, tt.latestHeight, true))
		})
	}
}

func TestTerminalPodRecoveryWithoutNodeUtilsEvidenceNoOpsForUnavailablePod(t *testing.T) {
	node := &appsv1.ChainNode{Spec: appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}}}
	assert.Equal(t, terminalPodNotTerminated, terminalPodRecoveryWithoutNodeUtilsEvidence(node, nil, 0, false))

	pod := terminalEvidencePod(t, 100, 100, 99, false, 0, "Completed")
	now := metav1.Now()
	pod.DeletionTimestamp = &now
	pod.Finalizers = []string{"test.cosmopilot.voluzi.com/terminating"}
	assert.Equal(t, terminalPodNotTerminated, terminalPodRecoveryWithoutNodeUtilsEvidence(node, pod, 0, false))
}

func TestReconcileHaltHeightHoldClearsStaleIntent(t *testing.T) {
	for _, tt := range []struct {
		name       string
		haltHeight *int64
	}{
		{name: "target changed", haltHeight: ptr.To[int64](101)},
		{name: "target removed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			node := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", Annotations: map[string]string{appsv1.AnnotationHaltHeightHold: "100"}},
				Spec:       appsv1.ChainNodeSpec{App: appsv1.AppSpec{App: "appd"}, Config: &appsv1.Config{HaltHeight: tt.haltHeight}},
			}
			pod := terminalEvidencePod(t, 100, 100, 99, false, 0, "Completed")
			scheme := runtime.NewScheme()
			require.NoError(t, appsv1.AddToScheme(scheme))
			backing := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()
			r := &Reconciler{Client: backing}

			require.NoError(t, r.reconcileHaltHeightHold(t.Context(), node, pod))

			stored := &appsv1.ChainNode{}
			require.NoError(t, backing.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.NotContains(t, stored.Annotations, appsv1.AnnotationHaltHeightHold)
			mustStop, _ := stored.MustStop()
			assert.False(t, mustStop)
		})
	}
}

func TestShouldMigrateLegacyHaltHeightHold(t *testing.T) {
	for _, tt := range []struct {
		name         string
		phase        appsv1.ChainNodePhase
		latest       int64
		haltHeight   *int64
		existingHold string
		want         bool
	}{
		{name: "legacy stopped at target", phase: appsv1.PhaseChainNodeStopped, latest: 100, haltHeight: ptr.To[int64](100), want: true},
		{name: "running phase", phase: appsv1.PhaseChainNodeRunning, latest: 100, haltHeight: ptr.To[int64](100)},
		{name: "restarting phase", phase: appsv1.PhaseChainNodeRestarting, latest: 100, haltHeight: ptr.To[int64](100)},
		{name: "height mismatch", phase: appsv1.PhaseChainNodeStopped, latest: 99, haltHeight: ptr.To[int64](100)},
		{name: "removed target", phase: appsv1.PhaseChainNodeStopped, latest: 100},
		{name: "zero target", phase: appsv1.PhaseChainNodeStopped, haltHeight: ptr.To[int64](0)},
		{name: "matching hold already exists", phase: appsv1.PhaseChainNodeStopped, latest: 100, haltHeight: ptr.To[int64](100), existingHold: "100"},
		{name: "stale hold is not migrated", phase: appsv1.PhaseChainNodeStopped, latest: 100, haltHeight: ptr.To[int64](100), existingHold: "99"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			node := &appsv1.ChainNode{
				Spec:   appsv1.ChainNodeSpec{Config: &appsv1.Config{HaltHeight: tt.haltHeight}},
				Status: appsv1.ChainNodeStatus{Phase: tt.phase, LatestHeight: tt.latest},
			}
			if tt.existingHold != "" {
				node.Annotations = map[string]string{appsv1.AnnotationHaltHeightHold: tt.existingHold}
			}
			assert.Equal(t, tt.want, shouldMigrateLegacyHaltHeightHold(node))
		})
	}
}

func terminalEvidencePod(
	t *testing.T,
	podHaltHeight int64,
	evidenceTarget int64,
	latestHeight int64,
	forced bool,
	appExitCode int32,
	appReason string,
) *corev1.Pod {
	t.Helper()
	evidence := nodeutils.TerminationEvidence{
		UpgradeStatus:  nodeutils.UpgradeStatus{LatestHeight: &latestHeight},
		HaltHeight:     evidenceTarget,
		ForcedShutdown: forced,
	}
	body, err := json.Marshal(evidence)
	require.NoError(t, err)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{UID: "pod-uid"},
		Spec: corev1.PodSpec{InitContainers: []corev1.Container{{
			Name: nodeUtilsContainerName,
			Env:  []corev1.EnvVar{{Name: "HALT_HEIGHT", Value: strconv.FormatInt(podHaltHeight, 10)}},
		}}},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:        "appd",
				ContainerID: "containerd://app",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode: appExitCode, Reason: appReason, ContainerID: "containerd://app",
					StartedAt:  metav1.NewTime(time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)),
					FinishedAt: metav1.NewTime(time.Date(2026, 9, 21, 12, 1, 0, 0, time.UTC)),
				}},
			}},
			InitContainerStatuses: []corev1.ContainerStatus{{
				Name:  nodeUtilsContainerName,
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Message: string(body)}},
			}},
		},
	}
}
