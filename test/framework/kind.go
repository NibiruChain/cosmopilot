package framework

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

const (
	// DefaultKindNodeImage is the default Kind node image
	DefaultKindNodeImage = "kindest/node:v1.32.0"

	// kindConfigTemplate is the Kind cluster configuration
	kindConfigTemplate = `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
`
)

// projectRoot returns the absolute path to the project root directory.
// It uses runtime.Caller to find the source file location and navigates up.
func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	// file is .../test/framework/kind.go, so go up 3 levels
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// KindFramework implements Framework using Kind clusters
type KindFramework struct {
	BaseFramework
	provider       *cluster.Provider
	clusterCreated bool
	helmPath       string
	kubectlPath    string
	kindPath       string
}

// NewKindFramework creates a new Kind-based framework
func NewKindFramework(opts ...Option) *KindFramework {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &KindFramework{
		BaseFramework: BaseFramework{
			cfg: cfg,
		},
	}
}

// Setup initializes the Kind cluster and installs dependencies
func (f *KindFramework) Setup(ctx context.Context) error {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
	log := logf.Log.WithName("kind-framework")

	// Create a cancelable context
	testCtx, cancel := context.WithCancel(ctx)
	f.SetContext(testCtx, cancel)

	// Find helm, kubectl and kind binaries
	f.helmPath = f.findBinary("helm")
	f.kubectlPath = f.findBinary("kubectl")
	f.kindPath = f.findBinary("kind")

	// Create Kind provider
	f.provider = cluster.NewProvider(
		cluster.ProviderWithLogger(cmd.NewLogger()),
	)

	// Check if cluster already exists
	clusters, err := f.provider.List()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	clusterExists := false
	for _, c := range clusters {
		if c == f.cfg.ClusterName {
			clusterExists = true
			break
		}
	}

	if clusterExists && f.cfg.ReuseCluster {
		log.Info("Reusing existing Kind cluster", "name", f.cfg.ClusterName)
	} else {
		if clusterExists {
			log.Info("Deleting existing Kind cluster", "name", f.cfg.ClusterName)
			if err := f.provider.Delete(f.cfg.ClusterName, ""); err != nil {
				return fmt.Errorf("failed to delete existing cluster: %w", err)
			}
		}

		log.Info("Creating Kind cluster", "name", f.cfg.ClusterName)

		// Write config to temp file
		configFile, err := os.CreateTemp("", "kind-config-*.yaml")
		if err != nil {
			return fmt.Errorf("failed to create temp config file: %w", err)
		}
		defer os.Remove(configFile.Name())

		if _, err := configFile.WriteString(kindConfigTemplate); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
		configFile.Close()

		if err := f.provider.Create(
			f.cfg.ClusterName,
			cluster.CreateWithConfigFile(configFile.Name()),
			cluster.CreateWithNodeImage(DefaultKindNodeImage),
			cluster.CreateWithWaitForReady(5*time.Minute),
		); err != nil {
			return fmt.Errorf("failed to create cluster: %w", err)
		}
		f.clusterCreated = true
	}

	// Get kubeconfig
	kubeconfig, err := f.provider.KubeConfig(f.cfg.ClusterName, false)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Create rest config from kubeconfig
	restCfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return fmt.Errorf("failed to create rest config: %w", err)
	}
	f.SetRestConfig(restCfg)

	// Register schemes
	if err := appsv1.AddToScheme(scheme.Scheme); err != nil {
		return fmt.Errorf("failed to add apps/v1 to scheme: %w", err)
	}
	if err := snapshotv1.AddToScheme(scheme.Scheme); err != nil {
		return fmt.Errorf("failed to add snapshot/v1 to scheme: %w", err)
	}

	// Create clients
	k8sClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}
	f.SetClient(k8sClient)

	clientSet, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	f.SetKubeClient(clientSet)

	// Install dependencies
	if f.cfg.InstallCertManager {
		log.Info("Installing cert-manager")
		if err := f.installCertManager(); err != nil {
			return fmt.Errorf("failed to install cert-manager: %w", err)
		}
	}

	if f.cfg.InstallCSIDriver {
		log.Info("Installing CSI hostpath driver")
		if err := f.installCSIHostpath(); err != nil {
			return fmt.Errorf("failed to install CSI hostpath driver: %w", err)
		}
	}

	if f.cfg.InstallIngressNginx {
		log.Info("Installing ingress-nginx")
		if err := f.installIngressNginx(); err != nil {
			return fmt.Errorf("failed to install ingress-nginx: %w", err)
		}
	}

	if f.cfg.InstallVault {
		log.Info("Installing HashiCorp Vault")
		if err := f.installVault(); err != nil {
			return fmt.Errorf("failed to install Vault: %w", err)
		}
	}

	return nil
}

