package main

import (
	"flag"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/nibiru-operator/pkg/nodeutils"
)

var (
	host           string
	port           int
	dataPath       string
	upgradesConfig string
	blockThreshold time.Duration
	traceStore     string
	logLevel       string
)

func main() {
	flag.Parse()

	if level, err := log.ParseLevel(logLevel); err == nil {
		log.SetLevel(level)
	}

	nodeUtilsServer, err := nodeutils.New(
		nodeutils.WithHost(host),
		nodeutils.WithPort(port),
		nodeutils.WithBlockThreshold(blockThreshold),
		nodeutils.WithDataPath(dataPath),
		nodeutils.WithUpgradesConfig(upgradesConfig),
		nodeutils.WithTraceStore(traceStore),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(nodeUtilsServer.Start())
}
