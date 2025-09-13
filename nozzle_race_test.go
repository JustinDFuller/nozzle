package nozzle_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

// TestNozzleSnapshotFieldValidation verifies that StateSnapshot fields contain valid values.
// This is the ONLY test that validates snapshot field ranges and consistency.
func TestNozzleSnapshotFieldValidation(t *testing.T) {
	t.Parallel()

	var validationCount atomic.Int32

	noz, err := nozzle.New(nozzle.Options[string]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 30,
		OnStateChange: func(ctx context.Context, snapshot nozzle.StateSnapshot) {
			validationCount.Add(1)

			// Validate timestamp is set
			if snapshot.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}

			// Validate all field ranges
			if snapshot.FlowRate < 0 || snapshot.FlowRate > 100 {
				t.Errorf("Invalid FlowRate: %d (should be 0-100)", snapshot.FlowRate)
			}

			if snapshot.State != nozzle.Opening && snapshot.State != nozzle.Closing {
				t.Errorf("Invalid State: %s (should be Opening or Closing)", snapshot.State)
			}

			if snapshot.FailureRate < 0 || snapshot.FailureRate > 100 {
				t.Errorf("Invalid FailureRate: %d (should be 0-100)", snapshot.FailureRate)
			}

			if snapshot.SuccessRate < 0 || snapshot.SuccessRate > 100 {
				t.Errorf("Invalid SuccessRate: %d (should be 0-100)", snapshot.SuccessRate)
			}

			// Verify rate consistency when there are operations
			if snapshot.Allowed > 0 || snapshot.Blocked > 0 {
				if snapshot.FailureRate+snapshot.SuccessRate != 100 {
					t.Errorf("FailureRate (%d) + SuccessRate (%d) != 100",
						snapshot.FailureRate, snapshot.SuccessRate)
				}
			}

			if snapshot.Allowed < 0 {
				t.Errorf("Invalid Allowed count: %d (should be >= 0)", snapshot.Allowed)
			}

			if snapshot.Blocked < 0 {
				t.Errorf("Invalid Blocked count: %d (should be >= 0)", snapshot.Blocked)
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Generate operations to trigger state changes
	for i := range 100 {
		switch {
		case i < 30:
			noz.DoBool(func() (string, bool) {
				return "failure", false
			})
		case i < 60:
			noz.DoBool(func() (string, bool) {
				return "success", true
			})
		default:
			noz.DoBool(func() (string, bool) {
				return "mixed", i%3 == 0
			})
		}

		if i%20 == 0 {
			noz.Wait()
		}
	}

	if validationCount.Load() == 0 {
		t.Error("OnStateChange callback was never invoked")
	}

	t.Logf("Validated %d snapshots", validationCount.Load())
}

// TestNozzleConcurrentStateChange verifies that concurrent operations don't cause race conditions
// when the OnStateChange callback is invoked.
func TestNozzleConcurrentStateChange(t *testing.T) {
	t.Parallel()

	var (
		callbackCount atomic.Int32
		wg            sync.WaitGroup
	)

	noz, err := nozzle.New(nozzle.Options[string]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 30,
		OnStateChange: func(ctx context.Context, _ nozzle.StateSnapshot) {
			// Simply count callbacks - no validation here
			callbackCount.Add(1)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Launch multiple goroutines that perform operations concurrently
	for goroutineIdx := range 10 {
		wg.Add(1)

		go func(_ int) {
			defer wg.Done()

			for j := range 100 {
				switch {
				case j < 30:
					noz.DoBool(func() (string, bool) {
						return "failure", false
					})
				case j < 60:
					noz.DoBool(func() (string, bool) {
						return "success", true
					})
				default:
					noz.DoBool(func() (string, bool) {
						return "mixed", j%3 == 0
					})
				}
			}
		}(goroutineIdx)
	}

	// Launch a goroutine that triggers state calculations
	wg.Add(1)

	go func() {
		defer wg.Done()

		for range 20 {
			time.Sleep(50 * time.Millisecond)
			noz.Wait()
		}
	}()

	wg.Wait()

	if callbackCount.Load() == 0 {
		t.Error("OnStateChange callback was never invoked")
	}

	t.Logf("Callback invoked %d times", callbackCount.Load())
}

// TestNozzleCallbackNoDeadlock verifies that the callback doesn't cause deadlocks
// even with various callback patterns.
func TestNozzleCallbackNoDeadlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		callback func(context.Context, nozzle.StateSnapshot)
	}{
		{
			name: "EmptyCallback",
			callback: func(ctx context.Context, _ nozzle.StateSnapshot) {
				// Do nothing
			},
		},
		{
			name: "SlowCallback",
			callback: func(ctx context.Context, _ nozzle.StateSnapshot) {
				time.Sleep(10 * time.Millisecond)
			},
		},
		{
			name: "AccessAllFields",
			callback: func(ctx context.Context, snapshot nozzle.StateSnapshot) {
				// Access all fields to verify no deadlock occurs
				total := snapshot.FlowRate + snapshot.FailureRate + snapshot.SuccessRate
				sum := snapshot.Allowed + snapshot.Blocked
				// Use variables to avoid compiler warnings
				if total < 0 || sum < 0 || snapshot.State == "" || snapshot.Timestamp.IsZero() {
					// This should never happen but satisfies the compiler
					return
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			done := make(chan struct{})

			noz, err := nozzle.New(nozzle.Options[string]{
				Interval:              10 * time.Millisecond,
				AllowedFailurePercent: 50,
				OnStateChange:         tt.callback,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			defer func() {
				if err := noz.Close(); err != nil {
					t.Errorf("Failed to close nozzle: %v", err)
				}
			}()

			// Run operations concurrently
			go func() {
				for i := range 100 {
					noz.DoBool(func() (string, bool) {
						return "test", i%2 == 0
					})
				}

				close(done)
			}()

			// Wait for completion with timeout
			select {
			case <-done:
				// Success - no deadlock
			case <-time.After(5 * time.Second):
				t.Fatal("Test timed out - possible deadlock")
			}
		})
	}
}

// TestNozzleRaceConditionRegression is a specific test to verify the race condition
// described in the PLAN.md has been fixed. It also verifies snapshot immutability
// and consistency during callback execution.
func TestNozzleRaceConditionRegression(t *testing.T) {
	t.Parallel()

	var (
		stateModified atomic.Bool
		wg            sync.WaitGroup
		snapshots     []nozzle.StateSnapshot
		snapshotMutex sync.Mutex
	)

	noz, err := nozzle.New(nozzle.Options[string]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(ctx context.Context, snapshot nozzle.StateSnapshot) {
			// Store snapshot for later verification
			snapshotMutex.Lock()
			snapshots = append(snapshots, snapshot)
			snapshotMutex.Unlock()

			// Store initial values to verify immutability
			initialFlowRate := snapshot.FlowRate
			initialState := snapshot.State
			initialTimestamp := snapshot.Timestamp

			// Give other goroutines a chance to interfere
			time.Sleep(5 * time.Millisecond)

			// The snapshot should be immutable
			if snapshot.FlowRate != initialFlowRate {
				stateModified.Store(true)
				t.Error("Snapshot FlowRate was modified during callback execution")
			}
			if snapshot.State != initialState {
				stateModified.Store(true)
				t.Error("Snapshot State was modified during callback execution")
			}
			if !snapshot.Timestamp.Equal(initialTimestamp) {
				stateModified.Store(true)
				t.Error("Snapshot Timestamp was modified during callback execution")
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Launch multiple goroutines that aggressively try to modify state
	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := range 100 {
				noz.DoBool(func() (string, bool) {
					return "test", j%2 == 0
				})

				if j%10 == 0 {
					noz.Wait() // Force state recalculation
				}
			}
		}()
	}

	wg.Wait()

	if stateModified.Load() {
		t.Fatal("Race condition detected: state was modified during callback execution")
	}

	// Verify we captured snapshots
	snapshotMutex.Lock()
	defer snapshotMutex.Unlock()

	if len(snapshots) == 0 {
		t.Error("No snapshots were captured")
	}

	t.Logf("Captured %d immutable snapshots", len(snapshots))
}
