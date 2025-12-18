package e2e

import (
	"math/rand"
	"time"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
)

// RandString generates a random string of the specified length
func RandString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[r.Intn(len(letterBytes))]
	}
	return string(b)
}

// WaitForChainNodeHeight waits for a ChainNode to reach the specified height
func WaitForChainNodeHeight(chainNode *appsv1.ChainNode, minHeight int64) {
	Eventually(func() int64 {
		current := appsv1.ChainNode{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
			return 0
		}
		return current.Status.LatestHeight
	}).Should(BeNumerically(">", minHeight))
}

// WaitForChainNodeSetRunning waits for a ChainNodeSet to reach the running phase
func WaitForChainNodeSetRunning(chainNodeSet *appsv1.ChainNodeSet) {
	Eventually(func() appsv1.ChainNodeSetPhase {
		current := appsv1.ChainNodeSet{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), &current); err != nil {
			return ""
		}
		return current.Status.Phase
	}).Should(Equal(appsv1.PhaseChainNodeSetRunning))
}

// WaitForChainNodeSetHeight waits for a ChainNodeSet to reach the specified height
func WaitForChainNodeSetHeight(chainNodeSet *appsv1.ChainNodeSet, minHeight int64) {
	Eventually(func() int64 {
		current := appsv1.ChainNodeSet{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), &current); err != nil {
			return 0
		}
		return current.Status.LatestHeight
	}).Should(BeNumerically(">", minHeight))
}

// RefreshChainNode fetches the latest state of a ChainNode
func RefreshChainNode(chainNode *appsv1.ChainNode) {
	err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), chainNode)
	Expect(err).NotTo(HaveOccurred())
}

// RefreshChainNodeSet fetches the latest state of a ChainNodeSet
func RefreshChainNodeSet(chainNodeSet *appsv1.ChainNodeSet) {
	err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), chainNodeSet)
	Expect(err).NotTo(HaveOccurred())
}

// CountChainNodes returns the number of ChainNodes in the namespace
func CountChainNodes(namespace string) int {
	chainNodeList := &appsv1.ChainNodeList{}
	err := Framework().Client().List(Framework().Context(), chainNodeList, &client.ListOptions{Namespace: namespace})
	Expect(err).NotTo(HaveOccurred())
	return len(chainNodeList.Items)
}

// WaitForChainNodeCount waits for the number of ChainNodes in the namespace to equal the expected count
func WaitForChainNodeCount(namespace string, expectedCount int) {
	Eventually(func() int {
		chainNodeList := &appsv1.ChainNodeList{}
		if err := Framework().Client().List(Framework().Context(), chainNodeList, &client.ListOptions{Namespace: namespace}); err != nil {
			return -1
		}
		return len(chainNodeList.Items)
	}).Should(Equal(expectedCount))
}

// WaitForChainNodesHeight waits for all ChainNodes in a ChainNodeSet to reach the minimum height
func WaitForChainNodesHeight(chainNodeSet *appsv1.ChainNodeSet, minHeight int64) {
	// Refresh the ChainNodeSet to get the latest status
	RefreshChainNodeSet(chainNodeSet)

	// Collect all node names
	var nodeNames []string

	// Add validator if present
	if chainNodeSet.Spec.Validator != nil {
		nodeNames = append(nodeNames, chainNodeSet.Name+"-validator")
	}

	// Add all nodes from status
	for _, node := range chainNodeSet.Status.Nodes {
		nodeNames = append(nodeNames, node.Name)
	}

	// Wait for each node to reach the minimum height
	for _, name := range nodeNames {
		Eventually(func() int64 {
			current := appsv1.ChainNode{}
			if err := Framework().Client().Get(Framework().Context(), client.ObjectKey{Namespace: chainNodeSet.Namespace, Name: name}, &current); err != nil {
				return 0
			}
			return current.Status.LatestHeight
		}).Should(BeNumerically(">", minHeight))
	}
}
