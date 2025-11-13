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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/pkg/nodeutils"
)

// Reconciler reconciles a ChainNode object
type Reconciler struct {
	client.Client
	ClientSet       *kubernetes.Clientset
	RestConfig      *rest.Config
	Scheme          *runtime.Scheme
	configCache     *ttlcache.Cache[string, map[string]interface{}]
	nodeClients     *ttlcache.Cache[string, *chainutils.Client]
	recorder        record.EventRecorder
	opts            *controllers.ControllerRunOptions
	disruptionLocks *lockManager
	configLocks     *configLockManager
}

func New(mgr ctrl.Manager, clientSet *kubernetes.Clientset, opts *controllers.ControllerRunOptions) (*Reconciler, error) {
	// Create config cache with capacity limit to prevent unbounded growth
	// 1000 entries should be sufficient for most deployments
	cfgCache := ttlcache.New(
		ttlcache.WithTTL[string, map[string]interface{}](2*time.Hour),
		ttlcache.WithCapacity[string, map[string]interface{}](1000),
	)

	// Create clients cache with capacity limit and eviction callback
	// 100 concurrent connections should be more than enough
	clientsCache := ttlcache.New(
		ttlcache.WithTTL[string, *chainutils.Client](2*time.Hour),
		ttlcache.WithCapacity[string, *chainutils.Client](100),
	)

	// Register eviction callback to properly close gRPC connections
	// This prevents connection leaks when entries are evicted from the cache
	clientsCache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[string, *chainutils.Client]) {
		if item != nil && item.Value() != nil {
			client := item.Value()
			if err := client.Close(); err != nil {
				log.FromContext(ctx).Error(err, "failed to close gRPC client connection during cache eviction", "host", item.Key())
			}
		}
	})

	r := &Reconciler{
		Client:          mgr.GetClient(),
		ClientSet:       clientSet,
		RestConfig:      mgr.GetConfig(),
		Scheme:          mgr.GetScheme(),
		configCache:     cfgCache,
		nodeClients:     clientsCache,
		recorder:        mgr.GetEventRecorderFor("chainnode-controller"),
		opts:            opts,
		disruptionLocks: newLockManager(),
		configLocks:     newConfigLockManager(),
	}
	if err := r.setupWithManager(mgr); err != nil {
		return nil, err
	}
	go cfgCache.Start()
	go clientsCache.Start()
	return r, nil
}

