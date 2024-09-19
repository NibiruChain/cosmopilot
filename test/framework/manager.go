package framework

import (
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
	"github.com/NibiruChain/cosmopilot/internal/controllers/chainnode"
	"github.com/NibiruChain/cosmopilot/internal/controllers/chainnodeset"
)

const (
	webhookServerMetricsBindAddress = "localhost:8080"
	webhookServerHost               = "127.0.0.1"
	webhookServerPort               = 9443
)

func (tf *TestFramework) RunManager() error {
	mgr, err := ctrl.NewManager(tf.RestCfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: webhookServerMetricsBindAddress,
		LeaderElection:     false,
		Host:               webhookServerHost,
		Port:               webhookServerPort,
		CertDir:            tf.Cfg.CertsDir,
	})
	if err != nil {
		return err
	}

	runOpts := controllers.ControllerRunOptions{
		WorkerCount:     tf.Cfg.WorkerCount,
		NodeUtilsImage:  tf.Cfg.NodeUtilsImg,
		DisableWebhooks: false,
	}

	if _, err = chainnode.New(mgr, tf.KubeClient, &runOpts); err != nil {
		return err
	}

	if _, err = chainnodeset.New(mgr, tf.KubeClient, &runOpts); err != nil {
		return err
	}

	if err := appsv1.SetupChainNodeValidationWebhook(mgr); err != nil {
		return err
	}

	if err := appsv1.SetupChainNodeSetValidationWebhook(mgr); err != nil {
		return err
	}

	return mgr.Start(tf.Context())
}
