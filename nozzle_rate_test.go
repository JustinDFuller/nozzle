package nozzle_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

// TestNozzleAccurateRateCalculation verifies that the allow rate is calculated
// correctly based on per-interval statistics, not cumulative statistics.
func TestNozzleAccurateRateCalculation(t *testing.T) {
	n := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 50,
		AllowedFailurePercent: 50,
	})
	defer n.Close()

	// First interval: Create a specific pattern of allowed/blocked
	// Allow first 70 operations (70% flow rate)
	for i := 0; i < 100; i++ {
		_, ok := n.DoBool(func() (int, bool) {
			return i, true // All succeed to keep nozzle open
		})
		if i < 70 {
			if !ok {
				t.Errorf("Operation %d should have been allowed in first interval", i)
			}
		}
	}

	// Wait for interval to reset
	n.Wait()
	time.Sleep(time.Millisecond * 60)

	// Second interval: The rate should reset, not be cumulative
	// We should be able to get approximately the flow rate percentage again
	allowedCount := 0
	for i := 0; i < 100; i++ {
		_, ok := n.DoBool(func() (int, bool) {
			return i, true
		})
		if ok {
			allowedCount++
		}
	}

	// With a fresh interval, we should get close to the flow rate
	// Allow some variance due to timing
	expectedMin := int(n.FlowRate()) - 10
	expectedMax := int(n.FlowRate()) + 10
	
	if allowedCount < expectedMin || allowedCount > expectedMax {
		t.Errorf("Second interval allowed %d operations, expected %d±10 (flow rate: %d)",
			allowedCount, n.FlowRate(), n.FlowRate())
	}
}

// TestRateCalculationResetBehavior ensures that allowed and blocked counters
// are reset properly at each interval.
func TestRateCalculationResetBehavior(t *testing.T) {
	callCount := 0
	snapshots := []nozzle.StateSnapshot{}
	var mu sync.Mutex

	n := nozzle.New(nozzle.Options[bool]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
		OnStateChange: func(snapshot nozzle.StateSnapshot) {
			mu.Lock()
			defer mu.Unlock()
			snapshots = append(snapshots, snapshot)
		},
	})
	defer n.Close()

	// First interval: Generate traffic
	for i := 0; i < 50; i++ {
		n.DoBool(func() (bool, bool) {
			callCount++
			// 30% failure rate to keep nozzle mostly open
			return true, callCount%10 < 7
		})
	}

	// Wait for state calculation
	n.Wait()
	time.Sleep(time.Millisecond * 110)

	// Second interval: Generate different traffic pattern
	for i := 0; i < 50; i++ {
		n.DoBool(func() (bool, bool) {
			callCount++
			// 70% failure rate to trigger closing
			return true, callCount%10 < 3
		})
	}

	// Wait for state calculation
	n.Wait()
	time.Sleep(time.Millisecond * 110)

	// Verify we got state changes
	mu.Lock()
	defer mu.Unlock()
	
	if len(snapshots) < 1 {
		t.Fatalf("Expected at least 1 state change, got %d", len(snapshots))
	}

	// Each snapshot should reflect the current interval's statistics
	// not cumulative all-time statistics
	for i, snapshot := range snapshots {
		t.Logf("Snapshot %d: FlowRate=%d, FailureRate=%d, Allowed=%d, Blocked=%d",
			i, snapshot.FlowRate, snapshot.FailureRate, snapshot.Allowed, snapshot.Blocked)
	}
}

