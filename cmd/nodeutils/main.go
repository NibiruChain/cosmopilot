package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	_ "go.uber.org/automaxprocs"

	"github.com/NibiruChain/cosmopilot/pkg/nodeutils"
)

var (
	host             string
	port             int
	dataPath         string
	upgradesConfig   string
	blockThreshold   time.Duration
	traceStore       string
	logLevel         string
	createFifo       bool
	enableTmkmsProxy bool
	nodeBinaryName   string
	haltHeight       int64
)

func main() {
	flag.Parse()

	if level, err := log.ParseLevel(logLevel); err == nil {
		log.SetLevel(level)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	nodeUtilsServer, err := nodeutils.New(
		nodeBinaryName,
		nodeutils.WithHost(host),
		nodeutils.WithPort(port),
		nodeutils.WithBlockThreshold(blockThreshold),
		nodeutils.WithDataPath(dataPath),
		nodeutils.WithUpgradesConfig(upgradesConfig),
		nodeutils.WithTraceStore(traceStore),
		nodeutils.CreateFifo(createFifo),
		nodeutils.WithTmkmsProxy(enableTmkmsProxy),
		nodeutils.WithHaltHeight(haltHeight),
	)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		sig := <-sigChan
		log.Infof("received signal: %v", sig)
		if err := nodeUtilsServer.Stop(false); err != nil {
			log.Errorf("failed to stop nodeutils server: %v", err)
		}
	}()

	if err := nodeUtilsServer.Start(); err != nil {
		log.Fatal(err)
	}
}
