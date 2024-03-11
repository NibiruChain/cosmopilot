package chainnodeset

import "time"

const (
	ChainNodeWaitTimeout = 3 * time.Minute
	ChainNodeKind        = "ChainNode"

	LabelChainNodeSet          = "nodeset"
	LabelChainNodeSetGroup     = "group"
	LabelChainNodeSetValidator = "validator"
	LabelGlobalIngress         = "global-ingress"
	LabelScope                 = "scope"

	scopeGlobal = "global"
	scopeGroup  = "group"

	ingressClassNameNginx = "nginx"

	validatorGroupName = "validator"
)

var (
	nginxGrpcAnnotations = map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": "GRPC",
	}
)
