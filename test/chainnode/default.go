package chainnode

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

const (
	ChainNodePrefix = "spo-nodeset-e2e-"
)

var (
	NibiruApp = appsv1.AppSpec{
		Image:   "ghcr.io/nibiruchain/nibiru",
		Version: pointer.String("1.0.0"),
		App:     "nibid",
	}
)

func NewChainNodeBasic(ns *corev1.Namespace, app appsv1.AppSpec) *appsv1.ChainNode {
	return &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: ChainNodePrefix,
			Namespace:    ns.Name,
		},
		Spec: appsv1.ChainNodeSpec{
			App: app,
		},
	}
}
