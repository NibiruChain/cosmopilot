package chainnode

import (
	"context"
	"errors"
	"strings"
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
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
)

func TestGeneratedPodComponentsContainNoTraceStoreArtifacts(t *testing.T) {
	chainNode := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node"},
		Spec: appsv1.ChainNodeSpec{
			App:    appsv1.AppSpec{App: "appd"},
			Config: &appsv1.Config{},
		},
	}
	r := &Reconciler{opts: &controllers.ControllerRunOptions{NodeUtilsImage: "node-utils:test"}}

	for _, volume := range r.buildBaseVolumes(chainNode) {
		if volume.Name == "trace" {
			t.Fatal("generated pod still contains trace volume")
		}
	}
	nodeUtils := r.buildNodeUtilsInitContainer(chainNode, 1000, ptr.To[int64](1000))
	for _, env := range nodeUtils.Env {
		if env.Name == "TRACE_STORE" || env.Name == "CREATE_FIFO" {
			t.Fatalf("node-utils still contains trace environment %s", env.Name)
		}
	}
	for _, mount := range nodeUtils.VolumeMounts {
		if mount.Name == "trace" || mount.MountPath == "/trace" {
			t.Fatalf("node-utils still contains trace mount %#v", mount)
		}
	}
	app := r.buildAppContainer(chainNode, nil, "/ready", corev1.ResourceRequirements{}, nil)
	for i, arg := range append(nodeUtils.Args, app.Args...) {
		if arg == "--trace-store" || strings.HasPrefix(arg, "--trace-store=") ||
			arg == "--create-fifo" || strings.HasPrefix(arg, "--create-fifo=") ||
			arg == "/trace/trace.fifo" {
			t.Fatalf("app arg %d still contains trace artifact %q", i, arg)
		}
	}
	for _, mount := range app.VolumeMounts {
		if mount.Name == "trace" || mount.MountPath == "/trace" {
			t.Fatalf("app still contains trace mount %#v", mount)
		}
	}
}

func TestGetPodSpecUsesEffectiveAppIdentityForNodeUtils(t *testing.T) {
	for _, tt := range []struct {
		name        string
		app         *corev1.SecurityContext
		wantUser    int64
		wantGroup   int64
		wantNonRoot bool
	}{
		{name: "pod-only override does not replace default app identity", wantUser: 1000, wantGroup: 1000, wantNonRoot: true},
		{
			name: "app override takes precedence over pod identity",
			app: &corev1.SecurityContext{
				RunAsUser:  ptr.To[int64](3000),
				RunAsGroup: ptr.To[int64](3001),
			},
			wantUser:    3000,
			wantGroup:   3001,
			wantNonRoot: true,
		},
		{
			name: "explicit numeric root identity is mirrored for compatibility",
			app: &corev1.SecurityContext{
				RunAsUser:  ptr.To[int64](0),
				RunAsGroup: ptr.To[int64](0),
			},
			wantUser: 0, wantGroup: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			if err := appsv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			chainNode := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
				Spec: appsv1.ChainNodeSpec{
					App: appsv1.AppSpec{App: "appd"},
					Config: &appsv1.Config{
						SecurityContext: tt.app,
						PodSecurityContext: &corev1.PodSecurityContext{
							RunAsUser:  ptr.To[int64](2000),
							RunAsGroup: ptr.To[int64](2001),
						},
					},
				},
			}
			config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: chainNode.Name, Namespace: chainNode.Namespace}}
			r := &Reconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(config).Build(),
				Scheme: scheme,
				opts:   &controllers.ControllerRunOptions{NodeUtilsImage: "node-utils:test"},
			}

			pod, err := r.getPodSpec(t.Context(), chainNode, "config-hash")
			if err != nil {
				t.Fatal(err)
			}
			app := pod.Spec.Containers[0]
			nodeUtils := pod.Spec.InitContainers[0]
			if got := *app.SecurityContext.RunAsUser; got != tt.wantUser {
				t.Fatalf("app runAsUser = %d, want %d", got, tt.wantUser)
			}
			if got := *nodeUtils.SecurityContext.RunAsUser; got != tt.wantUser {
				t.Fatalf("node-utils runAsUser = %d, want %d", got, tt.wantUser)
			}
			if got := *nodeUtils.SecurityContext.RunAsGroup; got != tt.wantGroup {
				t.Fatalf("node-utils runAsGroup = %d, want %d", got, tt.wantGroup)
			}
			if got := *nodeUtils.SecurityContext.RunAsNonRoot; got != tt.wantNonRoot {
				t.Fatalf("node-utils runAsNonRoot = %t, want %t", got, tt.wantNonRoot)
			}
			require.Contains(t, nodeUtils.SecurityContext.Capabilities.Drop, corev1.Capability("ALL"))
		})
	}
}

