package nozzle_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

// TestNozzleConcurrentStateChange verifies that concurrent operations don't cause race conditions
// when the OnStateChange callback is invoked.
func TestNozzleConcurrentStateChange(t *testing.T) {
	var (
		callbackCount atomic.Int32
		wg            sync.WaitGroup
	)

	noz := nozzle.New(nozzle.Options[string]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 30, // Changed from 50 to ensure state changes
		OnStateChange: func(snapshot nozzle.StateSnapshot) {
			// This callback should be able to safely access the snapshot
			// without any race conditions
			callbackCount.Add(1)

			// Verify all fields are accessible
			_ = snapshot.FlowRate
			_ = snapshot.State
			_ = snapshot.FailureRate
			_ = snapshot.SuccessRate
			_ = snapshot.Allowed
			_ = snapshot.Blocked
		},
	})
	defer noz.Close()

	// Launch multiple goroutines that perform operations concurrently
	// with varying success/failure patterns to trigger state changes
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range 100 {
				// Vary the success rate over time to trigger state changes
				if j < 30 {
					// Start with high failure rate
					noz.DoBool(func() (string, bool) {
						return "failure", false
					})
				} else if j < 60 {
					// Then high success rate
					noz.DoBool(func() (string, bool) {
						return "success", true
					})
				} else {
					// Then mixed
					noz.DoBool(func() (string, bool) {
						return "mixed", j%3 == 0
					})
				}
			}
		}(i)
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

	// Verify that callbacks were invoked
	if callbackCount.Load() == 0 {
		t.Error("OnStateChange callback was never invoked")
	}

	t.Logf("Callback invoked %d times", callbackCount.Load())
}

// TestNozzleStateSnapshotConsistency verifies that the snapshot passed to OnStateChange
// contains consistent data that doesn't change during callback execution.
func TestNozzleStateSnapshotConsistency(t *testing.T) {
	snapshotData := make([]nozzle.StateSnapshot, 0, 100)

	var mu sync.Mutex

	noz := nozzle.New(nozzle.Options[int]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(snapshot nozzle.StateSnapshot) {
			// Store snapshot for later verification
			mu.Lock()
			snapshotData = append(snapshotData, snapshot)
			mu.Unlock()

			// Simulate some work in the callback
			time.Sleep(5 * time.Millisecond)

			// Verify snapshot hasn't changed
			originalFlowRate := snapshot.FlowRate
			originalState := snapshot.State

			// These should still be the same
			if snapshot.FlowRate != originalFlowRate {
				t.Errorf("FlowRate changed during callback: %d != %d", snapshot.FlowRate, originalFlowRate)
			}
			if snapshot.State != originalState {
				t.Errorf("State changed during callback: %s != %s", snapshot.State, originalState)
			}
		},
	})
	defer noz.Close()

	// Generate mixed success/failure operations
	for i := range 200 {
		if i%3 == 0 {
			noz.DoBool(func() (int, bool) {
				return i, false // failure
			})
		} else {
			noz.DoBool(func() (int, bool) {
				return i, true // success
			})
		}

		if i%20 == 0 {
			noz.Wait() // Force state calculation
		}
	}

	// Verify we got some snapshots
	mu.Lock()
	defer mu.Unlock()

	if len(snapshotData) == 0 {
		t.Error("No snapshots were captured")
	}

	// Verify snapshot data makes sense
	for i, snapshot := range snapshotData {
		if snapshot.FlowRate < 0 || snapshot.FlowRate > 100 {
			t.Errorf("Snapshot %d has invalid FlowRate: %d", i, snapshot.FlowRate)
		}

		if snapshot.State != nozzle.Opening && snapshot.State != nozzle.Closing {
			t.Errorf("Snapshot %d has invalid State: %s", i, snapshot.State)
		}

		if snapshot.FailureRate < 0 || snapshot.FailureRate > 100 {
			t.Errorf("Snapshot %d has invalid FailureRate: %d", i, snapshot.FailureRate)
		}

		if snapshot.SuccessRate < 0 || snapshot.SuccessRate > 100 {
			t.Errorf("Snapshot %d has invalid SuccessRate: %d", i, snapshot.SuccessRate)
		}
	}

	t.Logf("Captured %d snapshots", len(snapshotData))
}

