package chainnode

import (
	"testing"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

func TestGetUpgrade(t *testing.T) {
	r := &Reconciler{}

	tests := []struct {
		name      string
		chainNode *appsv1.ChainNode
		height    int64
		want      *appsv1.Upgrade
	}{
		{
			name: "finds matching scheduled upgrade",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					Upgrades: []appsv1.Upgrade{
						{Height: 100, Image: "myapp:v1", Status: appsv1.UpgradeScheduled},
						{Height: 200, Image: "myapp:v2", Status: appsv1.UpgradeScheduled},
						{Height: 300, Image: "myapp:v3", Status: appsv1.UpgradeScheduled},
					},
				},
			},
			height: 200,
			want:   &appsv1.Upgrade{Height: 200, Image: "myapp:v2", Status: appsv1.UpgradeScheduled},
		},
		{
			name: "no matching upgrade",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					Upgrades: []appsv1.Upgrade{
						{Height: 100, Image: "myapp:v1", Status: appsv1.UpgradeScheduled},
						{Height: 200, Image: "myapp:v2", Status: appsv1.UpgradeScheduled},
					},
				},
			},
			height: 150,
			want:   nil,
		},
		{
			name: "empty upgrades list",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					Upgrades: []appsv1.Upgrade{},
				},
			},
			height: 100,
			want:   nil,
		},
		{
			name: "finds scheduled upgrade at height",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					Upgrades: []appsv1.Upgrade{
						{Height: 100, Image: "myapp:v1", Status: appsv1.UpgradeScheduled},
					},
				},
			},
			height: 100,
			want:   &appsv1.Upgrade{Height: 100, Image: "myapp:v1", Status: appsv1.UpgradeScheduled},
		},
		{
			name: "ignores completed upgrades",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					Upgrades: []appsv1.Upgrade{
						{Height: 100, Image: "myapp:v1", Status: appsv1.UpgradeCompleted},
						{Height: 200, Image: "myapp:v2", Status: appsv1.UpgradeScheduled},
					},
				},
			},
			height: 100,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.getUpgrade(tt.chainNode, tt.height)
			if tt.want == nil {
				if got != nil {
					t.Errorf("getUpgrade() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("getUpgrade() = nil, want %v", tt.want)
				return
			}
			if got.Height != tt.want.Height || got.Image != tt.want.Image || got.Status != tt.want.Status {
				t.Errorf("getUpgrade() = {Height: %d, Image: %s, Status: %s}, want {Height: %d, Image: %s, Status: %s}",
					got.Height, got.Image, got.Status, tt.want.Height, tt.want.Image, tt.want.Status)
			}
		})
	}
}

func TestAddOrUpdateUpgrade(t *testing.T) {
	tests := []struct {
		name          string
		upgrades      []appsv1.Upgrade
		upgrade       appsv1.Upgrade
		currentHeight int64
		wantLen       int
	}{
		{
			name:          "add new upgrade to empty list",
			upgrades:      []appsv1.Upgrade{},
			upgrade:       appsv1.Upgrade{Height: 100, Image: "myapp:v1"},
			currentHeight: 50,
			wantLen:       1,
		},
		{
			name: "add new upgrade to existing list",
			upgrades: []appsv1.Upgrade{
				{Height: 100, Image: "myapp:v1"},
			},
			upgrade:       appsv1.Upgrade{Height: 200, Image: "myapp:v2"},
			currentHeight: 50,
			wantLen:       2,
		},
		{
			name: "update existing upgrade with ImageMissing status",
			upgrades: []appsv1.Upgrade{
				{Height: 100, Image: "", Status: appsv1.UpgradeImageMissing},
			},
			upgrade:       appsv1.Upgrade{Height: 100, Image: "new:v1", Source: appsv1.OnChainUpgrade},
			currentHeight: 50,
			wantLen:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddOrUpdateUpgrade(tt.upgrades, tt.upgrade, tt.currentHeight)
			if len(result) != tt.wantLen {
				t.Errorf("AddOrUpdateUpgrade() returned %d upgrades, want %d", len(result), tt.wantLen)
			}
		})
	}
}
