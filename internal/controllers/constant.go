package controllers

const (
	FirewallRpcPort         = 16657
	FirewallLcdPort         = 11317
	FirewallGrpcPort        = 19090
	FirewallMetricsPortName = "fw-metrics"
	FirewallMetricsPort     = 9001
	FirewallEvmRpcPort      = 8545 //TODO: add correct port when implemented on firewall
	FirewallEvmRpcWsPort    = 8546 //TODO: add correct port when implemented on firewall

	EvmRpcPortName   = "evm-rpc"
	EvmRpcPort       = 8545
	EvmRpcWsPortName = "evm-rpc-ws"
	EvmRpcWsPort     = 8546

	AppTomlFile         = "app.toml"
	MinimumGasPricesKey = "minimum-gas-prices"
)