func TestGetPodSpecAllowsUIDWithoutGIDForNodeUtils(t *testing.T) {
	for _, tt := range []struct {
		name     string
		app      *corev1.SecurityContext
		pod      *corev1.PodSecurityContext
		wantUser int64
	}{
		{
			name:     "app UID only",
			app:      &corev1.SecurityContext{RunAsUser: ptr.To[int64](3000)},
			pod:      &corev1.PodSecurityContext{},
			wantUser: 3000,
		},
		{
			name:     "pod UID only",
			app:      &corev1.SecurityContext{},
			pod:      &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](2000)},
			wantUser: 2000,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			chainNode := &appsv1.ChainNode{
				ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default"},
				Spec: appsv1.ChainNodeSpec{
					App:    appsv1.AppSpec{App: "appd"},
					Config: &appsv1.Config{SecurityContext: tt.app, PodSecurityContext: tt.pod},
				},
			}
			config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: chainNode.Name, Namespace: chainNode.Namespace}}
			r := &Reconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(config).Build(),
				Scheme: scheme,
				opts:   &controllers.ControllerRunOptions{NodeUtilsImage: "node-utils:test"},
			}

			pod, err := r.getPodSpec(t.Context(), chainNode, "config-hash")
			require.NoError(t, err)
			nodeUtils := pod.Spec.InitContainers[0]
			require.NotNil(t, nodeUtils.SecurityContext.RunAsUser)
			assert.Equal(t, tt.wantUser, *nodeUtils.SecurityContext.RunAsUser)
			assert.Nil(t, nodeUtils.SecurityContext.RunAsGroup)
			assert.True(t, *nodeUtils.SecurityContext.RunAsNonRoot)
			require.Contains(t, nodeUtils.SecurityContext.Capabilities.Drop, corev1.Capability("ALL"))
		})
	}
}

func TestEffectiveRunIdentityRejectsNegativeExplicitIDs(t *testing.T) {
	for _, tt := range []struct {
		name string
		app  *corev1.SecurityContext
		pod  *corev1.PodSecurityContext
	}{
		{
			name: "negative app UID",
			app:  &corev1.SecurityContext{RunAsUser: ptr.To[int64](-1)},
		},
		{
			name: "negative pod UID",
			pod:  &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](-1)},
		},
		{
			name: "negative app GID",
			app: &corev1.SecurityContext{
				RunAsUser: ptr.To[int64](1000), RunAsGroup: ptr.To[int64](-1),
			},
		},
		{
			name: "negative pod GID",
			pod: &corev1.PodSecurityContext{
				RunAsUser: ptr.To[int64](1000), RunAsGroup: ptr.To[int64](-1),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := effectiveRunIdentity(tt.app, tt.pod)
			require.Error(t, err)
		})
	}
}

func TestEffectiveRunIdentityUsesContainerPrecedenceAndPodFallback(t *testing.T) {
	for _, tt := range []struct {
		name      string
		app       *corev1.SecurityContext
		pod       *corev1.PodSecurityContext
		wantUser  int64
		wantGroup int64
		wantError bool
	}{
		{
			name: "pod identity fills omitted container fields",
			app:  &corev1.SecurityContext{},
			pod: &corev1.PodSecurityContext{
				RunAsUser:  ptr.To[int64](2000),
				RunAsGroup: ptr.To[int64](2001),
			},
			wantUser:  2000,
			wantGroup: 2001,
		},
		{
			name: "explicit root identity is preserved",
			app: &corev1.SecurityContext{
				RunAsUser:  ptr.To[int64](0),
				RunAsGroup: ptr.To[int64](0),
			},
			wantUser: 0, wantGroup: 0,
		},
		{name: "unresolved identity is rejected", app: &corev1.SecurityContext{}, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runAsUser, runAsGroup, err := effectiveRunIdentity(tt.app, tt.pod)
			if tt.wantError {
				if err == nil {
					t.Fatal("effectiveRunIdentity() succeeded for an unresolved identity")
				}
				return
			}
			if err != nil {
				t.Fatalf("effectiveRunIdentity() error = %v", err)
			}
			if runAsUser != tt.wantUser {
				t.Fatalf("runAsUser = %d, want %d", runAsUser, tt.wantUser)
			}
			require.NotNil(t, runAsGroup)
			if *runAsGroup != tt.wantGroup {
				t.Fatalf("runAsGroup = %d, want %d", *runAsGroup, tt.wantGroup)
			}
		})
	}
}

