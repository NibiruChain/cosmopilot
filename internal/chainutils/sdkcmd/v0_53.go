package sdkcmd

import (
	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
)

func init() {
	RegisterSDK(appsv1.V0_53, func(globalOptions ...Option) SDK {
		return newV0_53(globalOptions...)
	})
}

func newV0_53(globalOptions ...Option) *v0_53 {
	return &v0_53{v0_50: *newV0_50(globalOptions...)}
}

type v0_53 struct {
	v0_50
}