// TearDown cleans up the Kind cluster
func (f *KindFramework) TearDown() error {
	f.Cancel()

	// Only delete if we created the cluster and reuse is disabled
	if f.clusterCreated && !f.cfg.ReuseCluster {
		if err := f.provider.Delete(f.cfg.ClusterName, ""); err != nil {
			return fmt.Errorf("failed to delete cluster: %w", err)
		}
	}

	return nil
}

// Type returns the framework type
func (f *KindFramework) Type() FrameworkType {
	return FrameworkTypeE2E
}

// LoadImage loads a Docker image into the Kind cluster
func (f *KindFramework) LoadImage(image string) error {
	nodes, err := f.provider.ListNodes(f.cfg.ClusterName)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	// Use kind load docker-image command
	cmd := exec.CommandContext(f.ctx, f.kindPath, "load", "docker-image", image, "--name", f.cfg.ClusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load image: %s: %w", string(output), err)
	}

	return nil
}

// DeployController deploys the controller to the cluster using Helm.
// If ChartVersion is set, deploys from OCI registry (release mode).
// Otherwise, deploys from local chart with ControllerImage (dev mode).
func (f *KindFramework) DeployController() error {
	log := logf.Log.WithName("kind-framework")

	// Always delete existing CRDs to ensure clean state when switching between dev and release modes.
	// This is safe because Helm uninstall doesn't remove CRDs, and we want the chart's CRDs to be deployed.
	log.Info("Deleting existing cosmopilot CRDs (if any)")
	_ = f.kubectl("delete", "crd", "chainnodes.apps.voluzi.io", "--ignore-not-found")
	_ = f.kubectl("delete", "crd", "chainnodesets.apps.voluzi.io", "--ignore-not-found")

	var chartRef string
	var extraArgs []string

	if f.cfg.ChartVersion != "" {
		// Release mode: use published chart from OCI registry
		chartRef = "oci://ghcr.io/voluzi/helm/cosmopilot"
		extraArgs = append(extraArgs, "--version", f.cfg.ChartVersion)
		log.Info("Deploying controller from OCI registry", "version", f.cfg.ChartVersion)
	} else {
		// Dev mode: use local chart and install CRDs manually
		root := projectRoot()
		chartRef = filepath.Join(root, "helm", "cosmopilot")

		// Install CRDs first (using server-side apply to handle large CRDs)
		log.Info("Installing CRDs")
		crdsPath := filepath.Join(root, "helm", "cosmopilot", "crds")
		if err := f.kubectl("apply", "-f", crdsPath, "--server-side", "--force-conflicts"); err != nil {
			return fmt.Errorf("failed to install CRDs: %w", err)
		}
		log.Info("Deploying controller from local chart")
	}

	// Deploy using Helm
	args := []string{
		"upgrade", "--install", "cosmopilot",
		chartRef,
		"--namespace", "cosmopilot-system",
		"--create-namespace",
		"--set", fmt.Sprintf("nodeUtilsImage=%s", f.cfg.NodeUtilsImage),
		"--set", fmt.Sprintf("workerCount=%d", f.cfg.WorkerCount),
		"--set", "debugMode=true",
		"--wait",
		"--timeout", "5m",
	}
	args = append(args, extraArgs...)

	if f.cfg.ControllerImage != "" {
		parts := strings.SplitN(f.cfg.ControllerImage, ":", 2)
		args = append(args, "--set", fmt.Sprintf("image=%s", parts[0]))
		if len(parts) > 1 {
			args = append(args, "--set", fmt.Sprintf("imageTag=%s", parts[1]))
		}
	}

	if err := f.helm(args...); err != nil {
		return fmt.Errorf("failed to deploy controller: %w", err)
	}

	return nil
}

