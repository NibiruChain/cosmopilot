package chainnode

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	ClientSet   *kubernetes.Clientset
	RestConfig  *rest.Config
	Scheme      *runtime.Scheme
	configCache *ttlcache.Cache[string, map[string]interface{}]
}

func NewReconciler(client client.Client, clientSet *kubernetes.Clientset, cfg *rest.Config, scheme *runtime.Scheme) *Reconciler {
	cfgCache := ttlcache.New(
		ttlcache.WithTTL[string, map[string]interface{}](24 * time.Hour),
	)
	go cfgCache.Start()
	return &Reconciler{
		Client:      client,
		ClientSet:   clientSet,
		RestConfig:  cfg,
		Scheme:      scheme,
		configCache: cfgCache,
	}
}

//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes;pods;persistentvolumeclaims;configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods/exec;pods/attach,verbs=create
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get

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

	// Create/update service for this node
	if err := r.ensureService(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Create/update configmap with config files
	configHash, err := r.ensureConfig(ctx, app, chainNode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create/update PVC
	if err := r.ensurePersistence(ctx, app, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure pod is running
	if err := r.ensurePod(ctx, chainNode, configHash); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ChainNode{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNode{}}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNode{}}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNode{}}).
		WithEventFilter(GenerationChangedPredicate{}).
		Complete(r)
}
