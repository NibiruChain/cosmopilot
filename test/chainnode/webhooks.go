package chainnode

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/NibiruChain/nibiru-operator/test/framework"
)

func testCreateWithoutGenesisOrValidatorInit(tf *framework.TestFramework, ns *corev1.Namespace) {
	chainNode := NewChainNodeBasic(ns, NibiruApp)
	err := tf.Client.Create(tf.Context(), chainNode)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(".spec.genesis is required except when initializing new genesis with .spec.validator.init"))
}