//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.k8s.nibiru.org,resources=chainnodes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims;configmaps;secrets;services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/exec;pods/attach,verbs=create
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;patch;create;update;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;patch;create;update;delete

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

	if chainNode.Labels[controllers.LabelWorkerName] != r.opts.WorkerName {
		logger.V(1).Info("skipping chainnode due to worker-name mismatch.")
		return ctrl.Result{}, nil
	}

	if r.opts.DisableWebhooks {
		warnings, err := chainNode.Validate(nil)
		if err != nil {
			logger.Error(err, "spec is invalid")
			r.recorder.Eventf(chainNode,
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

	// Eventually update seed mode in .status
	chainNode.Status.SeedMode = chainNode.Spec.Config.SeedModeEnabled()

	// Let's check if pod for this node is alive or not so that we can use this information
	// on methods before pod reconciliation
	nodePodRunning, err := r.isChainNodePodRunning(ctx, chainNode)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("pre-checking pod status", "pod-running", nodePodRunning)

	// Create/update secret with node key for this node.
	logger.V(1).Info("ensure node key exists")
	if err := r.ensureNodeKey(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	app, err := chainutils.NewApp(r.ClientSet, r.Scheme, r.RestConfig, chainNode, chainNode.Spec.App.GetSdkVersion(),
		chainutils.WithImage(chainNode.GetAppImage()),
		chainutils.WithImagePullPolicy(chainNode.Spec.App.ImagePullPolicy),
		chainutils.WithBinary(chainNode.Spec.App.App),
		chainutils.WithPriorityClass(r.opts.GetDefaultPriorityClassName()),
		chainutils.WithAffinityConfig(chainNode.Spec.Affinity),
		chainutils.WithNodeSelector(chainNode.Spec.NodeSelector),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if chainNode.RequiresPrivKey() {
		logger.V(1).Info("ensure validator signing key exists")
		if err = r.ensureSigningKey(ctx, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	if chainNode.RequiresAccount() {
		logger.V(1).Info("ensure account exists")
		if err = r.ensureAccount(ctx, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.V(1).Info("ensure data volume")
	pvc, err := r.ensureDataVolume(ctx, app, chainNode)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("ensure additional volumes")
	if err = r.ensureAdditionalVolumes(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// If PVC is being deleted lets wait before trying again.
	if pvc.DeletionTimestamp != nil {
		return ctrl.Result{RequeueAfter: pvcDeletionWaitPeriod}, nil
	}

	// Ensure snapshots are taken if enabled and check if they are ready
	logger.V(1).Info("ensure volume snapshots if applicable")
	if err = r.ensureVolumeSnapshots(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// If the node will be down during snapshot, most methods below will fail.
	if volumeSnapshotInProgress(chainNode) && chainNode.Spec.Persistence.Snapshots.ShouldStopNode() {
		logger.Info("exiting reconcile cycle while snapshot is in progress")
		return ctrl.Result{RequeueAfter: snapshotCheckPeriod}, nil
	}

	// Get or initialize a genesis
	logger.V(1).Info("ensure genesis")
	if err = r.ensureGenesis(ctx, app, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Create/update services for this node
	logger.V(1).Info("ensure services")
	if err = r.ensureServices(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Create/update configmap with config files
	logger.V(1).Info("ensure config")
	configHash, err := r.ensureConfigs(ctx, app, chainNode, nodePodRunning)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create/update upgrades config
	logger.V(1).Info("ensure upgrades")
	if err = r.ensureUpgrades(ctx, chainNode, nodePodRunning); err != nil {
		return ctrl.Result{}, err
	}

	// Deploy TMKMS configs if configured
	logger.V(1).Info("ensure tmkms config if applicable")
	if err = r.ensureTmKMSConfig(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure pod is running
	logger.V(1).Info("ensure pod")
	if err = r.ensurePod(ctx, app, chainNode, configHash); err != nil {
		return ctrl.Result{}, err
	}

	// If the node was set to stop, we will stop here as the pod is not running.
	if chainNode.Status.Phase == appsv1.PhaseChainNodeStopped {
		return ctrl.Result{RequeueAfter: chainNode.GetReconcilePeriod()}, nil
	}

	logger.V(1).Info("ensure ingresses")
	if err = r.ensureIngresses(ctx, chainNode); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("ensure pvc updates")
	if err = r.ensurePvcUpdates(ctx, chainNode, pvc); err != nil {
		return ctrl.Result{}, err
	}

	if chainNode.Status.Phase == appsv1.PhaseChainNodeRunning && chainNode.ShouldCreateValidator() {
		logger.V(1).Info("creating validator tx")
		if err = r.createValidator(ctx, app, chainNode); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	// Update validator status
	if chainNode.Status.Phase == appsv1.PhaseChainNodeRunning && chainNode.IsValidator() {
		logger.V(1).Info("updating validator status")
		if err = r.updateValidatorStatus(ctx, chainNode); err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.Info("finishing reconcile")
	return ctrl.Result{RequeueAfter: chainNode.GetReconcilePeriod()}, nil
}

func (r *Reconciler) updatePhase(ctx context.Context, chainNode *appsv1.ChainNode, phase appsv1.ChainNodePhase) error {
	log.FromContext(ctx).Info("updating .status.phase", "phase", phase)
	chainNode.Status.Phase = phase
	return r.Status().Update(ctx, chainNode)
}

// setupWithManager sets up the controller with the Manager.
func (r *Reconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.ChainNode{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		WithEventFilter(GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.opts.WorkerCount}).
		Complete(r)
}

func (r *Reconciler) getChainNodeClient(chainNode *appsv1.ChainNode) (*chainutils.Client, error) {
	return r.getChainNodeClientByHost(chainNode.GetNodeFQDN())
}

func (r *Reconciler) getChainNodeClientByHost(host string) (*chainutils.Client, error) {
	data := r.nodeClients.Get(host)
	if data != nil {
		return data.Value(), nil
	}
	c, err := chainutils.NewClient(host)
	if err != nil {
		return nil, err
	}
	r.nodeClients.Set(host, c, ttlcache.DefaultTTL)
	return c, nil
}

func (r *Reconciler) updateLatestHeight(ctx context.Context, chainNode *appsv1.ChainNode) error {
	height, err := nodeutils.NewClient(chainNode.GetNodeFQDN()).GetLatestHeight(ctx)
	if err != nil {
		return err
	}
	// If height is 0 then node-utils didn't grab latest height yet, so lets not update it.
	if height == 0 {
		return nil
	}

	// Avoid API call if there is nothing to change
	if height == chainNode.Status.LatestHeight {
		return nil
	}

	chainNode.Status.LatestHeight = height
	return r.Status().Update(ctx, chainNode)
}