func TestPodSpecHash(t *testing.T) {
	tests := []struct {
		name      string
		pod       *corev1.Pod
		wantError bool
		sameHash  *corev1.Pod // if set, hash should match this pod
	}{
		{
			name: "basic pod",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx:latest"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "identical pods produce same hash",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "myapp:v1"},
					},
				},
			},
			sameHash: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "myapp:v1"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "pod with volumes",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "myapp:v1"},
					},
					Volumes: []corev1.Volume{
						{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := podSpecHash(tt.pod)
			if (err != nil) != tt.wantError {
				t.Errorf("podSpecHash() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && hash == "" {
				t.Error("podSpecHash() returned empty hash")
			}

			if tt.sameHash != nil {
				sameHash, err := podSpecHash(tt.sameHash)
				if err != nil {
					t.Fatalf("podSpecHash() failed for comparison pod: %v", err)
				}
				if hash != sameHash {
					t.Errorf("podSpecHash() expected same hash for identical pods, got %s vs %s", hash, sameHash)
				}
			}
		})
	}
}

func TestPodSpecHash_DifferentSpecsProduceDifferentHashes(t *testing.T) {
	pod1 := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
			},
		},
	}

	pod2 := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v2"}, // Different image
			},
		},
	}

	hash1, err1 := podSpecHash(pod1)
	hash2, err2 := podSpecHash(pod2)

	if err1 != nil || err2 != nil {
		t.Fatalf("podSpecHash() failed: %v, %v", err1, err2)
	}

	if hash1 == hash2 {
		t.Error("podSpecHash() expected different hashes for different pod specs")
	}
}

func TestCommittedUpgradeImageSurvivesHeightMinusOneOnNextReconcile(t *testing.T) {
	node := &appsv1.ChainNode{
		Spec: appsv1.ChainNodeSpec{App: appsv1.AppSpec{Image: "app", Version: ptr.To("v1")}},
		Status: appsv1.ChainNodeStatus{
			LatestHeight: 100,
			AppVersion:   "v2",
			Upgrades: []appsv1.Upgrade{{
				Height: 100, Image: "app:v2", Status: appsv1.UpgradeCompleted,
			}},
		},
	}
	currentPod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name: "app", Image: node.GetAppImage(),
	}}}}
	currentHash, err := podSpecHash(currentPod)
	if err != nil {
		t.Fatalf("hash current pod: %v", err)
	}
	currentPod.Annotations = map[string]string{controllers.AnnotationPodSpecHash: currentHash}

	// Model the next reconciliation after node-utils reports the raw committed
	// ABCI height one below the upgrade target.
	node.Status.LatestHeight = 99
	desiredPod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name: "app", Image: node.GetAppImage(),
	}}}}
	desiredHash, err := podSpecHash(desiredPod)
	if err != nil {
		t.Fatalf("hash desired pod: %v", err)
	}
	desiredPod.Annotations = map[string]string{controllers.AnnotationPodSpecHash: desiredHash}

	if got := desiredPod.Spec.Containers[0].Image; got != "app:v2" {
		t.Fatalf("desired image = %q, want committed target app:v2", got)
	}
	if podSpecChanged(t.Context(), currentPod, desiredPod) {
		t.Fatal("height H-1 selected a downgrade recreation")
	}
}

