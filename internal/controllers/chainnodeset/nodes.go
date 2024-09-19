package chainnodeset

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

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

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/internal/utils"
	"github.com/NibiruChain/cosmopilot/pkg/informer"
)

func (r *Reconciler) ensureNodes(ctx context.Context, nodeSet *appsv1.ChainNodeSet) error {
	logger := log.FromContext(ctx)

	if nodeSet.Status.ChainID == "" {
		// let's wait for chainID to be available so we have a genesis to proceed
		return nil
	}

	totalInstances := 0
	nodeSetCopy := nodeSet.DeepCopy()

	if nodeSet.HasValidator() {
		totalInstances += 1
	}

	// Grab list of all nodeset nodes
	chainNodes, err := r.listNodeSetNodes(ctx, nodeSet)
	if err != nil {
		return err
	}

	// Create map of group nodes to track deleted ones
	groupList := map[string]int{}
	for _, node := range chainNodes.Items {
		if group, ok := node.Labels[LabelChainNodeSetGroup]; ok {
			groupList[group]++
		}
	}
	// Exclude validator group
	delete(groupList, validatorGroupName)

	for _, group := range nodeSet.Spec.Nodes {
		if err := r.ensureNodeGroup(ctx, nodeSet, group); err != nil {
			return err
		}
		totalInstances += group.GetInstances()
		delete(groupList, group.Name)
	}

	// Remove nodes from deleted groups
	for group, count := range groupList {
		for i := 0; i < count; i++ {
			nodeName := fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group, i)
			logger.Info("removing chainnode", "group", group, "chainnode", nodeName)
			if err := r.removeNode(ctx, nodeSet, group, i); err != nil {
				return err
			}
		}
	}

	if !reflect.DeepEqual(nodeSet.Status, nodeSetCopy.Status) {
		log.FromContext(ctx).Info("updating .status.instances", "instances", totalInstances)
		nodeSet.Status.Instances = totalInstances
		return r.Status().Update(ctx, nodeSet)
	}
	return nil
}

func (r *Reconciler) listNodeSetNodes(ctx context.Context, nodeSet *appsv1.ChainNodeSet, l ...string) (*appsv1.ChainNodeList, error) {
	if len(l)%2 != 0 {
		return nil, fmt.Errorf("list of labels must contain pairs of key-value")
	}

	selectorMap := map[string]string{LabelChainNodeSet: nodeSet.GetName()}
	for i := 0; i < len(l); i += 2 {
		selectorMap[l[i]] = l[i+1]
	}

	chainNodeList := &appsv1.ChainNodeList{}
	return chainNodeList, r.List(ctx, chainNodeList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(selectorMap),
	})
}

func (r *Reconciler) ensureNodeGroup(ctx context.Context, nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec) error {
	logger := log.FromContext(ctx)

	chainNodeList, err := r.listNodeSetNodes(ctx, nodeSet, LabelChainNodeSetGroup, group.Name)
	if err != nil {
		return err
	}

	currentSize := len(chainNodeList.Items)
	desiredSize := group.GetInstances()

	// Remove ChainNodes if necessary
	for i := currentSize - 1; i >= desiredSize; i-- {
		nodeName := fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group.Name, i)
		logger.Info("removing chainnode", "group", group.Name, "chainnode", nodeName)
		if err := r.removeNode(ctx, nodeSet, group.Name, i); err != nil {
			return err
		}
	}

	for i := 0; i < desiredSize; i++ {
		node, err := r.getNodeSpec(nodeSet, group, i)
		if err != nil {
			return err
		}
		if err := r.ensureNode(ctx, nodeSet, node, nodeSet.RollingUpdatesEnabled()); err != nil {
			return err
		}

		nodeStatus := appsv1.ChainNodeSetNodeStatus{
			Name:    node.Name,
			ID:      node.Status.NodeID,
			Address: node.Status.IP,
			Port:    chainutils.P2pPort,
			Seed:    node.Status.SeedMode,
			Group:   group.Name,
		}
		if node.Status.PublicAddress != "" {
			if parts := strings.Split(node.Status.PublicAddress, ":"); len(parts) == 2 {
				publicPort, err := strconv.Atoi(parts[1])
				if err != nil {
					return err
				}
				if parts = strings.Split(parts[0], "@"); len(parts) == 2 {
					nodeStatus.Public = true
					nodeStatus.PublicPort = publicPort
					nodeStatus.PublicAddress = parts[1]
				}
			}
		}
		AddOrUpdateNodeStatus(nodeSet, nodeStatus)
	}
	return nil
}

