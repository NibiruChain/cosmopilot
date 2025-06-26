package statscollector

import (
	"sync"
	"time"
)

// ProcessStats stores basic stats.
type ProcessStats struct {
	CPUTimeSec float64 `json:"cpu_time_sec"`
	MemoryRSS  uint64  `json:"memory_rss_bytes"`
}

// Sample represents a single timestamped metrics snapshot.
type Sample struct {
	Timestamp  time.Time
	CPUTimeSec float64
	MemoryRSS  uint64
}

// Collector collects and stores metrics history for multiple processes.
type Collector struct {
	samples []Sample
	lock    sync.RWMutex

	// maxSamples is the maximum number of samples to retain per PID
	maxSamples int
}

// NewCollector creates a new Collector with a sample retention limit.
func NewCollector(maxSamples int) *Collector {
	return &Collector{
		samples:    make([]Sample, 0),
		maxSamples: maxSamples,
	}
}
