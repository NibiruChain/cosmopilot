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
	blockThreshold time.Duration
)

func main() {
	flag.Parse()

	nodeUtilsServer, err := nodeutils.NewServer(
		nodeutils.WithHost(host),
		nodeutils.WithPort(port),
		nodeutils.WithBlockThreshold(blockThreshold),
		nodeutils.WithDataPath(dataPath),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(nodeUtilsServer.StartServer())
}
