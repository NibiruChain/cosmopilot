package nodeutils

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeABCIClient struct {
	mu      sync.Mutex
	heights []int64
	errs    []error
	called  chan context.Context
}

func (c *fakeABCIClient) GetAbciInfo(ctx context.Context) (abci.ResponseInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.called != nil {
		c.called <- ctx
	}
	var err error
	if len(c.errs) > 0 {
		err = c.errs[0]
		c.errs = c.errs[1:]
	}
	if err != nil {
		return abci.ResponseInfo{}, err
	}
	if len(c.heights) == 0 {
		return abci.ResponseInfo{}, nil
	}
	height := c.heights[0]
	c.heights = c.heights[1:]
	return abci.ResponseInfo{LastBlockHeight: height}, nil
}

func newMonitorTestChecker(t *testing.T, config string) (*UpgradeChecker, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "upgrades.json")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))
	checker, err := NewUpgradeChecker(configPath)
	require.NoError(t, err)
	return checker, filepath.Join(dir, "upgrade-info.json")
}

func TestUpgradeMonitorPollsImmediatelyWithTimeoutAndKeepsLastGoodHeight(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	client := &fakeABCIClient{
		heights: []int64{42},
		errs:    []error{nil, errors.New("offline")},
		called:  make(chan context.Context, 2),
	}
	monitor := newUpgradeMonitor(client, checker, infoPath, func() error { return nil })

	require.NoError(t, monitor.Reconcile(t.Context()))
	firstContext := <-client.called
	_, deadlineSet := firstContext.Deadline()
	assert.True(t, deadlineSet)
	status := monitor.Status()
	require.NotNil(t, status.LatestHeight)
	assert.Equal(t, int64(42), *status.LatestHeight)

	require.NoError(t, monitor.Reconcile(t.Context()))
	status = monitor.Status()
	require.NotNil(t, status.LatestHeight)
	assert.Equal(t, int64(42), *status.LatestHeight)
}

func TestUpgradeMonitorRecoversMatchingGovernanceMarker(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		marker    string
		height    int64
		wantError bool
		want      *RequiredUpgrade
	}{
		{
			name:   "scheduled at halt boundary",
			config: `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`,
			marker: `{"name":"v2","height":100}`,
			height: 99,
			want:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
		},
		{
			name:   "ongoing after sidecar restart",
			config: `{"upgrades":[{"height":100,"status":"ongoing","source":"on-chain"}]}`,
			marker: `{"name":"v2","height":100}`,
			height: 99,
			want:   &RequiredUpgrade{Height: 100, Source: OnChainUpgrade},
		},
		{
			name:   "completed marker is stale",
			config: `{"upgrades":[{"height":100,"status":"completed","source":"on-chain"}]}`,
			marker: `{"name":"v2","height":100}`,
			height: 99,
		},
		{
			name:   "unconfigured marker is ignored",
			config: `{"upgrades":[]}`,
			marker: `{"name":"v2","height":100}`,
			height: 99,
		},
		{
			name:   "far future marker is ignored",
			config: `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`,
			marker: `{"name":"v2","height":100}`,
			height: 98,
		},
		{
			name:   "already committed marker is stale",
			config: `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`,
			marker: `{"name":"v2","height":100}`,
			height: 100,
		},
		{
			name:      "malformed marker reports error",
			config:    `{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`,
			marker:    `{`,
			height:    99,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, infoPath := newMonitorTestChecker(t, tt.config)
			require.NoError(t, os.WriteFile(infoPath, []byte(tt.marker), 0o600))
			monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{tt.height}}, checker, infoPath, func() error { return nil })

			err := monitor.Reconcile(t.Context())
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, monitor.Status().RequiredUpgrade)
		})
	}
}

func TestUpgradeMonitorUsesMarkerWhenABCIIsUnavailableAfterRestart(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"ongoing","source":"on-chain"}]}`)
	require.NoError(t, os.WriteFile(infoPath, []byte(`{"name":"v2","height":100}`), 0o600))
	monitor := newUpgradeMonitor(&fakeABCIClient{errs: []error{errors.New("offline")}}, checker, infoPath, func() error { return nil })

	require.NoError(t, monitor.Reconcile(t.Context()))
	assert.Equal(t, &RequiredUpgrade{Height: 100, Source: OnChainUpgrade}, monitor.Status().RequiredUpgrade)
}

func TestReadSDKUpgradeInfoReadsOwnerOnlyMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade-info.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"name":"v2","height":100}`), 0o600))
	stat, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())

	info, err := readSDKUpgradeInfo(path)
	require.NoError(t, err)
	assert.Equal(t, sdkUpgradeInfo{Name: "v2", Height: 100}, info)
}

