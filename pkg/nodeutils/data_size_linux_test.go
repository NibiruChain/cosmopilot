//go:build linux

package nodeutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestMountProof(t *testing.T) {
	base := []mountRecord{{id: 1, parent: 1, root: "/", point: "/", fsType: "overlay"}, {id: 2, parent: 1, root: "/", point: "/data", fsType: "ext4"}}
	cases := []struct {
		name    string
		records []mountRecord
		want    bool
	}{
		{"whole root", base, true},
		{"subdirectory bind", []mountRecord{base[0], {id: 2, parent: 1, root: "/partial", point: "/data", fsType: "ext4"}}, false},
		{"nested mount", append(append([]mountRecord{}, base...), mountRecord{id: 3, parent: 2, root: "/", point: "/data/nested", fsType: "tmpfs"}), false},
		{"file bind", append(append([]mountRecord{}, base...), mountRecord{id: 3, parent: 2, root: "/file", point: "/data/file", fsType: "ext4"}), false},
		{"stacked mount", append(append([]mountRecord{}, base...), mountRecord{id: 3, parent: 1, root: "/", point: "/data", fsType: "tmpfs"}), false},
		{"sibling prefix", append(append([]mountRecord{}, base...), mountRecord{id: 3, parent: 1, root: "/", point: "/data-other", fsType: "tmpfs"}), true},
		{"wrong filesystem", []mountRecord{base[0], {id: 2, parent: 1, root: "/", point: "/data", fsType: "xfs"}}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) { assert.Equal(t, tt.want, qualifiedMount(tt.records, 2, "/data")) })
	}
	assert.False(t, qualifiedMount(base, 3, "/data"))
	assert.False(t, qualifiedMount(base, 2, "/data/subdir"))
}

func TestMountInfoParsing(t *testing.T) {
	records, err := parseMountInfo([]byte("1 1 8:1 / / rw - ext4 /dev/disk rw\n2 1 8:1 / /my\\040data\\011tab\\012line\\134slash rw shared:1 - ext4 /dev/disk rw\n"))
	require.NoError(t, err)
	assert.Equal(t, "/my data\ttab\nline\\slash", records[1].point)
	for _, input := range []string{
		"", "1 1 8:1 / / rw ext4 /dev/disk rw\n", "1 1 8:1 / / rw - ext4 /dev/disk rw",
		"1 1 8:1 / / rw - ext4 /dev/disk rw\n1 1 8:1 / /other rw - ext4 /dev/disk rw\n",
		"1 1 8:1 / /bad\\999 rw - ext4 /dev/disk rw\n",
	} {
		_, err := parseMountInfo([]byte(input))
		assert.Error(t, err, input)
	}
}

func TestMountInfoAcceptsZeroIDs(t *testing.T) {
	for _, input := range []string{
		"0 0 8:1 / / rw - ext4 /dev/disk rw\n",
		"1 0 8:1 / / rw - ext4 /dev/disk rw\n",
	} {
		records, err := parseMountInfo([]byte(input))
		require.NoError(t, err)
		require.Len(t, records, 1)
	}
	for _, input := range []string{
		"-1 0 8:1 / / rw - ext4 /dev/disk rw\n",
		"0 -1 8:1 / / rw - ext4 /dev/disk rw\n",
		"bad 0 8:1 / / rw - ext4 /dev/disk rw\n",
		"0 0 8:1 / / rw - ext4 /dev/disk rw\n0 0 8:1 / /other rw - ext4 /dev/disk rw\n",
	} {
		_, err := parseMountInfo([]byte(input))
		assert.Error(t, err, input)
	}
}

func TestParseMountID(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    string
		want    int
		wantErr bool
	}{
		{"zero", "pos:\t0\nmnt_id:\t0\n", 0, false},
		{"positive", "mnt_id:\t2\n", 2, false},
		{"negative", "mnt_id:\t-1\n", 0, true},
		{"malformed", "mnt_id:\tbad\n", 0, true},
		{"absent", "pos:\t0\n", 0, true},
		{"duplicate zero", "mnt_id:\t0\nmnt_id:\t0\n", 0, true},
		{"duplicate positive", "mnt_id:\t2\nmnt_id:\t2\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMountID([]byte(tc.data))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFilesystemProofChangesFallBackToFreshDirectoryScan(t *testing.T) {
	tests := []struct {
		name          string
		change        func(*filesystemProofOps, string, string)
		wantQualified bool
		wantSize      int64
	}{
		{"stable proof", func(*filesystemProofOps, string, string) {}, true, 5120},
		{"mountinfo", func(ops *filesystemProofOps, _, _ string) {
			first := ops.mountInfo
			calls := 0
			ops.mountInfo = func() ([]byte, error) {
				calls++
				data, err := first()
				if calls == 2 {
					return append(data, []byte("3 1 0:2 / /unrelated rw - tmpfs tmpfs rw\n")...), err
				}
				return data, err
			}
		}, false, 5},
		{"mount ID", func(ops *filesystemProofOps, _, _ string) {
			calls := 0
			ops.mountID = func(int) (int, error) {
				calls++
				if calls == 2 {
					return 3, nil
				}
				return 2, nil
			}
		}, false, 5},
		{"quota flags", func(ops *filesystemProofOps, _, _ string) {
			calls := 0
			ops.flags = func(int) (int, error) {
				calls++
				if calls == 2 {
					return fsProjInheritFlag, nil
				}
				return 0, nil
			}
		}, false, 5},
		{"configured path", func(ops *filesystemProofOps, root, other string) {
			calls := 0
			ops.resolve = func(string) (string, error) {
				calls++
				if calls == 2 {
					return other, nil
				}
				return root, nil
			}
		}, false, 5},
		{"reopened inode", func(ops *filesystemProofOps, _, other string) {
			ops.openDir = func(string) (*os.File, error) { return os.Open(other) }
		}, false, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			other := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "owned"), []byte("owned"), 0o600))
			mountInfo := []byte(fmt.Sprintf("1 1 0:1 / / rw - overlay overlay rw\n2 1 8:1 / %s rw - ext4 /dev/test rw\n", root))
			ops := filesystemProofOps{
				resolve:   func(string) (string, error) { return root, nil },
				mountID:   func(int) (int, error) { return 2, nil },
				mountInfo: func() ([]byte, error) { return mountInfo, nil },
				flags:     func(int) (int, error) { return 0, nil },
				statfs: func(_ int, stats *syscall.Statfs_t) error {
					stats.Type = unix.EXT4_SUPER_MAGIC
					stats.Blocks, stats.Bfree, stats.Bsize, stats.Frsize = 10, 5, 4096, 1024
					return nil
				},
				openDir: os.Open,
			}
			tt.change(&ops, root, other)
			proofCalled, qualified := false, false
			var proofErr error
			got, err := measureDataSizeWithProof(t.Context(), root, func(ctx context.Context, path string, opened *os.File) (int64, bool, error) {
				proofCalled = true
				bytes, fast, err := dedicatedFilesystemUsedBytesWithOps(ctx, path, opened, ops)
				qualified = fast
				proofErr = err
				return bytes, fast, err
			})
			require.NoError(t, err)
			assert.True(t, proofCalled)
			assert.NoError(t, proofErr)
			assert.Equal(t, tt.wantQualified, qualified)
			if tt.wantQualified {
				assert.Equal(t, tt.wantSize, got)
			} else {
				assert.Equal(t, int64(len("owned")), got)
			}
		})
	}
}
