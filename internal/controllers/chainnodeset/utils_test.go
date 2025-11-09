package chainnodeset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
)

func TestAddOrUpdateNodeStatus(t *testing.T) {
	tests := []struct {
		name     string
		nodeSet  *appsv1.ChainNodeSet
		status   appsv1.ChainNodeSetNodeStatus
		wantLen  int
		wantName string
	}{
		{
			name: "add to empty list",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: nil,
				},
			},
			status: appsv1.ChainNodeSetNodeStatus{
				Name: "node1",
			},
			wantLen:  1,
			wantName: "node1",
		},
		{
			name: "add new node",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: []appsv1.ChainNodeSetNodeStatus{
						{Name: "node1"},
					},
				},
			},
			status: appsv1.ChainNodeSetNodeStatus{
				Name: "node2",
			},
			wantLen:  2,
			wantName: "node2",
		},
		{
			name: "update existing node",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: []appsv1.ChainNodeSetNodeStatus{
						{Name: "node1", Public: true},
					},
				},
			},
			status: appsv1.ChainNodeSetNodeStatus{
				Name:   "node1",
				Public: false,
			},
			wantLen:  1,
			wantName: "node1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddOrUpdateNodeStatus(tt.nodeSet, tt.status)

			assert.Len(t, tt.nodeSet.Status.Nodes, tt.wantLen)

			// Find the node
			found := false
			for _, node := range tt.nodeSet.Status.Nodes {
				if node.Name == tt.wantName {
					found = true
					// If updating, check that status was updated
					if tt.name == "update existing node" {
						assert.Equal(t, tt.status.Public, node.Public)
					}
					break
				}
			}

			assert.True(t, found)
		})
	}
}

func TestDeleteNodeStatus(t *testing.T) {
	tests := []struct {
		name    string
		nodeSet *appsv1.ChainNodeSet
		delName string
		wantLen int
	}{
		{
			name: "delete from empty list",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: nil,
				},
			},
			delName: "node1",
			wantLen: 0,
		},
		{
			name: "delete existing node",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: []appsv1.ChainNodeSetNodeStatus{
						{Name: "node1"},
						{Name: "node2"},
						{Name: "node3"},
					},
				},
			},
			delName: "node2",
			wantLen: 2,
		},
		{
			name: "delete non-existent node",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: []appsv1.ChainNodeSetNodeStatus{
						{Name: "node1"},
						{Name: "node2"},
					},
				},
			},
			delName: "node3",
			wantLen: 2,
		},
		{
			name: "delete only node",
			nodeSet: &appsv1.ChainNodeSet{
				Status: appsv1.ChainNodeSetStatus{
					Nodes: []appsv1.ChainNodeSetNodeStatus{
						{Name: "node1"},
					},
				},
			},
			delName: "node1",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DeleteNodeStatus(tt.nodeSet, tt.delName)

			actualLen := 0
			if tt.nodeSet.Status.Nodes != nil {
				actualLen = len(tt.nodeSet.Status.Nodes)
			}

			assert.Equal(t, tt.wantLen, actualLen)

			// Verify the deleted node is actually gone
			for _, node := range tt.nodeSet.Status.Nodes {
				assert.NotEqual(t, tt.delName, node.Name)
			}
		})
	}
}

func TestWithChainNodeSetLabels(t *testing.T) {
	tests := []struct {
		name       string
		nodeSet    *appsv1.ChainNodeSet
		additional []map[string]string
		wantKeys   []string
	}{
		{
			name: "basic labels",
			nodeSet: &appsv1.ChainNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "myapp",
						"version": "v1",
					},
				},
			},
			additional: nil,
			wantKeys:   []string{"app", "version"},
		},
		{
			name: "merge additional labels",
			nodeSet: &appsv1.ChainNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			additional: []map[string]string{
				{"environment": "prod"},
				{"region": "us-west"},
			},
			wantKeys: []string{"app", "environment", "region"},
		},
		{
			name: "empty labels",
			nodeSet: &appsv1.ChainNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
			additional: []map[string]string{
				{"custom": "label"},
			},
			wantKeys: []string{"custom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WithChainNodeSetLabels(tt.nodeSet, tt.additional...)

			// Verify expected keys are present
			for _, key := range tt.wantKeys {
				assert.Contains(t, result, key)
			}

			// Verify count matches
			assert.Len(t, result, len(tt.wantKeys))
		})
	}
}

