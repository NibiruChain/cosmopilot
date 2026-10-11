package e2e

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
	"github.com/voluzi/cosmopilot/v2/test/framework"
)

// getVPAResourcesFromAnnotation extracts the resource requirements from the VPA annotation.
func getVPAResourcesFromAnnotation(annotations map[string]string) *corev1.ResourceRequirements {
	vpaStr, ok := annotations[controllers.AnnotationVPAResources]
	if !ok {
		return nil
	}

	var resources corev1.ResourceRequirements
	if err := json.Unmarshal([]byte(vpaStr), &resources); err != nil {
		return nil
	}
	return &resources
}

// getVPAMemoryRequestMiB extracts the memory request value from the VPA annotation in MiB.
// Returns 0 if annotation is not present or invalid.
func getVPAMemoryRequestMiB(annotations map[string]string) int64 {
	resources := getVPAResourcesFromAnnotation(annotations)
	if resources == nil {
		return 0
	}

	mem := resources.Requests.Memory()
	if mem == nil {
		return 0
	}
	return mem.Value() / (1024 * 1024)
}

// getVPACPURequestMillicores extracts the CPU request value from the VPA annotation in millicores.
// Returns 0 if annotation is not present or invalid.
func getVPACPURequestMillicores(annotations map[string]string) int64 {
	resources := getVPAResourcesFromAnnotation(annotations)
	if resources == nil {
		return 0
	}

	cpu := resources.Requests.Cpu()
	if cpu == nil {
		return 0
	}
	return cpu.MilliValue()
}

// getVPAMemoryLimitMiB extracts the memory limit value from the VPA annotation in MiB.
// Returns 0 if annotation is not present or invalid.
func getVPAMemoryLimitMiB(annotations map[string]string) int64 {
	resources := getVPAResourcesFromAnnotation(annotations)
	if resources == nil {
		return 0
	}

	mem := resources.Limits.Memory()
	if mem == nil {
		return 0
	}
	return mem.Value() / (1024 * 1024)
}

// getVPACPULimitMillicores extracts the CPU limit value from the VPA annotation in millicores.
// Returns 0 if annotation is not present or invalid.
func getVPACPULimitMillicores(annotations map[string]string) int64 {
	resources := getVPAResourcesFromAnnotation(annotations)
	if resources == nil {
		return 0
	}

	cpu := resources.Limits.Cpu()
	if cpu == nil {
		return 0
	}
	return cpu.MilliValue()
}

