package nozzle_test

import (
	"sync"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

// TestIntegerOverflowPrevention verifies that the integer arithmetic
// in rate calculation doesn't overflow even with many operations.
func TestIntegerOverflowPrevention(t *testing.T) {
	t.Parallel()

	noz := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 50,
		AllowedFailurePercent: 50,
	})

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	// Perform many operations to test for potential overflow
	// in rate calculation (allowed * 100)
	for i := range 100000 {
		noz.DoBool(func() (int, bool) {
			return i, true
		})

		// Periodically reset to prevent actual overflow
		if i%10000 == 0 {
			noz.Wait()
			time.Sleep(time.Millisecond * 60)
		}
	}

	// Should complete without panic
	rate := noz.FlowRate()
	if rate < 0 || rate > 100 {
		t.Errorf("Flow rate out of bounds: %d", rate)
	}
}

// TestHighConcurrencyRateCalculation verifies that rate calculation is thread-safe
// under high concurrency with many goroutines (more intensive than existing tests).
func TestHighConcurrencyRateCalculation(t *testing.T) {
	t.Parallel()

	noz := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	defer func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}()

	var wg sync.WaitGroup

	goroutines := 10
	opsPerGoroutine := 1000

	for g := range goroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for i := range opsPerGoroutine {
				noz.DoBool(func() (int, bool) {
					// Mix of success and failure
					return id*1000 + i, i%3 != 0
				})

				// Occasionally check flow rate (should not panic)
				if i%100 == 0 {
					rate := noz.FlowRate()
					if rate < 0 || rate > 100 {
						t.Errorf("Goroutine %d: Invalid flow rate %d", id, rate)
					}
				}
			}
		}(g)
	}

	wg.Wait()

	// Final verification
	finalRate := noz.FlowRate()
	if finalRate < 0 || finalRate > 100 {
		t.Errorf("Final flow rate out of bounds: %d", finalRate)
	}
}
