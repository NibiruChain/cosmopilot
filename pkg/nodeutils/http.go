package nodeutils

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cometbft/cometbft/libs/json"
	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

func (s *NodeUtils) registerRoutes() {
	s.router.HandleFunc("/ready", s.ready).Methods(http.MethodGet)
	s.router.HandleFunc("/health", s.health).Methods(http.MethodGet)
	s.router.HandleFunc("/data_size", s.dataSize).Methods(http.MethodGet)
	s.router.HandleFunc("/latest_height", s.latestHeight).Methods(http.MethodGet)
	s.router.HandleFunc("/must_upgrade", s.mustUpgrade).Methods(http.MethodGet)
}

func (s *NodeUtils) ready(w http.ResponseWriter, r *http.Request) {
	isSyncing, err := s.client.IsNodeSyncing(r.Context())
	if err != nil {
		log.Errorf("error getting syncing status: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.WithField("syncing", isSyncing).Info("got syncing status")
	if isSyncing {
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	if s.cfg.BlockThreshold > 0 {
		block, err := s.client.GetLatestBlock(r.Context())
		if err != nil {
			log.Errorf("error getting latest block: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		blockAge := time.Now().Sub(block.Header.Time)

		log.WithFields(map[string]interface{}{
			"height":    block.Header.Height,
			"time":      block.Header.Time,
			"threshold": s.cfg.BlockThreshold,
			"age":       blockAge,
		}).Info("got latest block")

		if blockAge > s.cfg.BlockThreshold {
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}
	}

	log.Info("node is ready")
	w.WriteHeader(http.StatusOK)
}

func (s *NodeUtils) health(w http.ResponseWriter, r *http.Request) {
	// TODO: this only makes sure node is listening on gRPC.
	// We should check for possible issues with the node.

	// Ensure LCD endpoint is available
	timeout := 1 * time.Second
	_, err := net.DialTimeout("tcp", "127.0.0.1:1317", timeout)
	if err != nil {
		log.Errorf("lcd enpoint is unavailable: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nodeInfo, err := s.client.NodeInfo(r.Context())
	if err != nil {
		log.Errorf("error getting node info: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(nodeInfo)
	if err != nil {
		log.Errorf("error encoding node info to json: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info("node is healthy")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func (s *NodeUtils) dataSize(w http.ResponseWriter, r *http.Request) {
	size, err := utils.DirSize(s.cfg.DataPath)
	if err != nil {
		log.Errorf("error getting data directory size: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.WithField("size", size).Info("retrieved data size")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.FormatInt(size, 10)))
}

func (s *NodeUtils) latestHeight(w http.ResponseWriter, r *http.Request) {
	log.WithField("size", s.latestBlockHeight.Load()).Info("retrieved latest height")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.FormatInt(s.latestBlockHeight.Load(), 10)))
}

func (s *NodeUtils) mustUpgrade(w http.ResponseWriter, r *http.Request) {
	log.WithField("must-upgrade", s.requiresUpgrade).Info("checked if should upgrade")
	if s.requiresUpgrade {
		w.WriteHeader(http.StatusUpgradeRequired)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write([]byte(strconv.FormatBool(s.requiresUpgrade)))
}