var _ = Describe("ChainNode VPA", Label("vpa"), func() {
	// ==========================================================================
	// Memory Scaling Tests
	// ==========================================================================
	Context("Memory Scale Up", func() {
		apps.ForEachApp("should scale up memory when usage exceeds threshold",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// Configure VPA with scale-up rule at 80% memory usage
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min: resource.MustParse("256Mi"),
						Max: resource.MustParse("2Gi"),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate high memory usage (85% of 512Mi = ~436Mi)
				err = mockHelper.SetMemoryMiB(436)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock memory")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 0
					}
					return getVPAMemoryRequestMiB(current.GetAnnotations())
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically(">", 512),
					"Memory should be scaled up from 512Mi")
			}),
		)
	})

	Context("Memory Scale Down", func() {
		apps.ForEachApp("should scale down memory when usage is below threshold",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min: resource.MustParse("256Mi"),
						Max: resource.MustParse("2Gi"),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleDown,
								UsagePercent: 40,
								StepPercent:  20,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("600Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate low memory usage (30% of 600Mi = 180Mi)
				err = mockHelper.SetMemoryMiB(180)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock memory")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 600
					}
					mem := getVPAMemoryRequestMiB(current.GetAnnotations())
					if mem == 0 {
						return 600
					}
					return mem
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically("<", 600),
					"Memory should be scaled down from 600Mi")
			}),
		)
	})

	// ==========================================================================
	// CPU Scaling Tests
	// ==========================================================================
	Context("CPU Scale Up", func() {
		apps.ForEachApp("should scale up CPU when usage exceeds threshold",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					CPU: &appsv1.VerticalAutoscalingMetricConfig{
						Min: resource.MustParse("100m"),
						Max: resource.MustParse("2000m"),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate high CPU usage (85% of 500m = 425m)
				err = mockHelper.SetCPUMillicores(425)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock CPU")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 0
					}
					return getVPACPURequestMillicores(current.GetAnnotations())
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically(">", 500),
					"CPU should be scaled up from 500m")
			}),
		)
	})

	Context("CPU Scale Down", func() {
		apps.ForEachApp("should scale down CPU when usage is below threshold",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// Default mock CPU is 100m. Use request=200m so default=50% (safe zone).
				// Then set mock to 60m (30%) to trigger scale-down.
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					CPU: &appsv1.VerticalAutoscalingMetricConfig{
						Min: resource.MustParse("50m"),
						Max: resource.MustParse("2000m"),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleDown,
								UsagePercent: 40,
								StepPercent:  20,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate low CPU usage (30% of 200m = 60m)
				err = mockHelper.SetCPUMillicores(60)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock CPU")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 200
					}
					cpu := getVPACPURequestMillicores(current.GetAnnotations())
					if cpu == 0 {
						return 200
					}
					return cpu
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically("<", 200),
					"CPU should be scaled down from 200m")
			}),
		)
	})

	// ==========================================================================
	// Safety Margin Tests
	// ==========================================================================
	Context("Memory Safety Margin", func() {
		apps.ForEachApp("should not scale down memory below safety margin",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// For safety margin to kick in, we need:
				// - newVal = request - (request * step%) < usage * 1.2
				// - With request=600Mi, step=80%: newVal = 120Mi
				// - With usage=180Mi, safetyMargin=20%: minSafe = 216Mi
				// - 120 < 216, so safety margin clamps to 216Mi (rounded to 217Mi due to MiB rounding)
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:                 resource.MustParse("100Mi"),
						Max:                 resource.MustParse("2Gi"),
						SafetyMarginPercent: ptr.To(20),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleDown,
								UsagePercent: 40,
								StepPercent:  80, // Very aggressive to trigger safety margin
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("600Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate low memory usage (30% of 600Mi = 180Mi)
				// Step 80% would give 120Mi, but safety margin (180 * 1.2 = 216Mi) clamps it
				err = mockHelper.SetMemoryMiB(180)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock memory")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 600
					}
					mem := getVPAMemoryRequestMiB(current.GetAnnotations())
					if mem == 0 {
						return 600
					}
					return mem
				}, 2*time.Minute, 5*time.Second).Should(And(
					BeNumerically("<", 600),
					BeNumerically(">=", 216), // 180 * 1.2 = 216
				), "Memory should be scaled down but stay above safety margin")
			}),
		)
	})

	Context("CPU Safety Margin", func() {
		apps.ForEachApp("should not scale down CPU below safety margin",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// Default mock CPU is 100m (0.1 cores). To avoid immediate scale-down before
				// we can set the mock value, use a request where 100m is in the safe zone (40-80%).
				// Request=200m means default 100m = 50% (safe zone).
				//
				// For safety margin to kick in, we need:
				// - newVal = request - (request * step%) < usage * 1.2
				// - With request=200m, step=80%: newVal = 40m
				// - With usage=60m, safetyMargin=20%: minSafe = 72m
				// - 40 < 72, so safety margin clamps to 72m
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					CPU: &appsv1.VerticalAutoscalingMetricConfig{
						Min:                 resource.MustParse("50m"),
						Max:                 resource.MustParse("2000m"),
						SafetyMarginPercent: ptr.To(20),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleDown,
								UsagePercent: 40,
								StepPercent:  80, // Very aggressive to trigger safety margin
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate low CPU usage (30% of 200m = 60m)
				// Step 80% would give 40m, but safety margin (60 * 1.2 = 72m) clamps it
				err = mockHelper.SetCPUMillicores(60)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock CPU")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 200
					}
					cpu := getVPACPURequestMillicores(current.GetAnnotations())
					if cpu == 0 {
						return 200
					}
					return cpu
				}, 2*time.Minute, 5*time.Second).Should(And(
					BeNumerically("<", 200),
					BeNumerically(">=", 72), // 60 * 1.2 = 72
				), "CPU should be scaled down but stay above safety margin")
			}),
		)
	})

	// ==========================================================================
	// Min/Max Bounds Tests
	// ==========================================================================
	Context("Min Bound", func() {
		apps.ForEachApp("should not scale down memory below min bound",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min: resource.MustParse("400Mi"), // Higher min to test clamping
						Max: resource.MustParse("2Gi"),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleDown,
								UsagePercent: 40,
								StepPercent:  50, // Would scale to 300Mi without min
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("600Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate very low memory usage (10% of 600Mi = 60Mi)
				err = mockHelper.SetMemoryMiB(60)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock memory")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 600
					}
					mem := getVPAMemoryRequestMiB(current.GetAnnotations())
					if mem == 0 {
						return 600
					}
					return mem
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically("==", 400),
					"Memory should be clamped at min bound (400Mi)")
			}),
		)
	})

	Context("Max Bound", func() {
		apps.ForEachApp("should not scale up memory above max bound",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min: resource.MustParse("256Mi"),
						Max: resource.MustParse("600Mi"), // Lower max to test clamping
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  50, // Would scale to 750Mi without max
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Simulate high memory usage (90% of 500Mi = 450Mi)
				err = mockHelper.SetMemoryMiB(450)
				Expect(err).NotTo(HaveOccurred(), "Failed to set mock memory")

				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 0
					}
					return getVPAMemoryRequestMiB(current.GetAnnotations())
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically("==", 600),
					"Memory should be clamped at max bound (600Mi)")
			}),
		)
	})

	// ==========================================================================
	// Cooldown Tests
	// ==========================================================================
	Context("Cooldown", func() {
		apps.ForEachApp("should respect cooldown period between scaling actions",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:      resource.MustParse("256Mi"),
						Max:      resource.MustParse("2Gi"),
						Cooldown: ptr.To("30s"), // 30 second cooldown
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Trigger first scale-up
				err = mockHelper.SetMemoryMiB(450) // ~88% of 512Mi
				Expect(err).NotTo(HaveOccurred())

				// Wait for first scale-up (512 -> 640)
				var firstScaledValue int64
				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 0
					}
					firstScaledValue = getVPAMemoryRequestMiB(current.GetAnnotations())
					return firstScaledValue
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically(">", 512))

				// Wait for pod to be recreated with the new resource values
				// (not just "ready" status, but actually has the scaled memory request)
				Eventually(func() int64 {
					pod, err := Framework().KubeClient().CoreV1().Pods(ns.Name).Get(
						Framework().Context(), chainNode.Name, metav1.GetOptions{})
					if err != nil {
						return 0
					}
					for _, c := range pod.Spec.Containers {
						if c.Name == chainNode.Spec.App.App {
							if mem := c.Resources.Requests.Memory(); mem != nil {
								return mem.Value() / (1024 * 1024)
							}
						}
					}
					return 0
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically(">=", firstScaledValue),
					"Pod should have the scaled memory request")

				WaitForPodReady(ns.Name, chainNode.Name)

				// Try to trigger another scale-up (still high usage relative to new value)
				err = mockHelper.SetMemoryMiB(560) // ~87% of 640Mi
				Expect(err).NotTo(HaveOccurred())

				// Verify no scaling happens within cooldown period (first 15 seconds)
				Consistently(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return firstScaledValue
					}
					return getVPAMemoryRequestMiB(current.GetAnnotations())
				}, 15*time.Second, 5*time.Second).Should(Equal(firstScaledValue),
					"Memory should not scale again within cooldown period")

				// Wait for cooldown to expire and verify scaling happens
				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return firstScaledValue
					}
					return getVPAMemoryRequestMiB(current.GetAnnotations())
				}, 45*time.Second, 5*time.Second).Should(BeNumerically(">", firstScaledValue),
					"Memory should scale again after cooldown expires")
			}),
		)
	})

	// ==========================================================================
	// Hysteresis Tests
	// ==========================================================================
	Context("Hysteresis", func() {
		apps.ForEachApp("should apply hysteresis to scale-down threshold",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// Scale-down threshold is 40%, hysteresis is 10%
				// Effective threshold becomes 30% (40 - 10)
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:               resource.MustParse("256Mi"),
						Max:               resource.MustParse("2Gi"),
						HysteresisPercent: ptr.To(10),
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleDown,
								UsagePercent: 40, // Effective: 30% due to hysteresis
								StepPercent:  20,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("600Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				// Set usage at 35% (210Mi) - below 40% but above effective 30%
				// Should NOT trigger scale-down due to hysteresis
				err = mockHelper.SetMemoryMiB(210)
				Expect(err).NotTo(HaveOccurred())

				// Verify no scaling happens (usage is in hysteresis zone)
				Consistently(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 600
					}
					mem := getVPAMemoryRequestMiB(current.GetAnnotations())
					if mem == 0 {
						return 600
					}
					return mem
				}, 20*time.Second, 5*time.Second).Should(BeNumerically("==", 600),
					"Memory should not scale when usage is in hysteresis zone")

				// Ensure pod is still ready before setting new memory value
				WaitForPodReady(ns.Name, chainNode.Name)

				// Now set usage below effective threshold (25% = 150Mi)
				err = mockHelper.SetMemoryMiB(150)
				Expect(err).NotTo(HaveOccurred())

				// Should trigger scale-down now
				Eventually(func() int64 {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return 600
					}
					mem := getVPAMemoryRequestMiB(current.GetAnnotations())
					if mem == 0 {
						return 600
					}
					return mem
				}, 2*time.Minute, 5*time.Second).Should(BeNumerically("<", 600),
					"Memory should scale down when usage is below effective threshold")
			}),
		)
	})

	// ==========================================================================
	// Limit Update Strategy Tests
	// ==========================================================================
	Context("Limit Strategy Equal", func() {
		apps.ForEachApp("should set limits equal to requests after scaling",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				equalStrategy := appsv1.LimitEqual
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:           resource.MustParse("256Mi"),
						Max:           resource.MustParse("2Gi"),
						LimitStrategy: &equalStrategy,
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				err = mockHelper.SetMemoryMiB(450) // ~88% of 512Mi
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return false
					}
					request := getVPAMemoryRequestMiB(current.GetAnnotations())
					limit := getVPAMemoryLimitMiB(current.GetAnnotations())
					return request > 512 && request == limit
				}, 2*time.Minute, 5*time.Second).Should(BeTrue(),
					"Limit should equal request after scaling")
			}),
		)
	})

	Context("Limit Strategy Max", func() {
		apps.ForEachApp("should set limits to VPA max after scaling",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				maxStrategy := appsv1.LimitVpaMax
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:           resource.MustParse("256Mi"),
						Max:           resource.MustParse("1Gi"),
						LimitStrategy: &maxStrategy,
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				err = mockHelper.SetMemoryMiB(450)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return false
					}
					request := getVPAMemoryRequestMiB(current.GetAnnotations())
					limit := getVPAMemoryLimitMiB(current.GetAnnotations())
					return request > 512 && limit == 1024 // 1Gi = 1024Mi
				}, 2*time.Minute, 5*time.Second).Should(BeTrue(),
					"Limit should be set to VPA max (1Gi)")
			}),
		)
	})

	Context("Limit Strategy Percentage", func() {
		apps.ForEachApp("should set limits to percentage of request after scaling",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				percentageStrategy := appsv1.LimitPercentage
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:             resource.MustParse("256Mi"),
						Max:             resource.MustParse("2Gi"),
						LimitStrategy:   &percentageStrategy,
						LimitPercentage: ptr.To(150), // 150% of request
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				err = mockHelper.SetMemoryMiB(450)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return false
					}
					request := getVPAMemoryRequestMiB(current.GetAnnotations())
					limit := getVPAMemoryLimitMiB(current.GetAnnotations())
					if request <= 512 {
						return false
					}
					// Limit should be ~150% of request (allow some rounding)
					expectedLimit := request * 150 / 100
					return limit >= expectedLimit-1 && limit <= expectedLimit+1
				}, 2*time.Minute, 5*time.Second).Should(BeTrue(),
					"Limit should be 150% of request after scaling")
			}),
		)
	})

	Context("Limit Strategy Unset", func() {
		apps.ForEachApp("should remove limits after scaling",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				unsetStrategy := appsv1.LimitUnset
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:           resource.MustParse("256Mi"),
						Max:           resource.MustParse("2Gi"),
						LimitStrategy: &unsetStrategy,
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				err = mockHelper.SetMemoryMiB(450)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return false
					}
					request := getVPAMemoryRequestMiB(current.GetAnnotations())
					limit := getVPAMemoryLimitMiB(current.GetAnnotations())
					return request > 512 && limit == 0
				}, 2*time.Minute, 5*time.Second).Should(BeTrue(),
					"Limit should be unset (0) after scaling")
			}),
		)
	})

	Context("Limit Strategy Retain", func() {
		apps.ForEachApp("should retain original limits after scaling",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				retainStrategy := appsv1.LimitRetain
				vpa := &appsv1.VerticalAutoscalingConfig{
					Enabled: true,
					Memory: &appsv1.VerticalAutoscalingMetricConfig{
						Min:           resource.MustParse("256Mi"),
						Max:           resource.MustParse("2Gi"),
						LimitStrategy: &retainStrategy,
						Rules: []*appsv1.VerticalAutoscalingRule{
							{
								Direction:    appsv1.ScaleUp,
								UsagePercent: 80,
								StepPercent:  25,
								Duration:     ptr.To("5s"),
								Cooldown:     ptr.To("5s"),
							},
						},
					},
				}

				chainNode := app.BuildChainNodeWithVPA(ns.Name, vpa)
				chainNode.Spec.Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("800Mi"),
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				WaitForChainNodeRunning(chainNode)
				WaitForPodReady(ns.Name, chainNode.Name)

				mockHelper := framework.NewMockNodeUtilsHelper(Framework(), ns.Name, chainNode.Name)

				err = mockHelper.SetMemoryMiB(450)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					current := appsv1.ChainNode{}
					if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), &current); err != nil {
						return false
					}
					request := getVPAMemoryRequestMiB(current.GetAnnotations())
					limit := getVPAMemoryLimitMiB(current.GetAnnotations())
					return request > 512 && limit == 800 // Original limit retained
				}, 2*time.Minute, 5*time.Second).Should(BeTrue(),
					"Limit should be retained at original value (800Mi)")
			}),
		)
	})
})
