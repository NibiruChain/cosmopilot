package nodeutils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/gorilla/mux"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"

	"github.com/voluzi/cosmopilot/v2/internal/chainutils"
	"github.com/voluzi/cosmopilot/v2/pkg/proxy"
	"github.com/voluzi/cosmopilot/v2/pkg/statscollector"
)

const (
	fineStatsCollectorInterval   = 10 * time.Second
	coarseStatsCollectorInterval = 5 * time.Minute
	terminationMessageMaxBytes   = 4096
)

type NodeUtils struct {
	server             *http.Server
	listener           net.Listener
	router             *mux.Router
	cfg                *Options
	client             *chainutils.Client
	upgradeChecker     *UpgradeChecker
	upgradeMonitor     *upgradeMonitor
	tmkmsActive        atomic.Bool
	tmkmsProxy         *proxy.TCP
	nodeBinaryName     string
	processMu          sync.Mutex
	appProcess         *process.Process
	fineStats          *statscollector.Collector
	coarseStats        *statscollector.Collector
	mockStats          *MockStats
	dataSizeSampler    *dataSizeSampler
	forcedShutdown     atomic.Bool
	terminationMu      sync.Mutex
	lifecycleMu        sync.Mutex
	stopRequested      bool
	stopNode           func() error
	shutdownHTTPServer func() error
	cancel             context.CancelFunc
}

// StopResult reports whether a graceful stop exited or deliberately stayed alive.
type StopResult uint8

const (
	StopCompleted StopResult = iota
	StopHeld
)

func New(nodeBinaryName string, opts ...Option) (*NodeUtils, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	nodeUtils := &NodeUtils{
		cfg:             options,
		router:          mux.NewRouter(),
		nodeBinaryName:  nodeBinaryName,
		fineStats:       statscollector.NewCollector(int(time.Hour / fineStatsCollectorInterval)),
		coarseStats:     statscollector.NewCollector(int((24 * time.Hour) / coarseStatsCollectorInterval)),
		dataSizeSampler: newDataSizeSampler(options.DataPath, measureDataSize),
	}

	uc, err := NewUpgradeChecker(options.UpgradesConfig)
	if err != nil {
		return nil, err
	}
	nodeUtils.upgradeChecker = uc
	nodeUtils.stopNode = nodeUtils.StopNode

	client, err := chainutils.NewClient("127.0.0.1")
	if err != nil {
		return nil, err
	}
	nodeUtils.client = client
	nodeUtils.upgradeMonitor = newUpgradeMonitor(
		client,
		uc,
		filepath.Join(options.DataPath, "upgrade-info.json"),
		nodeUtils.stopNode,
	)

	// In mock mode, we only mock CPU/memory stats - the blockchain still runs
	if options.MockMode {
		nodeUtils.mockStats = NewMockStats()
		log.Info("node-utils starting in mock mode")
		return nodeUtils, nil
	}

	if options.TmkmsProxy {
		nodeUtils.tmkmsProxy, err = proxy.NewTCPProxy(":26659", "127.0.0.1:5555", true)
		if err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return nodeUtils, nil
}

func (s *NodeUtils) Start() error {
	s.registerRoutes()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer s.client.Close()

	server := &http.Server{Addr: fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port), Handler: s.router}
	shutdownHTTPServer := func() error {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	}
	s.lifecycleMu.Lock()
	if s.stopRequested {
		s.lifecycleMu.Unlock()
		return nil
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		s.lifecycleMu.Unlock()
		return err
	}
	s.server = server
	s.listener = listener
	s.cancel = cancel
	s.shutdownHTTPServer = shutdownHTTPServer
	s.lifecycleMu.Unlock()
	go s.dataSizeSampler.Run(ctx)

	go func() {
		if err := s.upgradeChecker.WatchConfigFile(ctx); err != nil {
			log.Errorf("error watching config file: %v", err)
		}
	}()
	go s.upgradeMonitor.Run(ctx)

	if s.tmkmsProxy != nil {
		go func() {
			for {
				s.tmkmsActive.Store(true)
				err := s.tmkmsProxy.Start()
				log.Errorf("tmkms connection finished with error: %v", err)
				s.tmkmsActive.Store(false)

				// If an upgrade is required lets not restart proxy
				if s.upgradeMonitor.RequiresUpgrade() {
					return
				}

				// Wait one second before restarting
				time.Sleep(time.Second)
			}
		}()
	}

	// Fine-grained collector (1h window)
	go func() {
		ticker := time.NewTicker(fineStatsCollectorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.collectProcessStats(s.fineStats); err != nil {
					log.Errorf("error collecting process fine-grained stats: %v", err)
				}
			}
		}
	}()

	// Coarse-grained collector (24h window)
	go func() {
		ticker := time.NewTicker(coarseStatsCollectorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.collectProcessStats(s.coarseStats); err != nil {
					log.Errorf("error collecting process coarse-grained stats: %v", err)
				}
			}
		}
	}()

	log.Infof("server started listening on %s:%d ...\n\n", s.cfg.Host, s.cfg.Port)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (s *NodeUtils) Stop(force bool) error {
	_, err := s.StopWithResult(force)
	return err
}

