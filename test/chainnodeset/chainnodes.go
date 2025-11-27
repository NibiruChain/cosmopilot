package chainnodeset

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/test/framework"
)

func testCreateChainNodes(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
	chainNodeSet := NewChainNodeSetBasic(ns, app)
	chainNodeSet.Spec.Nodes = []appsv1.NodeGroupSpec{{Name: "fullnodes", Instances: ptr.To(1)}}
	chainNodeSet.Spec.Validator = &appsv1.NodeSetValidatorConfig{Init: &appsv1.GenesisInitConfig{
		ChainID:     "nibiru-localnet",
		Assets:      []string{"10000000unibi"},
		StakeAmount: "10000000unibi",
	}}
	err := tf.Client.Create(tf.Context(), chainNodeSet)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() appsv1.ChainNodeSetPhase {
		currentChainNodeSet := appsv1.ChainNodeSet{}
		err := tf.Client.Get(tf.Context(), client.ObjectKeyFromObject(chainNodeSet), &currentChainNodeSet)
		Expect(err).NotTo(HaveOccurred())
		return currentChainNodeSet.Status.Phase
	}).Should(Equal(appsv1.PhaseChainNodeSetRunning))

	chainNodeList := &appsv1.ChainNodeList{}
	err = tf.Client.List(tf.Context(), chainNodeList, &client.ListOptions{Namespace: ns.GetName()})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(chainNodeList.Items)).To(Equal(2))
}

func testScaleDownChainNodes(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
	chainNodeSet := NewChainNodeSetBasic(ns, app)
	chainNodeSet.Spec.Nodes = []appsv1.NodeGroupSpec{{Name: "fullnodes", Instances: ptr.To(2)}}
	chainNodeSet.Spec.Validator = &appsv1.NodeSetValidatorConfig{Init: &appsv1.GenesisInitConfig{
		ChainID:     "nibiru-localnet",
		Assets:      []string{"10000000unibi"},
		StakeAmount: "10000000unibi",
	}}
	err := tf.Client.Create(tf.Context(), chainNodeSet)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() appsv1.ChainNodeSetPhase {
		err := tf.Client.Get(tf.Context(), client.ObjectKeyFromObject(chainNodeSet), chainNodeSet)
		Expect(err).NotTo(HaveOccurred())
		return chainNodeSet.Status.Phase
	}).Should(Equal(appsv1.PhaseChainNodeSetRunning))

	chainNodeList := &appsv1.ChainNodeList{}
	err = tf.Client.List(tf.Context(), chainNodeList, &client.ListOptions{Namespace: ns.GetName()})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(chainNodeList.Items)).To(Equal(3))

	Eventually(func() bool {
		err = tf.Client.Get(tf.Context(), client.ObjectKeyFromObject(chainNodeSet), chainNodeSet)
		Expect(err).NotTo(HaveOccurred())
		chainNodeSet.Spec.Nodes[0].Instances = ptr.To(1)
		return tf.Client.Update(tf.Context(), chainNodeSet) == nil
	}).Should(BeTrue())

	Eventually(func() int {
		err = tf.Client.List(tf.Context(), chainNodeList, &client.ListOptions{Namespace: ns.GetName()})
		Expect(err).NotTo(HaveOccurred())
		return len(chainNodeList.Items)
	}).Should(Equal(2))
}
