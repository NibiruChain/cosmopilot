package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
)

var _ = Describe("ChainNode PVC", func() {
	Context("PVC Creation", func() {
		It("should create PVC with default size when persistence is not specified",
			WithNamespace(func(ns *corev1.Namespace) {
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

				// Wait for PVC to be created (data PVC has same name as ChainNode)
				WaitForPVC(ns.Name, chainNode.Name)

				// Verify PVC properties
				dataPvc := GetPVC(ns.Name, chainNode.Name)
				// Default size is 50Gi (as defined in the API defaults)
				Expect(dataPvc.Spec.Resources.Requests.Storage().String()).To(Equal(appsv1.DefaultPersistenceSize))
			}),
		)

		It("should create PVC with specified size",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Persistence: &appsv1.Persistence{
							Size: ptr.To("200Gi"),
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for PVC to be created
				WaitForPVC(ns.Name, chainNode.Name)

				// Verify PVC size
				dataPvc := GetPVC(ns.Name, chainNode.Name)
				Expect(dataPvc.Spec.Resources.Requests.Storage().String()).To(Equal("200Gi"))
			}),
		)

		It("should create PVC with specified storage class",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Persistence: &appsv1.Persistence{
							StorageClassName: ptr.To("fast-ssd"),
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for PVC to be created
				WaitForPVC(ns.Name, chainNode.Name)

				// Verify PVC storage class
				dataPvc := GetPVC(ns.Name, chainNode.Name)
				Expect(dataPvc.Spec.StorageClassName).NotTo(BeNil())
				Expect(*dataPvc.Spec.StorageClassName).To(Equal("fast-ssd"))
			}),
		)

		It("should create additional volumes when specified",
			WithNamespace(func(ns *corev1.Namespace) {
				chainNode := &appsv1.ChainNode{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: ChainNodePrefix,
						Namespace:    ns.Name,
					},
					Spec: appsv1.ChainNodeSpec{
						App:     DefaultChainNodeTestApp,
						Genesis: &appsv1.GenesisConfig{Url: ptr.To("https://example.com/genesis.json")},
						Persistence: &appsv1.Persistence{
							AdditionalVolumes: []appsv1.VolumeSpec{
								{
									Name: "wasm",
									Size: "10Gi",
									Path: "/home/app/wasm",
								},
							},
						},
					},
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for additional volume PVC to be created (name is {chainnode-name}-{volume-name})
				wasmPvcName := fmt.Sprintf("%s-wasm", chainNode.Name)
				Eventually(func() error {
					pvc := &corev1.PersistentVolumeClaim{}
					return Framework().Client().Get(Framework().Context(), client.ObjectKey{
						Namespace: ns.Name,
						Name:      wasmPvcName,
					}, pvc)
				}).Should(Succeed())

				// Verify wasm PVC properties
				wasmPvc := GetPVC(ns.Name, wasmPvcName)
				Expect(wasmPvc.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))
			}),
		)
	})
})
