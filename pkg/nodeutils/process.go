package nodeutils

import (
	"fmt"

	"github.com/shirou/gopsutil/process"
)

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
