package main

import (
	"flag"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/environ"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))

	flag.StringVar(&metricsAddr, "metrics-bind-address",
		environ.GetString("METRICS_BIND_ADDRESS", ":8080"),
		"The address the metric endpoint binds to.",
	)

	flag.StringVar(&probeAddr, "health-probe-bind-address",
		environ.GetString("HEALTH_PROBE_BIND_ADDRESS", ":8081"),
		"The address the probe endpoint binds to.",
	)

	flag.BoolVar(&enableLeaderElection, "enable-leader-election",
		environ.GetBool("ENABLE_LEADER_ELECTION", false),
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)

	flag.StringVar(&runOpts.NodeUtilsImage, "nodeutils-image",
		environ.GetString("NODE_UTILS_IMAGE", "ghcr.io/nibiruchain/node-utils"),
		"nodeutils image to be deployed with nodes.",
	)

	flag.StringVar(&runOpts.CosmosFirewallImage, "cosmos-firewall-image",
		environ.GetString("COSMOS_FIREWALL_IMAGE", "ghcr.io/nibiruchain/cosmos-firewall"),
		"cosmos-firewall image to be deployed with nodes when firewall is enabled.",
	)

	flag.StringVar(&runOpts.WorkerName, "worker-name",
		environ.GetString("WORKER_NAME", ""),
		"name of the worker, passed in label `worker-name`. Used for limiting resources processed by this operator instance.",
	)

	flag.IntVar(&runOpts.WorkerCount, "worker-count",
		environ.GetInt("WORKER_COUNT", 1),
		"number of maximum concurrent reconciles that can be run.",
	)

	flag.BoolVar(&runOpts.DisableWebhooks, "disable-webhooks",
		environ.GetBool("DISABLE_WEBHOOKS", false),
		"whether to disable admission webhooks.",
	)

	flag.BoolVar(&debugMode, "debug-mode",
		environ.GetBool("DEBUG_MODE", false),
		"whether to enable debug mode",
	)
}
