package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
)

var _ = Describe("ChainNodeSet Basic", func() {
	Context("ChainNodeSet Creation", func() {
		apps.ForEachApp("should create ChainNodes and reach running phase",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNodeSet := app.BuildChainNodeSet(ns.Name, 1)
				err := Framework().Client().Create(Framework().Context(), chainNodeSet)
				Expect(err).NotTo(HaveOccurred())

				// Wait for ChainNodeSet to reach running phase
				WaitForChainNodeSetRunning(chainNodeSet)

				// Verify ChainNodes were created (validator + 1 fullnode = 2)
				Expect(CountChainNodes(ns.Name)).To(Equal(2))
			}),
		)
	})
})
