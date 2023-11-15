package chainnode

import (
	"fmt"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/controllers/chainnode"
	"github.com/NibiruChain/nibiru-operator/test/framework"
)

func testCreateGenesis(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
	chainNode := NewChainNodeBasic(ns, app)
	chainNode.Spec.Validator = &appsv1.ValidatorConfig{Init: &appsv1.GenesisInitConfig{
		ChainID:     "nibiru-localnet",
		Assets:      []string{"10000000unibi"},
		StakeAmount: "10000000unibi",
	}}
	err := tf.Client.Create(tf.Context(), chainNode)
	Expect(err).NotTo(HaveOccurred())

	// Ensure node is running and generating blocks
	Eventually(func() int64 {
		currentChainNode := appsv1.ChainNode{}
		err := tf.Client.Get(tf.Context(), client.ObjectKeyFromObject(chainNode), &currentChainNode)
		Expect(err).NotTo(HaveOccurred())
		return currentChainNode.Status.LatestHeight
	}).Should(BeNumerically(">", 1))

	genesisCmName := fmt.Sprintf("%s-genesis", chainNode.Spec.Validator.Init.ChainID)
	genesis, err := tf.KubeClient.CoreV1().ConfigMaps(ns.GetName()).Get(tf.Context(), genesisCmName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(genesis.Data[chainnode.GenesisFilename]).NotTo(BeEmpty())
}
