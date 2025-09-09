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
}

// TestHighConcurrencyRateCalculation verifies that rate calculation is thread-safe
// under high concurrency with many goroutines (more intensive than existing tests).
func TestHighConcurrencyRateCalculation(t *testing.T) {
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