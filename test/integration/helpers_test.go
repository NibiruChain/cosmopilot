package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

const (
	ChainNodePrefix    = "chainnode-test-"
	ChainNodeSetPrefix = "chainnodeset-test-"
)

var (
	// DefaultChainNodeTestApp is the default application used for ChainNode testing
	DefaultChainNodeTestApp = appsv1.AppSpec{
		Image:   "ghcr.io/nibiruchain/nibiru",
		Version: ptr.To("1.0.0"),
		App:     "nibid",
	}

	// DefaultChainNodeSetTestApp is the default application used for ChainNodeSet testing
	DefaultChainNodeSetTestApp = appsv1.AppSpec{
		Image:   "ghcr.io/nibiruchain/nibiru",
		Version: ptr.To("1.0.0"),
		App:     "nibid",
	}
)

// CreateTestNamespace creates a random namespace and registers cleanup via DeferCleanup.
func CreateTestNamespace() *corev1.Namespace {
	ns, err := Framework().CreateRandomNamespace()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		err := Framework().DeleteNamespace(ns)
		Expect(err).NotTo(HaveOccurred())
	})
	return ns
}

// WithNamespace wraps a test function with automatic namespace creation and cleanup.
// Usage:
//
//	It("should do something", WithNamespace(func(ns *corev1.Namespace) {
//	    // test code using ns
//	}))
func WithNamespace(fn func(ns *corev1.Namespace)) func() {
	return func() {
		ns := CreateTestNamespace()
		fn(ns)
	}
}

// WaitForChainNodeCount waits for the number of ChainNodes in the namespace to equal the expected count
func WaitForChainNodeCount(namespace string, expectedCount int) {
	Eventually(func() int {
		chainNodeList := &appsv1.ChainNodeList{}
		if err := Framework().Client().List(Framework().Context(), chainNodeList, client.InNamespace(namespace)); err != nil {
			return 0
		}
		return len(chainNodeList.Items)
	}).Should(Equal(expectedCount))
}

// GetChainNodes returns all ChainNodes in the namespace
func GetChainNodes(namespace string) []appsv1.ChainNode {
	chainNodeList := &appsv1.ChainNodeList{}
	err := Framework().Client().List(Framework().Context(), chainNodeList, client.InNamespace(namespace))
	Expect(err).NotTo(HaveOccurred())
	return chainNodeList.Items
}

// WaitForPVC waits for a PVC to be created
func WaitForPVC(namespace, name string) {
	Eventually(func() error {
		pvc := &corev1.PersistentVolumeClaim{}
		return Framework().Client().Get(Framework().Context(), client.ObjectKey{Namespace: namespace, Name: name}, pvc)
	}).Should(Succeed())
}

// GetPVC returns a PVC by name
func GetPVC(namespace, name string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{}
	err := Framework().Client().Get(Framework().Context(), client.ObjectKey{Namespace: namespace, Name: name}, pvc)
	Expect(err).NotTo(HaveOccurred())
	return pvc
}

// NewGenesisConfigWithChainID creates a genesis config that skips download
// By setting ChainID and UseDataVolume, the controller will skip genesis download
// and proceed directly to creating ChainNodes
func NewGenesisConfigWithChainID(chainID string) *appsv1.GenesisConfig {
	return &appsv1.GenesisConfig{
		Url:           ptr.To("https://example.com/genesis.json"),
		ChainID:       ptr.To(chainID),
		UseDataVolume: ptr.To(true),
	}
}
