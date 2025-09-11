package nozzle

import (
	"testing"
	"time"
)

// TestFirstRequestBypassesThrottling demonstrates that the first request of each interval
// is always allowed through regardless of the flowRate, which bypasses throttling.
// This test should FAIL with the current implementation and PASS after implementing decay.
func TestFirstRequestBypassesThrottling(t *testing.T) {
	t.Parallel()

	n, err := New(Options[string]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 0, // Very strict - no failures allowed
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	// Drive the flowRate down by having failures
	for i := 0; i < 10; i++ {
		n.DoBool(func() (string, bool) {
			return "executed", false // Always fail to reduce flowRate
		})
	}
	
	// Wait for interval to process and reduce flowRate
	n.Wait()
	
	// FlowRate should now be reduced (likely 99%)
	if n.FlowRate() >= 100 {
		t.Skip("Could not reduce flowRate for test")
	}
	
	currentFlowRate := n.FlowRate()
	t.Logf("FlowRate reduced to %d%%", currentFlowRate)
	
	// Now test multiple intervals to see if first request is always allowed
	intervalsWithFirstRequestAllowed := 0
	
	for interval := 0; interval < 5; interval++ {
		// Check if the first request of this interval is allowed
		result, _ := n.DoBool(func() (string, bool) {
			return "first", false // Continue failing to keep flowRate low
		})
		
		firstRequestAllowed := result == "first"
		
		if firstRequestAllowed {
			intervalsWithFirstRequestAllowed++
		}
		
		// Make a few more requests in this interval (these should mostly be blocked)
		for i := 1; i < 5; i++ {
			n.DoBool(func() (string, bool) {
				return "other", false
			})
		}
		
		// Wait for next interval
		n.Wait()
		
		t.Logf("Interval %d: First request allowed=%v, FlowRate=%d%%", 
			interval+1, firstRequestAllowed, n.FlowRate())
	}
	
	// The issue: Even with low flowRate, the first request of each interval is allowed
	// This test expects that NOT all first requests should be allowed when flowRate is low
	if intervalsWithFirstRequestAllowed == 5 {
		t.Errorf("All 5 first requests were allowed despite flowRate being %d%%. "+
			"First request should respect throttling, not bypass it.", currentFlowRate)
	}
}