package e2e

import (
	"context"
	"math/rand"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/test/framework"
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

// WaitForChainNodeRunning waits for a ChainNode to reach the Running phase
func WaitForChainNodeRunning(chainNode *appsv1.ChainNode) {
	Eventually(func() appsv1.ChainNodePhase {
		current := appsv1.ChainNode{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
			return ""
		}
		return current.Status.Phase
	}).Should(Equal(appsv1.PhaseChainNodeRunning))
}

// WaitForPodReady waits for a pod to have all containers ready
func WaitForPodReady(namespace, name string) {
	Eventually(func() bool {
		pod := &corev1.Pod{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKey{Namespace: namespace, Name: name}, pod); err != nil {
			return false
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}).Should(BeTrue())
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

// GetVaultAddress returns the in-cluster Vault address
func GetVaultAddress() string {
	return framework.VaultAddress
}

// CopyVaultSecretsToNamespace copies Vault token and CA certificate secrets to the test namespace.
// Returns the names of the token secret and CA secret in the target namespace.
func CopyVaultSecretsToNamespace(namespace string) (tokenSecretName, caSecretName string) {
	ctx := context.Background()

	// Copy token secret
	tokenSecret, err := Framework().KubeClient().CoreV1().Secrets(framework.VaultNamespace).Get(
		ctx, framework.VaultTokenSecretName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	tokenSecretName = "vault-token"
	newTokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": tokenSecret.Data["token"],
		},
	}
	_, err = Framework().KubeClient().CoreV1().Secrets(namespace).Create(ctx, newTokenSecret, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Copy CA certificate secret
	caSecret, err := Framework().KubeClient().CoreV1().Secrets(framework.VaultNamespace).Get(
		ctx, framework.VaultCASecretName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	caSecretName = "vault-ca"
	newCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"ca.crt": caSecret.Data["ca.crt"],
		},
	}
	_, err = Framework().KubeClient().CoreV1().Secrets(namespace).Create(ctx, newCASecret, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	return tokenSecretName, caSecretName
}

// WaitForTmkmsContainerRunning waits for the TMKMS container to be ready in the ChainNode pod
func WaitForTmkmsContainerRunning(chainNode *appsv1.ChainNode) {
	Eventually(func() bool {
		pod := &corev1.Pod{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKey{
			Namespace: chainNode.Namespace,
			Name:      chainNode.Name,
		}, pod); err != nil {
			return false
		}

		// Check if tmkms container exists and is ready
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == "tmkms" {
				return cs.Ready
			}
		}
		return false
	}).Should(BeTrue())
}
