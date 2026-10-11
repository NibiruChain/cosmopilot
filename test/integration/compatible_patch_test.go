package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Compatible patches", func() {
	It("should propagate patch updates and removal from a node group", WithNamespace(func(ns *corev1.Namespace) {
		owner := &appsv1.ChainNodeSet{ObjectMeta: metav1.ObjectMeta{GenerateName: ChainNodeSetPrefix, Namespace: ns.Name}, Spec: appsv1.ChainNodeSetSpec{App: DefaultChainNodeSetTestApp, Genesis: NewGenesisConfigWithChainID("patch-test"), Nodes: []appsv1.NodeGroupSpec{{Name: "snapshot", CompatiblePatch: &appsv1.CompatiblePatchSpec{Version: "1.0.1", UpgradeHeight: 0}}, {Name: "unpatched"}}}}
		Expect(Framework().Client().Create(Framework().Context(), owner)).To(Succeed())
		WaitForChainNodeCount(ns.Name, 2)
		for _, version := range []string{"1.0.1", "1.0.2", ""} {
			if version != "1.0.1" {
				Eventually(func() error {
					current := &appsv1.ChainNodeSet{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(owner), current); err != nil {
						return err
					}
					if version == "" {
						current.Spec.Nodes[0].CompatiblePatch = nil
					} else {
						current.Spec.Nodes[0].CompatiblePatch.Version = version
					}
					return Framework().Client().Update(Framework().Context(), current)
				}).Should(Succeed())
			}
			Eventually(func() string {
				node := &appsv1.ChainNode{}
				if err := Framework().Client().Get(Framework().Context(), client.ObjectKey{Namespace: ns.Name, Name: owner.Name + "-snapshot-0"}, node); err != nil {
					return "not found"
				}
				if node.Spec.CompatiblePatch == nil {
					return ""
				}
				return node.Spec.CompatiblePatch.Version
			}).Should(Equal(version))
			other := &appsv1.ChainNode{}
			Expect(Framework().Client().Get(Framework().Context(), client.ObjectKey{Namespace: ns.Name, Name: owner.Name + "-unpatched-0"}, other)).To(Succeed())
			Expect(other.Spec.CompatiblePatch).To(BeNil())
		}
	}))
	DescribeTable("should reject invalid patches on every supported resource location", func(location string, patch appsv1.CompatiblePatchSpec) {
		ns := CreateTestNamespace()
		if location == "node" {
			node := &appsv1.ChainNode{ObjectMeta: metav1.ObjectMeta{GenerateName: ChainNodePrefix, Namespace: ns.Name}, Spec: appsv1.ChainNodeSpec{App: DefaultChainNodeTestApp, Genesis: NewGenesisConfigWithChainID("patch-test"), CompatiblePatch: &patch}}
			Expect(Framework().Client().Create(Framework().Context(), node)).NotTo(Succeed())
		} else {
			owner := &appsv1.ChainNodeSet{ObjectMeta: metav1.ObjectMeta{GenerateName: ChainNodeSetPrefix, Namespace: ns.Name}, Spec: appsv1.ChainNodeSetSpec{App: DefaultChainNodeSetTestApp, Genesis: NewGenesisConfigWithChainID("patch-test"), Nodes: []appsv1.NodeGroupSpec{{Name: "snapshot"}}}}
			if location == "validator" {
				owner.Spec.Validator = &appsv1.NodeSetValidatorConfig{CompatiblePatch: &patch}
			} else {
				owner.Spec.Nodes[0].CompatiblePatch = &patch
			}
			err := Framework().Client().Create(Framework().Context(), owner)
			Expect(err).To(HaveOccurred(), fmt.Sprintf("location %s", location))
		}
	},
		Entry("node empty version", "node", appsv1.CompatiblePatchSpec{UpgradeHeight: 0}),
		Entry("node negative height", "node", appsv1.CompatiblePatchSpec{Version: "1.0.1", UpgradeHeight: -1}),
		Entry("group empty version", "group", appsv1.CompatiblePatchSpec{UpgradeHeight: 0}),
		Entry("group negative height", "group", appsv1.CompatiblePatchSpec{Version: "1.0.1", UpgradeHeight: -1}),
		Entry("validator empty version", "validator", appsv1.CompatiblePatchSpec{UpgradeHeight: 0}),
		Entry("validator negative height", "validator", appsv1.CompatiblePatchSpec{Version: "1.0.1", UpgradeHeight: -1}),
	)
})
