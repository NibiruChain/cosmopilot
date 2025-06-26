package nodeutils

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/gorilla/mux"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/pkg/proxy"
	"github.com/NibiruChain/cosmopilot/pkg/statscollector"
	"github.com/NibiruChain/cosmopilot/pkg/tracer"
)

const (
	fineStatsCollectorInterval   = 10 * time.Second
	coarseStatsCollectorInterval = 5 * time.Minute
)

type NodeUtils struct {
	server            *http.Server
	router            *mux.Router
	cfg               *Options
	client            *chainutils.Client
	tracer            *tracer.StoreTracer
	latestBlockHeight atomic.Int64
	upgradeChecker    *UpgradeChecker
	requiresUpgrade   atomic.Bool
	tmkmsActive       atomic.Bool
	tmkmsProxy        *proxy.TCP
	nodeBinaryName    string
	appProcess        *process.Process
	fineStats         *statscollector.Collector
	coarseStats       *statscollector.Collector
}

func New(nodeBinaryName string, opts ...Option) (*NodeUtils, error) {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	client, err := chainutils.NewClient("127.0.0.1")
	if err != nil {
		return nil, err
	}

	t, err := tracer.NewStoreTracer(options.TraceStore, options.CreateFifo)
	if err != nil {
		return nil, err
	}

	uc, err := NewUpgradeChecker(options.UpgradesConfig)
	if err != nil {
		return nil, err
	}

	nodeUtils := &NodeUtils{
		cfg:            options,
		router:         mux.NewRouter(),
		client:         client,
		tracer:         t,
		upgradeChecker: uc,
		nodeBinaryName: nodeBinaryName,
		fineStats:      statscollector.NewCollector(int(time.Hour / fineStatsCollectorInterval)),
		coarseStats:    statscollector.NewCollector(int((24 * time.Hour) / coarseStatsCollectorInterval)),
	}

	if options.TmkmsProxy {
		nodeUtils.tmkmsProxy, err = proxy.NewTCPProxy(":26659", "127.0.0.1:5555", true)
		if err != nil {
			return nil, err
		}
	}

	return nodeUtils, nil
}

func (s *NodeUtils) Start() error {
	s.registerRoutes()
	go s.tracer.Start()
	go func() {
		if err := s.upgradeChecker.WatchConfigFile(); err != nil {
			log.Errorf("error watching config file: %v", err)
		}
	}()

	if s.tmkmsProxy != nil {
		go func() {
			for {
				s.tmkmsActive.Store(true)
				err := s.tmkmsProxy.Start()
				log.Errorf("tmkms connection finished with error: %v", err)
				s.tmkmsActive.Store(false)

				// If an upgrade is required lets not restart proxy
				if s.requiresUpgrade.Load() {
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
			case <-ticker.C:
				if err := s.collectProcessStats(s.coarseStats); err != nil {
					log.Errorf("error collecting process coarse-grained stats: %v", err)
				}
			}
		}
	}()

	// Goroutine to update latest height and check for upgrades
	go func() {
		for trace := range s.tracer.Traces {
			log.Trace(trace)
			log.Trace(trace.Metadata)

			if trace.Err != nil {
				log.Errorf("error on trace: %v", trace.Err)
				continue
			}

			if trace.Metadata != nil {
				heightUpdated := s.latestBlockHeight.CompareAndSwap(s.latestBlockHeight.Load(), trace.Metadata.BlockHeight)
				height := s.latestBlockHeight.Load()

				if s.upgradeChecker.ShouldUpgrade(height) {
					upgrade, _ := s.upgradeChecker.GetUpgrade(height)

					// If it's an on-chain upgrade, the application is supposed to panic and require the upgrade.
					// In manual upgrades case, we don't assume the application will panic but still want to stop the node at the
					// right height. However, application can send several traces with the same height, so if we want
					// stop the node after the whole block is processed, let's do it on the first trace of the next height
					if upgrade.Source == OnChainUpgrade {
						log.WithField("height", height).Info("on-chain upgrade: application should panic now")
						s.requiresUpgrade.Store(true)

					} else if heightUpdated {
						log.WithField("height", height).Warn("stopping node for upgrade")
						s.requiresUpgrade.Store(true)
						err := s.StopNode()
						if err == nil {
							return
						}
						log.Errorf("failed to stop node: %v", err)
					}
				}
			}
		}
	}()

	s.server = &http.Server{Addr: fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port), Handler: s.router}
	log.Infof("server started listening on %s:%d ...\n\n", s.cfg.Host, s.cfg.Port)
	err := s.server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *NodeUtils) Stop(force bool) error {
	log.WithField("force", force).Info("stopping server")

	// When Stop is not forced, in the case of an upgrade being required we ignore
	// the Stop call. This is most likely coming from SIGINT or SIGTERM signals and
	// we need to wait for cosmopilot to read /requires_upgrade before shutting down.
	// When stop is forced (coming from /shutdown endpoint mostly) we ignore the upgrade
	// requirement.
	// Another case is when respecting halt-height. We want to keep node-utils alive a bit
	// more so that cosmopilot can retrieve latest height before total shutdown.
	if !force && (s.requiresUpgrade.Load() || s.cfg.HaltHeight == s.latestBlockHeight.Load()) {
		log.Warn("node requires upgrade or is set to halt on specific height. ignoring stop call")
		return nil
	}

	if s.server == nil {
		return fmt.Errorf("server was not started")
	}

	// Stop tmkms proxy if it is still alive
	if s.tmkmsProxy != nil {
		log.Debug("stopping tmkms proxy")
		if err := s.tmkmsProxy.Stop(); err != nil {
			log.Errorf("failed to stop tmkms proxy: %v", err)
		}
	}

	// Ensure node is stopped too
	log.Debug("stopping node")
	if err := s.StopNode(); err != nil {
		log.Errorf("failed to stop node: %v", err)
	}

	// Shutdown main server
	log.Debug("shutting down http server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *NodeUtils) getNodeProcess() (*process.Process, error) {
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
