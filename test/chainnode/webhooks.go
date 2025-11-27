package chainnode

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/test/framework"
)

func testCreateWithoutGenesisOrValidatorInit(tf *framework.TestFramework, ns *corev1.Namespace) {
	chainNode := NewChainNodeBasic(ns, Nibiru_v1_0_0)
	err := tf.Client.Create(tf.Context(), chainNode)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(".spec.genesis is required except when initializing new genesis with .spec.validator.init"))
}

func testCreateWithBothGenesisAndInit(tf *framework.TestFramework, ns *corev1.Namespace) {
	chainNode := NewChainNodeBasic(ns, Nibiru_v1_0_0)
	chainNode.Spec.Genesis = &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis")}
	chainNode.Spec.Validator = &appsv1.ValidatorConfig{Init: &appsv1.GenesisInitConfig{
		ChainID:     "nibiru-localnet",
		Assets:      []string{"10000000unibi"},
		StakeAmount: "10000000unibi",
	}}
	err := tf.Client.Create(tf.Context(), chainNode)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(".spec.genesis and .spec.validator.init are mutually exclusive"))
}
