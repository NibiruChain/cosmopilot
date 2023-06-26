package chainnodeset

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/pkg/informer"
)

func (r *Reconciler) ensureNodes(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	if nodeSet.Status.ChainID == "" {
		// let's wait for chainID to be available so we have a genesis to proceed
		return nil
	}

	totalInstances := 0
	if nodeSet.HasValidator() {
		totalInstances += 1
	}
	for _, group := range nodeSet.Spec.Nodes {
		if err := r.ensureNodeGroup(ctx, nodeSet, group); err != nil {
			return err
		}
		totalInstances += group.GetInstances()
	}
	if nodeSet.Status.Instances != totalInstances {
		nodeSet.Status.Instances = totalInstances
		return r.Status().Update(ctx, nodeSet)
	}
	return nil
}

func (r *Reconciler) ensureNodeGroup(ctx context.Context, nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec) error {
	logger := log.FromContext(ctx)

	selector := labels.SelectorFromSet(map[string]string{
		LabelChainNodeSet:      nodeSet.GetName(),
		LabelChainNodeSetGroup: group.Name,
	})
	chainNodeList := &appsv1.ChainNodeList{}
	if err := r.List(ctx, chainNodeList, &client.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return err
	}

	currentSize := len(chainNodeList.Items)
	desiredSize := group.GetInstances()

	// Remove ChainNodes if necessary
	for i := currentSize - 1; i >= desiredSize; i-- {
		nodeName := fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group.Name, i)
		logger.Info("removing chainnode", "chainnode", nodeName)
		if err := r.removeNode(ctx, nodeSet, group, i); err != nil {
			return err
		}
	}

	for i := 0; i < desiredSize; i++ {
		node, err := r.getNodeSpec(nodeSet, group, i)
		if err != nil {
			return err
		}
		if err := r.ensureNode(ctx, nodeSet, node); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) ensureNode(ctx context.Context, nodeSet *appsv1.ChainNodeSet, node *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	currentNode := &appsv1.ChainNode{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(node), currentNode); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating chainnode", "chainnode", node.GetName())
			if err := r.Create(ctx, node); err != nil {
				return err
			}
			r.recorder.Eventf(nodeSet,
				corev1.EventTypeNormal,
				appsv1.ReasonNodeCreated,
				"ChainNode %s created",
				node.GetName(),
			)
			return r.waitChainNodeRunning(node)
		}
		return err
	}

	if !currentNode.Equal(node) {
		logger.Info("updating chainnode", "chainnode", node.GetName())
		node.ObjectMeta.ResourceVersion = currentNode.ObjectMeta.ResourceVersion
		if err := r.Update(ctx, node); err != nil {
			return err
		}
		r.recorder.Eventf(nodeSet,
			corev1.EventTypeNormal,
			appsv1.ReasonNodeUpdated,
			"ChainNode %s updated",
			node.GetName(),
		)
	} else {
		*node = *currentNode
	}

	return r.waitChainNodeRunning(node)
}

func (r *Reconciler) removeNode(ctx context.Context, nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec, index int) error {
	nodeName := fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group.Name, index)
	if err := r.Delete(ctx, &appsv1.ChainNode{ObjectMeta: metav1.ObjectMeta{Name: nodeName, Namespace: nodeSet.GetNamespace()}}); err != nil {
		return err
	}

	r.recorder.Eventf(nodeSet,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeDeleted,
		"ChainNode %s removed",
		nodeName,
	)

	return nil
}

func (r *Reconciler) getNodeSpec(nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec, index int) (*appsv1.ChainNode, error) {
	node := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group.Name, index),
			Namespace: nodeSet.GetNamespace(),
			Labels: map[string]string{
				LabelChainNodeSet:      nodeSet.GetName(),
				LabelChainNodeSetGroup: group.Name,
			},
		},
		Spec: appsv1.ChainNodeSpec{
			Genesis: &appsv1.GenesisConfig{
				ConfigMap: pointer.String(fmt.Sprintf("%s-genesis", nodeSet.Status.ChainID)),
			},
			App:         nodeSet.Spec.App,
			Config:      group.Config,
			Persistence: group.Persistence,
			Peers:       group.Peers,
			Expose:      group.Expose,
		},
	}
	return node, controllerutil.SetControllerReference(nodeSet, node, r.Scheme)
}

func (r *Reconciler) waitChainNodeRunning(node *appsv1.ChainNode) error {
	if node.Status.Phase == appsv1.PhaseChainNodeRunning {
		return nil
	}

	nodesInformer, err := informer.GetChainNodesInformer(r.RestConfig)
	if err != nil {
		return err
	}

	var exitErr error
	stopCh := make(chan struct{})

	if _, err := nodesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			chainNode := &appsv1.ChainNode{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(newObj.(*unstructured.Unstructured).UnstructuredContent(), chainNode); err != nil {
				exitErr = fmt.Errorf("error casting object to chainnode")
				close(stopCh)
			}
			if chainNode.GetName() == node.GetName() && chainNode.GetNamespace() == node.GetNamespace() {
				*node = *chainNode
				if node.Status.Phase == appsv1.PhaseChainNodeRunning {
					close(stopCh)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			chainNode := &appsv1.ChainNode{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), chainNode); err != nil {
				exitErr = fmt.Errorf("error casting object to chainnode")
				close(stopCh)
			}
			if chainNode.GetName() == node.GetName() && chainNode.GetNamespace() == node.GetNamespace() {
				exitErr = fmt.Errorf("chainnode was deleted")
				close(stopCh)
			}
		},
	}); err != nil {
		return err
	}
	nodesInformer.Informer().Run(stopCh)
	return exitErr
}