func (r *Reconciler) ensureNode(ctx context.Context, nodeSet *appsv1.ChainNodeSet, node *appsv1.ChainNode, waitRunning bool) error {
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
			if waitRunning {
				logger.V(1).Info("waiting for chainnode", "chainnode", node.GetName())
				return r.waitChainNodeRunningOrSyncing(node)
			}
			return nil
		}
		return err
	}

	if !currentNode.Equal(node) {
		logger.Info("updating chainnode", "chainnode", node.GetName())
		node.ObjectMeta.ResourceVersion = currentNode.ObjectMeta.ResourceVersion
		node.Annotations = currentNode.Annotations
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

	if waitRunning {
		logger.V(1).Info("waiting for chainnode", "chainnode", node.GetName())
		return r.waitChainNodeRunningOrSyncing(node)
	}
	return nil
}

func (r *Reconciler) removeNode(ctx context.Context, nodeSet *appsv1.ChainNodeSet, group string, index int) error {
	nodeName := fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group, index)
	if err := r.Delete(ctx, &appsv1.ChainNode{ObjectMeta: metav1.ObjectMeta{Name: nodeName, Namespace: nodeSet.GetNamespace()}}); err != nil {
		return err
	}
	DeleteNodeStatus(nodeSet, nodeName)

	r.recorder.Eventf(nodeSet,
		corev1.EventTypeNormal,
		appsv1.ReasonNodeDeleted,
		"ChainNode %s removed",
		nodeName,
	)

	return nil
}

func (r *Reconciler) getNodeSpec(nodeSet *appsv1.ChainNodeSet, group appsv1.NodeGroupSpec, index int) (*appsv1.ChainNode, error) {
	var genesisConfig *appsv1.GenesisConfig
	if nodeSet.Spec.Genesis.ShouldDownloadUsingContainer() {
		genesisConfig = nodeSet.Spec.Genesis
	} else {
		genesisConfig = &appsv1.GenesisConfig{
			ConfigMap: pointer.String(fmt.Sprintf("%s-genesis", nodeSet.Status.ChainID)),
		}
	}

	node := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%d", nodeSet.GetName(), group.Name, index),
			Namespace: nodeSet.GetNamespace(),
			Labels: WithChainNodeSetLabels(nodeSet, map[string]string{
				LabelChainNodeSet:      nodeSet.GetName(),
				LabelChainNodeSetGroup: group.Name,
			}),
		},
		Spec: appsv1.ChainNodeSpec{
			Genesis:          genesisConfig,
			App:              nodeSet.GetAppSpecWithUpgrades(),
			Config:           group.Config,
			Persistence:      group.Persistence,
			Peers:            group.Peers,
			Expose:           group.Expose,
			Resources:        group.Resources,
			Affinity:         group.Affinity,
			NodeSelector:     group.NodeSelector,
			StateSyncRestore: group.StateSyncRestore,
		},
	}

	if nodeSet.HasValidator() && group.ShouldInheritValidatorGasPrice() {
		price := nodeSet.GetValidatorMinimumGasPrices()
		if price != "" {
			data := map[string]string{
				controllers.MinimumGasPricesKey: price,
			}
			dataBytes, _ := json.Marshal(data)

			if node.Spec.Config == nil {
				node.Spec.Config = &appsv1.Config{
					Override: &map[string]runtime.RawExtension{
						controllers.AppTomlFile: {Raw: dataBytes},
					},
				}
			} else if node.Spec.Config.Override == nil {
				node.Spec.Config.Override = &map[string]runtime.RawExtension{
					controllers.AppTomlFile: {Raw: dataBytes},
				}
			} else {
				cfg := *node.Spec.Config.Override
				var cfgData map[string]interface{}
				if err := json.Unmarshal(cfg[controllers.AppTomlFile].Raw, &cfgData); err != nil {
					return nil, err
				}

				cfgData[controllers.MinimumGasPricesKey] = price
				newDataBytes, _ := json.Marshal(cfgData)
				cfg[controllers.AppTomlFile] = runtime.RawExtension{Raw: newDataBytes}
			}
		}
	}

	globalIngressLabels := map[string]string{}
	for _, ingress := range nodeSet.Spec.Ingresses {
		if ingress.HasGroup(group.Name) {
			globalIngressLabels[ingress.GetName(nodeSet)] = strconv.FormatBool(true)
		}
	}

	if len(globalIngressLabels) > 0 {
		node.Labels = utils.MergeMaps(node.Labels, globalIngressLabels)
	}

	// When enabling snapshots on a group, lets do it only for the first node of the group.
	// We will also name it after the group, and not the individual node.
	if index > 0 && group.Persistence != nil {
		group.Persistence.Snapshots = nil
	}

	setChainNodeServiceMonitor(nodeSet, node)
	return node, controllerutil.SetControllerReference(nodeSet, node, r.Scheme)
}

