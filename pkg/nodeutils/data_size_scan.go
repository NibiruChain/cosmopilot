package nodeutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

type scanLimits struct {
	maxDepth   int
	batchSize  int
	afterCount func(depth int, openFDs []uintptr)
}

var defaultScanLimits = scanLimits{maxDepth: 128, batchSize: 256}

type scanFrame struct {
	file  *os.File
	names []string
	next  int
	end   bool
}

func scanDataSize(ctx context.Context, path string, limits scanLimits) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	path = trimTrailingPathSeparators(path)
	info, err := os.Lstat(path)
	if err != nil {
		return 0, fmt.Errorf("stat data root %q: %w", path, err)
	}
	if !info.IsDir() {
		if info.Size() < 0 {
			return 0, fmt.Errorf("negative size at %q", path)
		}
		return info.Size(), nil
	}
	if limits.maxDepth < 1 || limits.batchSize < 1 || limits.batchSize > 256 {
		return 0, fmt.Errorf("invalid scan limits")
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, fmt.Errorf("open data root %q: %w", path, err)
	}
	frames := []scanFrame{{file: os.NewFile(uintptr(fd), path)}}
	defer func() {
		for i := range frames {
			_ = frames[i].file.Close()
		}
	}()
	var size int64
	for len(frames) > 0 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		top := &frames[len(frames)-1]
		if top.next == len(top.names) {
			if top.end {
				_ = top.file.Close()
				frames = frames[:len(frames)-1]
				continue
			}
			names, readErr := top.file.Readdirnames(limits.batchSize)
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return 0, fmt.Errorf("read directory %q: %w", top.file.Name(), readErr)
			}
			top.names, top.next, top.end = names, 0, errors.Is(readErr, io.EOF)
			if len(names) == 0 && !top.end {
				return 0, fmt.Errorf("empty directory batch at %q", top.file.Name())
			}
			continue
		}
		name := top.names[top.next]
		top.next++
		var st unix.Stat_t
		if err := unix.Fstatat(int(top.file.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return 0, fmt.Errorf("stat %q: %w", filepath.Join(top.file.Name(), name), err)
		}
		if st.Mode&syscall.S_IFMT == syscall.S_IFDIR {
			if len(frames) >= limits.maxDepth {
				return 0, fmt.Errorf("data directory depth exceeds %d", limits.maxDepth)
			}
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			childFD, err := unix.Openat(int(top.file.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return 0, fmt.Errorf("open directory %q: %w", filepath.Join(top.file.Name(), name), err)
			}
			frames = append(frames, scanFrame{file: os.NewFile(uintptr(childFD), filepath.Join(top.file.Name(), name))})
			continue
		}
		if st.Size < 0 || size > math.MaxInt64-st.Size {
			return 0, fmt.Errorf("data size overflow at %q", name)
		}
		size += st.Size
		if limits.afterCount != nil {
			fds := make([]uintptr, len(frames))
			for i := range frames {
				fds[i] = frames[i].file.Fd()
			}
			limits.afterCount(len(frames), fds)
		}
	}
	return size, nil
}
