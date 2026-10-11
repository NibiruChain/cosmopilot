package nodeutils

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"
)

// MockStats holds the mock CPU and memory values.
type MockStats struct {
	mu        sync.RWMutex
	cpuCores  float64 // CPU usage in cores (e.g., 0.5 = 500m)
	memoryRSS uint64  // Memory usage in bytes
}

// NewMockStats creates a new MockStats with default values.
// Default memory is 300Mi - chosen to be in the "safe zone" (40-80% usage)
// for typical test initial values, avoiding immediate VPA triggers.
func NewMockStats() *MockStats {
	return &MockStats{
		cpuCores:  0.1,               // Default 100m CPU
		memoryRSS: 300 * 1024 * 1024, // Default 300Mi
	}
}

// GetCPU returns the mock CPU usage in cores.
func (m *MockStats) GetCPU() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cpuCores
}

// SetCPU sets the mock CPU usage in cores.
func (m *MockStats) SetCPU(cores float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cpuCores = cores
	log.WithField("cpuCores", cores).Info("mock CPU usage updated")
}

// SetCPUMillicores sets the mock CPU usage in millicores.
func (m *MockStats) SetCPUMillicores(millicores int64) {
	m.SetCPU(float64(millicores) / 1000)
}

// GetMemory returns the mock memory usage in bytes.
func (m *MockStats) GetMemory() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.memoryRSS
}

// SetMemory sets the mock memory usage in bytes.
func (m *MockStats) SetMemory(bytes uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memoryRSS = bytes
	log.WithField("memoryBytes", bytes).Info("mock memory usage updated")
}

// SetMemoryMiB sets the mock memory usage in MiB.
func (m *MockStats) SetMemoryMiB(mib int64) {
	m.SetMemory(uint64(mib * 1024 * 1024))
}

// SetMemoryGiB sets the mock memory usage in GiB.
func (m *MockStats) SetMemoryGiB(gib float64) {
	m.SetMemory(uint64(gib * 1024 * 1024 * 1024))
}

// mockSetCPU handles POST /mock/cpu to set mock CPU usage.
// Accepts: ?millicores=500 or ?cores=0.5
func (s *NodeUtils) mockSetCPU(w http.ResponseWriter, r *http.Request) {
	log.Debug("mock endpoint: /mock/cpu called")

	if !s.cfg.MockMode {
		log.Warn("mock endpoint: /mock/cpu called but mock mode not enabled")
		http.Error(w, "mock mode not enabled", http.StatusForbidden)
		return
	}

	if mc := r.URL.Query().Get("millicores"); mc != "" {
		val, err := strconv.ParseInt(mc, 10, 64)
		if err != nil {
			log.WithField("value", mc).Warn("mock endpoint: invalid millicores value")
			http.Error(w, "invalid millicores value", http.StatusBadRequest)
			return
		}
		s.mockStats.SetCPUMillicores(val)
		log.WithField("millicores", val).Debug("mock endpoint: CPU set successfully")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	if cores := r.URL.Query().Get("cores"); cores != "" {
		val, err := strconv.ParseFloat(cores, 64)
		if err != nil {
			log.WithField("value", cores).Warn("mock endpoint: invalid cores value")
			http.Error(w, "invalid cores value", http.StatusBadRequest)
			return
		}
		s.mockStats.SetCPU(val)
		log.WithField("cores", val).Debug("mock endpoint: CPU set successfully")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	log.Warn("mock endpoint: /mock/cpu called without millicores or cores parameter")
	http.Error(w, "must specify 'millicores' or 'cores' query parameter", http.StatusBadRequest)
}

// mockSetMemory handles POST /mock/memory to set mock memory usage.
// Accepts: ?bytes=536870912 or ?mib=512 or ?gib=1.5
func (s *NodeUtils) mockSetMemory(w http.ResponseWriter, r *http.Request) {
	log.Debug("mock endpoint: /mock/memory called")

	if !s.cfg.MockMode {
		log.Warn("mock endpoint: /mock/memory called but mock mode not enabled")
		http.Error(w, "mock mode not enabled", http.StatusForbidden)
		return
	}

	if b := r.URL.Query().Get("bytes"); b != "" {
		val, err := strconv.ParseUint(b, 10, 64)
		if err != nil {
			log.WithField("value", b).Warn("mock endpoint: invalid bytes value")
			http.Error(w, "invalid bytes value", http.StatusBadRequest)
			return
		}
		s.mockStats.SetMemory(val)
		log.WithField("bytes", val).Debug("mock endpoint: memory set successfully")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	if mib := r.URL.Query().Get("mib"); mib != "" {
		val, err := strconv.ParseInt(mib, 10, 64)
		if err != nil {
			log.WithField("value", mib).Warn("mock endpoint: invalid mib value")
			http.Error(w, "invalid mib value", http.StatusBadRequest)
			return
		}
		s.mockStats.SetMemoryMiB(val)
		log.WithField("mib", val).Debug("mock endpoint: memory set successfully")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	if gib := r.URL.Query().Get("gib"); gib != "" {
		val, err := strconv.ParseFloat(gib, 64)
		if err != nil {
			log.WithField("value", gib).Warn("mock endpoint: invalid gib value")
			http.Error(w, "invalid gib value", http.StatusBadRequest)
			return
		}
		s.mockStats.SetMemoryGiB(val)
		log.WithField("gib", val).Debug("mock endpoint: memory set successfully")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	log.Warn("mock endpoint: /mock/memory called without bytes, mib, or gib parameter")
	http.Error(w, "must specify 'bytes', 'mib', or 'gib' query parameter", http.StatusBadRequest)
}

// mockGetStats returns the current mock stats as JSON.
func (s *NodeUtils) mockGetStats(w http.ResponseWriter, r *http.Request) {
	log.Debug("mock endpoint: /mock/stats called")

	if !s.cfg.MockMode {
		log.Warn("mock endpoint: /mock/stats called but mock mode not enabled")
		http.Error(w, "mock mode not enabled", http.StatusForbidden)
		return
	}

	cpuCores := s.mockStats.GetCPU()
	memoryBytes := s.mockStats.GetMemory()

	stats := map[string]interface{}{
		"cpuCores":    cpuCores,
		"memoryBytes": memoryBytes,
	}

	b, err := json.Marshal(stats)
	if err != nil {
		log.WithError(err).Error("mock endpoint: failed to marshal stats")
		http.Error(w, "failed to marshal stats", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"cpuCores":    cpuCores,
		"memoryBytes": memoryBytes,
	}).Debug("mock endpoint: returning stats")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
