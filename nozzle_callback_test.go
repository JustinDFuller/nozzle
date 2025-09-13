package nozzle_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

// TestSlowCallbackDoesNotBlockTicker verifies that a slow OnStateChange callback
// doesn't prevent the ticker from continuing to process state calculations.
func TestSlowCallbackDoesNotBlockTicker(t *testing.T) {
	t.Parallel()

	var (
		callbackStarted   atomic.Bool
		callbackCompleted atomic.Bool
		tickCount         atomic.Int32
	)

	// Create nozzle with a very short interval
	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(_ context.Context, _ nozzle.StateSnapshot) {
			callbackStarted.Store(true)
			// Simulate a slow callback that takes longer than the interval
			time.Sleep(100 * time.Millisecond)
			callbackCompleted.Store(true)
		},
	})
	if err != nil {
		t.Fatalf("Failed to create nozzle: %v", err)
	}

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Track when ticks happen by monitoring state changes
	go func() {
		for i := range 15 {
			// Force operations to trigger state changes
			noz.DoBool(func() (any, bool) {
				return nil, i%2 == 0
			})
			time.Sleep(10 * time.Millisecond)
			tickCount.Add(1)
		}
	}()

	// Wait for callback to start
	deadline := time.Now().Add(500 * time.Millisecond)
	for !callbackStarted.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if !callbackStarted.Load() {
		t.Fatal("Callback never started")
	}

	// Record tick count when callback is still running
	ticksWhileCallbackRunning := tickCount.Load()

	// Wait for callback to complete
	deadline = time.Now().Add(500 * time.Millisecond)
	for !callbackCompleted.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if !callbackCompleted.Load() {
		t.Fatal("Callback never completed")
	}

	finalTicks := tickCount.Load()

	// Verify that ticks continued while callback was running
	// We expect at least a few ticks to have occurred during the 100ms callback
	ticksDuringCallback := finalTicks - ticksWhileCallbackRunning
	if ticksDuringCallback < 2 {
		t.Errorf("Expected ticker to continue during slow callback, but only got %d ticks", ticksDuringCallback)
	}

	t.Logf("Ticks during callback: %d", ticksDuringCallback)
}

// TestCallbackPanicRecovery verifies that panics in callbacks are recovered
// and don't crash the program or affect nozzle operation.
func TestCallbackPanicRecovery(t *testing.T) {
	t.Parallel()

	var (
		panicCallbackInvoked atomic.Bool
		normalCallbackCount  atomic.Int32
	)

	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(_ context.Context, _ nozzle.StateSnapshot) {
			if !panicCallbackInvoked.Load() {
				panicCallbackInvoked.Store(true)
				panic("intentional test panic")
			}
			normalCallbackCount.Add(1)
		},
	})
	if err != nil {
		t.Fatalf("Failed to create nozzle: %v", err)
	}

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Trigger state changes
	for i := range 50 {
		noz.DoBool(func() (any, bool) {
			return nil, i%3 == 0
		})

		if i%10 == 0 {
			time.Sleep(15 * time.Millisecond)
		}
	}

	// Wait a bit for callbacks to process
	time.Sleep(100 * time.Millisecond)

	// Verify the panic happened and was recovered
	if !panicCallbackInvoked.Load() {
		t.Error("Panic callback was never invoked")
	}

	// Verify subsequent callbacks still work
	if normalCallbackCount.Load() == 0 {
		t.Error("No normal callbacks executed after panic")
	}

	// Verify nozzle is still operational
	_, ok := noz.DoBool(func() (any, bool) {
		return nil, true
	})
	if !ok {
		t.Error("Nozzle stopped working after callback panic")
	}

	t.Logf("Normal callbacks after panic: %d", normalCallbackCount.Load())
}

// TestCallbackContextCancellation verifies that callbacks receive a cancelled
// context when the nozzle is closed.
func TestCallbackContextCancellation(t *testing.T) {
	t.Parallel()

	var (
		callbackStarted     atomic.Bool
		contextWasCancelled atomic.Bool
	)

	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(ctx context.Context, _ nozzle.StateSnapshot) {
			callbackStarted.Store(true)

			// Wait a bit then check if context is cancelled
			time.Sleep(50 * time.Millisecond)

			select {
			case <-ctx.Done():
				contextWasCancelled.Store(true)
			default:
				// Context not cancelled
			}
		},
	})
	if err != nil {
		t.Fatalf("Failed to create nozzle: %v", err)
	}

	// Trigger a state change
	for range 10 {
		noz.DoBool(func() (any, bool) {
			return nil, false
		})
	}

	// Wait for callback to start
	deadline := time.Now().Add(200 * time.Millisecond)
	for !callbackStarted.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if !callbackStarted.Load() {
		t.Fatal("Callback never started")
	}

	// Close the nozzle while callback is running
	if err := noz.Close(); err != nil {
		t.Errorf("Failed to close nozzle: %v", err)
	}

	// Wait a bit for callback to detect cancellation
	time.Sleep(100 * time.Millisecond)

	if !contextWasCancelled.Load() {
		t.Error("Callback context was not cancelled when nozzle closed")
	}
}

// TestCallbackTimestampAccuracy verifies that the Timestamp field
// accurately reflects when the state change occurred.
func TestCallbackTimestampAccuracy(t *testing.T) {
	t.Parallel()

	var (
		mutex      sync.Mutex
		timestamps []time.Time
	)

	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(_ context.Context, snapshot nozzle.StateSnapshot) {
			mutex.Lock()
			timestamps = append(timestamps, snapshot.Timestamp)
			mutex.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Failed to create nozzle: %v", err)
	}

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Trigger state changes
	for i := range 30 {
		noz.DoBool(func() (any, bool) {
			return nil, i%2 == 0
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for callbacks to complete
	time.Sleep(200 * time.Millisecond)

	mutex.Lock()
	defer mutex.Unlock()

	if len(timestamps) < 2 {
		t.Fatalf("Not enough timestamps collected: %d", len(timestamps))
	}

	// Verify timestamps are set and increasing
	var lastTimestamp time.Time

	for i, ts := range timestamps {
		if ts.IsZero() {
			t.Errorf("Timestamp %d is zero", i)
		}

		if !lastTimestamp.IsZero() && ts.Before(lastTimestamp) {
			t.Errorf("Timestamp %d (%v) is before previous timestamp (%v)",
				i, ts, lastTimestamp)
		}

		lastTimestamp = ts
	}

	t.Logf("Collected %d timestamps, all valid and increasing", len(timestamps))
}
