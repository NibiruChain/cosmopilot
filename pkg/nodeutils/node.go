package nodeutils

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

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

			if trace.Err != nil {
				log.Errorf("error on trace: %v", trace.Err)
				continue
			}

			if trace.Metadata != nil {
				s.latestBlockHeight.Swap(trace.Metadata.BlockHeight)
				if s.upgradeChecker.ShouldUpgrade(s.latestBlockHeight.Load()) {
					log.WithField("height", s.latestBlockHeight.Load()).Warn("stopping tracer to force application stop for upgrade")
					s.requiresUpgrade = true

					// wait for upgrade-info.json to be written to disk
					time.Sleep(5 * time.Second)

					err := s.StopNode()
					if err != nil {
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
