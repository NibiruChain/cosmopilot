package chainnodeset

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

const (
	ChainNodeSetPrefix = "spo-chainnodeset-e2e-"
)

var (
	Nibiru_v1_0_0 = appsv1.AppSpec{
		Image:   "ghcr.io/nibiruchain/nibiru",
		Version: pointer.String("1.0.0"),
		App:     "nibid",
	}
)

func NewChainNodeSetBasic(ns *corev1.Namespace, app appsv1.AppSpec) *appsv1.ChainNodeSet {
	return &appsv1.ChainNodeSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: ChainNodeSetPrefix,
			Namespace:    ns.Name,
		},
		Spec: appsv1.ChainNodeSetSpec{
			App: app,
		},
	}
}
