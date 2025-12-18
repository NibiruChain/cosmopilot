package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
)

var _ = Describe("ChainNode Webhook Validation", func() {
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
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App: DefaultChainNodeTestApp,
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".spec.genesis is required except when initializing new genesis with .spec.validator.init"))
	})

	It("should reject creation with both .spec.genesis and .spec.validator.init specified", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:     DefaultChainNodeTestApp,
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis")},
				Validator: &appsv1.ValidatorConfig{
					Init: &appsv1.GenesisInitConfig{
						ChainID:     "test-localnet",
						Assets:      []string{"10000000unibi"},
						StakeAmount: "10000000unibi",
					}},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".spec.genesis and .spec.validator.init are mutually exclusive"))
	})

	It("should accept creation with valid genesis init config", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App: DefaultChainNodeTestApp,
				Validator: &appsv1.ValidatorConfig{Init: &appsv1.GenesisInitConfig{
					ChainID:     "test-localnet",
					Assets:      []string{"10000000unibi"},
					StakeAmount: "10000000unibi",
				}},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should accept creation with valid genesis URL", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:     DefaultChainNodeTestApp,
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should reject invalid persistence size format", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:     DefaultChainNodeTestApp,
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
				Persistence: &appsv1.Persistence{
					Size: ptr.To("invalid-size"),
				},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".spec.size"))
	})

	It("should reject snapshots with both retention and retain specified", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:     DefaultChainNodeTestApp,
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
				Persistence: &appsv1.Persistence{
					Snapshots: &appsv1.VolumeSnapshotsConfig{
						Frequency: "1h",
						Retention: ptr.To("24h"),
						Retain:    ptr.To(int32(5)),
					},
				},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("retention"))
	})

	It("should accept valid persistence configuration", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:     DefaultChainNodeTestApp,
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
				Persistence: &appsv1.Persistence{
					Size:             ptr.To("100Gi"),
					StorageClassName: ptr.To("standard"),
				},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should accept valid snapshots configuration with retain", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:     DefaultChainNodeTestApp,
				Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
				Persistence: &appsv1.Persistence{
					Snapshots: &appsv1.VolumeSnapshotsConfig{
						Frequency: "1h",
						Retain:    ptr.To(int32(5)),
					},
				},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should accept validator with no init when genesis is provided", func() {
		chainNode := &appsv1.ChainNode{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: ChainNodePrefix,
				Namespace:    ns.Name,
			},
			Spec: appsv1.ChainNodeSpec{
				App:       DefaultChainNodeTestApp,
				Genesis:   &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
				Validator: &appsv1.ValidatorConfig{},
			},
		}

		err := Framework().Client().Create(Framework().Context(), chainNode)
		Expect(err).NotTo(HaveOccurred())
	})
})
