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

	flag.BoolVar(&enableLeaderElection, "leader-elect",
		environ.GetBool("LEADER_ELECT", false),
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)

	flag.StringVar(&nodeUtilsImage, "nodeutils-image",
		environ.GetString("NODE_UTILS_IMAGE", "ghcr.io/nibiruchain/node-utils"),
		"nodeutils image to be deployed with nodes.",
	)
}
