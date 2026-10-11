package main

import (
	"flag"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/pkg/environ"
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
		environ.GetString("NODE_UTILS_IMAGE", "ghcr.io/voluzi/node-utils"),
		"nodeutils image to be deployed with nodes.",
	)

	flag.StringVar(&runOpts.CosmoGuardImage, "cosmoguard-image",
		environ.GetString("COSMOGUARD_IMAGE", "ghcr.io/voluzi/cosmoguard"),
		"cosmoguard image to be deployed with nodes when enabled.",
	)

	flag.StringVar(&runOpts.CosmoseedImage, "cosmoseed-image",
		environ.GetString("COSMOSEED_IMAGE", "ghcr.io/voluzi/cosmoseed"),
		"image to be used in cosmoseed deployments when enabled.",
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

	flag.StringVar(&certsDir, "certs-dir",
		environ.GetString("CERTS_DIR", ""),
		"directory where manager should look for certificates for serving webhooks",
	)

	flag.StringVar(&runOpts.ReleaseName, "release-name",
		environ.GetString("RELEASE_NAME", "cosmopilot"),
		"the release-name passed in helm (used to get PriorityClass names to assign to pods)",
	)

	flag.BoolVar(&runOpts.DisruptionCheckEnabled, "disruption-checks-enabled",
		environ.GetBool("DISRUPTION_CHECKS_ENABLED", true),
		"whether to enable pod disruption checks.",
	)

	flag.IntVar(&runOpts.DisruptionMaxUnavailable, "disruption-max-unavailable",
		environ.GetInt("DISRUPTION_MAX_UNAVAILABLE", 1),
		"maximum number of unavailable pods with same labels.",
	)
}
