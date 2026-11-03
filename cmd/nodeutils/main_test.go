package main

import (
	"errors"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voluzi/cosmopilot/v2/pkg/nodeutils"
)

func TestTerminationSignalTerminatesSidecarOnSecondSignalAfterHeldStop(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	t.Cleanup(func() { close(sigChan) })
	sigChan <- syscall.SIGTERM
	var stops atomic.Int32
	var exits atomic.Int32
	done := make(chan error, 1)

	go func() {
		done <- handleTerminationSignals(
			sigChan,
			func(force bool) (nodeutils.StopResult, error) {
				assert.False(t, force)
				stops.Add(1)
				return nodeutils.StopHeld, nil
			},
			func() { exits.Add(1) },
		)
	}()
	require.Eventually(t, func() bool { return stops.Load() == 1 }, time.Second, time.Millisecond)
	sigChan <- syscall.SIGTERM

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal handler")
	}
	assert.Equal(t, int32(1), exits.Load())
}

func TestTerminationSignalReturnsAfterCompletedStop(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	sigChan <- syscall.SIGTERM
	var exits atomic.Int32

	err := handleTerminationSignals(
		sigChan,
		func(bool) (nodeutils.StopResult, error) { return nodeutils.StopCompleted, nil },
		func() { exits.Add(1) },
	)

	require.NoError(t, err)
	assert.Zero(t, exits.Load())
}

func TestTerminationSignalCallbackErrorStillHonorsSubsequentSignal(t *testing.T) {
	sigChan := make(chan os.Signal, 2)
	sigChan <- syscall.SIGTERM
	sigChan <- syscall.SIGTERM
	var exits atomic.Int32
	err := handleTerminationSignals(
		sigChan,
		func(bool) (nodeutils.StopResult, error) {
			return nodeutils.StopCompleted, errors.New("callback failed")
		},
		func() { exits.Add(1) },
	)
	require.ErrorContains(t, err, "callback failed")
	assert.Equal(t, int32(1), exits.Load())
}
