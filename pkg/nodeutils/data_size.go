package nodeutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func measureDataSize(ctx context.Context, path string) (int64, error) {
	return measureDataSizeWithProof(ctx, path, dedicatedFilesystemUsedBytes)
}

type filesystemProofFunc func(context.Context, string, *os.File) (int64, bool, error)

func trimTrailingPathSeparators(path string) string {
	if path == "" {
		return path
	}
	trimmed := strings.TrimRight(path, string(os.PathSeparator))
	if trimmed == "" {
		return string(os.PathSeparator)
	}
	return trimmed
}

func measureDataSizeWithProof(ctx context.Context, path string, proof filesystemProofFunc) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	path = trimTrailingPathSeparators(path)
	info, err := os.Lstat(path)
	if err != nil {
		return 0, fmt.Errorf("stat data root %q: %w", path, err)
	}
	if !info.IsDir() {
		return scanDataSize(ctx, path, defaultScanLimits)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("absolute data path %q: %w", path, err)
	}
	canonical, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return 0, fmt.Errorf("resolve data path %q: %w", path, err)
	}
	fd, err := unix.Open(canonical, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, fmt.Errorf("open data root %q: %w", canonical, err)
	}
	root := os.NewFile(uintptr(fd), canonical)
	defer root.Close()
	openedInfo, err := root.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat open data root %q: %w", canonical, err)
	}
	if !os.SameFile(info, openedInfo) {
		return scanDataSize(ctx, path, defaultScanLimits)
	}
	if size, qualified, err := proof(ctx, absPath, root); err == nil && qualified && sameDataRoot(path, openedInfo) {
		return size, nil
	}
	return scanDataSize(ctx, path, defaultScanLimits)
}

func sameDataRoot(path string, openedInfo os.FileInfo) bool {
	current, err := os.Lstat(trimTrailingPathSeparators(path))
	return err == nil && current.IsDir() && os.SameFile(current, openedInfo)
}

func usedBytesFromBlocks(blocks, freeBlocks uint64, blockSize, fragmentSize int64) (int64, error) {
	if fragmentSize == 0 {
		fragmentSize = blockSize
	}
	if fragmentSize <= 0 {
		return 0, fmt.Errorf("invalid filesystem fragment size: %d", fragmentSize)
	}
	if freeBlocks > blocks {
		return 0, fmt.Errorf("invalid filesystem counters: %d free blocks exceed %d total blocks", freeBlocks, blocks)
	}

	usedBlocks := blocks - freeBlocks
	bytesPerBlock := uint64(fragmentSize)
	const maxInt64 = uint64(1<<63 - 1)
	if usedBlocks > maxInt64/bytesPerBlock {
		return 0, fmt.Errorf("filesystem used bytes overflow int64")
	}

	return int64(usedBlocks * bytesPerBlock), nil
}
