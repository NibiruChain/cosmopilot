package nodeutils

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeCheckerPreservesLastGoodConfigAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrades.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades":[{"height":100,"status":"scheduled","source":"on-chain"}]}`), 0o600))
	checker, err := NewUpgradeChecker(path)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades":[`), 0o600))
	require.Error(t, checker.reload())
	assert.Equal(t, []Upgrade{{Height: 100, Status: UpgradeScheduled, Source: OnChainUpgrade}}, checker.Snapshot().Upgrades)

	require.NoError(t, os.WriteFile(path, []byte(`{"upgrades":[{"height":200,"status":"scheduled","source":"manual"}]}`), 0o600))
	require.NoError(t, checker.reload())
	assert.Equal(t, int64(200), checker.Snapshot().Upgrades[0].Height)
}

func TestUpgradeCheckerTreatsScheduledAndOngoingAsPending(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  string
		pending bool
	}{
		{name: "scheduled", status: UpgradeScheduled, pending: true},
		{name: "ongoing", status: UpgradeOnGoing, pending: true},
		{name: "completed", status: UpgradeCompleted},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "upgrades.json")
			config := `{"upgrades":[{"height":100,"status":"` + tt.status + `"}]}`
			require.NoError(t, os.WriteFile(path, []byte(config), 0o600))
			checker, err := NewUpgradeChecker(path)
			require.NoError(t, err)

			upgrade, err := checker.GetUpgrade(100)
			if !tt.pending {
				require.Error(t, err)
				assert.Nil(t, upgrade)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.status, upgrade.Status)
		})
	}
}

func TestUpgradeCheckerWatchesConfigMapSymlinkReplacement(t *testing.T) {
	dir := t.TempDir()
	writeVersion := func(name, contents string) {
		t.Helper()
		versionDir := filepath.Join(dir, name)
		require.NoError(t, os.Mkdir(versionDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(versionDir, "upgrades.json"), []byte(contents), 0o600))
		require.NoError(t, os.Symlink(name, filepath.Join(dir, "..data-new")))
		require.NoError(t, os.Rename(filepath.Join(dir, "..data-new"), filepath.Join(dir, "..data")))
	}

	writeVersion("..2026_01", `{"upgrades":[{"height":100,"status":"scheduled","source":"manual"}]}`)
	require.NoError(t, os.Symlink(filepath.Join("..data", "upgrades.json"), filepath.Join(dir, "upgrades.json")))
	checker, err := NewUpgradeChecker(filepath.Join(dir, "upgrades.json"))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- checker.WatchConfigFile(ctx) }()
	select {
	case <-checker.watching:
	case err := <-done:
		require.NoError(t, err)
		t.Fatal("upgrade config watcher stopped before becoming ready")
	case <-time.After(time.Second):
		t.Fatal("upgrade config watcher did not become ready")
	}

	require.NoError(t, os.Remove(filepath.Join(dir, "..data")))
	writeVersion("..2026_02", `{"upgrades":[{"height":200,"status":"scheduled","source":"on-chain"}]}`)
	require.Eventually(t, func() bool {
		return checker.Snapshot().Upgrades[0].Height == 200
	}, 3*time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
}
