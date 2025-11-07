package chainnode

import (
	"testing"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/controllers"
)

func TestGetConfigHash(t *testing.T) {
	tests := []struct {
		name      string
		configs   map[string]interface{}
		wantError bool
	}{
		{
			name: "simple config",
			configs: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantError: false,
		},
		{
			name: "nested config",
			configs: map[string]interface{}{
				"app": map[string]interface{}{
					"setting1": "value1",
					"setting2": 123,
				},
			},
			wantError: false,
		},
		{
			name:      "empty config",
			configs:   map[string]interface{}{},
			wantError: false,
		},
		{
			name: "config with slices",
			configs: map[string]interface{}{
				"list": []string{"item1", "item2", "item3"},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := getConfigHash(tt.configs)
			if (err != nil) != tt.wantError {
				t.Errorf("getConfigHash() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && hash == "" {
				t.Error("getConfigHash() returned empty hash")
			}
		})
	}
}

func TestGetConfigHash_Consistency(t *testing.T) {
	config1 := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	config2 := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	hash1, err1 := getConfigHash(config1)
	hash2, err2 := getConfigHash(config2)

	if err1 != nil || err2 != nil {
		t.Fatalf("getConfigHash() failed: %v, %v", err1, err2)
	}

	if hash1 != hash2 {
		t.Errorf("getConfigHash() inconsistent for same configs: %s vs %s", hash1, hash2)
	}
}

func TestGetConfigHash_DifferentForDifferentConfigs(t *testing.T) {
	config1 := map[string]interface{}{
		"key1": "value1",
	}
	config2 := map[string]interface{}{
		"key1": "value2",
	}

	hash1, err1 := getConfigHash(config1)
	hash2, err2 := getConfigHash(config2)

	if err1 != nil || err2 != nil {
		t.Fatalf("getConfigHash() failed: %v, %v", err1, err2)
	}

	if hash1 == hash2 {
		t.Error("getConfigHash() returned same hash for different configs")
	}
}

func TestGetMostRecentHeightFromServicesAnnotations(t *testing.T) {
	tests := []struct {
		name            string
		annotationsList []map[string]string
		wantHeight      int64
		wantHash        string
	}{
		{
			name: "single annotation",
			annotationsList: []map[string]string{
				{
					controllers.AnnotationStateSyncTrustHeight: "100",
					controllers.AnnotationStateSyncTrustHash:   "hash100",
				},
			},
			wantHeight: 100,
			wantHash:   "hash100",
		},
		{
			name: "multiple annotations, find max",
			annotationsList: []map[string]string{
				{
					controllers.AnnotationStateSyncTrustHeight: "100",
					controllers.AnnotationStateSyncTrustHash:   "hash100",
				},
				{
					controllers.AnnotationStateSyncTrustHeight: "200",
					controllers.AnnotationStateSyncTrustHash:   "hash200",
				},
				{
					controllers.AnnotationStateSyncTrustHeight: "150",
					controllers.AnnotationStateSyncTrustHash:   "hash150",
				},
			},
			wantHeight: 200,
			wantHash:   "hash200",
		},
		{
			name:            "empty annotations",
			annotationsList: []map[string]string{},
			wantHeight:      0,
			wantHash:        "",
		},
		{
			name: "missing height annotations",
			annotationsList: []map[string]string{
				{
					"other-annotation": "value",
				},
			},
			wantHeight: 0,
			wantHash:   "",
		},
		{
			name: "invalid height format",
			annotationsList: []map[string]string{
				{
					controllers.AnnotationStateSyncTrustHeight: "not-a-number",
					controllers.AnnotationStateSyncTrustHash:   "hash",
				},
			},
			wantHeight: 0,
			wantHash:   "",
		},
		{
			name: "mixed valid and invalid heights",
			annotationsList: []map[string]string{
				{
					controllers.AnnotationStateSyncTrustHeight: "invalid",
					controllers.AnnotationStateSyncTrustHash:   "hash1",
				},
				{
					controllers.AnnotationStateSyncTrustHeight: "100",
					controllers.AnnotationStateSyncTrustHash:   "hash100",
				},
			},
			wantHeight: 100,
			wantHash:   "hash100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			height, hash := getMostRecentHeightFromServicesAnnotations(tt.annotationsList)
			if height != tt.wantHeight {
				t.Errorf("getMostRecentHeightFromServicesAnnotations() height = %v, want %v", height, tt.wantHeight)
			}
			if hash != tt.wantHash {
				t.Errorf("getMostRecentHeightFromServicesAnnotations() hash = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestGetExternalAddress(t *testing.T) {
	tests := []struct {
		name        string
		chainNode   *appsv1.ChainNode
		wantAddress string
		wantOk      bool
	}{
		{
			name: "valid public address",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					PublicAddress: "nodeid@example.com:26656",
				},
			},
			wantAddress: "example.com:26656",
			wantOk:      true,
		},
		{
			name: "empty public address",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					PublicAddress: "",
				},
			},
			wantAddress: "",
			wantOk:      false,
		},
		{
			name: "invalid format - no @",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					PublicAddress: "example.com:26656",
				},
			},
			wantAddress: "",
			wantOk:      false,
		},
		{
			name: "invalid format - multiple @",
			chainNode: &appsv1.ChainNode{
				Status: appsv1.ChainNodeStatus{
					PublicAddress: "nodeid@host@example.com:26656",
				},
			},
			wantAddress: "",
			wantOk:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, ok := getExternalAddress(tt.chainNode)
			if address != tt.wantAddress {
				t.Errorf("getExternalAddress() address = %v, want %v", address, tt.wantAddress)
			}
			if ok != tt.wantOk {
				t.Errorf("getExternalAddress() ok = %v, want %v", ok, tt.wantOk)
			}
		})
	}
}