func TestContainsGroup(t *testing.T) {
	tests := []struct {
		name      string
		groups    []appsv1.NodeGroupSpec
		groupName string
		want      bool
	}{
		{
			name: "group exists",
			groups: []appsv1.NodeGroupSpec{
				{Name: "validators"},
				{Name: "sentries"},
			},
			groupName: "validators",
			want:      true,
		},
		{
			name: "group does not exist",
			groups: []appsv1.NodeGroupSpec{
				{Name: "validators"},
			},
			groupName: "sentries",
			want:      false,
		},
		{
			name:      "empty groups",
			groups:    []appsv1.NodeGroupSpec{},
			groupName: "validators",
			want:      false,
		},
		{
			name:      "nil groups",
			groups:    nil,
			groupName: "validators",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsGroup(tt.groups, tt.groupName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContainsGlobalIngress(t *testing.T) {
	tests := []struct {
		name               string
		ingresses          []appsv1.GlobalIngressConfig
		ingressName        string
		ignoreServicesOnly bool
		want               bool
	}{
		{
			name: "ingress exists, not services only",
			ingresses: []appsv1.GlobalIngressConfig{
				{Name: "ingress1"},
			},
			ingressName:        "ingress1",
			ignoreServicesOnly: false,
			want:               true,
		},
		{
			name: "ingress does not exist",
			ingresses: []appsv1.GlobalIngressConfig{
				{Name: "ingress1"},
			},
			ingressName:        "ingress2",
			ignoreServicesOnly: false,
			want:               false,
		},
		{
			name:               "empty ingresses",
			ingresses:          []appsv1.GlobalIngressConfig{},
			ingressName:        "ingress1",
			ignoreServicesOnly: false,
			want:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsGlobalIngress(tt.ingresses, tt.ingressName, tt.ignoreServicesOnly)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddressWithPortFromFullAddress(t *testing.T) {
	tests := []struct {
		name        string
		fullAddress string
		want        string
	}{
		{
			name:        "valid full address",
			fullAddress: "nodeid@example.com:26656",
			want:        "example.com:26656",
		},
		{
			name:        "no @ separator",
			fullAddress: "example.com:26656",
			want:        "example.com:26656",
		},
		{
			name:        "multiple @ separators",
			fullAddress: "id@host@example.com:26656",
			want:        "id@host@example.com:26656",
		},
		{
			name:        "empty string",
			fullAddress: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddressWithPortFromFullAddress(tt.fullAddress)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRemoveIdFromFullAddresses(t *testing.T) {
	tests := []struct {
		name          string
		fullAddresses []string
		want          []string
	}{
		{
			name: "multiple addresses",
			fullAddresses: []string{
				"node1@host1.com:26656",
				"node2@host2.com:26656",
				"node3@host3.com:26656",
			},
			want: []string{
				"host1.com:26656",
				"host2.com:26656",
				"host3.com:26656",
			},
		},
		{
			name:          "empty list",
			fullAddresses: []string{},
			want:          []string{},
		},
		{
			name: "mixed formats",
			fullAddresses: []string{
				"node1@host1.com:26656",
				"host2.com:26656",
			},
			want: []string{
				"host1.com:26656",
				"host2.com:26656",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveIdFromFullAddresses(tt.fullAddresses)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetGlobalIngressLabels(t *testing.T) {
	tests := []struct {
		name     string
		nodeSet  *appsv1.ChainNodeSet
		group    string
		wantKeys []string
	}{
		{
			name: "ingress for group",
			nodeSet: &appsv1.ChainNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodeset",
				},
				Spec: appsv1.ChainNodeSetSpec{
					Ingresses: []appsv1.GlobalIngressConfig{
						{
							Name:   "rpc",
							Groups: []string{"validators", "sentries"},
						},
					},
				},
			},
			group:    "validators",
			wantKeys: []string{"test-nodeset-global-rpc"},
		},
		{
			name: "no ingress for group",
			nodeSet: &appsv1.ChainNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodeset",
				},
				Spec: appsv1.ChainNodeSetSpec{
					Ingresses: []appsv1.GlobalIngressConfig{
						{
							Name:   "rpc",
							Groups: []string{"validators"},
						},
					},
				},
			},
			group:    "seeds",
			wantKeys: []string{},
		},
		{
			name: "multiple ingresses for group",
			nodeSet: &appsv1.ChainNodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodeset",
				},
				Spec: appsv1.ChainNodeSetSpec{
					Ingresses: []appsv1.GlobalIngressConfig{
						{
							Name:   "rpc",
							Groups: []string{"validators"},
						},
						{
							Name:   "api",
							Groups: []string{"validators"},
						},
					},
				},
			},
			group:    "validators",
			wantKeys: []string{"test-nodeset-global-rpc", "test-nodeset-global-api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetGlobalIngressLabels(tt.nodeSet, tt.group)

			assert.Len(t, got, len(tt.wantKeys))

			for _, key := range tt.wantKeys {
				assert.Contains(t, got, key)
			}
		})
	}
}
