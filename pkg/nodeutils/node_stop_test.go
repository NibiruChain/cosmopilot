package nodeutils

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopOnlyStopsApplicationWhenForced(t *testing.T) {
	for _, tt := range []struct {
		name      string
		force     bool
		wantStops int32
	}{
		{name: "ordinary sidecar stop leaves application running", force: false},
		{name: "explicit shutdown terminates application", force: true, wantStops: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
			monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error { return nil })
			var stops atomic.Int32
			server := &NodeUtils{
				cfg:            &Options{},
				server:         &http.Server{},
				upgradeMonitor: monitor,
				stopNode: func() error {
					stops.Add(1)
					return nil
				},
			}

			require.NoError(t, server.Stop(tt.force))
			assert.Equal(t, tt.wantStops, stops.Load())
		})
	}
}

func TestStopWritesBoundedTerminationEvidence(t *testing.T) {
	for _, tt := range []struct {
		name       string
		force      bool
		wantForced bool
	}{
		{name: "graceful sidecar exit records halt intent"},
		{name: "forced shutdown is explicit", force: true, wantForced: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
			height := int64(98)
			monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error { return nil })
			monitor.status.LatestHeight = &height
			terminationPath := filepath.Join(t.TempDir(), "termination.log")
			server := &NodeUtils{
				cfg: &Options{
					HaltHeight:             100,
					TerminationMessagePath: terminationPath,
				},
				server:             &http.Server{},
				upgradeMonitor:     monitor,
				stopNode:           func() error { return nil },
				shutdownHTTPServer: func() error { return nil },
			}

			require.NoError(t, server.Stop(tt.force))
			body, err := os.ReadFile(terminationPath)
			require.NoError(t, err)
			assert.LessOrEqual(t, len(body), 4096)
			var evidence TerminationEvidence
			require.NoError(t, json.Unmarshal(body, &evidence))
			require.NotNil(t, evidence.LatestHeight)
			assert.Equal(t, height, *evidence.LatestHeight)
			assert.Equal(t, int64(100), evidence.HaltHeight)
			assert.Equal(t, tt.wantForced, evidence.ForcedShutdown)
		})
	}
}

func TestGracefulStopStaysAliveForRequiredUpgradeAndHaltBoundary(t *testing.T) {
	for _, tt := range []struct {
		name     string
		height   int64
		required *RequiredUpgrade
	}{
		{name: "required upgrade", height: 50, required: &RequiredUpgrade{Height: 100, Source: OnChainUpgrade}},
		{name: "halt height minus one", height: 99},
		{name: "halt height", height: 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := `{"upgrades":[]}`
			if tt.required != nil {
				config = `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`
			}
			checker, infoPath := newMonitorTestChecker(t, config)
			if tt.required != nil {
				require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
			}
			monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error { return nil })
			monitor.status.LatestHeight = &tt.height
			monitor.status.RequiredUpgrade = tt.required
			var shutdowns atomic.Int32
			server := &NodeUtils{
				cfg:            &Options{HaltHeight: 100},
				server:         &http.Server{},
				upgradeMonitor: monitor,
				shutdownHTTPServer: func() error {
					shutdowns.Add(1)
					return nil
				},
				stopNode: func() error { return nil },
			}

			require.NoError(t, server.Stop(false))
			assert.Zero(t, shutdowns.Load())
		})
	}
}

func TestGracefulStopReportsHeldAfterWritingRequiredUpgradeEvidence(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`)
	require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
	height := int64(99)
	monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error { return nil })
	monitor.status.LatestHeight = &height
	monitor.status.RequiredUpgrade = &RequiredUpgrade{Height: 100, Source: OnChainUpgrade}
	terminationPath := filepath.Join(t.TempDir(), "termination.log")
	var shutdowns atomic.Int32
	server := &NodeUtils{
		cfg: &Options{
			TerminationMessagePath: terminationPath,
		},
		server:         &http.Server{},
		upgradeMonitor: monitor,
		shutdownHTTPServer: func() error {
			shutdowns.Add(1)
			return nil
		},
		stopNode: func() error { return nil },
	}

	result, err := server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopHeld, result)
	assert.Zero(t, shutdowns.Load())
	body, err := os.ReadFile(terminationPath)
	require.NoError(t, err)
	var evidence TerminationEvidence
	require.NoError(t, json.Unmarshal(body, &evidence))
	require.NotNil(t, evidence.RequiredUpgrade)
	assert.Equal(t, int64(100), evidence.RequiredUpgrade.Height)
	assert.False(t, evidence.ForcedShutdown)
}

func TestGracefulStopReportsCompletedForOrdinaryShutdown(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{50}}, checker, infoPath, func() error { return nil })
	var shutdowns atomic.Int32
	server := &NodeUtils{
		cfg:            &Options{},
		server:         &http.Server{},
		upgradeMonitor: monitor,
		shutdownHTTPServer: func() error {
			shutdowns.Add(1)
			return nil
		},
		stopNode: func() error { return nil },
	}

	result, err := server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopCompleted, result)
	assert.Equal(t, int32(1), shutdowns.Load())
}

func TestGracefulStopHoldsWhenFinalUpgradeStateIsUnknownAndRecovers(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`)
	terminationPath := filepath.Join(t.TempDir(), "termination.log")
	monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline"), errors.New("offline")}}, checker, infoPath, func() error { return nil })
	server := &NodeUtils{
		cfg:    &Options{TerminationMessagePath: terminationPath},
		server: &http.Server{}, upgradeMonitor: monitor,
		shutdownHTTPServer: func() error { return nil },
		stopNode:           func() error { return nil },
	}

	result, err := server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopHeld, result)
	_, err = os.Stat(terminationPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
	result, err = server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopHeld, result)
	body, err := os.ReadFile(terminationPath)
	require.NoError(t, err)
	var evidence TerminationEvidence
	require.NoError(t, json.Unmarshal(body, &evidence))
	require.NotNil(t, evidence.RequiredUpgrade)
	assert.Equal(t, int64(100), evidence.RequiredUpgrade.Height)
}

