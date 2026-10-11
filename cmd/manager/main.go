package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/controllers"
	"github.com/voluzi/cosmopilot/v2/internal/controllers/chainnode"
	"github.com/voluzi/cosmopilot/v2/internal/controllers/chainnodeset"
)

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	runOpts              controllers.ControllerRunOptions
	debugMode            bool
	zapOpts              zap.Options
	certsDir             string
)

func main() {
	flag.Parse()

	zapOpts.Development = debugMode
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	if err := monitoring.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "unable to add prometheus crds to scheme")
		os.Exit(1)
	}

	if err := snapshotv1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "unable to add volumesnapshot crds to scheme")
		os.Exit(1)
	}

	leaderElectionID := fmt.Sprintf("%s.cosmopilot.voluzi.com", runOpts.ReleaseName)
	if runOpts.WorkerName != "" {
		leaderElectionID = fmt.Sprintf("%s.%s.cosmopilot.voluzi.com", runOpts.WorkerName, runOpts.ReleaseName)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       leaderElectionID,
		CertDir:                certsDir,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clientSet, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Fatalf("unable to create clientset: %v", err)
	}

	if _, err = chainnode.New(mgr, clientSet, &runOpts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ChainNode")
		os.Exit(1)
	}

	if _, err = chainnodeset.New(mgr, clientSet, &runOpts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ChainNodeSet")
		os.Exit(1)
	}

	if !runOpts.DisableWebhooks {
		if err := appsv1.SetupChainNodeValidationWebhook(mgr); err != nil {
			setupLog.Error(err, "unable to setup validation webhook", "resource", "ChainNode")
			os.Exit(1)
		}

		if err := appsv1.SetupChainNodeSetValidationWebhook(mgr); err != nil {
			setupLog.Error(err, "unable to setup validation webhook", "resource", "ChainNodeSet")
			os.Exit(1)
		}
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
