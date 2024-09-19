package chainnode

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/NibiruChain/cosmopilot/test/framework"
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

		It("cannot be created with both .spec.genesis and .spec.validator.init specified", func() {
			testCreateWithBothGenesisAndInit(tf, ns)
		})
	})

	Context("on nibiru v1.0.0", func() {
		BeforeEach(func() {
			ns, err = tf.CreateRandomNamespace()
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func() {
			err = tf.DeleteNamespace(ns)
			Expect(err).NotTo(HaveOccurred())
		})

		appConfig := Nibiru_v1_0_0

		It("creates private key", func() {
			testCreatePrivateKey(tf, ns, appConfig)
		})

		It("imports private key", func() {
			testImportPrivateKey(tf, ns, appConfig)
		})

		It("creates account", func() {
			testCreateAccount(tf, ns, appConfig)
		})

		It("imports account", func() {
			testImportAccount(tf, ns, appConfig)
		})

		It("creates a working genesis", func() {
			testCreateGenesis(tf, ns, appConfig)
		})
	})
})