func TestGracefulStopHoldsOnInvalidMarkerUntilItBecomesValid(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`)
	require.NoError(t, os.WriteFile(infoPath, []byte(`{`), 0o600))
	terminationPath := filepath.Join(t.TempDir(), "termination.log")
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{99}, errs: []error{nil, errors.New("offline")}}, checker, infoPath, func() error { return nil })
	server := &NodeUtils{
		cfg:    &Options{TerminationMessagePath: terminationPath},
		server: &http.Server{}, upgradeMonitor: monitor,
		shutdownHTTPServer: func() error { return nil },
		stopNode:           func() error { return nil },
	}

	result, err := server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopHeld, result)
	_, err = os.Stat(terminationPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
	result, err = server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopHeld, result)
	_, err = os.Stat(terminationPath)
	require.NoError(t, err)
}

func TestForcedStopOverridesUnknownUpgradeState(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	terminationPath := filepath.Join(t.TempDir(), "termination.log")
	monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error { return nil })
	server := &NodeUtils{
		cfg:    &Options{TerminationMessagePath: terminationPath},
		server: &http.Server{}, upgradeMonitor: monitor,
		shutdownHTTPServer: func() error { return nil },
		stopNode:           func() error { return nil },
	}

	result, err := server.StopWithResult(true)
	require.NoError(t, err)
	assert.Equal(t, StopCompleted, result)
	body, err := os.ReadFile(terminationPath)
	require.NoError(t, err)
	var evidence TerminationEvidence
	require.NoError(t, json.Unmarshal(body, &evidence))
	assert.True(t, evidence.ForcedShutdown)
}

func TestForcedStopOverridesPendingUpgradeAtHaltBoundary(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"ongoing","source":"on-chain"}]}`)
	height := int64(99)
	monitor := newUpgradeMonitor(&fakeABCIClient{}, checker, infoPath, func() error { return nil })
	monitor.status = UpgradeStatus{
		LatestHeight:    &height,
		RequiredUpgrade: &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
	}
	terminationPath := filepath.Join(t.TempDir(), "termination.log")
	var shutdowns atomic.Int32
	server := &NodeUtils{
		cfg:    &Options{HaltHeight: 100, TerminationMessagePath: terminationPath},
		server: &http.Server{}, upgradeMonitor: monitor,
		shutdownHTTPServer: func() error { shutdowns.Add(1); return nil },
		stopNode:           func() error { return nil },
	}

	result, err := server.StopWithResult(true)
	require.NoError(t, err)
	assert.Equal(t, StopCompleted, result)
	assert.Equal(t, int32(1), shutdowns.Load())
	body, err := os.ReadFile(terminationPath)
	require.NoError(t, err)
	var evidence TerminationEvidence
	require.NoError(t, json.Unmarshal(body, &evidence))
	assert.True(t, evidence.ForcedShutdown)
	require.NotNil(t, evidence.RequiredUpgrade)
}

func TestStopBeforeStartIsRemembered(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{50}}, checker, infoPath, func() error { return nil })
	server := &NodeUtils{cfg: &Options{}, upgradeMonitor: monitor, stopNode: func() error { return nil }}

	result, err := server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopCompleted, result)
	assert.True(t, server.stopRequested)
}

func TestStopBeforeStartPreventsListenerPublication(t *testing.T) {
	checker, configPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	server, err := New("appd", WithHost("127.0.0.1"), WithPort(0), WithDataPath(t.TempDir()), WithUpgradesConfig(checker.configFile), WithTerminationMessagePath(filepath.Join(t.TempDir(), "termination.log")))
	require.NoError(t, err)
	server.upgradeMonitor.client = &fakeABCIClient{heights: []int64{50}}
	server.upgradeMonitor.upgradeInfoPath = configPath

	result, err := server.StopWithResult(false)
	require.NoError(t, err)
	assert.Equal(t, StopCompleted, result)
	require.NoError(t, server.Start())
	assert.Nil(t, server.listener)
}

func TestForcedStopWaitsForApplicationShutdownBeforeStartReturns(t *testing.T) {
	checker, _ := newMonitorTestChecker(t, `{"upgrades":[]}`)
	terminationPath := filepath.Join(t.TempDir(), "termination.log")
	server, err := New(
		"appd",
		WithHost("127.0.0.1"),
		WithPort(0),
		WithDataPath(t.TempDir()),
		WithUpgradesConfig(checker.configFile),
		WithTerminationMessagePath(terminationPath),
	)
	require.NoError(t, err)
	stopStarted := make(chan struct{})
	releaseStop := make(chan struct{})
	server.stopNode = func() error {
		close(stopStarted)
		<-releaseStop
		return nil
	}
	startDone := make(chan error, 1)
	go func() { startDone <- server.Start() }()
	require.Eventually(t, func() bool {
		server.lifecycleMu.Lock()
		defer server.lifecycleMu.Unlock()
		return server.listener != nil
	}, time.Second, time.Millisecond)

	stopDone := make(chan error, 1)
	go func() {
		_, err := server.StopWithResult(true)
		stopDone <- err
	}()
	select {
	case <-stopStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for application shutdown")
	}
	select {
	case err := <-startDone:
		t.Fatalf("Start returned before application shutdown completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseStop)
	require.NoError(t, <-stopDone)
	select {
	case err := <-startDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Start to return")
	}
}