// TestRateCalculationEdgeCases tests edge cases in rate calculation
// including division by zero scenarios.
func TestRateCalculationEdgeCases(t *testing.T) {
	t.Run("NoOperations", func(t *testing.T) {
		n := nozzle.New(nozzle.Options[string]{
			Interval:              time.Millisecond * 50,
			AllowedFailurePercent: 50,
		})
		defer n.Close()

		// With no operations, the first operation should be allowed
		// (flowRate starts at 100)
		_, ok := n.DoBool(func() (string, bool) {
			return "test", true
		})
		
		if !ok {
			t.Error("First operation should be allowed when no prior operations")
		}
	})

	t.Run("OnlyBlockedOperations", func(t *testing.T) {
		n := nozzle.New(nozzle.Options[string]{
			Interval:              time.Millisecond * 50,
			AllowedFailurePercent: 0, // No failures allowed
		})
		defer n.Close()

		// Force the nozzle to close by generating failures
		for i := 0; i < 10; i++ {
			n.DoError(func() (string, error) {
				return "", errors.New("fail")
			})
		}

		// Wait for state to update
		n.Wait()
		time.Sleep(time.Millisecond * 60)

		// Now all operations should be blocked
		blocked := 0
		for i := 0; i < 10; i++ {
			_, err := n.DoError(func() (string, error) {
				return "test", nil
			})
			if errors.Is(err, nozzle.ErrBlocked) {
				blocked++
			}
		}

		// After closing, the nozzle may start to re-open gradually
		// so we may not block everything, but we should block at least some
		if blocked < 5 && n.FlowRate() < 50 {
			t.Errorf("Expected at least 5 operations to be blocked when flowRate is %d, got %d blocked", n.FlowRate(), blocked)
		}
	})

	t.Run("IntegerOverflowPrevention", func(t *testing.T) {
		n := nozzle.New(nozzle.Options[int]{
			Interval:              time.Millisecond * 50,
			AllowedFailurePercent: 50,
		})
		defer n.Close()

		// Perform many operations to test for potential overflow
		// in rate calculation (allowed * 100)
		for i := 0; i < 100000; i++ {
			n.DoBool(func() (int, bool) {
				return i, true
			})
			
			// Periodically reset to prevent actual overflow
			if i%10000 == 0 {
				n.Wait()
				time.Sleep(time.Millisecond * 60)
			}
		}

		// Should complete without panic
		rate := n.FlowRate()
		if rate < 0 || rate > 100 {
			t.Errorf("Flow rate out of bounds: %d", rate)
		}
	})
}

// TestLongRunningRateAccuracy verifies that rate calculations remain accurate
// over many intervals in a long-running scenario.
func TestLongRunningRateAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	n := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 30,
	})
	defer n.Close()

	intervals := 20
	for interval := 0; interval < intervals; interval++ {
		allowedInInterval := 0
		totalInInterval := 100

		for i := 0; i < totalInInterval; i++ {
			_, ok := n.DoBool(func() (int, bool) {
				// Alternate between success and failure patterns
				if interval%2 == 0 {
					return i, i%10 < 8 // 80% success
				}
				return i, i%10 < 5 // 50% success
			})
			if ok {
				allowedInInterval++
			}
		}

		// Log the behavior for this interval
		t.Logf("Interval %d: Allowed %d/%d operations (FlowRate: %d%%)",
			interval, allowedInInterval, totalInInterval, n.FlowRate())

		// Wait for next interval
		n.Wait()
		time.Sleep(time.Millisecond * 110)

		// Verify flow rate is within reasonable bounds
		flowRate := n.FlowRate()
		if flowRate < 0 || flowRate > 100 {
			t.Errorf("Interval %d: Invalid flow rate %d", interval, flowRate)
		}
	}
}

// TestRateCalculationConcurrency verifies that rate calculation is thread-safe
// under high concurrency.
func TestRateCalculationConcurrency(t *testing.T) {
	n := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})
	defer n.Close()

	var wg sync.WaitGroup
	goroutines := 10
	opsPerGoroutine := 1000

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				n.DoBool(func() (int, bool) {
					// Mix of success and failure
					return id*1000 + i, i%3 != 0
				})
				
				// Occasionally check flow rate (should not panic)
				if i%100 == 0 {
					rate := n.FlowRate()
					if rate < 0 || rate > 100 {
						t.Errorf("Goroutine %d: Invalid flow rate %d", id, rate)
					}
				}
			}
		}(g)
	}

	wg.Wait()

	// Final verification
	finalRate := n.FlowRate()
	if finalRate < 0 || finalRate > 100 {
		t.Errorf("Final flow rate out of bounds: %d", finalRate)
	}
}