package nodeutils

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/cosmopilot/internal/utils"
)

func (s *NodeUtils) registerRoutes() {
	s.router.HandleFunc("/ready", s.ready).Methods(http.MethodGet)
	s.router.HandleFunc("/health", s.health).Methods(http.MethodGet)
	s.router.HandleFunc("/data_size", s.dataSize).Methods(http.MethodGet)
	s.router.HandleFunc("/latest_height", s.latestHeight).Methods(http.MethodGet)
	s.router.HandleFunc("/must_upgrade", s.mustUpgrade).Methods(http.MethodGet)
	s.router.HandleFunc("/tmkms_active", s.tmkmsConnectionActive).Methods(http.MethodGet)
	s.router.HandleFunc("/snapshots", s.listSnapshots).Methods(http.MethodGet)
	s.router.HandleFunc("/shutdown", s.shutdownServer).Methods(http.MethodGet, http.MethodPost)
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
	// TODO: this only makes sure node is responding to rpc.
	// We should check for possible issues with the node.

	status, err := s.client.GetNodeStatus(r.Context())
	if err != nil {
		log.Errorf("error getting node status: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(status)
	if err != nil {
		log.Errorf("error encoding node status to json: %v", err)
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
	log.WithField("height", s.latestBlockHeight.Load()).Info("retrieved latest height")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.FormatInt(s.latestBlockHeight.Load(), 10)))
}

func (s *NodeUtils) mustUpgrade(w http.ResponseWriter, r *http.Request) {
	log.WithField("must-upgrade", s.requiresUpgrade.Load()).Info("checked if should upgrade")
	if s.requiresUpgrade.Load() {
		w.WriteHeader(http.StatusUpgradeRequired)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write([]byte(strconv.FormatBool(s.requiresUpgrade.Load())))
}

func (s *NodeUtils) tmkmsConnectionActive(w http.ResponseWriter, r *http.Request) {
	log.WithField("tmkms-active", s.tmkmsActive.Load()).Info("checked if tmkms is active")
	if s.tmkmsActive.Load() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNotAcceptable)
	}
	w.Write([]byte(strconv.FormatBool(s.tmkmsActive.Load())))
}

func (s *NodeUtils) shutdownServer(w http.ResponseWriter, r *http.Request) {
	log.Info("shutting down server")
	if err := s.Stop(true); err != nil {
		log.Errorf("error stopping server: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *NodeUtils) listSnapshots(w http.ResponseWriter, r *http.Request) {
	log.Info("retrieving snapshots")
	files, err := os.ReadDir(path.Join(s.cfg.DataPath, "snapshots"))
	if err != nil {
		log.Errorf("error reading directory: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var heights []int64
	heightRegex := regexp.MustCompile(`^\d+$`) // Match only numbers

	for _, file := range files {
		if file.IsDir() && heightRegex.MatchString(file.Name()) {
			// Convert the directory name to int64
			if height, err := strconv.ParseInt(file.Name(), 10, 64); err == nil {
				heights = append(heights, height)
			}
		}
	}
	// Sort heights in ascending order
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })

	b, err := json.Marshal(heights)
	if err != nil {
		log.Errorf("error building json response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
