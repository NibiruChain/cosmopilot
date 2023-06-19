package chainnode

import (
	"context"
	"fmt"
	"time"

	"github.com/jellydator/ttlcache/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
)

// Reconciler reconciles a ChainNode object
type Reconciler struct {
	client.Client
	ClientSet      *kubernetes.Clientset
	RestConfig     *rest.Config
	Scheme         *runtime.Scheme
	configCache    *ttlcache.Cache[string, map[string]interface{}]
	nodeUtilsImage string
	queryClients   map[string]*chainutils.QueryClient
	recorder       record.EventRecorder
}

func New(mgr ctrl.Manager, clientSet *kubernetes.Clientset, nodeUtilsImage string) (*Reconciler, error) {
	cfgCache := ttlcache.New(
		ttlcache.WithTTL[string, map[string]interface{}](24 * time.Hour),
	)

	r := &Reconciler{
		Client:         mgr.GetClient(),
		ClientSet:      clientSet,
		RestConfig:     mgr.GetConfig(),
		Scheme:         mgr.GetScheme(),
		configCache:    cfgCache,
		nodeUtilsImage: nodeUtilsImage,
		queryClients:   make(map[string]*chainutils.QueryClient),
		recorder:       mgr.GetEventRecorderFor("chainnode-controller"),
	}
	if err := r.setupWithManager(mgr); err != nil {
		return nil, err
	}
	go cfgCache.Start()
	return r, nil
}

//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes;pods;persistentvolumeclaims;configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods/exec;pods/attach,verbs=create
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	chainNode := &appsv1.ChainNode{}
	if err := r.Get(ctx, req.NamespacedName, chainNode); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch chainnode")
		return ctrl.Result{}, err
	}

	logger.Info("starting reconcile")

	// Create/update secret with node key for this node.
	if err := r.ensureNodeKey(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	app := chainutils.NewApp(r.ClientSet, r.Scheme, r.RestConfig, chainNode,
		chainutils.WithImage(chainNode.GetImage()),
		chainutils.WithImagePullPolicy(chainNode.Spec.App.ImagePullPolicy),
		chainutils.WithBinary(chainNode.Spec.App.App),
	)

	// Create a private key for signing and an account for this node if it is a validator
	if chainNode.IsValidator() {
		if err := r.ensureSigningKey(ctx, chainNode); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.ensureAccount(ctx, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get or initialize a genesis
	if err := r.ensureGenesis(ctx, app, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Create/update services for this node
	if err := r.ensureServices(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Create/update configmap with config files
	configHash, err := r.ensureConfig(ctx, app, chainNode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// If we don't have a PVC yet, lets create it before deploying the pod. But for any updates to the PVC
	// we want to do it after the pod deployment because auto-resize feature requires the node running.
	if chainNode.Status.PvcSize == "" {
		if err := r.ensurePersistence(ctx, app, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Ensure pod is running
	if err := r.ensurePod(ctx, chainNode, configHash); err != nil {
		return ctrl.Result{}, err
	}

	// For updating the PVC we want to do it after the pod deployment because auto-resize feature requires the node running.
	if chainNode.Status.PvcSize != "" {
		if err := r.ensurePersistence(ctx, app, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Wait for node to be synced before continuing
	if chainNode.Status.Phase == appsv1.PhaseSyncing {
		return ctrl.Result{RequeueAfter: chainNode.GetReconcilePeriod()}, nil
	}

	// Update jailed status
	if chainNode.IsValidator() {
		if err := r.updateJailedStatus(ctx, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: chainNode.GetReconcilePeriod()}, nil
}

func (r *Reconciler) updatePhase(ctx context.Context, chainNode *appsv1.ChainNode, phase appsv1.ChainNodePhase) error {
	chainNode.Status.Phase = phase
	return r.Status().Update(ctx, chainNode)
}

// setupWithManager sets up the controller with the Manager.
func (r *Reconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ChainNode{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNode{}}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNode{}}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNode{}}).
		WithEventFilter(GenerationChangedPredicate{}).
		Complete(r)
}

func (r *Reconciler) getQueryClient(chainNode *appsv1.ChainNode) (*chainutils.QueryClient, error) {
	address := fmt.Sprintf("%s:%d", chainNode.GetNodeFQDN(), chainutils.GrpcPort)
	if _, ok := r.queryClients[address]; ok {
		return r.queryClients[address], nil
	}
	c, err := chainutils.NewQueryClient(address)
	if err != nil {
		return nil, err
	}
	r.queryClients[address] = c
	return r.queryClients[address], nil
}
