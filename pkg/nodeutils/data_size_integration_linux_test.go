//go:build linux

package nodeutils

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// The opt-in path must be a disposable, otherwise idle filesystem fixture.
func TestDataSizeMountIntegration(t *testing.T) {
	path := os.Getenv("COSMOPILOT_DATA_SIZE_TEST_PATH")
	if path == "" {
		t.Skip("requires disposable Linux mount fixtures")
	}
	fastValue := os.Getenv("COSMOPILOT_DATA_SIZE_TEST_FAST")
	require.NotEmpty(t, fastValue, "COSMOPILOT_DATA_SIZE_TEST_FAST is required when COSMOPILOT_DATA_SIZE_TEST_PATH is set")
	wantFast, err := strconv.ParseBool(fastValue)
	require.NoError(t, err, "COSMOPILOT_DATA_SIZE_TEST_FAST must be a valid boolean")
	root, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	_, fast, err := dedicatedFilesystemUsedBytes(context.Background(), path, root)
	require.NoError(t, err)
	require.Equal(t, wantFast, fast, "unexpected mount qualification for %s", path)
	size, err := measureDataSize(context.Background(), path)
	require.NoError(t, err)
	require.GreaterOrEqual(t, size, int64(0))
	if expected := os.Getenv("COSMOPILOT_DATA_SIZE_TEST_BYTES"); expected != "" {
		want, err := strconv.ParseInt(expected, 10, 64)
		require.NoError(t, err)
		require.Equal(t, want, size)
	}
}

func TestDataSizeExt4RandomFileDelta(t *testing.T) {
	path := os.Getenv("COSMOPILOT_DATA_SIZE_TEST_DELTA_PATH")
	if path == "" {
		t.Skip("requires disposable idle ext4 filesystem")
	}
	const fileSize = int64(32 << 20)
	const tolerance = int64(1 << 20)
	ctx := context.Background()
	root, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	before, fast, err := dedicatedFilesystemUsedBytes(ctx, path, root)
	require.NoError(t, err)
	require.True(t, fast)
	file, err := os.CreateTemp(path, "data-size-random-*")
	require.NoError(t, err)
	name := file.Name()
	t.Cleanup(func() { require.NoError(t, os.Remove(name)) })
	n, writeErr := io.CopyN(file, rand.Reader, fileSize)
	syncErr := file.Sync()
	closeErr := file.Close()
	require.NoError(t, writeErr)
	require.NoError(t, syncErr)
	require.NoError(t, closeErr)
	require.Equal(t, fileSize, n)
	info, err := os.Stat(filepath.Clean(name))
	require.NoError(t, err)
	require.Equal(t, fileSize, info.Size())
	after, fast, err := dedicatedFilesystemUsedBytes(ctx, path, root)
	require.NoError(t, err)
	require.True(t, fast)
	delta := after - before
	require.GreaterOrEqual(t, delta, fileSize-tolerance)
	require.LessOrEqual(t, delta, fileSize+tolerance)
	t.Logf("32 MiB random-file allocation delta: %d bytes (tolerance %d)", delta, tolerance)
}
