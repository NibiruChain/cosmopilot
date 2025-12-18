package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
	"github.com/voluzi/cosmopilot/internal/controllers/chainnode"
	"github.com/voluzi/cosmopilot/test/e2e/apps"
)

var _ = Describe("ChainNode Keys", func() {
	Context("Private Key Creation", func() {
		apps.ForEachApp("should create private key secret",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNode := app.BuildChainNode(ns.Name)
				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for node to start
				WaitForChainNodeHeight(chainNode, 1)

				// Refresh the chainnode to get updated status
				RefreshChainNode(chainNode)

				// Verify private key secret was created
				secretName := fmt.Sprintf("%s-priv-key", chainNode.GetName())
				secret, err := Framework().KubeClient().CoreV1().Secrets(ns.GetName()).Get(Framework().Context(), secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Data[chainnode.PrivKeyFilename]).NotTo(BeEmpty())
			}),
		)
	})

	Context("Private Key Import", func() {
		apps.ForEachApp("should import existing private key",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// Skip apps that don't have private key test data configured
				if app.ValidatorConfig.PrivKey == "" {
					Skip("No private key test data configured for this app")
				}

				chainNodeName := fmt.Sprintf("e2e-chainnode-%s", RandString(6))

				// Create the private key secret first
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-priv-key", chainNodeName),
						Namespace: ns.GetName(),
					},
					Data: map[string][]byte{
						chainnode.PrivKeyFilename: []byte(app.ValidatorConfig.PrivKey),
					},
				}
				_, err := Framework().KubeClient().CoreV1().Secrets(ns.GetName()).Create(Framework().Context(), secret, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Create the ChainNode
				chainNode := app.BuildChainNode(ns.Name)
				chainNode.Name = chainNodeName
				chainNode.GenerateName = ""

				err = Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for node to start and verify the pub key matches
				Eventually(func() string {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return ""
					}
					return current.Status.PubKey
				}).Should(Equal(app.ValidatorConfig.ExpectedPubKey))
			}),
		)
	})
})