func TestPersistDataHeightResetClearsHaltHoldBeforeFreshOrSnapshotData(t *testing.T) {
	for _, tt := range []struct {
		name        string
		height      int64
		wantVersion string
	}{
		{name: "fresh PVC", height: 0, wantVersion: "v1"},
		{name: "snapshot below upgrade", height: 50, wantVersion: "v1"},
		{name: "snapshot above upgrade", height: 150, wantVersion: "v2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, appsv1.AddToScheme(scheme))
			node := dataHeightResetTestNode()
			base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node).Build()
			tracking := &dataHeightResetPersistenceClient{Client: base}
			r := &Reconciler{Client: tracking}
			current := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), current))

			require.NoError(t, r.persistDataHeightReset(t.Context(), current, tt.height))
			assert.Equal(t, []string{"status", "metadata"}, tracking.writes)
			stored := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.NotContains(t, stored.Annotations, appsv1.AnnotationHaltHeightHold)
			assert.Equal(t, tt.height, stored.Status.LatestHeight)
			assert.Empty(t, stored.Status.AppVersion)
			assert.Equal(t, tt.wantVersion, stored.GetAppVersion())
		})
	}
}

func TestPersistDataHeightResetKeepsNodeStoppedOnPersistenceFailure(t *testing.T) {
	for _, tt := range []struct {
		name           string
		failStatus     bool
		failMetadata   bool
		wantErr        string
		wantWrites     []string
		wantHeight     int64
		wantAppVersion string
	}{
		{
			name: "status failure", failStatus: true, wantErr: "status persistence failed",
			wantWrites: []string{"status"}, wantHeight: 100, wantAppVersion: "v2",
		},
		{
			name: "metadata failure", failMetadata: true, wantErr: "metadata persistence failed",
			wantWrites: []string{"status", "metadata"}, wantHeight: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, appsv1.AddToScheme(scheme))
			node := dataHeightResetTestNode()
			base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(node).WithObjects(node).Build()
			tracking := &dataHeightResetPersistenceClient{
				Client: base, failMetadata: tt.failMetadata, failStatus: tt.failStatus,
			}
			r := &Reconciler{Client: tracking}
			current := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), current))

			err := r.persistDataHeightReset(t.Context(), current, 0)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Equal(t, tt.wantWrites, tracking.writes)
			assert.Equal(t, "100", current.Annotations[appsv1.AnnotationHaltHeightHold])
			stored := &appsv1.ChainNode{}
			require.NoError(t, tracking.Get(t.Context(), client.ObjectKeyFromObject(node), stored))
			assert.Equal(t, "100", stored.Annotations[appsv1.AnnotationHaltHeightHold])
			assert.Equal(t, tt.wantHeight, stored.Status.LatestHeight)
			assert.Equal(t, tt.wantAppVersion, stored.Status.AppVersion)
		})
	}
}

func dataHeightResetTestNode() *appsv1.ChainNode {
	return &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node", Namespace: "default",
			Annotations: map[string]string{appsv1.AnnotationHaltHeightHold: "100"},
		},
		Spec: appsv1.ChainNodeSpec{App: appsv1.AppSpec{Image: "app", Version: ptr.To("v1")}},
		Status: appsv1.ChainNodeStatus{
			LatestHeight: 100,
			AppVersion:   "v2",
			Upgrades: []appsv1.Upgrade{{
				Height: 100, Image: "app:v2", Status: appsv1.UpgradeCompleted,
			}},
		},
	}
}

type dataHeightResetPersistenceClient struct {
	client.Client
	writes           []string
	failMetadata     bool
	failStatus       bool
	failStatusHeight *int64
}

func (c *dataHeightResetPersistenceClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*appsv1.ChainNode); ok {
		c.writes = append(c.writes, "metadata")
		if c.failMetadata {
			return errors.New("metadata persistence failed")
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *dataHeightResetPersistenceClient) Status() client.StatusWriter {
	return &dataHeightResetStatusWriter{StatusWriter: c.Client.Status(), client: c}
}

type dataHeightResetStatusWriter struct {
	client.StatusWriter
	client *dataHeightResetPersistenceClient
}

func (w *dataHeightResetStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if node, ok := obj.(*appsv1.ChainNode); ok {
		w.client.writes = append(w.client.writes, "status")
		if w.client.failStatus ||
			(w.client.failStatusHeight != nil && node.Status.LatestHeight == *w.client.failStatusHeight) {
			return errors.New("status persistence failed")
		}
	}
	return w.StatusWriter.Update(ctx, obj, opts...)
}

func TestIsPodTerminating(t *testing.T) {
	now := metav1.Now()

	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "pod is terminating",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			},
			want: true,
		},
		{
			name: "pod is not terminating",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: nil,
				},
			},
			want: false,
		},
		{
			name: "new pod",
			pod:  &corev1.Pod{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodTerminating(tt.pod); got != tt.want {
				t.Errorf("isPodTerminating() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodSpecChanged(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		existing *corev1.Pod
		new      *corev1.Pod
		want     bool
	}{
		{
			name: "no hash annotation - considers changed",
			existing: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			new: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "myapp:v1"}},
				},
			},
			want: true,
		},
		{
			name: "invalid hash annotation - considers changed",
			existing: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"cosmopilot.voluzi.com/pod-spec-hash": "invalid",
					},
				},
			},
			new: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "myapp:v1"}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := podSpecChanged(ctx, tt.existing, tt.new); got != tt.want {
				t.Errorf("podSpecChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrderVolumes(t *testing.T) {
	tests := []struct {
		name     string
		podSpec  *corev1.PodSpec
		expected []string // Expected order of volume names
	}{
		{
			name: "volumes get sorted",
			podSpec: &corev1.PodSpec{
				Volumes: []corev1.Volume{
					{Name: "zebra"},
					{Name: "apple"},
					{Name: "banana"},
				},
			},
			expected: []string{"apple", "banana", "zebra"},
		},
		{
			name: "already sorted volumes stay sorted",
			podSpec: &corev1.PodSpec{
				Volumes: []corev1.Volume{
					{Name: "a"},
					{Name: "b"},
					{Name: "c"},
				},
			},
			expected: []string{"a", "b", "c"},
		},
		{
			name: "empty volumes",
			podSpec: &corev1.PodSpec{
				Volumes: []corev1.Volume{},
			},
			expected: []string{},
		},
		{
			name: "single volume",
			podSpec: &corev1.PodSpec{
				Volumes: []corev1.Volume{
					{Name: "only"},
				},
			},
			expected: []string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderVolumes(tt.podSpec)
			if len(tt.podSpec.Volumes) != len(tt.expected) {
				t.Errorf("orderVolumes() resulted in %d volumes, want %d", len(tt.podSpec.Volumes), len(tt.expected))
				return
			}
			for i, vol := range tt.podSpec.Volumes {
				if vol.Name != tt.expected[i] {
					t.Errorf("orderVolumes() volume[%d] = %s, want %s", i, vol.Name, tt.expected[i])
				}
			}
		})
	}
}

func TestIsImagePullFailure(t *testing.T) {
	tests := []struct {
		name  string
		state *corev1.ContainerStateWaiting
		want  bool
	}{
		{
			name: "ImagePullBackOff",
			state: &corev1.ContainerStateWaiting{
				Reason: "ImagePullBackOff",
			},
			want: true,
		},
		{
			name: "ErrImagePull",
			state: &corev1.ContainerStateWaiting{
				Reason: "ErrImagePull",
			},
			want: true,
		},
		{
			name: "Other reason",
			state: &corev1.ContainerStateWaiting{
				Reason: "CrashLoopBackOff",
			},
			want: false,
		},
		{
			name:  "nil state",
			state: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isImagePullFailure(tt.state); got != tt.want {
				t.Errorf("isImagePullFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeUtilsIsInFailedState(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "node-utils running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "node-utils",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.Time{Time: time.Now()},
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "no node-utils container",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "app",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "restartable node-utils sidecar terminated",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					InitContainerStatuses: []corev1.ContainerStatus{{
						Name:  nodeUtilsContainerName,
						Ready: false,
						State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
						}},
					}},
				},
			},
			want: false,
		},
		{
			name: "failed pod",
			pod:  &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeUtilsIsInFailedState(tt.pod); got != tt.want {
				t.Errorf("nodeUtilsIsInFailedState() = %v, want %v", got, tt.want)
			}
		})
	}
}
