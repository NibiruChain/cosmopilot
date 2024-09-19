package chainnode

import (
	"math/rand"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

const (
	ChainNodePrefix = "spo-chainnode-e2e-"
	letterBytes     = "abcdefghijklmnopqrstuvwxyz"
)

var (
	Nibiru_v1_0_0 = appsv1.AppSpec{
		Image:   "ghcr.io/nibiruchain/nibiru",
		Version: pointer.String("1.0.0"),
		App:     "nibid",
	}
)

func GetRandomChainNodeName() string {
	return ChainNodePrefix + RandString(6)
}

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

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
