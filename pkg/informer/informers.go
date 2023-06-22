package informer

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
)

const (
	ChainNodeType    = "chainnodes"
	ChainNodeSetType = "chainnodesets"
)

func GetChainNodesInformer(cfg *rest.Config) (informers.GenericInformer, error) {
	return getInformer(cfg, ChainNodeType)
}

func GetChainNodeSetsInformer(cfg *rest.Config) (informers.GenericInformer, error) {
	return getInformer(cfg, ChainNodeSetType)
}

func getInformer(cfg *rest.Config, resourceType string) (informers.GenericInformer, error) {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc, 0, corev1.NamespaceAll, nil)
	return factory.ForResource(schema.GroupVersionResource{
		Group:    appsv1.GroupVersion.Group,
		Version:  appsv1.GroupVersion.Version,
		Resource: resourceType,
	}), nil
}
