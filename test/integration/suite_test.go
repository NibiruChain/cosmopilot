package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/voluzi/cosmopilot/test/framework"
)

var (
	tf *framework.EnvTestFramework
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(30 * time.Second)
	RunSpecs(t, "Integration Test Suite")
}

var _ = BeforeSuite(func() {
	By("Setting up integration test framework")

	ctx := context.Background()

	tf = framework.NewEnvTestFramework(
		framework.WithCertsDir("/tmp/cosmopilot-integration"),
		framework.WithWorkerCount(1),
		framework.WithNodeUtilsImage("ghcr.io/voluzi/node-utils"),
	)

	err := tf.Setup(ctx)
	Expect(err).NotTo(HaveOccurred())

	By("Starting controller manager")
	err = tf.StartManager()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("Tearing down integration test framework")
	if tf != nil {
		err := tf.TearDown()
		Expect(err).NotTo(HaveOccurred())
	}
})

// Framework returns the test framework for use in tests
func Framework() *framework.EnvTestFramework {
	return tf
}