// UndeployController removes the controller from the cluster
func (f *KindFramework) UndeployController() error {
	return f.helm("uninstall", "cosmopilot", "--namespace", "cosmopilot-system")
}

// CreateClusterIssuer creates a self-signed ClusterIssuer
func (f *KindFramework) CreateClusterIssuer(name string) error {
	manifest := fmt.Sprintf(`apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: %s
spec:
  selfSigned: {}
`, name)

	return f.kubectlApply(manifest)
}

// installCertManager installs cert-manager using kubectl
func (f *KindFramework) installCertManager() error {
	// Apply cert-manager manifest
	if err := f.kubectl("apply", "-f",
		"https://github.com/cert-manager/cert-manager/releases/download/v1.15.3/cert-manager.yaml",
		"--server-side", "--force-conflicts"); err != nil {
		return err
	}

	// Wait for cert-manager webhook to be ready
	return f.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/component=webhook",
		"-n", "cert-manager",
		"--timeout=120s")
}

// installCSIHostpath installs snapshot support for the CSI hostpath driver.
// Kind already has a working StorageClass (standard), so we only add snapshot capabilities.
func (f *KindFramework) installCSIHostpath() error {
	log := logf.Log.WithName("kind-framework")

	// Install snapshot CRDs
	log.Info("Installing VolumeSnapshot CRDs")
	if err := f.kubectl("apply", "-f",
		"https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml"); err != nil {
		return err
	}
	if err := f.kubectl("apply", "-f",
		"https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml"); err != nil {
		return err
	}
	if err := f.kubectl("apply", "-f",
		"https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml"); err != nil {
		return err
	}

	// Install snapshot controller
	log.Info("Installing snapshot controller")
	if err := f.kubectl("apply", "-f",
		"https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml"); err != nil {
		return err
	}
	if err := f.kubectl("apply", "-f",
		"https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml"); err != nil {
		return err
	}

	// Install CSI sidecar RBAC (required by csi-hostpath-plugin.yaml)
	// The plugin references ClusterRoles that must exist before deployment
	log.Info("Installing CSI sidecar RBAC")
	sidecarRBAC := []string{
		"https://raw.githubusercontent.com/kubernetes-csi/external-provisioner/v5.2.0/deploy/kubernetes/rbac.yaml",
		"https://raw.githubusercontent.com/kubernetes-csi/external-attacher/v4.8.0/deploy/kubernetes/rbac.yaml",
		"https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/deploy/kubernetes/csi-snapshotter/rbac-csi-snapshotter.yaml",
		"https://raw.githubusercontent.com/kubernetes-csi/external-resizer/v1.13.1/deploy/kubernetes/rbac.yaml",
	}
	for _, url := range sidecarRBAC {
		if err := f.kubectl("apply", "-f", url); err != nil {
			return err
		}
	}

	// Install CSI hostpath driver (needed for snapshot support)
	// Note: Kind's default StorageClass (standard) uses rancher/local-path-provisioner
	// which doesn't support snapshots, so we need this driver for snapshot tests
	log.Info("Installing CSI hostpath driver")
	csiBaseURL := "https://raw.githubusercontent.com/kubernetes-csi/csi-driver-host-path/v1.17.0/deploy/kubernetes-1.30/hostpath"
	csiFiles := []string{
		"csi-hostpath-driverinfo.yaml",
		"csi-hostpath-plugin.yaml",
	}
	for _, file := range csiFiles {
		if err := f.kubectl("apply", "-f", fmt.Sprintf("%s/%s", csiBaseURL, file)); err != nil {
			return err
		}
	}

	// Create StorageClass for CSI hostpath (NOT default - Kind's standard remains default)
	// Use this StorageClass explicitly when you need snapshot support
	// Use server-side apply to handle existing resources when reusing clusters
	storageClass := `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-hostpath-sc
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
`
	if err := f.kubectlApplyServerSide(storageClass); err != nil {
		return err
	}

	// Create VolumeSnapshotClass
	// Use server-side apply to handle existing resources when reusing clusters
	snapshotClass := `apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-hostpath-snapclass
  annotations:
    snapshot.storage.kubernetes.io/is-default-class: "true"
driver: hostpath.csi.k8s.io
deletionPolicy: Delete
`
	if err := f.kubectlApplyServerSide(snapshotClass); err != nil {
		return err
	}

	// Wait for snapshot controller to be ready
	log.Info("Waiting for snapshot controller")
	if err := f.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/name=snapshot-controller",
		"-n", "kube-system",
		"--timeout=120s"); err != nil {
		return err
	}

	// Wait for CSI driver to be ready
	log.Info("Waiting for CSI hostpath driver")
	return f.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/name=csi-hostpathplugin",
		"-n", "default",
		"--timeout=120s")
}