func TestReadSDKUpgradeInfoSizeBoundary(t *testing.T) {
	const maxSize = 1024 * 1024
	prefix := `{"name":"v2","height":100,"info":"`
	suffix := `"}`
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "one byte below limit", size: maxSize - 1},
		{name: "at limit", size: maxSize},
		{name: "one byte over limit", size: maxSize + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(prefix + strings.Repeat("x", tt.size-len(prefix)-len(suffix)) + suffix)
			require.Len(t, body, tt.size)
			path := filepath.Join(t.TempDir(), "upgrade-info.json")
			require.NoError(t, os.WriteFile(path, body, 0o600))

			info, err := readSDKUpgradeInfo(path)
			if tt.wantErr {
				require.ErrorContains(t, err, "too large")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, sdkUpgradeInfo{Name: "v2", Height: 100}, info)
		})
	}
}

func TestReadSDKUpgradeInfoRejectsUnsafeFilesAndRecovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade-info.json")
	require.NoError(t, syscall.Mkfifo(path, 0o600))
	_, err := readSDKUpgradeInfo(path)
	require.ErrorContains(t, err, "regular file")
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o700))
	_, err = readSDKUpgradeInfo(path)
	require.ErrorContains(t, err, "regular file")
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("x", sdkUpgradeInfoMaxBytes+1)), 0o600))
	_, err = readSDKUpgradeInfo(path)
	require.ErrorContains(t, err, "too large")
	require.NoError(t, os.WriteFile(path, []byte(`{"name":"v2","height":100}`), 0o600))
	info, err := readSDKUpgradeInfo(path)
	require.NoError(t, err)
	assert.Equal(t, sdkUpgradeInfo{Name: "v2", Height: 100}, info)
}

func TestUpgradeMonitorManualStopRetriesThenRunsOnlyOnce(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"manual"}]}`)
	client := &fakeABCIClient{heights: []int64{99, 99, 99}}
	var attempts atomic.Int32
	monitor := newUpgradeMonitor(client, checker, infoPath, func() error {
		if attempts.Add(1) == 1 {
			return errors.New("temporary failure")
		}
		return nil
	})

	require.Error(t, monitor.Reconcile(t.Context()))
	require.NoError(t, monitor.Reconcile(t.Context()))
	require.NoError(t, monitor.Reconcile(t.Context()))
	assert.Equal(t, int32(2), attempts.Load())
	assert.Equal(t, &RequiredUpgrade{Height: 100, Source: ManualUpgrade}, monitor.Status().RequiredUpgrade)
}

func TestUpgradeMonitorClearsRequirementWhenConfigChanges(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[{"height":100,"status":"scheduled","source":"manual"}]}`)
	monitor := newUpgradeMonitor(&fakeABCIClient{heights: []int64{99, 99}}, checker, infoPath, func() error { return nil })
	require.NoError(t, monitor.Reconcile(t.Context()))
	require.NotNil(t, monitor.Status().RequiredUpgrade)

	require.NoError(t, os.WriteFile(checker.configFile, []byte(`{"upgrades":[{"height":100,"status":"completed","source":"manual"}]}`), 0o600))
	require.NoError(t, checker.reload())
	require.NoError(t, monitor.Reconcile(t.Context()))
	assert.Nil(t, monitor.Status().RequiredUpgrade)
}

func TestUpgradeMonitorRunPollsWithoutExternalWakeup(t *testing.T) {
	checker, infoPath := newMonitorTestChecker(t, `{"upgrades":[]}`)
	client := &fakeABCIClient{heights: []int64{7, 8}, called: make(chan context.Context, 2)}
	monitor := newUpgradeMonitor(client, checker, infoPath, func() error { return nil })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go monitor.Run(ctx)
	select {
	case <-client.called:
	case <-time.After(2 * time.Second):
		t.Fatal("upgrade monitor did not poll")
	}
	select {
	case <-client.called:
	case <-time.After(2 * time.Second):
		t.Fatal("upgrade monitor did not poll on ticker")
	}
}
