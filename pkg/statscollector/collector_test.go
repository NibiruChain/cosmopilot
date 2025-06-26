package statscollector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatsCollector_AverageUsage(t *testing.T) {
	tests := []struct {
		Samples         []ProcessStats
		ExpectedAverage ProcessStats
	}{
		{
			Samples: []ProcessStats{
				{
					CPUTimeSec: 5,
					MemoryRSS:  1000,
				},
				{
					CPUTimeSec: 10,
					MemoryRSS:  1500,
				},
				{
					CPUTimeSec: 15,
					MemoryRSS:  1000,
				},
				{
					CPUTimeSec: 20,
					MemoryRSS:  1500,
				},
				{
					CPUTimeSec: 25,
					MemoryRSS:  1000,
				},
				{
					CPUTimeSec: 30,
					MemoryRSS:  1500,
				},
			},
			ExpectedAverage: ProcessStats{
				CPUTimeSec: 5,
				MemoryRSS:  1250,
			},
		},
	}

	for _, test := range tests {
		collector := NewCollector(len(test.Samples))

		now := time.Now()
		interval := 1 * time.Second
		for i, sample := range test.Samples {
			collector.samples = append(collector.samples, Sample{
				Timestamp:  now.Add(time.Duration(i) * interval),
				CPUTimeSec: sample.CPUTimeSec,
				MemoryRSS:  sample.MemoryRSS,
			})
		}

		cpuAvg := collector.AverageCPUUsage(time.Minute)
		assert.Equal(t, test.ExpectedAverage.CPUTimeSec, cpuAvg)

		memAvg := collector.AverageMemoryUsage(time.Minute)
		assert.Equal(t, test.ExpectedAverage.MemoryRSS, memAvg)
	}
}

func TestStatsCollector_TrimSamples(t *testing.T) {
	collector := NewCollector(3) // only allow 3 samples max

	for i := 0; i < 5; i++ {
		collector.AddSample(ProcessStats{
			CPUTimeSec: float64(i * 10),
			MemoryRSS:  uint64(i * 100),
		})
		time.Sleep(1 * time.Millisecond) // ensure unique timestamps
	}

	assert.Len(t, collector.samples, 3)
	assert.Equal(t, float64(20), collector.samples[0].CPUTimeSec)
	assert.Equal(t, float64(30), collector.samples[1].CPUTimeSec)
	assert.Equal(t, float64(40), collector.samples[2].CPUTimeSec)
}

func TestStatsCollector_AverageUsage_RespectsWindow(t *testing.T) {
	collector := NewCollector(10)

	now := time.Now()

	// Add one old sample (outside 1m window)
	collector.samples = append(collector.samples, Sample{
		Timestamp:  now.Add(-2 * time.Minute),
		CPUTimeSec: 100,
		MemoryRSS:  9999,
	})

	// Add recent samples (inside 1m window)
	collector.samples = append(collector.samples, Sample{
		Timestamp:  now.Add(-2 * time.Second),
		CPUTimeSec: 5,
		MemoryRSS:  1000,
	})
	collector.samples = append(collector.samples, Sample{
		Timestamp:  now.Add(-1 * time.Second),
		CPUTimeSec: 10,
		MemoryRSS:  1500,
	})

	cpuAvg := collector.AverageCPUUsage(time.Minute)
	memAvg := collector.AverageMemoryUsage(time.Minute)

	// Average is based only on deltas between last two (10 - 5) / 15s = ~0.333... but we assume 1s interval
	assert.Equal(t, float64(5), cpuAvg)
	assert.Equal(t, uint64(1250), memAvg) // avg of 1000 and 1500
}
