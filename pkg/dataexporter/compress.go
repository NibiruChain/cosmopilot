package dataexporter

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/klauspost/pgzip"
)

func compressTarGz(dir string, out io.Writer) error {
	// Setup parallel gzip
	gz, err := pgzip.NewWriterLevel(out, pgzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("pgzip writer failed: %v", err)
	}
	defer gz.Close()

	gz.UncompressedSize()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// Walk the directory
	return filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err // skip if error
		}
		if info.IsDir() {
			return nil
		}

		// Create tar header
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return err
		}
		hdr.Name = relPath // set correct name in tar

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write tar header: %v", err)
		}

		// Open file
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}
