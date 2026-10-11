package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestCompatiblePatchVersionSelection(t *testing.T) {
	upgrade := func(height int64, version string, status UpgradePhase) Upgrade {
		return Upgrade{Height: height, Image: "example/nibid:" + version, Status: status}
	}
	baseline := upgrade(100, "2.21.0", UpgradeCompleted)
	tests := []struct {
		name        string
		height      int64
		patch       *CompatiblePatchSpec
		override    *string
		upgrades    []Upgrade
		app, latest string
	}{
		{name: "absent", height: 150, upgrades: []Upgrade{baseline}, app: "2.21.0", latest: "2.21.0"},
		{name: "matching", height: 150, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline}, app: "2.21.2", latest: "2.21.2"},
		{name: "unmatched", height: 150, patch: &CompatiblePatchSpec{"2.21.2", 99}, upgrades: []Upgrade{baseline}, app: "2.21.0", latest: "2.21.0"},
		{name: "genesis", patch: &CompatiblePatchSpec{"1.0.1", 0}, app: "1.0.1", latest: "1.0.1"},
		{name: "before baseline", height: 99, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{upgrade(100, "2.21.0", UpgradeScheduled)}, app: "1.0.0", latest: "1.0.0"},
		{name: "scheduled future", height: 150, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeScheduled)}, app: "2.21.2", latest: "2.21.2"},
		{name: "scheduled at height", height: 200, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeScheduled)}, app: "2.21.2", latest: "2.21.2"},
		{name: "missing future image", height: 200, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "", UpgradeImageMissing)}, app: "2.21.2", latest: "2.21.2"},
		{name: "ongoing future", height: 200, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeOnGoing)}, app: "2.22.0", latest: "2.21.2"},
		{name: "completed future", height: 201, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeCompleted)}, app: "2.22.0", latest: "2.22.0"},
		{name: "skipped future", height: 201, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeSkipped)}, app: "2.22.0", latest: "2.22.0"},
		{name: "unordered", height: 301, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{upgrade(300, "2.23.0", UpgradeCompleted), baseline, upgrade(200, "2.22.0", UpgradeSkipped)}, app: "2.23.0", latest: "2.23.0"},
		{name: "restore from known history", patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline}, app: "1.0.0", latest: "2.21.2"},
		{name: "future completed beyond observed height", height: 150, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeCompleted)}, app: "2.21.2", latest: "2.22.0"},
		{name: "explicit downgrade upgrade", height: 201, patch: &CompatiblePatchSpec{"2.21.2", 100}, upgrades: []Upgrade{baseline, upgrade(200, "2.20.0", UpgradeCompleted)}, app: "2.20.0", latest: "2.20.0"},
		{name: "legacy override wins", height: 201, patch: &CompatiblePatchSpec{"2.21.2", 100}, override: ptr.To("2.21.3"), upgrades: []Upgrade{baseline, upgrade(200, "2.22.0", UpgradeCompleted)}, app: "2.21.3", latest: "2.21.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &ChainNode{Spec: ChainNodeSpec{App: AppSpec{Image: "example/nibid", Version: ptr.To("1.0.0")}, CompatiblePatch: tt.patch, OverrideVersion: tt.override}, Status: ChainNodeStatus{LatestHeight: tt.height, Upgrades: tt.upgrades}}
			require.Equal(t, tt.app, node.GetAppVersion())
			require.Equal(t, tt.latest, node.GetLatestVersion())
			require.Equal(t, "example/nibid:"+tt.app, node.GetAppImage())
			require.Equal(t, "example/nibid:"+tt.latest, node.GetLatestAppImage())
		})
	}
}

func TestValidateCompatiblePatch(t *testing.T) {
	for _, tt := range []struct {
		name  string
		patch *CompatiblePatchSpec
		error bool
	}{
		{"absent", nil, false},
		{"valid", &CompatiblePatchSpec{"2.21.2", 100}, false},
		{"genesis", &CompatiblePatchSpec{"1.0.1", 0}, false},
		{"empty version", &CompatiblePatchSpec{"", 100}, true},
		{"negative height", &CompatiblePatchSpec{"2.21.2", -1}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCompatiblePatch(tt.patch, ".spec.compatiblePatch")
			if tt.error {
				require.ErrorContains(t, err, ".spec.compatiblePatch")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
