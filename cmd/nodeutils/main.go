package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	_ "go.uber.org/automaxprocs"

	"github.com/voluzi/cosmopilot/v2/pkg/environ"
	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
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
	// Check for CLI subcommands before parsing flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mock":
			handleMockCommand(os.Args[2:])
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}

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
		nodeutils.WithMockMode(mockMode),
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

func printHelp() {
	fmt.Println(`node-utils - Node utilities sidecar for cosmopilot

Usage:
  node-utils [flags]           Start the node-utils server
  node-utils mock <command>    Control mock mode (use from kubectl exec)
  node-utils help              Show this help

Mock Commands (for E2E testing):
  node-utils mock set-cpu <millicores>     Set mock CPU usage (e.g., 500 for 500m)
  node-utils mock set-memory <mib>         Set mock memory usage in MiB (e.g., 512)
  node-utils mock get                      Get current mock stats

Flags:`)
	flag.PrintDefaults()
}

// handleMockCommand processes mock subcommands for controlling mock mode via CLI.
// This is useful for E2E tests running kubectl exec against distroless containers.
func handleMockCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: node-utils mock <command>")
		fmt.Println("Commands: set-cpu <millicores>, set-memory <mib>, get")
		os.Exit(1)
	}

	// Get port from environment variable or use default (8000)
	// Note: We can't use the `port` flag variable here because flag.Parse() hasn't been called yet
	mockPort := environ.GetInt("PORT", 8000)

	baseURL := fmt.Sprintf("http://localhost:%d", mockPort)

	switch args[0] {
	case "set-cpu":
		if len(args) < 2 {
			fmt.Println("Usage: node-utils mock set-cpu <millicores>")
			os.Exit(1)
		}
		url := fmt.Sprintf("%s/mock/cpu?millicores=%s", baseURL, args[1])
		resp, err := http.Post(url, "text/plain", bytes.NewBuffer(nil))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Error: %s\n", body)
			os.Exit(1)
		}
		fmt.Printf("CPU set to %s millicores\n", args[1])

	case "set-memory":
		if len(args) < 2 {
			fmt.Println("Usage: node-utils mock set-memory <mib>")
			os.Exit(1)
		}
		url := fmt.Sprintf("%s/mock/memory?mib=%s", baseURL, args[1])
		fmt.Printf("DEBUG: POST %s\n", url)
		resp, err := http.Post(url, "text/plain", bytes.NewBuffer(nil))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error making request to %s: %v\n", url, err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("DEBUG: Response status=%d body=%s\n", resp.StatusCode, string(body))
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Error: %s\n", body)
			os.Exit(1)
		}
		fmt.Printf("Memory set to %s MiB\n", args[1])

	case "get":
		url := fmt.Sprintf("%s/mock/stats", baseURL)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Error: %s\n", body)
			os.Exit(1)
		}
		fmt.Println(string(body))

	default:
		fmt.Printf("Unknown mock command: %s\n", args[0])
		fmt.Println("Commands: set-cpu <millicores>, set-memory <mib>, get")
		os.Exit(1)
	}
}
