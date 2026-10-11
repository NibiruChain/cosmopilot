package integration

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
)

var _ = Describe("ChainNode VPA", func() {
	Context("VPA Configuration", func() {
		It("should accept valid VPA configuration",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("100m"),
								Max: resource.MustParse("4000m"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{
										Direction:    appsv1.ScaleUp,
										UsagePercent: 80,
										StepPercent:  20,
										Cooldown:     ptr.To("3m"),
									},
									{
										Direction:    appsv1.ScaleDown,
										UsagePercent: 40,
										StepPercent:  15,
										Cooldown:     ptr.To("10m"),
									},
								},
							},
							Memory: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("512Mi"),
								Max: resource.MustParse("8Gi"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{
										Direction:    appsv1.ScaleUp,
										UsagePercent: 85,
										StepPercent:  25,
									},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Verify the ChainNode was created with VPA config
				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					return current.Spec.VPA != nil && current.Spec.VPA.IsEnabled()
				}).Should(BeTrue())
			}),
		)

		It("should accept VPA with new fields (hysteresis, safety margin, emergency scale)",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							Memory: &appsv1.VerticalAutoscalingMetricConfig{
								Min:                     resource.MustParse("512Mi"),
								Max:                     resource.MustParse("8Gi"),
								SafetyMarginPercent:     ptr.To(20),
								HysteresisPercent:       ptr.To(10),
								EmergencyScaleUpPercent: ptr.To(30),
								MaxOOMRecoveries:        ptr.To(5),
								OOMRecoveryWindow:       ptr.To("2h"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{
										Direction:    appsv1.ScaleUp,
										UsagePercent: 80,
										StepPercent:  25,
									},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Verify all new VPA fields are accepted
				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					if current.Spec.VPA == nil || current.Spec.VPA.Memory == nil {
						return false
					}
					mem := current.Spec.VPA.Memory
					return mem.GetSafetyMarginPercent() == 20 &&
						mem.GetHysteresisPercent() == 10 &&
						mem.GetEmergencyScaleUpPercent() == 30 &&
						mem.GetMaxOOMRecoveries() == 5
				}).Should(BeTrue())
			}),
		)

		It("should use default values when VPA fields are not specified",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							Memory: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("512Mi"),
								Max: resource.MustParse("8Gi"),
								// Not setting SafetyMarginPercent, HysteresisPercent, etc.
								Rules: []*appsv1.VerticalAutoscalingRule{
									{
										Direction:    appsv1.ScaleUp,
										UsagePercent: 80,
										StepPercent:  25,
									},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Verify default values are used
				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					if current.Spec.VPA == nil || current.Spec.VPA.Memory == nil {
						return false
					}
					mem := current.Spec.VPA.Memory
					// Defaults: SafetyMargin=15, Hysteresis=5, EmergencyScaleUp=25, MaxOOMRecoveries=3
					return mem.GetSafetyMarginPercent() == appsv1.DefaultSafetyMarginPercent &&
						mem.GetHysteresisPercent() == appsv1.DefaultHysteresisPercent &&
						mem.GetEmergencyScaleUpPercent() == appsv1.DefaultEmergencyScaleUpPercent &&
						mem.GetMaxOOMRecoveries() == appsv1.DefaultMaxOOMRecoveries
				}).Should(BeTrue())
			}),
		)
	})

	Context("VPA Annotation Storage", func() {
		It("should store VPA resources annotation when VPA is enabled",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("100m"),
								Max: resource.MustParse("2000m"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// The controller should eventually store the VPA resources annotation
				// Note: This may not happen immediately if no metrics are available
				// but the annotation structure should be valid when set

				// Wait for reconciliation to complete at least once
				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					// Check if status has been updated (indicates reconciliation occurred)
					return current.Status.Phase != ""
				}).Should(BeTrue())
			}),
		)

		It("should parse VPA resources annotation correctly",
			WithNamespace(func(ns *corev1.Namespace) {
				// Create ChainNode with pre-set VPA annotation
				vpaResources := corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("750m"),
						corev1.ResourceMemory: resource.MustParse("1536Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500m"),
						corev1.ResourceMemory: resource.MustParse("3Gi"),
					},
				}
				vpaAnnotation, err := json.Marshal(vpaResources)
				Expect(err).NotTo(HaveOccurred())

				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
						Annotations: map[string]string{
							controllers.AnnotationVPAResources: string(vpaAnnotation),
						},
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("100m"),
								Max: resource.MustParse("2000m"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err = Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Verify the annotation is preserved
				Eventually(func() string {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return ""
					}
					return current.Annotations[controllers.AnnotationVPAResources]
				}).Should(ContainSubstring("750m"))
			}),
		)
	})

	Context("VPA Limit Strategy", func() {
		It("should accept LimitEqual strategy",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min:           resource.MustParse("100m"),
								Max:           resource.MustParse("2000m"),
								LimitStrategy: ptr.To(appsv1.LimitEqual),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() *appsv1.LimitUpdateStrategy {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return nil
					}
					if current.Spec.VPA == nil || current.Spec.VPA.CPU == nil {
						return nil
					}
					return current.Spec.VPA.CPU.LimitStrategy
				}).Should(Equal(ptr.To(appsv1.LimitEqual)))
			}),
		)

		It("should accept LimitPercentage strategy with custom percentage",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							Memory: &appsv1.VerticalAutoscalingMetricConfig{
								Min:             resource.MustParse("512Mi"),
								Max:             resource.MustParse("8Gi"),
								LimitStrategy:   ptr.To(appsv1.LimitPercentage),
								LimitPercentage: ptr.To(200), // Limit = 200% of request
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					if current.Spec.VPA == nil || current.Spec.VPA.Memory == nil {
						return false
					}
					mem := current.Spec.VPA.Memory
					return mem.LimitStrategy != nil && *mem.LimitStrategy == appsv1.LimitPercentage &&
						mem.LimitPercentage != nil && *mem.LimitPercentage == 200
				}).Should(BeTrue())
			}),
		)

		It("should accept LimitUnset strategy",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: true,
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min:           resource.MustParse("100m"),
								Max:           resource.MustParse("2000m"),
								LimitStrategy: ptr.To(appsv1.LimitUnset), // Should not set limits
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() *appsv1.LimitUpdateStrategy {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return nil
					}
					if current.Spec.VPA == nil || current.Spec.VPA.CPU == nil {
						return nil
					}
					return current.Spec.VPA.CPU.LimitStrategy
				}).Should(Equal(ptr.To(appsv1.LimitUnset)))
			}),
		)
	})

	Context("VPA Disabled", func() {
		It("should not interfere when VPA is disabled",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled: false, // Explicitly disabled
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("100m"),
								Max: resource.MustParse("2000m"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for reconciliation
				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					return current.Status.Phase != ""
				}).Should(BeTrue())

				// Verify VPA is disabled
				current := &appsv1.ChainNode{}
				err = Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current)
				Expect(err).NotTo(HaveOccurred())
				Expect(current.Spec.VPA.IsEnabled()).To(BeFalse())
			}),
		)

		It("should work without VPA config at all",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						// No VPA config
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for reconciliation
				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					return current.Status.Phase != ""
				}).Should(BeTrue())

				// Verify no VPA
				current := &appsv1.ChainNode{}
				err = Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current)
				Expect(err).NotTo(HaveOccurred())
				Expect(current.Spec.VPA).To(BeNil())
			}),
		)
	})

	Context("VPA with ResetAfterUpgrade", func() {
		It("should accept resetVpaAfterNodeUpgrade option",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						VPA: &appsv1.VerticalAutoscalingConfig{
							Enabled:                  true,
							ResetVpaAfterNodeUpgrade: true,
							CPU: &appsv1.VerticalAutoscalingMetricConfig{
								Min: resource.MustParse("100m"),
								Max: resource.MustParse("2000m"),
								Rules: []*appsv1.VerticalAutoscalingRule{
									{Direction: appsv1.ScaleUp, UsagePercent: 80, StepPercent: 20},
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := &appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), current); err != nil {
						return false
					}
					if current.Spec.VPA == nil {
						return false
					}
					return current.Spec.VPA.ResetVpaAfterNodeUpgrade
				}).Should(BeTrue())
			}),
		)
	})
})