// StopWithResult lets the signal handler restore default signal behavior when
// node-utils stays alive solely to expose halt or upgrade evidence.
func (s *NodeUtils) StopWithResult(force bool) (StopResult, error) {
	log.WithField("force", force).Info("stopping server")
	var status UpgradeStatus
	if force {
		s.forcedShutdown.Store(true)
		s.terminationMu.Lock()
		status = s.upgradeMonitor.Status()
		s.writeTerminationEvidence(status, true)
		s.terminationMu.Unlock()
	} else {
		s.terminationMu.Lock()
		if s.forcedShutdown.Load() {
			s.terminationMu.Unlock()
			return StopCompleted, nil
		}
		reconcileErr := s.upgradeMonitor.reconcile(context.Background(), false)
		status = s.upgradeMonitor.Status()
		if reconcileErr != nil || (status.LatestHeight == nil && status.RequiredUpgrade == nil) {
			if reconcileErr != nil {
				log.WithError(reconcileErr).Warn("final upgrade reconciliation did not complete")
			}
			s.terminationMu.Unlock()
			if s.forcedShutdown.Load() {
				return StopCompleted, nil
			}
			return StopHeld, nil
		}
		if !s.forcedShutdown.Load() {
			s.writeTerminationEvidence(status, false)
		}
		s.terminationMu.Unlock()
		if s.forcedShutdown.Load() {
			return StopCompleted, nil
		}
	}

	// When Stop is not forced, in the case of an upgrade being required we ignore
	// the Stop call. This is most likely coming from SIGINT or SIGTERM signals and
	// keep node-utils available so cosmopilot can read /must_upgrade before shutting down.
	// When stop is forced (coming from /shutdown endpoint mostly) we ignore the upgrade
	// requirement.
	// Another case is when respecting halt-height. We want to keep node-utils alive a bit
	// more so that cosmopilot can retrieve latest height before total shutdown.
	// Note: Only check halt-height if it's actually configured (> 0), otherwise
	// halt-height=0 would match latestBlockHeight=0 and prevent shutdown.
	var latestHeight int64
	if status.LatestHeight != nil {
		latestHeight = *status.LatestHeight
	}
	atHaltBoundary := s.cfg.HaltHeight > 0 &&
		(latestHeight == s.cfg.HaltHeight || latestHeight == s.cfg.HaltHeight-1)
	if !force && (status.RequiredUpgrade != nil || atHaltBoundary) {
		log.Warn("node requires upgrade or is set to halt on specific height. ignoring stop call")
		return StopHeld, nil
	}

	s.lifecycleMu.Lock()
	server := s.server
	listener := s.listener
	cancel := s.cancel
	shutdownHTTPServer := s.shutdownHTTPServer
	if server == nil {
		s.stopRequested = true
	}
	s.lifecycleMu.Unlock()

	// Stop tmkms proxy if it is still alive
	if s.tmkmsProxy != nil {
		log.Debug("stopping tmkms proxy")
		if err := s.tmkmsProxy.Stop(); err != nil {
			log.Errorf("failed to stop tmkms proxy: %v", err)
		}
	}

	if force {
		log.Debug("stopping node")
		if err := s.stopNode(); err != nil {
			log.Errorf("failed to stop node: %v", err)
		}
	}
	if cancel != nil {
		cancel()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if server == nil {
		return StopCompleted, nil
	}

	log.Debug("shutting down http server")
	if shutdownHTTPServer != nil {
		return StopCompleted, shutdownHTTPServer()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return StopCompleted, server.Shutdown(ctx)
}

func (s *NodeUtils) writeTerminationEvidence(status UpgradeStatus, forced bool) {
	if s.cfg.TerminationMessagePath == "" {
		return
	}
	evidence := TerminationEvidence{
		UpgradeStatus:  status,
		HaltHeight:     s.cfg.HaltHeight,
		ForcedShutdown: forced,
	}
	body, err := json.Marshal(evidence)
	if err != nil {
		log.WithError(err).Warn("failed to marshal node-utils termination evidence")
		return
	}
	if len(body) > terminationMessageMaxBytes {
		log.WithField("bytes", len(body)).Warn("node-utils termination evidence exceeds kubelet limit")
		return
	}
	if err := os.WriteFile(s.cfg.TerminationMessagePath, body, 0o600); err != nil {
		log.WithError(err).Warn("failed to persist node-utils termination evidence")
	}
}

func (s *NodeUtils) getNodeProcess() (*process.Process, error) {
	s.processMu.Lock()
	defer s.processMu.Unlock()
	if s.appProcess != nil {
		return s.appProcess, nil
	}
	var err error
	s.appProcess, err = findProcessByName(s.nodeBinaryName)
	return s.appProcess, err
}

func (s *NodeUtils) StopNode() error {
	p, err := s.getNodeProcess()
	if err != nil {
		return err
	}
	return p.Terminate()
}

func (s *NodeUtils) collectProcessStats(collector *statscollector.Collector) error {
	p, err := s.getNodeProcess()
	if err != nil {
		return err
	}

	stats, err := GetProcessStats(p)
	if err != nil {
		return err
	}

	collector.AddSample(statscollector.ProcessStats{
		CPUTimeSec: stats.CPUTimeSec,
		MemoryRSS:  stats.MemoryRSS,
	})
	return nil
}
