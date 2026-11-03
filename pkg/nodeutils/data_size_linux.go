//go:build linux

package nodeutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	maxMountInfoBytes = 8 << 20
	maxMountLineBytes = 1 << 20
	maxFDInfoBytes    = 64 << 10
	fsProjInheritFlag = 0x20000000
)

type mountRecord struct {
	id, parent          int
	root, point, fsType string
}

type filesystemProofOps struct {
	resolve   func(string) (string, error)
	mountID   func(int) (int, error)
	mountInfo func() ([]byte, error)
	flags     func(int) (int, error)
	statfs    func(int, *syscall.Statfs_t) error
	openDir   func(string) (*os.File, error)
}

func dedicatedFilesystemUsedBytes(ctx context.Context, path string, root *os.File) (int64, bool, error) {
	return dedicatedFilesystemUsedBytesWithOps(ctx, path, root, filesystemProofOps{
		resolve: func(path string) (string, error) {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return "", err
			}
			return filepath.Abs(resolved)
		},
		mountID:   readMountID,
		mountInfo: func() ([]byte, error) { return readBoundedFile("/proc/self/mountinfo", maxMountInfoBytes) },
		flags:     func(fd int) (int, error) { return unix.IoctlGetInt(fd, unix.FS_IOC_GETFLAGS) },
		statfs:    syscall.Fstatfs,
		openDir: func(path string) (*os.File, error) {
			fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err != nil {
				return nil, err
			}
			return os.NewFile(uintptr(fd), path), nil
		},
	})
}

func dedicatedFilesystemUsedBytesWithOps(ctx context.Context, path string, root *os.File, ops filesystemProofOps) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	canonical, err := ops.resolve(path)
	if err != nil {
		return 0, false, nil
	}
	fd := int(root.Fd())
	id, err := ops.mountID(fd)
	if err != nil {
		return 0, false, nil
	}
	first, err := ops.mountInfo()
	if err != nil {
		return 0, false, nil
	}
	mounts, err := parseMountInfo(first)
	if err != nil || !qualifiedMount(mounts, id, canonical) {
		return 0, false, nil
	}
	flags, err := ops.flags(fd)
	if err != nil || flags&fsProjInheritFlag != 0 {
		return 0, false, nil
	}
	var stats syscall.Statfs_t
	if err := ops.statfs(fd, &stats); err != nil {
		return 0, false, nil
	}
	if stats.Type != unix.EXT4_SUPER_MAGIC {
		return 0, false, nil
	}
	size, err := usedBytesFromBlocks(uint64(stats.Blocks), uint64(stats.Bfree), int64(stats.Bsize), int64(stats.Frsize))
	if err != nil {
		return 0, false, err
	}
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	second, err := ops.mountInfo()
	if err != nil || !bytes.Equal(first, second) {
		return 0, false, nil
	}
	secondID, err := ops.mountID(fd)
	if err != nil || secondID != id {
		return 0, false, nil
	}
	secondFlags, err := ops.flags(fd)
	if err != nil || secondFlags != flags {
		return 0, false, nil
	}
	reResolved, err := ops.resolve(path)
	if err != nil || reResolved != canonical {
		return 0, false, nil
	}
	reopened, err := ops.openDir(canonical)
	if err != nil {
		return 0, false, nil
	}
	defer reopened.Close()
	newID, err := ops.mountID(int(reopened.Fd()))
	if err != nil || newID != id {
		return 0, false, nil
	}
	before, err := root.Stat()
	if err != nil {
		return 0, false, nil
	}
	after, err := reopened.Stat()
	if err != nil || !os.SameFile(before, after) {
		return 0, false, nil
	}
	return size, true, nil
}

func readBoundedFile(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, max)
	}
	return data, nil
}

func readMountID(fd int) (int, error) {
	data, err := readBoundedFile(fmt.Sprintf("/proc/self/fdinfo/%d", fd), maxFDInfoBytes)
	if err != nil {
		return 0, err
	}
	return parseMountID(data)
}

