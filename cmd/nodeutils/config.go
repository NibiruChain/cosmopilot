package main

import (
	"flag"

	"github.com/NibiruChain/nibiru-operator/internal/environ"
)

func init() {
	flag.StringVar(&host, "host",
		environ.GetString("HOST", "0.0.0.0"),
		"the host at which this server will be listening to",
	)

	flag.IntVar(&port, "port",
		environ.GetInt("PORT", 8000),
		"the port at which this server will be listening to",
	)

	flag.StringVar(&dataPath, "data-dir",
		environ.GetString("DATA_DIR", "/home/app/data"),
		"the directory where data volume is mounted",
	)

	flag.DurationVar(&blockThreshold, "block-threshold",
		environ.GetDuration("BLOCK_THRESHOLD", 0),
		"the time to wait for a block before considering node unhealthy",
	)

	flag.StringVar(&upgradesConfig, "upgrades-config",
		environ.GetString("UPGRADES_CONFIG", "/config/upgrades.json"),
		"the file containing upgrades configuration",
	)

	flag.StringVar(&traceStore, "trace-store",
		environ.GetString("TRACE_STORE", "/trace/trace.fifo"),
		"file or fifo to watch for traces",
	)

	flag.StringVar(&logLevel, "log-level",
		environ.GetString("LOG_LEVEL", "info"),
		"log level",
	)
}
