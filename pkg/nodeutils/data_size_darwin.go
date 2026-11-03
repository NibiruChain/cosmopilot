//go:build darwin

package nodeutils

import (
	"context"
	"os"
)

func dedicatedFilesystemUsedBytes(context.Context, string, *os.File) (int64, bool, error) {
	return 0, false, nil
}