func (r *Reconciler) waitChainNodeRunningOrSyncing(node *appsv1.ChainNode) error {
	if node.Status.Phase == appsv1.PhaseChainNodeRunning {
		return nil
	}

	validateFunc := func(chainNode *appsv1.ChainNode) bool {
		return chainNode.Status.Phase == appsv1.PhaseChainNodeRunning ||
			chainNode.Status.Phase == appsv1.PhaseChainNodeSyncing ||
			chainNode.Status.Phase == appsv1.PhaseChainNodeSnapshotting
	}

	nodesInformer, err := informer.GetChainNodesInformer(r.RestConfig)
	if err != nil {
		return err
	}

	var exitErr error
	stopCh := make(chan struct{})
	once := sync.Once{}

	go func() {
		time.Sleep(ChainNodeWaitTimeout)
		exitErr = fmt.Errorf("timeout waiting for chainnode %s running or syncing status", node.GetName())
		once.Do(func() { close(stopCh) })
	}()

	if _, err := nodesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			chainNode := &appsv1.ChainNode{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), chainNode); err != nil {
				exitErr = fmt.Errorf("error casting object to chainnode")
				once.Do(func() { close(stopCh) })
			}
			if chainNode.GetName() == node.GetName() && chainNode.GetNamespace() == node.GetNamespace() {
				*node = *chainNode
				if validateFunc(node) {
					once.Do(func() { close(stopCh) })
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			chainNode := &appsv1.ChainNode{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(newObj.(*unstructured.Unstructured).UnstructuredContent(), chainNode); err != nil {
				exitErr = fmt.Errorf("error casting object to chainnode")
				once.Do(func() { close(stopCh) })
			}
			if chainNode.GetName() == node.GetName() && chainNode.GetNamespace() == node.GetNamespace() {
				*node = *chainNode
				if validateFunc(node) {
					once.Do(func() { close(stopCh) })
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			chainNode := &appsv1.ChainNode{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), chainNode); err != nil {
				exitErr = fmt.Errorf("error casting object to chainnode")
				once.Do(func() { close(stopCh) })
			}
			if chainNode.GetName() == node.GetName() && chainNode.GetNamespace() == node.GetNamespace() {
				exitErr = fmt.Errorf("chainnode was deleted")
				once.Do(func() { close(stopCh) })
			}
		},
	}); err != nil {
		return err
	}
	nodesInformer.Informer().Run(stopCh)
	return exitErr
}

func (r *Reconciler) maybeDeleteNode(ctx context.Context, nodeSet *appsv1.ChainNodeSet, name string) error {
	node := &appsv1.ChainNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodeSet.GetNamespace(),
		},
	}
	if err := r.Delete(ctx, node); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	DeleteNodeStatus(nodeSet, name)
	return nil
}
