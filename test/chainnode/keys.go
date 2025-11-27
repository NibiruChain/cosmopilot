package chainnode

import (
	"fmt"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers/chainnode"
	"github.com/NibiruChain/cosmopilot/test/framework"
)

const (
	privKey = `{"address":"DE623086321818A30ADF4A8D68EEBEBDBF78B0F9","pub_key":{"type":"tendermint/PubKeyEd25519","value":"vwvZODnQoT31PwNN4ZhwIOwfSQ/iar4QAa0C6Tr5yVw="},"priv_key":{"type":"tendermint/PrivKeyEd25519","value":"LXYv7Ogc5tiniSiKRUrkcVP5IpgyE5qr9h5wSTmphwu/C9k4OdChPfU/A03hmHAg7B9JD+JqvhABrQLpOvnJXA=="}}`
	pubKey  = `{"@type":"/cosmos.crypto.ed25519.PubKey","key":"vwvZODnQoT31PwNN4ZhwIOwfSQ/iar4QAa0C6Tr5yVw="}`

	accountMnemonic   = "valve casual witness chase peace quick setup tip episode video huge attack book dash anchor parrot glove gossip habit brand message curtain much cupboard"
	accountAddress    = "nibi1c6f4v52znljnnsw2uhauuvets9p0q8eh67eqqp"
	accountValAddress = "nibivaloper1c6f4v52znljnnsw2uhauuvets9p0q8ehn9hm5u"
)

func testCreatePrivateKey(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
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

	secretName := fmt.Sprintf("%s-priv-key", chainNode.GetName())
	secret, err := tf.KubeClient.CoreV1().Secrets(ns.GetName()).Get(tf.Context(), secretName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(secret.Data[chainnode.PrivKeyFilename]).NotTo(BeEmpty())
}

func testImportPrivateKey(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
	chainNodeName := GetRandomChainNodeName()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-priv-key", chainNodeName),
			Namespace: ns.GetName(),
		},
		Data: map[string][]byte{
			chainnode.PrivKeyFilename: []byte(privKey),
		},
	}

	_, err := tf.KubeClient.CoreV1().Secrets(ns.GetName()).Create(tf.Context(), secret, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	chainNode := NewChainNodeBasic(ns, app)
	chainNode.GenerateName = ""
	chainNode.Name = chainNodeName
	chainNode.Spec.Validator = &appsv1.ValidatorConfig{Init: &appsv1.GenesisInitConfig{
		ChainID:     "nibiru-localnet",
		Assets:      []string{"10000000unibi"},
		StakeAmount: "10000000unibi",
	}}
	err = tf.Client.Create(tf.Context(), chainNode)
	Expect(err).NotTo(HaveOccurred())

	// Ensure node is running and generating blocks
	currentChainNode := appsv1.ChainNode{}
	Eventually(func() int64 {
		err := tf.Client.Get(tf.Context(), client.ObjectKeyFromObject(chainNode), &currentChainNode)
		Expect(err).NotTo(HaveOccurred())
		return currentChainNode.Status.LatestHeight
	}).Should(BeNumerically(">", 1))

	Expect(currentChainNode.Status.PubKey).To(Equal(pubKey))
}

func testCreateAccount(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
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

	secretName := fmt.Sprintf("%s-account", chainNode.GetName())
	secret, err := tf.KubeClient.CoreV1().Secrets(ns.GetName()).Get(tf.Context(), secretName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(secret.Data[chainnode.MnemonicKey]).NotTo(BeEmpty())
}

func testImportAccount(tf *framework.TestFramework, ns *corev1.Namespace, app appsv1.AppSpec) {
	chainNodeName := GetRandomChainNodeName()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-account", chainNodeName),
			Namespace: ns.GetName(),
		},
		Data: map[string][]byte{
			chainnode.MnemonicKey: []byte(accountMnemonic),
		},
	}

	_, err := tf.KubeClient.CoreV1().Secrets(ns.GetName()).Create(tf.Context(), secret, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	chainNode := NewChainNodeBasic(ns, app)
	chainNode.GenerateName = ""
	chainNode.Name = chainNodeName
	chainNode.Spec.Validator = &appsv1.ValidatorConfig{Init: &appsv1.GenesisInitConfig{
		ChainID:     "nibiru-localnet",
		Assets:      []string{"10000000unibi"},
		StakeAmount: "10000000unibi",
	}}
	err = tf.Client.Create(tf.Context(), chainNode)
	Expect(err).NotTo(HaveOccurred())

	// Ensure node is running and generating blocks
	currentChainNode := appsv1.ChainNode{}
	Eventually(func() int64 {
		err := tf.Client.Get(tf.Context(), client.ObjectKeyFromObject(chainNode), &currentChainNode)
		Expect(err).NotTo(HaveOccurred())
		return currentChainNode.Status.LatestHeight
	}).Should(BeNumerically(">", 1))

	Expect(currentChainNode.Status.AccountAddress).To(Equal(accountAddress))
	Expect(currentChainNode.Status.ValidatorAddress).To(Equal(accountValAddress))
}
