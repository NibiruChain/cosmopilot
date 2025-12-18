package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/voluzi/cosmopilot/test/e2e/apps"
)

var _ = Describe("ChainNodeSet Scaling", func() {
	Context("Scale Operations", func() {
		apps.ForEachApp("should scale down ChainNodes",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNodeSet := app.BuildChainNodeSet(ns.Name, 2)
				err := Framework().Client().Create(Framework().Context(), chainNodeSet)
				Expect(err).NotTo(HaveOccurred())

				// Wait for running phase
				WaitForChainNodeSetRunning(chainNodeSet)

				// Verify initial count (validator + 2 fullnodes = 3)
				Expect(CountChainNodes(ns.Name)).To(Equal(3))

				// Scale down to 1 fullnode
				Eventually(func() bool {
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), chainNodeSet); err != nil {
						return false
					}
					chainNodeSet.Spec.Nodes[0].Instances = ptr.To(1)
					return Framework().Client().Update(Framework().Context(), chainNodeSet) == nil
				}).Should(BeTrue())

				// Wait for scale down (validator + 1 fullnode = 2)
				WaitForChainNodeCount(ns.Name, 2)
			}),
		)

		apps.ForEachApp("should scale up ChainNodes",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNodeSet := app.BuildChainNodeSet(ns.Name, 1)
				err := Framework().Client().Create(Framework().Context(), chainNodeSet)
				Expect(err).NotTo(HaveOccurred())

				// Wait for running phase
				WaitForChainNodeSetRunning(chainNodeSet)

				// Verify initial count (validator + 1 fullnode = 2)
				Expect(CountChainNodes(ns.Name)).To(Equal(2))

				// Scale up to 3 fullnodes
				Eventually(func() bool {
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), chainNodeSet); err != nil {
						return false
					}
					chainNodeSet.Spec.Nodes[0].Instances = ptr.To(3)
					return Framework().Client().Update(Framework().Context(), chainNodeSet) == nil
				}).Should(BeTrue())

				// Wait for scale up (validator + 3 fullnodes = 4)
				WaitForChainNodeCount(ns.Name, 4)
			}),
		)
	})
})
