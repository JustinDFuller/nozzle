package nozzle

import (
	"testing"
	"time"
)

// TestRateCalculationConceptualIssue demonstrates the mismatch between
// allowed/blocked (used for throttling) and successes/failures (used for flow rate adjustment)
func TestRateCalculationConceptualIssue(t *testing.T) {
	t.Parallel()

	n, err := New(Options[any]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 30, // Allow up to 30% failure rate
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	// Scenario: All allowed requests fail
	// This demonstrates that the throttling decision (based on allowed/blocked)
	// is disconnected from the actual outcome (successes/failures)

	t.Log("=== INTERVAL 1: All requests allowed, but all fail ===")
	
	// First interval: Allow everything but they all fail
	requestsInInterval := 10
	allowedCount := 0
	failedCount := 0
	
	for i := 0; i < requestsInInterval; i++ {
		result, ok := n.DoBool(func() (any, bool) {
			// Simulate a service that's failing
			return "executed", false // Always fail
		})
		if ok {
			t.Errorf("Request %d: Expected ok=false but got ok=true", i)
		}
		// Check if callback was executed
		if result == "executed" {
			allowedCount++
			failedCount++ // We know callback returned false
		} else {
			// Callback wasn't executed - request was blocked by nozzle
			t.Logf("Request %d was blocked by nozzle", i)
		}
	}
	
	t.Logf("Interval 1 results:")
	t.Logf("  Allowed through nozzle: %d/%d", allowedCount, requestsInInterval)
	t.Logf("  Actual failures: %d/%d", failedCount, requestsInInterval)
	t.Logf("  FlowRate: %d%%", n.FlowRate())
	
	// At this point:
	// - n.allowed = 10, n.blocked = 0 (all were allowed through the nozzle)
	// - n.successes = 0, n.failures = 10 (but all callbacks failed!)
	// - allowRate = 100% (10 allowed / 10 total)
	// - failureRate = 100% (10 failures / 10 total operations)
	
	// Wait for interval to process
	n.Wait()
	
	t.Logf("After calculate():")
	t.Logf("  FlowRate adjusted to: %d%% (should be reduced due to 100%% failure rate)", n.FlowRate())
	
	t.Log("\n=== INTERVAL 2: Demonstrating the issue ===")
	
	// The issue: In the next interval, allowed and blocked counters reset to 0
	// So even with reduced flowRate, the first requests will still be allowed
	// because allowRate starts at 0% (0 allowed / 0 total = 0%)
	// and 0% < flowRate, so requests get through
	
	allowedInSecond := 0
	blockedInSecond := 0
	
	for i := 0; i < requestsInInterval; i++ {
		_, ok := n.DoBool(func() (any, bool) {
			return nil, false // Still failing
		})
		if ok {
			allowedInSecond++
		} else {
			blockedInSecond++
		}
		
		// Log the decision process for first few requests
		if i < 3 {
			// We need to check internal state to understand the decision
			// In production code, we wouldn't access these directly
			n.mut.RLock()
			currentAllowRate := int64(0)
			if n.allowed > 0 {
				total := n.allowed + n.blocked
				if total > 0 {
					currentAllowRate = (n.allowed * 100) / total
				}
			}
			n.mut.RUnlock()
			
			t.Logf("  Request %d: allowRate=%d%%, flowRate=%d%%, allowed=%v",
				i+1, currentAllowRate, n.FlowRate(), ok)
		}
	}
	
	t.Logf("\nInterval 2 results:")
	t.Logf("  Allowed: %d, Blocked: %d", allowedInSecond, blockedInSecond)
	t.Logf("  FlowRate: %d%%", n.FlowRate())
	
	// The conceptual issue:
	// 1. The throttling decision uses (allowed/blocked) ratio within the current interval
	// 2. The flow rate adjustment uses (successes/failures) from actual callback results
	// 3. These two metrics can diverge significantly:
	//    - A service could be allowing requests through (high allowed/blocked ratio)
	//    - But those requests could all be failing (high failures/successes ratio)
	// 4. This creates a lag in the feedback loop where the nozzle continues to
	//    allow requests through even when the actual failure rate is very high
	
	if allowedInSecond > 0 && n.FlowRate() < 50 {
		t.Logf("\nISSUE DEMONSTRATED: Despite flowRate being %d%%, we still allowed %d requests",
			n.FlowRate(), allowedInSecond)
		t.Log("This happens because allowRate starts at 0% each interval,")
		t.Log("so early requests are always allowed until allowRate >= flowRate")
	}
}

// TestRateCalculationWithMixedOutcomes shows how the issue affects mixed success/failure scenarios
func TestRateCalculationWithMixedOutcomes(t *testing.T) {
	t.Parallel()

	n, err := New(Options[any]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	t.Log("=== Scenario: First batch succeeds, second batch fails ===")
	
	// First interval: All succeed
	for i := 0; i < 10; i++ {
		n.DoBool(func() (any, bool) {
			return nil, true // All succeed
		})
	}
	
	n.mut.RLock()
	t.Logf("After successful batch: allowed=%d, blocked=%d, successes=%d, failures=%d",
		n.allowed, n.blocked, n.successes, n.failures)
	n.mut.RUnlock()
	
	n.Wait() // Process interval
	
	t.Logf("FlowRate after successful interval: %d%% (should stay high)", n.FlowRate())
	
	// Second interval: All requests get through but fail
	allowedButFailed := 0
	for i := 0; i < 10; i++ {
		_, ok := n.DoBool(func() (any, bool) {
			return nil, false // Now they fail
		})
		if ok {
			allowedButFailed++
		}
	}
	
	t.Logf("Requests allowed through (but failed): %d/10", allowedButFailed)
	
	n.Wait() // Process interval
	
	t.Logf("FlowRate after failed interval: %d%%", n.FlowRate())
	
	// Third interval: Should throttle but doesn't immediately
	earlyAllowed := 0
	for i := 0; i < 3; i++ {
		_, ok := n.DoBool(func() (any, bool) {
			return nil, false
		})
		if ok {
			earlyAllowed++
		}
	}
	
	t.Logf("Early requests in third interval allowed: %d/3", earlyAllowed)
	
	if earlyAllowed > 0 && n.FlowRate() < 30 {
		t.Log("\nISSUE: Early requests in each interval bypass throttling")
		t.Log("because allowRate starts at 0% < flowRate")
	}
}

// TestRateCalculationEdgeCase demonstrates the edge case where flowRate=0 but requests still get through
func TestRateCalculationEdgeCase(t *testing.T) {
	t.Parallel()

	n, err := New(Options[any]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 0, // No failures allowed
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	// Force immediate closure by having failures
	for i := 0; i < 5; i++ {
		n.DoBool(func() (any, bool) {
			return nil, false // Fail to trigger closing
		})
	}
	
	// Wait for multiple intervals to drive flowRate to 0
	for i := 0; i < 10; i++ {
		n.Wait()
		if n.FlowRate() == 0 {
			break
		}
		// Keep failing to drive it down
		n.DoBool(func() (any, bool) {
			return nil, false
		})
	}
	
	if n.FlowRate() != 0 {
		t.Skipf("Could not drive flowRate to 0, got %d%%", n.FlowRate())
	}
	
	t.Logf("FlowRate is now: %d%%", n.FlowRate())
	
	// The edge case: even with flowRate=0, the first request might get through
	// because when allowed=0 and blocked=0, allowRate is considered 0
	// But the check is: if n.flowRate > 0 { allow = allowRate < n.flowRate }
	// With flowRate=0, this entire condition is skipped, so allow remains false
	// This is actually correct behavior!
	
	_, ok := n.DoBool(func() (any, bool) {
		return nil, true
	})
	
	if ok {
		t.Error("Request was allowed when flowRate=0 (this would be a bug)")
	} else {
		t.Log("Correctly blocked request when flowRate=0")
	}
	
	// However, there's still the issue of the initial request in each interval
	// when flowRate > 0 but should be throttling
}