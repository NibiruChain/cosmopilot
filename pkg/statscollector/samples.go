package statscollector

import "time"

// AddSample records new sample.
func (sc *Collector) AddSample(stats ProcessStats) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	sc.samples = append(sc.samples, Sample{
		Timestamp:  time.Now(),
		CPUTimeSec: stats.CPUTimeSec,
		MemoryRSS:  stats.MemoryRSS,
	})

	if len(sc.samples) > sc.maxSamples {
		sc.samples = sc.samples[len(sc.samples)-sc.maxSamples:]
	}
}

// GetSamples returns all samples within the given time window.
func (sc *Collector) GetSamples(since time.Duration) []Sample {
	cutoff := time.Now().Add(-since)

	sc.lock.RLock()
	defer sc.lock.RUnlock()

	var result []Sample
	for _, s := range sc.samples {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}
