package dataexporter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
	}{
		{
			name:     "unsupported provider",
			provider: Provider("unsupported"),
			wantErr:  true,
		},
		// GCS provider test would require actual GCS credentials
		// so we skip it in unit tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromProvider(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDirSize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	subDir := filepath.Join(tmpDir, "subdir")
	file3 := filepath.Join(subDir, "file3.txt")

	_ = os.WriteFile(file1, []byte("hello"), 0644)
	_ = os.WriteFile(file2, []byte("world!!!"), 0644)
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(file3, []byte("nested content"), 0644)

	size, err := GetDirSize(tmpDir)
	if err != nil {
		t.Fatalf("GetDirSize() error = %v", err)
	}

	// Size should be at least the sum of our files (5 + 8 + 14 = 27 bytes)
	expectedMin := 27
	if int(size) < expectedMin {
		t.Errorf("GetDirSize() = %d, want at least %d", size, expectedMin)
	}
}

func TestGetDirSize_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	size, err := GetDirSize(tmpDir)
	if err != nil {
		t.Fatalf("GetDirSize() error = %v", err)
	}

	// Empty directory should have size 0
	if size != 0 {
		t.Errorf("GetDirSize() = %d, want 0 for empty directory", size)
	}
}

func TestGetDigitCount(t *testing.T) {
	tests := []struct {
		name   string
		maxVal int
		want   int
	}{
		{"zero", 0, 1},
		{"negative", -5, 1},
		{"single digit", 5, 1},
		{"nine", 9, 1},
		{"ten", 10, 2},
		{"99", 99, 2},
		{"100", 100, 3},
		{"999", 999, 3},
		{"1000", 1000, 4},
		{"large number", 999999, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDigitCount(tt.maxVal)
			if got != tt.want {
				t.Errorf("getDigitCount(%d) = %d, want %d", tt.maxVal, got, tt.want)
			}
		})
	}
}

func TestCompressTarGz(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create test files
	testContent := map[string]string{
		"file1.txt":        "hello world",
		"subdir/file2.txt": "nested content",
	}

	for relPath, content := range testContent {
		fullPath := filepath.Join(tmpDir, relPath)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Compress
	var buf bytes.Buffer
	err := compressTarGz(tmpDir, &buf)
	if err != nil {
		t.Fatalf("compressTarGz() error = %v", err)
	}

	// Verify we got a valid gzip
	gzReader, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	// Verify we got a valid tar
	tarReader := tar.NewReader(gzReader)
	foundFiles := make(map[string]string)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar: %v", err)
		}

		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("failed to read tar content: %v", err)
		}
		foundFiles[header.Name] = string(content)
	}

	// Verify all expected files are present
	for relPath, expectedContent := range testContent {
		actualContent, ok := foundFiles[relPath]
		if !ok {
			t.Errorf("missing file in archive: %s", relPath)
			continue
		}
		if actualContent != expectedContent {
			t.Errorf("file %s content = %q, want %q", relPath, actualContent, expectedContent)
		}
	}
}

func TestCompressTarGz_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	var buf bytes.Buffer
	err := compressTarGz(tmpDir, &buf)
	if err != nil {
		t.Fatalf("compressTarGz() error = %v", err)
	}

	// Should still produce valid gzip output
	gzReader, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	gzReader.Close()
}

func TestCompressTarGz_NonExistentDir(t *testing.T) {
	var buf bytes.Buffer
	err := compressTarGz("/nonexistent/dir", &buf)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestUploadOptions_Defaults(t *testing.T) {
	opts := defaultUploadOptions()

	if opts.ChunkSize.String() != "250MB" {
		t.Errorf("default ChunkSize = %s, want 250MB", opts.ChunkSize.String())
	}
	if opts.PartSize.String() != "500GB" {
		t.Errorf("default PartSize = %s, want 500GB", opts.PartSize.String())
	}
	if opts.SizeLimit.String() != "5TB" {
		t.Errorf("default SizeLimit = %s, want 5TB", opts.SizeLimit.String())
	}
	if opts.BufferSize.String() != "32MB" {
		t.Errorf("default BufferSize = %s, want 32MB", opts.BufferSize.String())
	}
	if opts.ReportPeriod != time.Second {
		t.Errorf("default ReportPeriod = %v, want %v", opts.ReportPeriod, time.Second)
	}
	if opts.ConcurrentJobs != 10 {
		t.Errorf("default ConcurrentJobs = %d, want 10", opts.ConcurrentJobs)
	}
}

func TestUploadOptions_WithFunctions(t *testing.T) {
	opts := defaultUploadOptions()

	WithChunkSize("100MB")(opts)
	if opts.ChunkSize.String() != "100MB" {
		t.Errorf("WithChunkSize() ChunkSize = %s, want 100MB", opts.ChunkSize.String())
	}

	WithPartSize("1GB")(opts)
	if opts.PartSize.String() != "1GB" {
		t.Errorf("WithPartSize() PartSize = %s, want 1GB", opts.PartSize.String())
	}

	WithSizeLimit("10GB")(opts)
	if opts.SizeLimit.String() != "10GB" {
		t.Errorf("WithSizeLimit() SizeLimit = %s, want 10GB", opts.SizeLimit.String())
	}

	WithBufferSize("64MB")(opts)
	if opts.BufferSize.String() != "64MB" {
		t.Errorf("WithBufferSize() BufferSize = %s, want 64MB", opts.BufferSize.String())
	}

	WithReportPeriod(5 * time.Second)(opts)
	if opts.ReportPeriod != 5*time.Second {
		t.Errorf("WithReportPeriod() ReportPeriod = %v, want 5s", opts.ReportPeriod)
	}

	WithConcurrentUploadJobs(20)(opts)
	if opts.ConcurrentJobs != 20 {
		t.Errorf("WithConcurrentUploadJobs() ConcurrentJobs = %d, want 20", opts.ConcurrentJobs)
	}
}

func TestDeleteOptions_Defaults(t *testing.T) {
	opts := defaultDeleteOptions()

	if opts.ConcurrentJobs != 10 {
		t.Errorf("default ConcurrentJobs = %d, want 10", opts.ConcurrentJobs)
	}
}

func TestDeleteOptions_WithFunctions(t *testing.T) {
	opts := defaultDeleteOptions()

	WithConcurrentDeleteJobs(50)(opts)
	if opts.ConcurrentJobs != 50 {
		t.Errorf("WithConcurrentDeleteJobs() ConcurrentJobs = %d, want 50", opts.ConcurrentJobs)
	}
}

func TestProviderConstant(t *testing.T) {
	if GCS != Provider("gcs") {
		t.Errorf("GCS constant = %q, want %q", GCS, "gcs")
	}
}

func TestCompositionBatchLimit(t *testing.T) {
	if CompositionBatchLimit != 32 {
		t.Errorf("CompositionBatchLimit = %d, want 32", CompositionBatchLimit)
	}
}
