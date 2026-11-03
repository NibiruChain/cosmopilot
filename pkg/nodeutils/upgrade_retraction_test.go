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
			node := &NodeUtils{upgradeChecker: checker}
			node.latestBlockHeight.Store(100)
			node.requiresUpgrade.Store(true)
			before := httptest.NewRecorder()
			node.mustUpgrade(before, httptest.NewRequest(http.MethodGet, "/requires_upgrade", nil))
			require.Equal(t, http.StatusUpgradeRequired, before.Code)
			write(replacement)
			require.NoError(t, checker.loadConfig())
			after := httptest.NewRecorder()
			node.mustUpgrade(after, httptest.NewRequest(http.MethodGet, "/requires_upgrade", nil))
			require.Equal(t, http.StatusOK, after.Code)
			require.Equal(t, "false", after.Body.String())
			require.False(t, node.upgradeRequired())
		})
	}
}

func TestOngoingUpgradeKeepsRequiredSignal(t *testing.T) {
	checker := &UpgradeChecker{config: UpgradesConfig{Upgrades: []Upgrade{{Height: 100, Status: UpgradeOnGoing}}}}
	node := &NodeUtils{upgradeChecker: checker}
	node.latestBlockHeight.Store(100)
	node.requiresUpgrade.Store(true)
	require.True(t, node.upgradeRequired(), "publishing ongoing status must not retract a real halt")
}

func TestMalformedConfigPreservesLastGoodPlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrades.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades":[{"height":100,"status":"scheduled"}]}`), 0o600))
	checker, err := NewUpgradeChecker(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades": [`), 0o600))
	require.Error(t, checker.loadConfig())
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
			if err := checker.loadConfig(); err != nil {
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
				_ = checker.hasActiveUpgrade(100)
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
}
