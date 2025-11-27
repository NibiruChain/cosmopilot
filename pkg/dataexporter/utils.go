package dataexporter

import (
	"fmt"
	"math"

	"github.com/c2h5oh/datasize"

	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

// GetDirSize calculates the total size of a directory and its contents.
func GetDirSize(path string) (datasize.ByteSize, error) {
	totalSize, err := utils.DirSize(path)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size: %v", err)
	}
	return datasize.ByteSize(totalSize), nil
}

func getDigitCount(maxVal int) int {
	if maxVal <= 0 {
		return 1 // At least one digit needed
	}
	return int(math.Log10(float64(maxVal))) + 1
}
