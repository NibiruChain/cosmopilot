package nodeutils

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestScanDataSizeBatchesAndCountsLogicalEntries(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(nested, 0o700))
	for i := range 300 {
		require.NoError(t, os.WriteFile(filepath.Join(root, strconv.Itoa(i)), []byte("abc"), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(nested, "child"), []byte("12345"), 0o600))
	external := filepath.Join(outside, "large")
	require.NoError(t, os.WriteFile(external, make([]byte, 4096), 0o600))
	link := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(external, link))
	sparse := filepath.Join(root, "sparse")
	file, err := os.Create(sparse)
	require.NoError(t, err)
	require.NoError(t, file.Truncate(1<<20))
	require.NoError(t, file.Close())
	got, err := scanDataSize(t.Context(), root, scanLimits{maxDepth: 4, batchSize: 7})
	require.NoError(t, err)
	assert.Equal(t, int64(300*3+5+(1<<20)+len(external)), got)
}

func TestScanDataSizeRejectsMissingRootAndDepthLimit(t *testing.T) {
	root := t.TempDir()
	_, err := scanDataSize(t.Context(), filepath.Join(root, "missing"), defaultScanLimits)
	assert.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.Mkdir(filepath.Join(root, "child"), 0o700))
	_, err = scanDataSize(t.Context(), root, scanLimits{maxDepth: 1, batchSize: 2})
	assert.ErrorContains(t, err, "data directory depth exceeds 1")
}

func TestScanDataSizeDoesNotFollowRootSymlink(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(target, "file"), []byte("long data"), 0o600))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))
	for _, suffix := range []string{"", "/", "///"} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			got, err := scanDataSize(t.Context(), link+suffix, defaultScanLimits)
			require.NoError(t, err)
			assert.Equal(t, int64(len(target)), got)
		})
	}
}

func TestScanDataSizeStopsOnCancellation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := scanDataSize(ctx, root, defaultScanLimits)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestScanDataSizeCancelsWithNestedDescriptorsOpen(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	require.NoError(t, os.Mkdir(nested, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "counted"), []byte("partial bytes"), 0o600))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var openFDs []uintptr
	limits := scanLimits{maxDepth: 4, batchSize: 1, afterCount: func(depth int, fds []uintptr) {
		if depth == 2 {
			openFDs = append([]uintptr(nil), fds...)
			cancel()
		}
	}}
	got, err := scanDataSize(ctx, root, limits)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, got)
	require.Len(t, openFDs, 2)
	for _, fd := range openFDs {
		_, err := unix.FcntlInt(fd, unix.F_GETFD, 0)
		assert.ErrorIs(t, err, unix.EBADF)
	}
}
