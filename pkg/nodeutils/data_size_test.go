package nodeutils

import (
	"context"
	"crypto/rand"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSizeWithRealFilesystem(t *testing.T) {
	const fileSize = int64(32 << 20)
	parent := t.TempDir()
	dataPath := filepath.Join(parent, "data")
	sibling := filepath.Join(parent, "sibling")
	require.NoError(t, os.Mkdir(dataPath, 0o700))
	require.NoError(t, os.Mkdir(sibling, 0o700))
	writeRandomFile(t, filepath.Join(dataPath, "allocated"), fileSize)
	server := newDataSizeTestServer(t, dataPath, measureDataSize)
	assertDataSizeResponse(t, server, fileSize)
	writeRandomFile(t, filepath.Join(sibling, "unrelated"), fileSize)
	server.dataSizeSampler.mu.Lock()
	server.dataSizeSampler.cachedAt = time.Now().Add(-time.Hour)
	server.dataSizeSampler.mu.Unlock()
	assertDataSizeResponse(t, server, fileSize)
	sparse, err := os.Create(filepath.Join(dataPath, "sparse"))
	require.NoError(t, err)
	require.NoError(t, sparse.Truncate(1<<20))
	require.NoError(t, sparse.Close())
	require.NoError(t, os.Symlink(filepath.Join(sibling, "unrelated"), filepath.Join(dataPath, "link")))
	server.dataSizeSampler.mu.Lock()
	server.dataSizeSampler.cachedAt = time.Now().Add(-time.Hour)
	server.dataSizeSampler.mu.Unlock()
	assertDataSizeResponse(t, server, fileSize+(1<<20)+int64(len(filepath.Join(sibling, "unrelated"))))
}

func writeRandomFile(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = io.CopyN(f, rand.Reader, size)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())
}

func TestUsedBytesFromBlocks(t *testing.T) {
	tests := []struct {
		name            string
		blocks, free    uint64
		block, fragment int64
		want            int64
		wantErr         bool
	}{
		{"fragment", 10, 5, 4096, 1024, 5120, false},
		{"fallback block", 10, 5, 4096, 0, 20480, false},
		{"zero block", 10, 5, 0, 0, 0, true},
		{"negative fragment", 10, 5, 4096, -1, 0, true},
		{"invalid counters", 10, 11, 4096, 0, 0, true},
		{"overflow", math.MaxUint64, 0, 4096, 2, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := usedBytesFromBlocks(tt.blocks, tt.free, tt.block, tt.fragment)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDataSizeMissingRoot(t *testing.T) {
	server := newDataSizeTestServer(t, filepath.Join(t.TempDir(), "missing"), measureDataSize)
	response := requestDataSize(server)
	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestSameDataRootRejectsReplacementWithSymlink(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "data")
	replacement := filepath.Join(parent, "replacement")
	require.NoError(t, os.Mkdir(root, 0o700))
	require.NoError(t, os.Mkdir(replacement, 0o700))
	opened, err := os.Stat(root)
	require.NoError(t, err)
	assert.True(t, sameDataRoot(root, opened))
	require.NoError(t, os.Rename(root, filepath.Join(parent, "old")))
	require.NoError(t, os.Symlink(replacement, root))
	assert.False(t, sameDataRoot(root, opened))
}

func TestMeasureDataSizeDoesNotFollowRootSymlink(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(target, "payload"), []byte("payload with a different size"), 0o600))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))
	for _, suffix := range []string{"", "/", "///"} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			proofCalled := false
			got, err := measureDataSizeWithProof(t.Context(), link+suffix, func(context.Context, string, *os.File) (int64, bool, error) {
				proofCalled = true
				return 999, true, nil
			})
			require.NoError(t, err)
			assert.Equal(t, int64(len(target)), got)
			assert.False(t, proofCalled)
		})
	}
}

func TestSameDataRootRejectsSameInodeSymlink(t *testing.T) {
	root := t.TempDir()
	opened, err := os.Stat(root)
	require.NoError(t, err)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))
	for _, suffix := range []string{"", "/", "///"} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			assert.False(t, sameDataRoot(link+suffix, opened))
		})
	}
}

func TestTrimTrailingPathSeparators(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""},
		{"/", "/"},
		{"///", "/"},
		{"/data///", "/data"},
		{"parent-link/../child/", "parent-link/../child"},
	} {
		t.Run(strconv.Quote(tc.input), func(t *testing.T) {
			assert.Equal(t, tc.want, trimTrailingPathSeparators(tc.input))
		})
	}
}

func TestMeasureDataSizeDirectoryTrailingSeparatorsAndEmptyRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("five!"), 0o600))
	for _, suffix := range []string{"/", "///"} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			got, err := measureDataSizeWithProof(t.Context(), root+suffix, func(context.Context, string, *os.File) (int64, bool, error) {
				return 0, false, nil
			})
			require.NoError(t, err)
			assert.Equal(t, int64(5), got)
		})
	}
	_, err := measureDataSize(t.Context(), "")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func newDataSizeTestServer(t *testing.T, path string, measure dataSizeMeasureFunc) *NodeUtils {
	t.Helper()
	server := &NodeUtils{cfg: &Options{DataPath: path}, router: mux.NewRouter(), dataSizeSampler: newDataSizeSampler(path, measure)}
	runSamplerForTest(t, server.dataSizeSampler)
	server.registerRoutes()
	return server
}

func requestDataSize(server *NodeUtils) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/data_size", nil))
	return response
}

func assertDataSizeResponse(t *testing.T, server *NodeUtils, want int64) {
	t.Helper()
	response := requestDataSize(server)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	got, err := strconv.ParseInt(response.Body.String(), 10, 64)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