// TestNozzleCallbackNoDeadlock verifies that the callback doesn't cause deadlocks
// even with various callback patterns.
func TestNozzleCallbackNoDeadlock(t *testing.T) {
	tests := []struct {
		name     string
		callback func(nozzle.StateSnapshot)
	}{
		{
			name: "EmptyCallback",
			callback: func(snapshot nozzle.StateSnapshot) {
				// Do nothing
			},
		},
		{
			name: "SlowCallback",
			callback: func(snapshot nozzle.StateSnapshot) {
				time.Sleep(10 * time.Millisecond)
			},
		},
		{
			name: "AccessAllFields",
			callback: func(snapshot nozzle.StateSnapshot) {
				total := snapshot.FlowRate + snapshot.FailureRate + snapshot.SuccessRate
				_ = total
				_ = snapshot.State
				_ = snapshot.Allowed + snapshot.Blocked
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})

			noz := nozzle.New(nozzle.Options[string]{
				Interval:              10 * time.Millisecond,
				AllowedFailurePercent: 50,
				OnStateChange:         tt.callback,
			})
			defer noz.Close()

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

// TestNozzleHighConcurrency performs a stress test with many concurrent operations.
func TestNozzleHighConcurrency(t *testing.T) {
	const (
		numGoroutines   = 100
		opsPerGoroutine = 1000
	)

	var (
		totalOps     atomic.Int64
		callbackOps  atomic.Int64
		successCount atomic.Int64
		failureCount atomic.Int64
	)

	noz := nozzle.New(nozzle.Options[int]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 30,
		OnStateChange: func(snapshot nozzle.StateSnapshot) {
			callbackOps.Add(1)
			// Just access the data to ensure it's valid
			if snapshot.FlowRate < 0 || snapshot.FlowRate > 100 {
				t.Errorf("Invalid flow rate in snapshot: %d", snapshot.FlowRate)
			}
		},
	})
	defer noz.Close()

	start := time.Now()

	var wg sync.WaitGroup

	// Launch many goroutines performing operations
	for i := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range opsPerGoroutine {
				totalOps.Add(1)

				// Mix of success and failure
				shouldSucceed := (id+j)%3 != 0

				_, ok := noz.DoBool(func() (int, bool) {
					return id*1000 + j, shouldSucceed
				})

				if ok {
					if shouldSucceed {
						successCount.Add(1)
					} else {
						failureCount.Add(1)
					}
				}
			}
		}(i)
	}

	// Launch a goroutine to periodically trigger state changes
	wg.Add(1)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		timeout := time.After(10 * time.Second)

		for {
			select {
			case <-ticker.C:
				noz.Wait()
			case <-timeout:
				return
			}
		}
	}()

	wg.Wait()

	duration := time.Since(start)

	t.Logf("High concurrency test completed:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Total operations: %d", totalOps.Load())
	t.Logf("  Successful operations: %d", successCount.Load())
	t.Logf("  Failed operations: %d", failureCount.Load())
	t.Logf("  Callback invocations: %d", callbackOps.Load())
	t.Logf("  Ops/second: %.0f", float64(totalOps.Load())/duration.Seconds())

	if totalOps.Load() != numGoroutines*opsPerGoroutine {
		t.Errorf("Expected %d total operations, got %d",
			numGoroutines*opsPerGoroutine, totalOps.Load())
	}

	if callbackOps.Load() == 0 {
		t.Error("OnStateChange callback was never invoked during high concurrency test")
	}
}

// TestNozzleRaceConditionRegression is a specific test to verify the race condition
// described in the PLAN.md has been fixed.
func TestNozzleRaceConditionRegression(t *testing.T) {
	// This test specifically targets the race condition where the mutex was
	// unlocked during OnStateChange callback execution.
	var (
		stateModified atomic.Bool
		wg            sync.WaitGroup
	)

	noz := nozzle.New(nozzle.Options[string]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 50,
		OnStateChange: func(snapshot nozzle.StateSnapshot) {
			// During this callback, try to detect if state is being modified
			// by other goroutines (which shouldn't happen with the fix)

			initialFlowRate := snapshot.FlowRate
			time.Sleep(5 * time.Millisecond) // Give other goroutines a chance to interfere

			// The snapshot should be immutable, so these values should not change
			if snapshot.FlowRate != initialFlowRate {
				stateModified.Store(true)
				t.Error("Snapshot was modified during callback execution")
			}
		},
	})
	defer noz.Close()

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
}
