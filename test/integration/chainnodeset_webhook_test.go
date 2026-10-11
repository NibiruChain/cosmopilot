package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

var _ = Describe("ChainNodeSet Webhook Validation", func() {
	var (
		ns  *corev1.Namespace
		err error
	)

	BeforeEach(func() {
		ns, err = Framework().CreateRandomNamespace()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if ns != nil {
			err = Framework().DeleteNamespace(ns)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("should reject creation without .spec.genesis when .spec.validator.init is not specified", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App:   DefaultChainNodeSetTestApp,
				Nodes: []appsv1.NodeGroupSpec{{Name: "fullnodes"}},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".spec.genesis is required except when initializing new genesis with .spec.validator.init"))
	})

	It("should reject creation with both .spec.genesis and .spec.validator.init specified", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App:     DefaultChainNodeSetTestApp,
				Nodes:   []appsv1.NodeGroupSpec{{Name: "fullnodes"}},
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis")},
				Validator: &appsv1.NodeSetValidatorConfig{Init: &appsv1.GenesisInitConfig{
					ChainID:     "test-localnet",
					Assets:      []string{"10000000unibi"},
					StakeAmount: "10000000unibi",
				}},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".spec.genesis and .spec.validator.init are mutually exclusive"))
	})

	It("should accept creation with valid genesis config using chainID and useDataVolume", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App:   DefaultChainNodeSetTestApp,
				Nodes: []appsv1.NodeGroupSpec{{Name: "fullnodes", Instances: ptr.To(1)}},
				Genesis: &appsv1.GenesisConfig{
					Url:           ptr.To("https://example.com/genesis.json"),
					ChainID:       ptr.To("test-localnet"),
					UseDataVolume: ptr.To(true),
				},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should accept creation with valid genesis URL", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App:     DefaultChainNodeSetTestApp,
				Nodes:   []appsv1.NodeGroupSpec{{Name: "fullnodes", Instances: ptr.To(1)}},
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should reject invalid node group persistence size", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App: DefaultChainNodeSetTestApp,
				Nodes: []appsv1.NodeGroupSpec{{
					Name:      "fullnodes",
					Instances: ptr.To(1),
					Persistence: &appsv1.Persistence{
						Size: ptr.To("invalid"),
					},
				}},
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("persistence.size"))
	})

	It("should accept multiple node groups", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App: DefaultChainNodeSetTestApp,
				Nodes: []appsv1.NodeGroupSpec{
					{Name: "fullnodes", Instances: ptr.To(2)},
					{Name: "archive", Instances: ptr.To(1)},
				},
				Genesis: &appsv1.GenesisConfig{
					Url:           ptr.To("https://example.com/genesis.json"),
					ChainID:       ptr.To("test-localnet"),
					UseDataVolume: ptr.To(true),
				},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should accept creation without validator when genesis is provided", func() {
		chainNodeSet := &appsv1.ChainNodeSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodeSetPrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSetSpec{
				App:     DefaultChainNodeSetTestApp,
				Nodes:   []appsv1.NodeGroupSpec{{Name: "fullnodes", Instances: ptr.To(1)}},
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNodeSet)
		Expect(err).NotTo(HaveOccurred())
	})
})
