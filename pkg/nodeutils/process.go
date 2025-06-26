package nodeutils

import (
	"fmt"

	"github.com/shirou/gopsutil/process"
)

type ProcessStats struct {
	CPUTimeSec float64 `json:"cpu_time_sec"`     // total CPU time in seconds
	MemoryRSS  uint64  `json:"memory_rss_bytes"` // resident memory usage
}

// findProcessByName searches for a process by name using gopsutil
func findProcessByName(name string) (*process.Process, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	for _, proc := range processes {
		pname, err := proc.Name()
		if err == nil && pname == name {
			return proc, nil
		}
	}
	return nil, fmt.Errorf("process %s not found", name)
}

// GetProcessStats extracts CPU + memory usage from a gopsutil Process
func GetProcessStats(proc *process.Process) (*ProcessStats, error) {
	times, err := proc.Times()
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU times: %w", err)
	}

	mem, err := proc.MemoryInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}

	return &ProcessStats{
		CPUTimeSec: times.User + times.System,
		MemoryRSS:  mem.RSS,
	}, nil
}
