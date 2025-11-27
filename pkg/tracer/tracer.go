// Package tracer provides functionality for tracing blockchain store operations
// by reading and parsing trace output from a FIFO pipe.
package tracer

import (
	"context"
	"strings"
	"syscall"

	"github.com/goccy/go-json"

	"github.com/containerd/fifo"
	"github.com/nxadm/tail"
)

// StoreTracer reads and parses store operation traces from a file or FIFO pipe.
type StoreTracer struct {
	tail   *tail.Tail
	Traces chan *Trace
}

// Trace represents a single store operation trace event.
type Trace struct {
	Operation string    `json:"operation"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Metadata  *Metadata `json:"metadata,omitempty"`
	Err       error     `json:"-"`
}

// Metadata contains contextual information about a trace event.
type Metadata struct {
	BlockHeight int64  `json:"blockHeight"`
	StoreName   string `json:"store_name"`
}

// NewStoreTracer creates a new StoreTracer that reads from the specified path.
// If createFifo is true, a FIFO pipe will be created at the path.
// The tracer must be started with Start() to begin processing traces.
func NewStoreTracer(path string, createFifo bool) (*StoreTracer, error) {
	if createFifo {
		f, err := fifo.OpenFifo(context.Background(), path, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0655)
		if err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	t, err := tail.TailFile(path, tail.Config{
		ReOpen: true,
		Pipe:   true,
		Follow: true,
		Logger: tail.DiscardingLogger,
	})
	if err != nil {
		return nil, err
	}

	return &StoreTracer{
		tail:   t,
		Traces: make(chan *Trace),
	}, nil
}

// Stop stops the tracer and closes the underlying file.
func (t *StoreTracer) Stop() error {
	return t.tail.Stop()
}

// Start begins processing trace events and sending them to the Traces channel.
// This method blocks until the tracer is stopped or the file is closed.
// The Traces channel is closed when Start returns, allowing consumers to detect completion.
func (t *StoreTracer) Start() {
	defer close(t.Traces)
	for line := range t.tail.Lines {
		if line.Err != nil {
			t.Traces <- &Trace{Err: line.Err}
			continue
		}

		if strings.TrimSpace(line.Text) != "" {
			trace := Trace{}
			if err := json.Unmarshal([]byte(line.Text), &trace); err != nil {
				t.Traces <- &Trace{Err: err}
			} else {
				t.Traces <- &trace
			}
		}
	}
}
