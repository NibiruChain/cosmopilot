package nodeutils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestHeightProjectsOnlyMatchingScheduledRequirement(t *testing.T) {
	for _, tt := range []struct {
		name       string
		required   *RequiredUpgrade
		upgrades   []Upgrade
		wantHeight string
	}{
		{
			name:       "matching scheduled governance upgrade",
			required:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
			upgrades:   []Upgrade{{Height: 100, Source: OnChainUpgrade, Status: UpgradeScheduled}},
			wantHeight: "100",
		},
		{
			name:       "unknown marker",
			required:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
			wantHeight: "99",
		},
		{
			name:       "matching ongoing upgrade remains pending",
			required:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
			upgrades:   []Upgrade{{Height: 100, Source: OnChainUpgrade, Status: UpgradeOnGoing}},
			wantHeight: "100",
		},
		{
			name:       "completed is not replayed",
			required:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
			upgrades:   []Upgrade{{Height: 100, Source: OnChainUpgrade, Status: UpgradeCompleted}},
			wantHeight: "99",
		},
		{
			name:       "different source",
			required:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
			upgrades:   []Upgrade{{Height: 100, Source: ManualUpgrade, Status: UpgradeScheduled}},
			wantHeight: "99",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			height := int64(99)
			server := &NodeUtils{
				cfg:    &Options{},
				router: mux.NewRouter(),
				upgradeMonitor: &upgradeMonitor{
					checker: &UpgradeChecker{config: UpgradesConfig{Upgrades: tt.upgrades}},
					status:  UpgradeStatus{LatestHeight: &height, RequiredUpgrade: tt.required},
				},
			}
			server.registerRoutes()

			response := httptest.NewRecorder()
			server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/latest_height", nil))
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, tt.wantHeight, response.Body.String())
		})
	}
}

func TestMustUpgradeUsesMonitorRequirement(t *testing.T) {
	server := &NodeUtils{
		cfg:    &Options{},
		router: mux.NewRouter(),
		upgradeMonitor: &upgradeMonitor{status: UpgradeStatus{
			RequiredUpgrade: &RequiredUpgrade{Height: 100, Source: ManualUpgrade},
		}},
	}
	server.registerRoutes()

	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/must_upgrade", nil))
	assert.Equal(t, http.StatusUpgradeRequired, response.Code)
	assert.Equal(t, "true", response.Body.String())
}

func TestMustUpgradeRefreshReconcilesDurableMarkerWithoutStoppingApp(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`)
	require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
	var stops atomic.Int32
	monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error {
		stops.Add(1)
		return nil
	})
	server := &NodeUtils{cfg: &Options{}, router: mux.NewRouter(), upgradeMonitor: monitor}
	server.registerRoutes()

	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/must_upgrade?refresh=true", nil))
	assert.Equal(t, http.StatusUpgradeRequired, response.Code)
	assert.Equal(t, "true", response.Body.String())
	assert.Zero(t, stops.Load())
}

func TestMustUpgradeRefreshNeverStopsAppForManualUpgrade(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"manual"}]}`)
	var stops atomic.Int32
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{99}}, checker, infoPath, func() error {
		stops.Add(1)
		return nil
	})
	server := &NodeUtils{cfg: &Options{}, router: mux.NewRouter(), upgradeMonitor: monitor}
	server.registerRoutes()

	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/must_upgrade?refresh=true", nil))
	assert.Equal(t, http.StatusUpgradeRequired, response.Code)
	assert.Equal(t, "true", response.Body.String())
	assert.Zero(t, stops.Load())
}

func TestMustUpgradeRefreshFailsInsteadOfReturningCachedFalse(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`)
	require.NoError(t, os.WriteFile(infoPath, []byte(`{`), 0o600))
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{99}}, checker, infoPath, func() error { return nil })
	server := &NodeUtils{cfg: &Options{}, router: mux.NewRouter(), upgradeMonitor: monitor}
	server.registerRoutes()

	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/must_upgrade?refresh=true", nil))
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.False(t, monitor.RequiresUpgrade())
}

func TestMustUpgradeRefreshReturnsFreshNegative(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{50}}, checker, infoPath, func() error { return nil })
	server := &NodeUtils{cfg: &Options{}, router: mux.NewRouter(), upgradeMonitor: monitor}
	server.registerRoutes()

	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/must_upgrade?refresh=true", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "false", response.Body.String())
}
