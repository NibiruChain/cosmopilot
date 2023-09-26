package tracer

import (
	"context"
	"encoding/json"
	"strings"
	"syscall"

	"github.com/containerd/fifo"
	"github.com/nxadm/tail"
)

type StoreTracer struct {
	tail   *tail.Tail
	Traces chan *Trace
}

type Trace struct {
	Operation string    `json:"operation"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Metadata  *Metadata `json:"metadata,omitempty"`
	Err       error     `json:"-"`
}

type Metadata struct {
	BlockHeight int64  `json:"blockHeight"`
	StoreName   string `json:"store_name"`
}

func NewStoreTracer(path string) (*StoreTracer, error) {
	f, err := fifo.OpenFifo(context.Background(), path, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0655)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
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

func (t *StoreTracer) Stop() error {
	return t.tail.Stop()
}

func (t *StoreTracer) Start() {
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
