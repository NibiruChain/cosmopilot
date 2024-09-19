package nodeutils

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/pkg/proxy"
	"github.com/NibiruChain/cosmopilot/pkg/tracer"
)

type NodeUtils struct {
	server            *http.Server
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
					// In manual upgrades case, we don't assume the application will panic but still want to stop the node at the
					// right height. However, application can send several traces with the same height, so if we want
					// stop the node after the whole block is processed, let's do it on the first trace of the next height
					if upgrade.Source == OnChainUpgrade {
						log.WithField("height", height).Info("on-chain upgrade: application should panic now")
						s.requiresUpgrade = true

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

	s.server = &http.Server{Addr: fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port), Handler: s.router}
	log.Infof("server started listening on %s:%d ...\n\n", s.cfg.Host, s.cfg.Port)
	err := s.server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *NodeUtils) Stop() error {
	if s.server == nil {
		return fmt.Errorf("server was not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *NodeUtils) StopNode() error {
	return s.tracer.Stop()
}
