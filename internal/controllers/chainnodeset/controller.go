package chainnodeset

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
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
)

// Reconciler reconciles a ChainNode object
type Reconciler struct {
	client.Client
	ClientSet  *kubernetes.Clientset
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
	recorder   record.EventRecorder
}

func New(mgr ctrl.Manager, clientSet *kubernetes.Clientset) (*Reconciler, error) {
	cfgCache := ttlcache.New(
		ttlcache.WithTTL[string, map[string]interface{}](24 * time.Hour),
	)

	r := &Reconciler{
		Client:     mgr.GetClient(),
		ClientSet:  clientSet,
		RestConfig: mgr.GetConfig(),
		Scheme:     mgr.GetScheme(),
		recorder:   mgr.GetEventRecorderFor("chainnodeset-controller"),
	}
	if err := r.setupWithManager(mgr); err != nil {
		return nil, err
	}
	go cfgCache.Start()
	return r, nil
}

//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodesets;chainnodes;ingresses;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodesets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods/exec;pods/attach,verbs=create
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	nodeSet := &appsv1.ChainNodeSet{}
	if err := r.Get(ctx, req.NamespacedName, nodeSet); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch chainnodeset")
		return ctrl.Result{}, err
	}

	logger.Info("starting reconcile")

	if nodeSet.Status.ChainID == "" {
		if err := r.updatePhase(ctx, nodeSet, appsv1.PhaseChainNodeSetInitialing); err != nil {
			return ctrl.Result{}, err
		}
	}

	if nodeSet.HasValidator() {
		if err := r.ensureValidator(ctx, nodeSet); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.ensureNodes(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureServices(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureIngresses(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if nodeSet.Status.Phase != appsv1.PhaseChainNodeSetRunning {
		nodeSet.Status.Phase = appsv1.PhaseChainNodeSetRunning
		return ctrl.Result{}, r.Status().Update(ctx, nodeSet)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) updatePhase(ctx context.Context, nodeSet *appsv1.ChainNodeSet, phase appsv1.ChainNodeSetPhase) error {
	nodeSet.Status.Phase = phase
	return r.Status().Update(ctx, nodeSet)
}

// setupWithManager sets up the controller with the Manager.
func (r *Reconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ChainNodeSet{}).
		Watches(&source.Kind{Type: &appsv1.ChainNode{}}, &handler.EnqueueRequestForOwner{OwnerType: &appsv1.ChainNodeSet{}}).
		WithEventFilter(GenerationChangedPredicate{}).
		Complete(r)
}
