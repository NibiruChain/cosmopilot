package chainnodeset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/voluzi/cosmopilot/api/v1"
	"github.com/voluzi/cosmopilot/internal/chainutils"
	"github.com/voluzi/cosmopilot/internal/chainutils/sdkcmd"
	"github.com/voluzi/cosmopilot/internal/controllers"
)

// Reconciler reconciles a ChainNode object
type Reconciler struct {
	client.Client
	ClientSet  *kubernetes.Clientset
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
	recorder   record.EventRecorder
	opts       *controllers.ControllerRunOptions
}

func New(mgr ctrl.Manager, clientSet *kubernetes.Clientset, opts *controllers.ControllerRunOptions) (*Reconciler, error) {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		ClientSet:  clientSet,
		RestConfig: mgr.GetConfig(),
		Scheme:     mgr.GetScheme(),
		recorder:   mgr.GetEventRecorderFor("chainnodeset-controller"),
		opts:       opts,
	}
	if err := r.setupWithManager(mgr); err != nil {
		return nil, err
	}
	return r, nil
}

//+kubebuilder:rbac:groups=cosmopilot.voluzi.com,resources=chainnodesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cosmopilot.voluzi.com,resources=chainnodesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cosmopilot.voluzi.com,resources=chainnodesets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

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

	// Check if namespace is being terminated - if so, skip reconcile to avoid errors
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Namespace}, ns); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if ns.DeletionTimestamp != nil {
		logger.V(1).Info("namespace is being terminated, skipping reconcile")
		return ctrl.Result{}, nil
	}

	if nodeSet.Labels[controllers.LabelWorkerName] != r.opts.WorkerName {
		logger.V(1).Info("skipping chainnodeset due to worker-name mismatch.")
		return ctrl.Result{}, nil
	}

	if r.opts.DisableWebhooks {
		warnings, err := nodeSet.Validate(nil)
		if err != nil {
			logger.Error(err, "spec is invalid")
			r.recorder.Eventf(nodeSet,
				corev1.EventTypeWarning,
				appsv1.ReasonInvalid,
				"spec is invalid: %v",
				err,
			)
			return ctrl.Result{}, err
		}
		if len(warnings) > 0 {
			logger.Error(nil, "validation warnings", "warnings", warnings)
		}
	}

	// Clearly log beginning and end of reconcile cycle
	logger.Info("starting reconcile")
	defer logger.Info("finishing reconcile")

	app, err := chainutils.NewApp(r.ClientSet, r.Scheme, r.RestConfig, nodeSet,
		nodeSet.Spec.App.GetSdkVersion(),
		[]sdkcmd.Option{sdkcmd.WithGenesisSubcommand(nodeSet.Spec.App.UseGenesisSubcommand())},
		chainutils.WithImage(nodeSet.Spec.App.GetImage()),
		chainutils.WithImagePullPolicy(nodeSet.Spec.App.ImagePullPolicy),
		chainutils.WithBinary(nodeSet.Spec.App.App),
		chainutils.WithPriorityClass(r.opts.GetDefaultPriorityClassName()),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if nodeSet.Status.ChainID == "" {
		if err := r.updatePhase(ctx, nodeSet, appsv1.PhaseChainNodeSetInitializing); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Start validator first if we are initializing a new genesis
	if nodeSet.Status.Phase == appsv1.PhaseChainNodeSetRunning || nodeSet.ShouldInitGenesis() {
		if err := r.ensureValidator(ctx, nodeSet); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.ensureGenesis(ctx, app, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureNodes(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureSeedNodes(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureUpgrades(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureServices(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureIngresses(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensurePodDisruptionBudgets(ctx, nodeSet); err != nil {
		return ctrl.Result{}, err
	}

	if nodeSet.Status.Phase != appsv1.PhaseChainNodeSetRunning || nodeSet.GetLastUpgradeVersion() != nodeSet.Status.AppVersion {
		log.FromContext(ctx).Info("updating .status.appVersion", "version", nodeSet.GetLastUpgradeVersion())
		nodeSet.Status.AppVersion = nodeSet.GetLastUpgradeVersion()
		return ctrl.Result{}, r.updatePhase(ctx, nodeSet, appsv1.PhaseChainNodeSetRunning)
	}
	return ctrl.Result{RequeueAfter: appsv1.DefaultReconcilePeriod}, nil
}

func (r *Reconciler) updatePhase(ctx context.Context, nodeSet *appsv1.ChainNodeSet, phase appsv1.ChainNodeSetPhase) error {
	if nodeSet.Status.Phase == phase {
		return nil
	}
	log.FromContext(ctx).Info("updating .status.phase", "phase", phase)
	nodeSet.Status.Phase = phase
	return r.Status().Update(ctx, nodeSet)
}

// setupWithManager sets up the controller with the Manager.
func (r *Reconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ChainNodeSet{}).
		Owns(&appsv1.ChainNode{}).
		WithEventFilter(GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.opts.WorkerCount}).
		Complete(r)
}
