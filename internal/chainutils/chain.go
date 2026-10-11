package chainutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/internal/chainutils/sdkcmd"
)

type App struct {
	client     *kubernetes.Clientset
	scheme     *runtime.Scheme
	restConfig *rest.Config
	cmd        sdkcmd.SDK
	owner      metav1.Object
	sdkVersion appsv1.SdkVersion

	binary            string
	image             string
	pullPolicy        corev1.PullPolicy
	priorityClassName string
	NodeSelector      map[string]string
	Affinity          *corev1.Affinity
}

type Params struct {
	ChainID                 string
	Assets                  []string
	StakeAmount             string
	Accounts                []AccountAssets
	UnbondingTime           string
	VotingPeriod            string
	ExpeditedVotingPeriod   string
	CommissionMaxChangeRate string
	CommissionMaxRate       string
	CommissionRate          string
	MinSelfDelegation       *string
	GasPrices               string
}

type NodeInfo struct {
	Moniker  string
	Details  *string
	Website  *string
	Identity *string
}

type AccountAssets struct {
	Address string
	Assets  []string
}

type InitCommand struct {
	Image     string
	Command   []string
	Args      []string
	Resources corev1.ResourceRequirements
	Env       []corev1.EnvVar
}

type AdditionalVolume struct {
	Name    string // Volume name
	PVCName string // Full PVC name (nodeName-volumeName)
	Path    string // Mount path
}

func NewApp(client *kubernetes.Clientset, scheme *runtime.Scheme, cfg *rest.Config,
	owner metav1.Object, sdkVersion appsv1.SdkVersion, sdkOpts []sdkcmd.Option, options ...Option) (*App, error) {
	allSdkOpts := append([]sdkcmd.Option{sdkcmd.WithGlobalArg(sdkcmd.Home, defaultHome)}, sdkOpts...)
	cmd, err := sdkcmd.GetSDK(sdkVersion, allSdkOpts...)
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

func WithPriorityClass(name string) Option {
	return func(c *App) {
		c.priorityClassName = name
	}
}

func WithAffinityConfig(affinity *corev1.Affinity) Option {
	return func(c *App) {
		c.Affinity = affinity
	}
}

func WithNodeSelector(selector map[string]string) Option {
	return func(c *App) {
		c.NodeSelector = selector
	}
}
