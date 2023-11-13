package framework

func defaultConfig() *Configs {
	return &Configs{
		CertsDir:     "/tmp/k8s-webhook-server/serving-certs",
		IssuerName:   "no-e2e",
		WorkerCount:  1,
		NodeUtilsImg: "ghcr.io/nibiruchain/node-utils",
	}
}

type Configs struct {
	CertsDir     string
	IssuerName   string
	WorkerCount  int
	NodeUtilsImg string
}

type Config func(*Configs)

func WithCertsDir(s string) Config {
	return func(cfgs *Configs) {
		cfgs.CertsDir = s
	}
}

func WithIssuerName(s string) Config {
	return func(cfgs *Configs) {
		cfgs.IssuerName = s
	}
}

func WithWorkerCount(v int) Config {
	return func(cfgs *Configs) {
		cfgs.WorkerCount = v
	}
}

func WithNodeUtilsImage(s string) Config {
	return func(cfgs *Configs) {
		cfgs.NodeUtilsImg = s
	}
}