// installIngressNginx installs ingress-nginx for Kind
func (f *KindFramework) installIngressNginx() error {
	// Apply ingress-nginx manifest for Kind
	if err := f.kubectl("apply", "-f",
		"https://raw.githubusercontent.com/kubernetes/ingress-nginx/helm-chart-4.11.2/deploy/static/provider/kind/deploy.yaml"); err != nil {
		return err
	}

	// Wait for ingress-nginx to be ready
	return f.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/component=controller",
		"-n", "ingress-nginx",
		"--timeout=120s")
}

// kubectl runs a kubectl command
func (f *KindFramework) kubectl(args ...string) error {
	kubeconfig, err := f.provider.KubeConfig(f.cfg.ClusterName, false)
	if err != nil {
		return err
	}

	// Write kubeconfig to temp file
	kubeconfigFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfigFile.Name())

	if _, err := kubeconfigFile.WriteString(kubeconfig); err != nil {
		return err
	}
	kubeconfigFile.Close()

	cmdArgs := append([]string{"--kubeconfig", kubeconfigFile.Name()}, args...)
	cmd := exec.CommandContext(f.ctx, f.kubectlPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// kubectlApply applies a manifest from a string
func (f *KindFramework) kubectlApply(manifest string) error {
	return f.kubectlApplyWithOptions(manifest, false)
}

// kubectlApplyServerSide applies a manifest using server-side apply with force-conflicts.
// This handles existing resources properly when reusing clusters.
func (f *KindFramework) kubectlApplyServerSide(manifest string) error {
	return f.kubectlApplyWithOptions(manifest, true)
}

// kubectlApplyWithOptions applies a manifest with optional server-side apply
func (f *KindFramework) kubectlApplyWithOptions(manifest string, serverSide bool) error {
	kubeconfig, err := f.provider.KubeConfig(f.cfg.ClusterName, false)
	if err != nil {
		return err
	}

	// Write kubeconfig to temp file
	kubeconfigFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfigFile.Name())

	if _, err := kubeconfigFile.WriteString(kubeconfig); err != nil {
		return err
	}
	kubeconfigFile.Close()

	args := []string{"--kubeconfig", kubeconfigFile.Name(), "apply", "-f", "-"}
	if serverSide {
		args = append(args, "--server-side", "--force-conflicts")
	}

	cmd := exec.CommandContext(f.ctx, f.kubectlPath, args...)
	cmd.Stdin = bytes.NewBufferString(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// helm runs a helm command
func (f *KindFramework) helm(args ...string) error {
	kubeconfig, err := f.provider.KubeConfig(f.cfg.ClusterName, false)
	if err != nil {
		return err
	}

	// Write kubeconfig to temp file
	kubeconfigFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfigFile.Name())

	if _, err := kubeconfigFile.WriteString(kubeconfig); err != nil {
		return err
	}
	kubeconfigFile.Close()

	cmdArgs := append([]string{"--kubeconfig", kubeconfigFile.Name()}, args...)
	cmd := exec.CommandContext(f.ctx, f.helmPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// findBinary looks for a binary in PATH and project bin directory
func (f *KindFramework) findBinary(name string) string {
	// First check project bin directory
	binPath := filepath.Join(projectRoot(), "bin", name)
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}

	// Fall back to PATH
	path, err := exec.LookPath(name)
	if err == nil {
		return path
	}

	return name
}

// Provider returns the Kind provider
func (f *KindFramework) Provider() *cluster.Provider {
	return f.provider
}
