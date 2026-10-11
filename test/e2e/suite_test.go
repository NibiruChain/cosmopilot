package e2e

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/voluzi/cosmopilot/v2/pkg/environ"
	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
	"github.com/voluzi/cosmopilot/v2/test/framework"
)

var (
	tf *framework.KindFramework
)

func TestE2E(t *testing.T) {
	if !environ.GetBool("E2E_TEST", false) {
		t.Skip("Skipping e2e tests. Set E2E_TEST=true to run.")
	}

	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Minute)
	RunSpecs(t, "E2E Test Suite")
}

// SynchronizedBeforeSuite runs setup in two phases:
// 1. First function runs only on Process 1 (sets up cluster and deploys controller)
// 2. Second function runs on all processes (connects to the cluster)
var _ = SynchronizedBeforeSuite(func() []byte {
	// This runs only on Process 1
	By("Setting up e2e test framework with Kind cluster (Process 1)")

	ctx := context.Background()

	clusterName := environ.GetString("CLUSTER_NAME", "cosmopilot-e2e")
	controllerImage := environ.GetString("CONTROLLER_IMAGE", "")
	chartVersion := environ.GetString("CHART_VERSION", "")
	nodeUtilsImage := environ.GetString("NODE_UTILS_IMAGE", "ghcr.io/voluzi/node-utils")
	reuseCluster := environ.GetBool("REUSE_CLUSTER", true)
	installVault := environ.GetBool("INSTALL_VAULT", true)

	setupFramework := framework.NewKindFramework(
		framework.WithClusterName(clusterName),
		framework.WithControllerImage(controllerImage),
		framework.WithChartVersion(chartVersion),
		framework.WithNodeUtilsImage(nodeUtilsImage),
		framework.WithReuseCluster(reuseCluster),
		framework.WithCertManager(true),
		framework.WithCSIDriver(true),
		framework.WithIngressNginx(true),
		framework.WithVault(installVault),
	)

	err := setupFramework.Setup(ctx)
	Expect(err).NotTo(HaveOccurred())

	// Load and deploy controller
	// Dev mode: controllerImage is set, load it into Kind and deploy local chart
	// Release mode: chartVersion is set, deploy from OCI registry
	if controllerImage != "" {
		By("Loading controller image into Kind cluster")
		err = setupFramework.LoadImage(controllerImage)
		Expect(err).NotTo(HaveOccurred())
	}

	// Load locally built node-utils image if requested
	if environ.GetBool("BUILD_NODE_UTILS", false) && nodeUtilsImage != "" {
		By("Loading node-utils image into Kind cluster")
		err = setupFramework.LoadImage(nodeUtilsImage)
		Expect(err).NotTo(HaveOccurred())
	}

	By("Deploying controller to cluster")
	err = setupFramework.DeployController()
	Expect(err).NotTo(HaveOccurred())

	By("Creating ClusterIssuer for cert-manager")
	err = setupFramework.CreateClusterIssuer("cosmopilot-e2e")
	// Ignore error if already exists
	_ = err

	// Return empty data - other processes don't need any data from Process 1
	return nil
}, func(data []byte) {
	// This runs on all processes (including Process 1)
	By("Connecting to Kind cluster")

	ctx := context.Background()

	clusterName := environ.GetString("CLUSTER_NAME", "cosmopilot-e2e")
	controllerImage := environ.GetString("CONTROLLER_IMAGE", "")
	chartVersion := environ.GetString("CHART_VERSION", "")
	nodeUtilsImage := environ.GetString("NODE_UTILS_IMAGE", "ghcr.io/voluzi/node-utils")
	installVault := environ.GetBool("INSTALL_VAULT", true)

	// Always reuse the cluster that Process 1 set up
	tf = framework.NewKindFramework(
		framework.WithClusterName(clusterName),
		framework.WithControllerImage(controllerImage),
		framework.WithChartVersion(chartVersion),
		framework.WithNodeUtilsImage(nodeUtilsImage),
		framework.WithReuseCluster(true), // Always reuse - cluster is already set up
		framework.WithCertManager(true),
		framework.WithCSIDriver(true),
		framework.WithIngressNginx(true),
		framework.WithVault(installVault),
	)

	err := tf.Setup(ctx)
	Expect(err).NotTo(HaveOccurred())
})

// SynchronizedAfterSuite runs teardown in two phases:
// 1. First function runs on all processes (cleanup per-process resources)
// 2. Second function runs only on Process 1 (final cluster teardown)
var _ = SynchronizedAfterSuite(func() {
	// This runs on all processes - cancel context
	if tf != nil {
		tf.Cancel()
	}
}, func() {
	// This runs only on Process 1
	By("Tearing down e2e test framework (Process 1)")

	reuseCluster := environ.GetBool("REUSE_CLUSTER", true)

	// Don't undeploy if we want to keep the cluster for debugging
	if !reuseCluster && tf != nil {
		_ = tf.UndeployController()
		err := tf.TearDown()
		Expect(err).NotTo(HaveOccurred())
	}
})

// Framework returns the test framework for use in tests
func Framework() *framework.KindFramework {
	return tf
}

// WithNamespace wraps a test function with automatic namespace creation and cleanup.
// Use with ForEachApp to avoid boilerplate:
//
//	apps.ForEachApp("should do something",
//	    WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
//	        // test code using ns
//	    }),
//	)
func WithNamespace(fn func(app apps.TestApp, ns *corev1.Namespace)) func(apps.TestApp) {
	return func(app apps.TestApp) {
		ns := CreateTestNamespace()
		fn(app, ns)
	}
}

// CreateTestNamespace creates a random namespace and registers cleanup via DeferCleanup.
// Use this in tests that need direct namespace control.
func CreateTestNamespace() *corev1.Namespace {
	ns, err := Framework().CreateRandomNamespace()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		err := Framework().DeleteNamespace(ns)
		Expect(err).NotTo(HaveOccurred())
	})
	return ns
}

// WithNs wraps a test function with automatic namespace creation and cleanup.
// Use this directly with It() when not using ForEachApp:
//
//	It("should do something", WithNs(func(ns *corev1.Namespace) {
//	    // test code using ns
//	}))
func WithNs(fn func(ns *corev1.Namespace)) func() {
	return func() {
		ns := CreateTestNamespace()
		fn(ns)
	}
}
