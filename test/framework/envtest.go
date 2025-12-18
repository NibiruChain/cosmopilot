package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
	"github.com/voluzi/cosmopilot/internal/controllers"
	"github.com/voluzi/cosmopilot/internal/controllers/chainnode"
	"github.com/voluzi/cosmopilot/internal/controllers/chainnodeset"
)

const (
	webhookServerMetricsBindAddress = "localhost:8080"
)

var (
	// CRDsPath is the path to the CRD files
	CRDsPath = filepath.Join("..", "..", "helm", "cosmopilot", "crds")

	// ExternalCRDsPath is the path to external CRD files (cert-manager, volumesnapshot)
	ExternalCRDsPath = filepath.Join("..", "..", "test", "testdata", "crds")

	// WebhooksPath is the path to webhook configuration files for envtest
	WebhooksPath = filepath.Join("..", "..", "test", "testdata", "webhooks")
)

// EnvTestFramework implements Framework using controller-runtime's envtest
type EnvTestFramework struct {
	BaseFramework
	env        *envtest.Environment
	mgrCancel  context.CancelFunc
	mgrStarted bool
}

// NewEnvTestFramework creates a new envtest-based framework
func NewEnvTestFramework(opts ...Option) *EnvTestFramework {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &EnvTestFramework{
		BaseFramework: BaseFramework{
			cfg: cfg,
		},
	}
}

// Setup initializes the envtest environment
func (f *EnvTestFramework) Setup(ctx context.Context) error {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create a cancelable context
	testCtx, cancel := context.WithCancel(ctx)
	f.SetContext(testCtx, cancel)

	// Initialize envtest environment
	f.env = &envtest.Environment{
		CRDDirectoryPaths: []string{
			CRDsPath,
			ExternalCRDsPath,
		},
		ErrorIfCRDPathMissing: false, // External CRDs are optional
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{WebhooksPath},
		},
	}

	// Start the environment
	restCfg, err := f.env.Start()
	if err != nil {
		return fmt.Errorf("failed to start envtest: %w", err)
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

	return nil
}

// StartManager starts the controller manager
func (f *EnvTestFramework) StartManager() error {
	if f.mgrStarted {
		return nil
	}

	// Use webhook settings from envtest
	webhookOpts := f.env.WebhookInstallOptions

	mgr, err := ctrl.NewManager(f.restCfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: webhookServerMetricsBindAddress,
		LeaderElection:     false,
		Host:               webhookOpts.LocalServingHost,
		Port:               webhookOpts.LocalServingPort,
		CertDir:            webhookOpts.LocalServingCertDir,
	})
	if err != nil {
		return fmt.Errorf("failed to create manager: %w", err)
	}

	runOpts := controllers.ControllerRunOptions{
		WorkerCount:     f.cfg.WorkerCount,
		NodeUtilsImage:  f.cfg.NodeUtilsImage,
		DisableWebhooks: false,
	}

	if _, err = chainnode.New(mgr, f.kubeClient, &runOpts); err != nil {
		return fmt.Errorf("failed to create chainnode controller: %w", err)
	}

	if _, err = chainnodeset.New(mgr, f.kubeClient, &runOpts); err != nil {
		return fmt.Errorf("failed to create chainnodeset controller: %w", err)
	}

	if err := appsv1.SetupChainNodeValidationWebhook(mgr); err != nil {
		return fmt.Errorf("failed to setup chainnode webhook: %w", err)
	}

	if err := appsv1.SetupChainNodeSetValidationWebhook(mgr); err != nil {
		return fmt.Errorf("failed to setup chainnodeset webhook: %w", err)
	}

	// Start manager in a goroutine
	mgrCtx, mgrCancel := context.WithCancel(f.ctx)
	f.mgrCancel = mgrCancel

	go func() {
		if err := mgr.Start(mgrCtx); err != nil {
			logf.Log.Error(err, "manager stopped with error")
		}
	}()

	// Wait for webhook server to be ready
	dialer := &net.Dialer{Timeout: time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", webhookOpts.LocalServingHost, webhookOpts.LocalServingPort), &tls.Config{
			InsecureSkipVerify: true,
		})
		if err == nil {
			conn.Close()
			f.mgrStarted = true
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("webhook server did not start in time")
}

// StopManager stops the controller manager
func (f *EnvTestFramework) StopManager() {
	if f.mgrCancel != nil {
		f.mgrCancel()
		f.mgrStarted = false
	}
}

// TearDown cleans up the envtest environment
func (f *EnvTestFramework) TearDown() error {
	f.StopManager()

	// Give the manager time to fully shut down before stopping envtest
	time.Sleep(500 * time.Millisecond)

	f.Cancel()

	if f.env != nil {
		if err := f.env.Stop(); err != nil {
			// Log but don't fail - envtest shutdown timeouts are common and don't affect test results
			logf.Log.Info("envtest environment stop returned error (this is usually harmless)", "error", err)
		}
	}
	return nil
}

// Type returns the framework type
func (f *EnvTestFramework) Type() FrameworkType {
	return FrameworkTypeIntegration
}

// Env returns the underlying envtest environment
func (f *EnvTestFramework) Env() *envtest.Environment {
	return f.env
}
