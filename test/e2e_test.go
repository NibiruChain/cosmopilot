package test

import (
	"flag"
	"testing"
	"time"

	"github.com/NibiruChain/cosmopilot/internal/environ"
)

func init() {
	flag.DurationVar(&eventuallyTimeout,
		"eventually-timeout",
		environ.GetDuration("EVENTUALLY_TIMEOUT", 5*time.Minute),
		"sets the default timeout duration for eventually",
	)

	flag.StringVar(&certsDir,
		"certs-dir",
		environ.GetString("CERTS_DIR", "/tmp/k8s-webhook-server/serving-certs"),
		"directory containing the certificates to be used by webhook server",
	)

	flag.StringVar(&issuerName,
		"cert-issuer-name",
		environ.GetString("CERT_ISSUER_NAME", "spo-e2e"),
		"the name of the cert-manager cluster issuer to be used in the test suite",
	)

	flag.IntVar(&workerCount,
		"worker-count",
		environ.GetInt("WORKER_COUNT", 1),
		"the number of workers per controller",
	)

	flag.StringVar(&nodeUtilsImage,
		"nodeutils-image",
		environ.GetString("NODE_UTILS_IMAGE", "ghcr.io/nibiruchain/node-utils"),
		"nodeutils image to be deployed with nodes.",
	)
}

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
