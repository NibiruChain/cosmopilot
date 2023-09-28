package chainutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils/sdkcmd"
)

type App struct {
	client     *kubernetes.Clientset
	scheme     *runtime.Scheme
	restConfig *rest.Config
	cmd        sdkcmd.SDK

	owner      metav1.Object
	binary     string
	image      string
	pullPolicy corev1.PullPolicy
	sdkVersion appsv1.SdkVersion
}

func NewApp(client *kubernetes.Clientset, scheme *runtime.Scheme, cfg *rest.Config, owner metav1.Object, sdkVersion appsv1.SdkVersion, options ...Option) (*App, error) {
	cmd, err := sdkcmd.GetSDK(sdkVersion, sdkcmd.WithGlobalArg(sdkcmd.Home, defaultHome))
	if err != nil {
		return nil, err
	}
	app := &App{
		client:     client,
		owner:      owner,
		scheme:     scheme,
		restConfig: cfg,
		cmd:        cmd,
		sdkVersion: sdkVersion,
	}
	applyOptions(app, options)
	return app, nil
}

type Option func(*App)

func applyOptions(c *App, options []Option) {
	for _, option := range options {
		option(c)
	}
}

func WithBinary(name string) Option {
	return func(c *App) {
		c.binary = name
	}
}

func WithImage(image string) Option {
	return func(c *App) {
		c.image = image
	}
}

func WithImagePullPolicy(p corev1.PullPolicy) Option {
	return func(c *App) {
		c.pullPolicy = p
	}
}
