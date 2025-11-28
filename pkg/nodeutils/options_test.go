package nodeutils

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	if opts.Host != DefaultHost {
		t.Errorf("expected Host %s, got %s", DefaultHost, opts.Host)
	}
	if opts.Port != DefaultPort {
		t.Errorf("expected Port %d, got %d", DefaultPort, opts.Port)
	}
	if opts.DataPath != DefaultDataPath {
		t.Errorf("expected DataPath %s, got %s", DefaultDataPath, opts.DataPath)
	}
	if opts.UpgradesConfig != DefaultUpgradesConfig {
		t.Errorf("expected UpgradesConfig %s, got %s", DefaultUpgradesConfig, opts.UpgradesConfig)
	}
	if opts.TraceStore != DefaultTraceStore {
		t.Errorf("expected TraceStore %s, got %s", DefaultTraceStore, opts.TraceStore)
	}
	if opts.BlockThreshold != 0 {
		t.Errorf("expected BlockThreshold 0, got %v", opts.BlockThreshold)
	}
	if opts.CreateFifo != false {
		t.Errorf("expected CreateFifo false, got %v", opts.CreateFifo)
	}
	if opts.TmkmsProxy != false {
		t.Errorf("expected TmkmsProxy false, got %v", opts.TmkmsProxy)
	}
	if opts.HaltHeight != 0 {
		t.Errorf("expected HaltHeight 0, got %v", opts.HaltHeight)
	}
}

func TestWithHost(t *testing.T) {
	opts := defaultOptions()
	WithHost("192.168.1.1")(opts)

	if opts.Host != "192.168.1.1" {
		t.Errorf("expected Host 192.168.1.1, got %s", opts.Host)
	}
}

func TestWithPort(t *testing.T) {
	opts := defaultOptions()
	WithPort(9000)(opts)

	if opts.Port != 9000 {
		t.Errorf("expected Port 9000, got %d", opts.Port)
	}
}

func TestWithDataPath(t *testing.T) {
	opts := defaultOptions()
	WithDataPath("/custom/data")(opts)

	if opts.DataPath != "/custom/data" {
		t.Errorf("expected DataPath /custom/data, got %s", opts.DataPath)
	}
}

func TestWithUpgradesConfig(t *testing.T) {
	opts := defaultOptions()
	WithUpgradesConfig("/custom/upgrades.json")(opts)

	if opts.UpgradesConfig != "/custom/upgrades.json" {
		t.Errorf("expected UpgradesConfig /custom/upgrades.json, got %s", opts.UpgradesConfig)
	}
}

func TestWithBlockThreshold(t *testing.T) {
	opts := defaultOptions()
	WithBlockThreshold(5 * time.Minute)(opts)

	if opts.BlockThreshold != 5*time.Minute {
		t.Errorf("expected BlockThreshold 5m, got %v", opts.BlockThreshold)
	}
}

func TestWithTraceStore(t *testing.T) {
	opts := defaultOptions()
	WithTraceStore("/custom/trace.fifo")(opts)

	if opts.TraceStore != "/custom/trace.fifo" {
		t.Errorf("expected TraceStore /custom/trace.fifo, got %s", opts.TraceStore)
	}
}

func TestCreateFifo(t *testing.T) {
	opts := defaultOptions()
	CreateFifo(true)(opts)

	if opts.CreateFifo != true {
		t.Errorf("expected CreateFifo true, got %v", opts.CreateFifo)
	}
}

func TestWithTmkmsProxy(t *testing.T) {
	opts := defaultOptions()
	WithTmkmsProxy(true)(opts)

	if opts.TmkmsProxy != true {
		t.Errorf("expected TmkmsProxy true, got %v", opts.TmkmsProxy)
	}
}

func TestWithHaltHeight(t *testing.T) {
	opts := defaultOptions()
	WithHaltHeight(100000)(opts)

	if opts.HaltHeight != 100000 {
		t.Errorf("expected HaltHeight 100000, got %d", opts.HaltHeight)
	}
}

func TestMultipleOptions(t *testing.T) {
	opts := defaultOptions()

	// Apply multiple options
	WithHost("10.0.0.1")(opts)
	WithPort(8080)(opts)
	WithDataPath("/data")(opts)
	WithHaltHeight(50000)(opts)
	WithTmkmsProxy(true)(opts)

	if opts.Host != "10.0.0.1" {
		t.Errorf("expected Host 10.0.0.1, got %s", opts.Host)
	}
	if opts.Port != 8080 {
		t.Errorf("expected Port 8080, got %d", opts.Port)
	}
	if opts.DataPath != "/data" {
		t.Errorf("expected DataPath /data, got %s", opts.DataPath)
	}
	if opts.HaltHeight != 50000 {
		t.Errorf("expected HaltHeight 50000, got %d", opts.HaltHeight)
	}
	if opts.TmkmsProxy != true {
		t.Errorf("expected TmkmsProxy true, got %v", opts.TmkmsProxy)
	}
}
