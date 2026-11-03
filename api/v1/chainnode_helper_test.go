package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestChainNodeMustStopUsesOnlyCurrentHaltHold(t *testing.T) {
	for _, tt := range []struct {
		name       string
		haltHeight *int64
		hold       string
		want       bool
	}{
		{name: "exact height without verified hold", haltHeight: ptr.To[int64](100)},
		{name: "matching hold", haltHeight: ptr.To[int64](100), hold: "100", want: true},
		{name: "changed target rejects old hold", haltHeight: ptr.To[int64](101), hold: "100"},
		{name: "removed target rejects old hold", hold: "100"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			node := &ChainNode{
				Spec:   ChainNodeSpec{Config: &Config{HaltHeight: tt.haltHeight}},
				Status: ChainNodeStatus{LatestHeight: 100},
			}
			if tt.hold != "" {
				node.Annotations = map[string]string{AnnotationHaltHeightHold: tt.hold}
			}

			got, _ := node.MustStop()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestChainNodeGetAppVersionUsesOnlyMatchingCommittedAnchor(t *testing.T) {
	tests := []struct {
		name        string
		baseVersion string
		height      int64
		appVersion  string
		upgrades    []Upgrade
		want        string
	}{
		{
			name: "completed target survives raw height minus one", baseVersion: "v1", height: 99, appVersion: "v2",
			upgrades: []Upgrade{{Height: 100, Image: "app:v2", Status: UpgradeCompleted}}, want: "v2",
		},
		{
			name: "ongoing target survives raw height minus one", baseVersion: "v1", height: 99, appVersion: "v2",
			upgrades: []Upgrade{{Height: 100, Image: "app:v2", Status: UpgradeOnGoing}}, want: "v2",
		},
		{
			name: "empty anchor preserves height selection", baseVersion: "v1", height: 99,
			upgrades: []Upgrade{{Height: 100, Image: "app:v2", Status: UpgradeCompleted}}, want: "v1",
		},
		{
			name: "unmatched anchor does not override explicit base", baseVersion: "v4", height: 99, appVersion: "stale",
			upgrades: []Upgrade{{Height: 100, Image: "app:v2", Status: UpgradeCompleted}}, want: "v4",
		},
		{
			name: "scheduled match is not a committed anchor", baseVersion: "v4", height: 99, appVersion: "v2",
			upgrades: []Upgrade{{Height: 100, Image: "app:v2", Status: UpgradeScheduled}}, want: "v4",
		},
		{
			name: "skipped match is not a committed anchor", baseVersion: "v4", height: 99, appVersion: "v2",
			upgrades: []Upgrade{{Height: 100, Image: "app:v2", Status: UpgradeSkipped}}, want: "v4",
		},
		{
			name: "later eligible completed upgrade wins", baseVersion: "v1", height: 200, appVersion: "v2",
			upgrades: []Upgrade{
				{Height: 100, Image: "app:v2", Status: UpgradeCompleted},
				{Height: 200, Image: "app:v3", Status: UpgradeCompleted},
			},
			want: "v3",
		},
		{
			name: "later ineligible upgrade does not displace anchor", baseVersion: "v1", height: 199, appVersion: "v2",
			upgrades: []Upgrade{
				{Height: 100, Image: "app:v2", Status: UpgradeCompleted},
				{Height: 200, Image: "app:v3", Status: UpgradeCompleted},
			},
			want: "v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &ChainNode{
				Spec: ChainNodeSpec{App: AppSpec{Image: "app", Version: ptr.To(tt.baseVersion)}},
				Status: ChainNodeStatus{
					LatestHeight: tt.height,
					AppVersion:   tt.appVersion,
					Upgrades:     tt.upgrades,
				},
			}

			assert.Equal(t, tt.want, node.GetAppVersion())
		})
	}
}
