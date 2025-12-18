package framework

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Framework defines the interface for test frameworks.
// Both integration (envtest) and e2e (Kind) frameworks implement this interface.
type Framework interface {
	// Setup initializes the test framework
	Setup(ctx context.Context) error

	// TearDown cleans up the test framework
	TearDown() error

	// Context returns the context used by the framework
	Context() context.Context

	// Client returns the controller-runtime client
	Client() client.Client

	// KubeClient returns the kubernetes clientset
	KubeClient() *kubernetes.Clientset

	// RestConfig returns the REST config for the cluster
	RestConfig() *rest.Config

	// Config returns the framework configuration
	Config() *Config

	// CreateRandomNamespace creates a namespace with a random name
	CreateRandomNamespace() (*corev1.Namespace, error)

	// DeleteNamespace deletes the given namespace
	DeleteNamespace(ns *corev1.Namespace) error

	// PodExec executes a command in a pod container and returns stdout
	PodExec(namespace, podName, container string, command ...string) (string, error)

	// RunAppCommand creates a temporary pod with the account secret mounted and runs a command
	RunAppCommand(namespace, image, appBinary, accountSecretName, command string) (string, error)

	// Type returns the type of framework (integration or e2e)
	Type() FrameworkType
}

// FrameworkType identifies the type of test framework
type FrameworkType string

const (
	// FrameworkTypeIntegration is for envtest-based integration tests
	FrameworkTypeIntegration FrameworkType = "integration"

	// FrameworkTypeE2E is for Kind-based e2e tests
	FrameworkTypeE2E FrameworkType = "e2e"
)

// Config holds configuration for test frameworks
type Config struct {
	// CertsDir is the directory containing webhook certificates
	CertsDir string

	// IssuerName is the cert-manager ClusterIssuer name
	IssuerName string

	// WorkerCount is the number of controller workers
	WorkerCount int

	// NodeUtilsImage is the node-utils image to use
	NodeUtilsImage string

	// ControllerImage is the controller image (for e2e only)
	ControllerImage string

	// ChartVersion is the helm chart version to deploy (for e2e release mode)
	// If set, deploys from OCI registry instead of local chart
	ChartVersion string

	// ClusterName is the Kind cluster name (for e2e only)
	ClusterName string

	// ReuseCluster indicates whether to reuse an existing cluster (for e2e only)
	ReuseCluster bool

	// InstallCertManager indicates whether to install cert-manager (for e2e only)
	InstallCertManager bool

	// InstallCSIDriver indicates whether to install CSI hostpath driver (for e2e only)
	InstallCSIDriver bool

	// InstallIngressNginx indicates whether to install ingress-nginx (for e2e only)
	InstallIngressNginx bool
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		CertsDir:            "/tmp/k8s-webhook-server/serving-certs",
		IssuerName:          "cosmopilot-test",
		WorkerCount:         10,
		NodeUtilsImage:      "ghcr.io/voluzi/node-utils",
		ClusterName:         "cosmopilot-test",
		ReuseCluster:        true,
		InstallCertManager:  true,
		InstallCSIDriver:    true,
		InstallIngressNginx: true,
	}
}

// Option is a function that modifies the Config
type Option func(*Config)

// WithCertsDir sets the certificates directory
func WithCertsDir(dir string) Option {
	return func(c *Config) {
		c.CertsDir = dir
	}
}

// WithIssuerName sets the cert-manager issuer name
func WithIssuerName(name string) Option {
	return func(c *Config) {
		c.IssuerName = name
	}
}

// WithWorkerCount sets the number of controller workers
func WithWorkerCount(count int) Option {
	return func(c *Config) {
		c.WorkerCount = count
	}
}

// WithNodeUtilsImage sets the node-utils image
func WithNodeUtilsImage(image string) Option {
	return func(c *Config) {
		c.NodeUtilsImage = image
	}
}

// WithControllerImage sets the controller image (for e2e)
func WithControllerImage(image string) Option {
	return func(c *Config) {
		c.ControllerImage = image
	}
}

// WithChartVersion sets the helm chart version (for e2e release mode)
func WithChartVersion(version string) Option {
	return func(c *Config) {
		c.ChartVersion = version
	}
}

// WithClusterName sets the Kind cluster name (for e2e)
func WithClusterName(name string) Option {
	return func(c *Config) {
		c.ClusterName = name
	}
}

// WithReuseCluster sets whether to reuse an existing cluster (for e2e)
func WithReuseCluster(reuse bool) Option {
	return func(c *Config) {
		c.ReuseCluster = reuse
	}
}

// WithCertManager sets whether to install cert-manager (for e2e)
func WithCertManager(install bool) Option {
	return func(c *Config) {
		c.InstallCertManager = install
	}
}

// WithCSIDriver sets whether to install CSI hostpath driver (for e2e)
func WithCSIDriver(install bool) Option {
	return func(c *Config) {
		c.InstallCSIDriver = install
	}
}

// WithIngressNginx sets whether to install ingress-nginx (for e2e)
func WithIngressNginx(install bool) Option {
	return func(c *Config) {
		c.InstallIngressNginx = install
	}
}
