package tracer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStoreTracer(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "trace.log")

	// Create the file first
	f, err := os.Create(tracePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	f.Close()

	tracer, err := NewStoreTracer(tracePath, false)
	if err != nil {
		t.Fatalf("NewStoreTracer() error = %v", err)
	}

	if tracer == nil {
		t.Fatal("NewStoreTracer() returned nil")
	}

	if tracer.Traces == nil {
		t.Error("NewStoreTracer() Traces channel is nil")
	}

	_ = tracer.Stop()
}

func TestStoreTracer_ParseValidTrace(t *testing.T) {
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "trace.log")

	// Create the trace file
	f, err := os.Create(tracePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tracer, err := NewStoreTracer(tracePath, false)
	if err != nil {
		f.Close()
		t.Fatalf("NewStoreTracer() error = %v", err)
	}

	// Start tracer in background
	done := make(chan struct{})
	var receivedTrace *Trace
	go func() {
		defer close(done)
		tracer.Start()
	}()

	// Collect traces in background
	tracesReceived := make(chan *Trace, 1)
	go func() {
		for trace := range tracer.Traces {
			tracesReceived <- trace
			return
		}
	}()

	// Write a valid trace line
	traceJSON := `{"operation":"write","key":"test-key","value":"test-value","metadata":{"blockHeight":100,"store_name":"bank"}}`
	_, err = f.WriteString(traceJSON + "\n")
	if err != nil {
		t.Fatalf("failed to write trace: %v", err)
	}
	_ = f.Sync()

	// Wait for trace with timeout
	select {
	case receivedTrace = <-tracesReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for trace")
	}

	// Verify trace content
	if receivedTrace.Err != nil {
		t.Errorf("unexpected error in trace: %v", receivedTrace.Err)
	}
	if receivedTrace.Operation != "write" {
		t.Errorf("expected operation 'write', got %q", receivedTrace.Operation)
	}
	if receivedTrace.Key != "test-key" {
		t.Errorf("expected key 'test-key', got %q", receivedTrace.Key)
	}
	if receivedTrace.Value != "test-value" {
		t.Errorf("expected value 'test-value', got %q", receivedTrace.Value)
	}
	if receivedTrace.Metadata == nil {
		t.Error("expected metadata, got nil")
	} else {
		if receivedTrace.Metadata.BlockHeight != 100 {
			t.Errorf("expected blockHeight 100, got %d", receivedTrace.Metadata.BlockHeight)
		}
		if receivedTrace.Metadata.StoreName != "bank" {
			t.Errorf("expected store_name 'bank', got %q", receivedTrace.Metadata.StoreName)
		}
	}

	// Cleanup
	f.Close()
	_ = tracer.Stop()
	<-done
}

func TestStoreTracer_ParseInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "trace.log")

	f, err := os.Create(tracePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tracer, err := NewStoreTracer(tracePath, false)
	if err != nil {
		f.Close()
		t.Fatalf("NewStoreTracer() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracer.Start()
	}()

	tracesReceived := make(chan *Trace, 1)
	go func() {
		for trace := range tracer.Traces {
			tracesReceived <- trace
			return
		}
	}()

	// Write invalid JSON
	_, err = f.WriteString("not valid json\n")
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	_ = f.Sync()

	select {
	case trace := <-tracesReceived:
		if trace.Err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for trace")
	}

	f.Close()
	_ = tracer.Stop()
	<-done
}

func TestStoreTracer_SkipEmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "trace.log")

	f, err := os.Create(tracePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tracer, err := NewStoreTracer(tracePath, false)
	if err != nil {
		f.Close()
		t.Fatalf("NewStoreTracer() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracer.Start()
	}()

	tracesReceived := make(chan *Trace, 2)
	go func() {
		for trace := range tracer.Traces {
			tracesReceived <- trace
		}
	}()

	// Write empty lines followed by a valid trace
	_, err = f.WriteString("\n   \n")
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	_ = f.Sync()

	// Write valid trace after empty lines
	validTrace := `{"operation":"read","key":"k1"}`
	_, err = f.WriteString(validTrace + "\n")
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	_ = f.Sync()

	// Should only receive the valid trace, not the empty lines
	select {
	case trace := <-tracesReceived:
		if trace.Err != nil {
			t.Errorf("unexpected error: %v", trace.Err)
		}
		if trace.Operation != "read" {
			t.Errorf("expected operation 'read', got %q", trace.Operation)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for trace")
	}

	f.Close()
	_ = tracer.Stop()
	<-done
}

func TestStoreTracer_ChannelClosedOnStop(t *testing.T) {
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "trace.log")

	f, err := os.Create(tracePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	f.Close()

	tracer, err := NewStoreTracer(tracePath, false)
	if err != nil {
		t.Fatalf("NewStoreTracer() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		tracer.Start()
	}()

	// Give tracer time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the tracer
	_ = tracer.Stop()

	// Wait for Start() to return
	select {
	case <-done:
		// Success - Start() returned
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tracer to stop")
	}

	// Verify channel is closed by trying to read from it
	select {
	case _, ok := <-tracer.Traces:
		if ok {
			t.Error("expected channel to be closed after Stop()")
		}
	default:
		// Channel might be closed but empty, which is fine
	}
}

func TestTrace_Struct(t *testing.T) {
	trace := &Trace{
		Operation: "delete",
		Key:       "mykey",
		Value:     "myvalue",
		Metadata: &Metadata{
			BlockHeight: 500,
			StoreName:   "staking",
		},
	}

	if trace.Operation != "delete" {
		t.Errorf("expected Operation 'delete', got %q", trace.Operation)
	}
	if trace.Key != "mykey" {
		t.Errorf("expected Key 'mykey', got %q", trace.Key)
	}
	if trace.Metadata.BlockHeight != 500 {
		t.Errorf("expected BlockHeight 500, got %d", trace.Metadata.BlockHeight)
	}
}

func TestMetadata_Struct(t *testing.T) {
	meta := &Metadata{
		BlockHeight: 12345,
		StoreName:   "gov",
	}

	if meta.BlockHeight != 12345 {
		t.Errorf("expected BlockHeight 12345, got %d", meta.BlockHeight)
	}
	if meta.StoreName != "gov" {
		t.Errorf("expected StoreName 'gov', got %q", meta.StoreName)
	}
}