func parseMountID(data []byte) (int, error) {
	id := 0
	found := false
	var err error
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "mnt_id:") {
			continue
		}
		if found {
			return 0, fmt.Errorf("duplicate mount ID")
		}
		found = true
		value := strings.TrimSpace(strings.TrimPrefix(line, "mnt_id:"))
		id, err = strconv.Atoi(value)
		if err != nil || id < 0 {
			return 0, fmt.Errorf("invalid mount ID %q", value)
		}
	}
	if !found {
		return 0, fmt.Errorf("mount ID absent")
	}
	return id, nil
}

func parseMountInfo(data []byte) ([]mountRecord, error) {
	if len(data) == 0 || len(data) > maxMountInfoBytes {
		return nil, fmt.Errorf("invalid mountinfo length")
	}
	if data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("unterminated mountinfo")
	}
	records := make([]mountRecord, 0)
	ids := make(map[int]bool)
	for _, line := range bytes.Split(data[:len(data)-1], []byte{'\n'}) {
		if len(line) == 0 || len(line) > maxMountLineBytes {
			return nil, fmt.Errorf("invalid mountinfo line length")
		}
		fields := strings.Fields(string(line))
		separator := -1
		for i, field := range fields {
			if field == "-" {
				if separator >= 0 {
					return nil, fmt.Errorf("ambiguous mountinfo separator")
				}
				separator = i
			}
		}
		if separator < 6 || len(fields)-separator < 4 {
			return nil, fmt.Errorf("malformed mountinfo line")
		}
		id, idErr := strconv.Atoi(fields[0])
		parent, parentErr := strconv.Atoi(fields[1])
		if idErr != nil || parentErr != nil || id < 0 || parent < 0 || ids[id] {
			return nil, fmt.Errorf("invalid or duplicate mount ID")
		}
		ids[id] = true
		if !strings.Contains(fields[2], ":") {
			return nil, fmt.Errorf("invalid mount device")
		}
		root, err := decodeMountPath(fields[3])
		if err != nil {
			return nil, err
		}
		point, err := decodeMountPath(fields[4])
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(root) || !filepath.IsAbs(point) || filepath.Clean(root) != root || filepath.Clean(point) != point {
			return nil, fmt.Errorf("invalid mount path")
		}
		records = append(records, mountRecord{id: id, parent: parent, root: root, point: point, fsType: fields[separator+1]})
	}
	return records, nil
}

func decodeMountPath(value string) (string, error) {
	var decoded strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			decoded.WriteByte(value[i])
			continue
		}
		if i+3 >= len(value) {
			return "", fmt.Errorf("short mount escape")
		}
		switch value[i+1 : i+4] {
		case "040":
			decoded.WriteByte(' ')
		case "011":
			decoded.WriteByte('\t')
		case "012":
			decoded.WriteByte('\n')
		case "134":
			decoded.WriteByte('\\')
		default:
			return "", fmt.Errorf("invalid mount escape")
		}
		i += 3
	}
	return decoded.String(), nil
}

func strictDescendant(path, parent string) bool {
	if parent == "/" {
		return path != "/" && strings.HasPrefix(path, "/")
	}
	return strings.HasPrefix(path, parent+"/")
}

func qualifiedMount(records []mountRecord, mountID int, path string) bool {
	var candidate *mountRecord
	byID := make(map[int]mountRecord, len(records))
	for i := range records {
		byID[records[i].id] = records[i]
		if records[i].id == mountID {
			candidate = &records[i]
		}
	}
	if candidate == nil || candidate.point != path || candidate.root != "/" || candidate.fsType != "ext4" {
		return false
	}
	for _, record := range records {
		if record.id != mountID && (record.point == path || strictDescendant(record.point, path)) {
			return false
		}
	}
	if parent, ok := byID[candidate.parent]; ok && parent.point != path && !strictDescendant(path, parent.point) {
		return false
	}
	return true
}
