package nodeutils

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplerTestTimeout = 5 * time.Second

func waitForSamplerSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(samplerTestTimeout):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func runSamplerForTest(t *testing.T, sampler *dataSizeSampler, release ...func()) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { sampler.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		for _, unblock := range release {
			unblock()
		}
		waitForSamplerSignal(t, done, "sampler worker shutdown")
	})
	return cancel
}

func TestDataSizeSamplerSharesOneMeasurementAndCachesSuccess(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	releaseMeasurement := sync.OnceFunc(func() { close(release) })
	var calls atomic.Int32
	sampler := newDataSizeSampler("/data", func(ctx context.Context, _ string) (int64, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-release:
			return 123, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})
	runSamplerForTest(t, sampler, releaseMeasurement)
	const clients = 32
	clientsCtx, cancelClients := context.WithCancel(t.Context())
	clientsDone := make(chan struct{})
	results := make(chan int64, clients)
	var completed atomic.Int32
	for range clients {
		go func() {
			defer func() {
				if completed.Add(1) == clients {
					close(clientsDone)
				}
			}()
			got, err := sampler.Size(clientsCtx)
			if err == nil {
				results <- got
			}
		}()
	}
	t.Cleanup(func() {
		cancelClients()
		releaseMeasurement()
		waitForSamplerSignal(t, clientsDone, "shared-measurement clients")
	})
	waitForSamplerSignal(t, entered, "shared measurement start")
	releaseMeasurement()
	waitForSamplerSignal(t, clientsDone, "shared-measurement clients")
	close(results)
	count := 0
	for got := range results {
		assert.Equal(t, int64(123), got)
		count++
	}
	assert.Equal(t, clients, count)
	assert.Equal(t, int32(1), calls.Load())
	got, err := sampler.Size(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(123), got)
	assert.Equal(t, int32(1), calls.Load())
}

func TestDataSizeSamplerExpiryFailureAndRetry(t *testing.T) {
	var calls atomic.Int32
	now := time.Now()
	sampler := newDataSizeSampler("/data", func(context.Context, string) (int64, error) {
		switch calls.Add(1) {
		case 1:
			return 10, nil
		case 2:
			return 0, errors.New("scan failed")
		default:
			return 30, nil
		}
	})
	sampler.now = func() time.Time { return now }
	runSamplerForTest(t, sampler)
	got, err := sampler.Size(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(10), got)
	now = now.Add(dataSizeFreshness)
	got, err = sampler.Size(t.Context())
	assert.Error(t, err)
	assert.Zero(t, got)
	got, err = sampler.Size(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(30), got)
	assert.Equal(t, int32(3), calls.Load())
}

func TestDataSizeSamplerWaitTimeoutDoesNotCancelMeasurement(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	releaseMeasurement := sync.OnceFunc(func() { close(release) })
	var calls atomic.Int32
	sampler := newDataSizeSampler("/data", func(ctx context.Context, _ string) (int64, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return 77, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})
	sampler.waitLimit = 20 * time.Millisecond
	runSamplerForTest(t, sampler, releaseMeasurement)
	_, err := sampler.Size(t.Context())
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	waitForSamplerSignal(t, entered, "timed-out measurement start")
	releaseMeasurement()
	require.Eventually(t, func() bool {
		sampler.mu.Lock()
		defer sampler.mu.Unlock()
		return sampler.hasCache
	}, time.Second, time.Millisecond)
	got, err := sampler.Size(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(77), got)
	assert.Equal(t, int32(1), calls.Load())
}

func TestDataSizeSamplerCancellationReleasesWaiters(t *testing.T) {
	entered := make(chan struct{})
	sampler := newDataSizeSampler("/data", func(ctx context.Context, _ string) (int64, error) {
		close(entered)
		<-ctx.Done()
		return 0, ctx.Err()
	})
	cancelWorker := runSamplerForTest(t, sampler)
	result := make(chan error, 1)
	go func() { _, err := sampler.Size(t.Context()); result <- err }()
	waitForSamplerSignal(t, entered, "canceled measurement start")
	cancelWorker()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(samplerTestTimeout):
		t.Fatal("timed out waiting for canceled caller")
	}
	_, err := sampler.Size(t.Context())
	assert.ErrorIs(t, err, errDataSizeStopped)
}

func TestDataSizeSamplerCanceledCompletionStopsBeforeReleasingFlight(t *testing.T) {
	sampler := newDataSizeSampler("/data", func(context.Context, string) (int64, error) {
		t.Fatal("canceled completion must not start another measurement")
		return 0, nil
	})
	flight := &dataSizeFlight{done: make(chan struct{})}
	sampler.flight = flight
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.False(t, sampler.finishFlight(ctx, flight, 0, context.Canceled))
	select {
	case <-flight.done:
	default:
		t.Fatal("canceled flight remained open")
	}
	assert.ErrorIs(t, flight.err, context.Canceled)
	_, err := sampler.Size(t.Context())
	assert.ErrorIs(t, err, errDataSizeStopped)
}
