package nodeutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetractedConfigClearsUpgradeRequired(t *testing.T) {
	for _, replacement := range [][]Upgrade{nil, {{Height: 200, Status: UpgradeScheduled, Source: OnChainUpgrade}}} {
		name := "cancelled"
		if len(replacement) > 0 {
			name = "replaced"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "upgrades.json")
			write := func(upgrades []Upgrade) {
				bz, err := json.Marshal(UpgradesConfig{Upgrades: upgrades})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, bz, 0o600))
			}
			write([]Upgrade{{Height: 100, Status: UpgradeScheduled, Source: OnChainUpgrade}})
			checker, err := NewUpgradeChecker(path)
			require.NoError(t, err)
			infoPath := filepath.Join(t.TempDir(), "upgrade-info.json")
			require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
			monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{99, 99}}, checker, infoPath, func() error { return nil })
			require.NoError(t, monitor.Reconcile(t.Context()))
			node := &NodeUtils{upgradeMonitor: monitor}
			before := httptest.NewRecorder()
			node.mustUpgrade(before, httptest.NewRequest(http.MethodGet, "/requires_upgrade", nil))
			require.Equal(t, http.StatusUpgradeRequired, before.Code)
			write(replacement)
			require.NoError(t, checker.reload())
			require.NoError(t, monitor.Reconcile(t.Context()))
			after := httptest.NewRecorder()
			node.mustUpgrade(after, httptest.NewRequest(http.MethodGet, "/requires_upgrade", nil))
			require.Equal(t, http.StatusOK, after.Code)
			require.Equal(t, "false", after.Body.String())
			require.False(t, monitor.RequiresUpgrade())
		})
	}
}

func TestOngoingUpgradeKeepsRequiredSignal(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"ongoing","source":"on-chain"}]}`)
	require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{99}}, checker, infoPath, func() error { return nil })
	require.NoError(t, monitor.Reconcile(t.Context()))
	require.True(t, monitor.RequiresUpgrade(), "publishing ongoing status must not retract a real halt")
}

func TestMalformedConfigPreservesLastGoodPlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrades.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades":[{"height":100,"status":"scheduled"}]}`), 0o600))
	checker, err := NewUpgradeChecker(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades": [`), 0o600))
	require.Error(t, checker.reload())
	require.True(t, checker.ShouldUpgrade(100))
}

func TestConcurrentConfigReloadAndUpgradeChecks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrades.json")
	data := []byte(`{"upgrades":[{"height":100,"status":"scheduled"}]}`)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	checker, err := NewUpgradeChecker(path)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errors := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			if err := checker.reload(); err != nil {
				errors <- err
				return
			}
		}
	}()
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				_, _ = checker.GetUpgrade(100)
				_ = checker.Snapshot()
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
}
