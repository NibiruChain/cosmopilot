package nodeutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/NibiruChain/cosmopilot/pkg/statscollector"
	"github.com/NibiruChain/cosmopilot/pkg/utils"
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
	s.router.HandleFunc("/stats", s.stats).Methods(http.MethodGet)
	s.router.HandleFunc("/stats/cpu", s.statsCPU).Methods(http.MethodGet)
	s.router.HandleFunc("/stats/memory", s.statsMemory).Methods(http.MethodGet)
}

func writeError(w http.ResponseWriter, format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	log.Error(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		writeError(w, "error encoding json response: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
}

func (s *NodeUtils) ready(w http.ResponseWriter, r *http.Request) {
	isSyncing, err := s.client.IsNodeSyncing(r.Context())
	if err != nil {
		writeError(w, "error getting syncing status: %v", err)
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
			writeError(w, "error getting latest block: %v", err)
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
		writeError(w, "error getting node status: %v", err)
		return
	}

	log.Info("node is healthy")
	writeJSON(w, http.StatusOK, status)
}

func (s *NodeUtils) dataSize(w http.ResponseWriter, r *http.Request) {
	size, err := utils.DirSize(s.cfg.DataPath)
	if err != nil {
		writeError(w, "error getting data size: %v", err)
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
		writeError(w, "error shutting down server: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *NodeUtils) listSnapshots(w http.ResponseWriter, r *http.Request) {
	log.Info("retrieving snapshots")
	files, err := os.ReadDir(path.Join(s.cfg.DataPath, "snapshots"))
	if err != nil {
		writeError(w, "error reading snapshots: %v", err)
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
	writeJSON(w, http.StatusOK, heights)
}

func (s *NodeUtils) stats(w http.ResponseWriter, r *http.Request) {
	log.Info("retrieving stats")

	p, err := s.getNodeProcess()
	if err != nil {
		log.Errorf("error retrieving app process: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats, err := GetProcessStats(p)
	if err != nil {
		log.Errorf("error retrieving app stats: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *NodeUtils) selectCollector(duration time.Duration) *statscollector.Collector {
	if duration <= time.Hour {
		return s.fineStats
	}
	return s.coarseStats
}

func (s *NodeUtils) statsCPU(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("average")
	if query == "" {
		// Live stats
		p, err := s.getNodeProcess()
		if err != nil {
			http.Error(w, fmt.Sprintf("process error: %v", err), http.StatusInternalServerError)
			return
		}
		stats, err := GetProcessStats(p)
		if err != nil {
			http.Error(w, fmt.Sprintf("stats error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte(strconv.FormatFloat(stats.CPUTimeSec, 'E', -1, 64)))
		return
	}

	// Average over duration
	dur, err := time.ParseDuration(query)
	if err != nil {
		http.Error(w, "invalid duration format", http.StatusBadRequest)
		return
	}

	collector := s.selectCollector(dur)
	avg := collector.AverageCPUUsage(dur)
	w.Write([]byte(strconv.FormatFloat(avg, 'E', -1, 64)))
}

func (s *NodeUtils) statsMemory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("average")
	if query == "" {
		p, err := s.getNodeProcess()
		if err != nil {
			http.Error(w, fmt.Sprintf("process error: %v", err), http.StatusInternalServerError)
			return
		}
		stats, err := GetProcessStats(p)
		if err != nil {
			http.Error(w, fmt.Sprintf("stats error: %v", err), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(strconv.FormatUint(stats.MemoryRSS, 10)))
		return
	}

	dur, err := time.ParseDuration(query)
	if err != nil {
		http.Error(w, "invalid duration format", http.StatusBadRequest)
		return
	}

	collector := s.selectCollector(dur)
	avg := collector.AverageMemoryUsage(dur)
	w.Write([]byte(strconv.FormatUint(avg, 10)))
}
