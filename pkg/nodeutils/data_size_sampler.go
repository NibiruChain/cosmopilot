package nodeutils

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	dataSizeFreshness    = 15 * time.Second
	dataSizeWaitLimit    = 25 * time.Second
	dataSizeMeasureLimit = 10 * time.Minute
)

var errDataSizeStopped = errors.New("data size sampler stopped")

type dataSizeMeasureFunc func(context.Context, string) (int64, error)

type dataSizeFlight struct {
	done chan struct{}
	size int64
	err  error
}

type dataSizeSampler struct {
	path       string
	measure    dataSizeMeasureFunc
	now        func() time.Time
	waitLimit  time.Duration
	wake       chan struct{}
	mu         sync.Mutex
	flight     *dataSizeFlight
	cachedSize int64
	cachedAt   time.Time
	hasCache   bool
	stopped    bool
}

func newDataSizeSampler(path string, measure dataSizeMeasureFunc) *dataSizeSampler {
	return &dataSizeSampler{path: path, measure: measure, now: time.Now, waitLimit: dataSizeWaitLimit, wake: make(chan struct{}, 1)}
}

func (s *dataSizeSampler) Size(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return 0, errDataSizeStopped
	}
	if s.hasCache && s.now().Sub(s.cachedAt) < dataSizeFreshness {
		size := s.cachedSize
		s.mu.Unlock()
		return size, nil
	}
	flight := s.flight
	if flight == nil {
		flight = &dataSizeFlight{done: make(chan struct{})}
		s.flight = flight
		s.wake <- struct{}{}
	}
	s.mu.Unlock()
	// A caller's deadline only ends its wait, not the shared measurement.
	waitCtx, cancel := context.WithTimeout(ctx, s.waitLimit)
	defer cancel()
	select {
	case <-flight.done:
		return flight.size, flight.err
	case <-waitCtx.Done():
		return 0, waitCtx.Err()
	}
}

func (s *dataSizeSampler) Run(ctx context.Context) {
	workerDone := make(chan struct{})
	defer close(workerDone)
	go func() {
		select {
		case <-ctx.Done():
			s.stop(ctx.Err())
		case <-workerDone:
		}
	}()
	for {
		select {
		case <-ctx.Done():
			s.stop(ctx.Err())
			return
		case <-s.wake:
		}
		s.mu.Lock()
		if err := ctx.Err(); err != nil {
			s.stopLocked(err)
			s.mu.Unlock()
			return
		}
		flight := s.flight
		stopped := s.stopped
		s.mu.Unlock()
		if stopped || flight == nil {
			return
		}
		measureCtx, cancel := context.WithTimeout(ctx, dataSizeMeasureLimit)
		size, err := s.measure(measureCtx, s.path)
		cancel()
		if !s.finishFlight(ctx, flight, size, err) {
			return
		}
	}
}

func (s *dataSizeSampler) finishFlight(ctx context.Context, flight *dataSizeFlight, size int64, err error) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if serviceErr := ctx.Err(); serviceErr != nil {
		s.stopLocked(serviceErr)
		return false
	}
	if s.stopped || s.flight != flight {
		return false
	}
	flight.size, flight.err = size, err
	if err == nil {
		s.cachedSize, s.cachedAt, s.hasCache = size, s.now(), true
	}
	s.flight = nil
	close(flight.done)
	return true
}

func (s *dataSizeSampler) stop(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked(err)
}

func (s *dataSizeSampler) stopLocked(err error) {
	if s.stopped {
		return
	}
	s.stopped = true
	if s.flight != nil {
		s.flight.err = err
		close(s.flight.done)
		s.flight = nil
	}
}