func TestNewConfigLockManager(t *testing.T) {
	clm := newConfigLockManager()
	if clm == nil {
		t.Fatal("newConfigLockManager() returned nil")
	}
	if clm.locks == nil {
		t.Error("newConfigLockManager() did not initialize locks map")
	}
}

func TestConfigLockManager_GetLockForVersion(t *testing.T) {
	clm := newConfigLockManager()

	// Get lock for version1
	lock1 := clm.getLockForVersion("v1.0.0")
	if lock1 == nil {
		t.Fatal("getLockForVersion() returned nil for v1.0.0")
	}

	// Get lock for same version
	lock2 := clm.getLockForVersion("v1.0.0")
	if lock2 == nil {
		t.Fatal("getLockForVersion() returned nil for v1.0.0 (second call)")
	}

	// Should return same lock instance for same version
	if lock1 != lock2 {
		t.Error("getLockForVersion() returned different locks for same version")
	}

	// Get lock for different version
	lock3 := clm.getLockForVersion("v2.0.0")
	if lock3 == nil {
		t.Fatal("getLockForVersion() returned nil for v2.0.0")
	}

	// Should return different lock for different version
	if lock1 == lock3 {
		t.Error("getLockForVersion() returned same lock for different versions")
	}
}

func TestConfigLockManager_CapacityLimit(t *testing.T) {
	clm := newConfigLockManager()

	// Fill up to capacity
	locks := make(map[string]interface{})
	for i := 0; i < maxConfigLocks; i++ {
		version := string(rune('a' + i))
		lock := clm.getLockForVersion(version)
		locks[version] = lock
	}

	// Should have maxConfigLocks entries
	if len(clm.locks) != maxConfigLocks {
		t.Errorf("Expected %d locks, got %d", maxConfigLocks, len(clm.locks))
	}

	// Try to add one more
	extraLock := clm.getLockForVersion("extra-version")
	if extraLock == nil {
		t.Fatal("getLockForVersion() returned nil when at capacity")
	}

	// Should still have maxConfigLocks entries (reused existing lock)
	if len(clm.locks) > maxConfigLocks {
		t.Errorf("Lock map grew beyond capacity: %d > %d", len(clm.locks), maxConfigLocks)
	}
}
