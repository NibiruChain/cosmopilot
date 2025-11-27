package framework

import (
	"context"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

var crdsPath = filepath.Join("..", "helm", "cosmopilot", "crds")

type TestFramework struct {
	RestCfg    *rest.Config
	Client     client.Client
	KubeClient *kubernetes.Clientset
	Env        *envtest.Environment
	Cfg        *Configs
	Ctx        context.Context
}

func New(ctx context.Context, cfgs ...Config) (*TestFramework, error) {
	config := defaultConfig()
	for _, cfg := range cfgs {
		cfg(config)
	}

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdsPath},
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    ptr.To(true),
	}

	restConfig, err := testEnv.Start()
	if err != nil {
		return nil, err
	}

	err = appsv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &TestFramework{
		RestCfg:    restConfig,
		Client:     k8sClient,
		KubeClient: clientSet,
		Env:        testEnv,
		Cfg:        config,
		Ctx:        ctx,
	}, nil
}

func (tf *TestFramework) TearDown() error {
	return tf.Env.Stop()
}

func (tf *TestFramework) Context() context.Context {
	return tf.Ctx
}
