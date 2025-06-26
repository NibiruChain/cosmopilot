package statscollector

import "time"

// AverageMemoryUsage returns the average stats over the given time window.
func (sc *Collector) AverageMemoryUsage(since time.Duration) uint64 {
	samples := sc.GetSamples(since)
	if len(samples) == 0 {
		return 0
	}

	var totalMem uint64
	for _, s := range samples {
		totalMem += s.MemoryRSS
	}

	return totalMem / uint64(len(samples))
}
