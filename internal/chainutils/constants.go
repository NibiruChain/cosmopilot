package chainutils

import (
	"time"
)

const (
	P2pPortName = "p2p"
	P2pPort     = 26656

	RpcPortName = "rpc"
	RpcPort     = 26657

	LcdPortName = "lcd"
	LcdPort     = 1317

	GrpcPortName = "grpc"
	GrpcPort     = 9090

	PrometheusPortName = "prometheus"
	PrometheusPort     = 26660

	PrivValPortName = "privvalidator"
	PrivValPort     = 26659

	none               = "none"
	defaultAccountName = "account"
	defaultHome        = "/home/app"
	defaultData        = "data"
	defaultConfig      = "config"
	defaultGenesisFile = "config/genesis.json"
	GenesisFilename    = "genesis.json"

	paginationLimit = 1000
	httpTimeout     = 10 * time.Second
)
