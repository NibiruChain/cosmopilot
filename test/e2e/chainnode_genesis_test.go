package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/voluzi/cosmopilot/v2/internal/chainutils"
	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
)

var _ = Describe("ChainNode Genesis", func() {
	Context("Genesis Creation", func() {
		apps.ForEachApp("should create a working genesis and start generating blocks",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNode := app.BuildChainNode(ns.Name)
				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for node to start generating blocks
				WaitForChainNodeHeight(chainNode, 1)

				// Verify genesis ConfigMap was created
				genesisCmName := fmt.Sprintf("%s-genesis", chainNode.Spec.Validator.Init.ChainID)
				genesis, err := Framework().KubeClient().CoreV1().ConfigMaps(ns.GetName()).Get(Framework().Context(), genesisCmName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(genesis.Data[chainutils.GenesisFilename]).NotTo(BeEmpty())
			}),
		)
	})
})
