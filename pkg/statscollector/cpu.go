package statscollector

import "time"

// AverageCPUUsage computes average CPU usage as a fraction of one core.
func (sc *Collector) AverageCPUUsage(since time.Duration) float64 {
	samples := sc.GetSamples(since)
	if len(samples) < 2 {
		return 0
	}

	var totalUtil float64
	var count int

	for i := 1; i < len(samples); i++ {
		deltaCPU := samples[i].CPUTimeSec - samples[i-1].CPUTimeSec
		deltaTime := samples[i].Timestamp.Sub(samples[i-1].Timestamp).Seconds()
		if deltaTime > 0 {
			totalUtil += deltaCPU / deltaTime
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return totalUtil / float64(count)
}
