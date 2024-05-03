package nodeutils

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/pkg/proxy"
	"github.com/NibiruChain/nibiru-operator/pkg/tracer"
)

type NodeUtils struct {
	router            *mux.Router
	cfg               *Options
	client            *chainutils.Client
	tracer            *tracer.StoreTracer
	latestBlockHeight atomic.Int64
	upgradeChecker    *UpgradeChecker
	requiresUpgrade   bool
	tmkmsActive       bool
	tmkmsProxy        *proxy.TCP
}

func New(opts ...Option) (*NodeUtils, error) {
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
		s.tmkmsActive = true
		go func() {
			err := s.tmkmsProxy.Start()
			log.Errorf("tmkms connection finished with error: %v", err)
			s.tmkmsActive = false
		}()
	}

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
					// Just in case, we still validate that the upgrade-info.json file contains the expected upgrade info
					// for this height, before marking node-utils with upgrade required.
					// In manual upgrades case, we don't assume the application will panic but still want to stop the node at the
					// right height. However, application can send several traces with the same height, so if we want
					// stop the node after the whole block is processed, let's do it on the first trace of the next height
					if upgrade.Source == OnChainUpgrade {
						// wait for upgrade-info.json to be written to disk
						log.WithField("height", height).Info("waiting for upgrade-info.json to be written to disk")
						for {
							hasUpgradeInfo, err := s.upgradeChecker.HasUpgradeInfo(height, s.cfg.DataPath)
							if err != nil {
								log.Errorf("failed to check if upgrade-info has expected upgrade: %v", err)
								continue
							}

							if hasUpgradeInfo {
								s.requiresUpgrade = true
								break
							}
						}
					} else if heightUpdated {
						log.WithField("height", height).Warn("stopping tracer to force application stop for upgrade")
						s.requiresUpgrade = true
						err := s.StopNode()
						if err == nil {
							return
						}
						log.Errorf("failed to stop tracer: %v", err)
					}
				}
			}
		}
	}()

	log.Infof("server started listening on %s:%d ...\n\n", s.cfg.Host, s.cfg.Port)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port), s.router)
}

func (s *NodeUtils) StopNode() error {
	return s.tracer.Stop()
}
