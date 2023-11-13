package chainnode

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/NibiruChain/nibiru-operator/test/framework"
)

var tf *framework.TestFramework

func RegisterTestFramework(testFramework *framework.TestFramework) {
	tf = testFramework
}

var _ = Describe("ChainNode", func() {
	var err error
	ns := &corev1.Namespace{}

	Context("with webhooks enabled", func() {
		BeforeEach(func() {
			ns, err = tf.CreateRandomNamespace()
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			err = tf.DeleteNamespace(ns)
			Expect(err).NotTo(HaveOccurred())
		})

		It("cannot be created without .spec.genesis when .spec.validator.init is not specified", func() {
			testCreateWithoutGenesisOrValidatorInit(tf, ns)
		})
	})
})
