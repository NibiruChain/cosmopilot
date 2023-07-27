package chainnodeset

import "time"

const (
	ChainNodeWaitTimeout = 3 * time.Minute

	LabelChainNodeSet          = "nodeset"
	LabelChainNodeSetGroup     = "group"
	LabelChainNodeSetValidator = "validator"
	ingressClassNameNginx      = "nginx"

	validatorGroupName = "validator"
)

var (
	nginxAnnotations = map[string]string{
		"nginx.ingress.kubernetes.io/proxy-buffering":  "on",
		"nginx.ingress.kubernetes.io/service-upstream": "false",
	}

	nginxGrpcAnnotations = map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
	}
)
