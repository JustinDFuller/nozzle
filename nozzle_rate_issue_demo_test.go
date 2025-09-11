package nozzle

import (
	"testing"
	"time"
)

// TestRateCalculationIssueDemo demonstrates the actual issue with rate calculation
func TestRateCalculationIssueDemo(t *testing.T) {
	t.Parallel()

	n, err := New(Options[string]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 30, // Allow up to 30% failure rate
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	t.Log("=== Demonstrating the ACTUAL issue ===")
	t.Log("The issue: DoBool/DoError returns false for BOTH blocked requests AND failed callbacks")
	t.Log("This makes it impossible to distinguish between throttling and actual failures")
	
	// Helper to track what actually happens
	type result struct {
		wasExecuted bool
		succeeded   bool
	}
	
	executeRequest := func() result {
		returnValue, ok := n.DoBool(func() (string, bool) {
			// This callback always fails when executed
			return "executed", false
		})
		
		// If returnValue is "executed", the callback ran
		// If returnValue is "", the callback was blocked
		wasExecuted := returnValue == "executed"
		
		return result{
			wasExecuted: wasExecuted,
			succeeded:   ok, // This is false for BOTH blocked and failed
		}
	}
	
	t.Log("\n--- Interval 1: Initial requests ---")
	
	// First interval: should all be allowed (flowRate starts at 100%)
	executed := 0
	succeeded := 0
	for i := 0; i < 10; i++ {
		r := executeRequest()
		if r.wasExecuted {
			executed++
			if r.succeeded {
				succeeded++
			}
		}
	}
	
	t.Logf("Requests executed: %d/10, Succeeded: %d/10", executed, succeeded)
	t.Logf("Current FlowRate: %d%%", n.FlowRate())
	
	// Check internal state
	n.mut.RLock()
	t.Logf("Internal state: allowed=%d, blocked=%d, successes=%d, failures=%d",
		n.allowed, n.blocked, n.successes, n.failures)
	n.mut.RUnlock()
	
	// Wait for interval to process
	n.Wait()
	
	t.Logf("After calculate(): FlowRate=%d%% (should decrease due to failures)", n.FlowRate())
	
	t.Log("\n--- Interval 2: After flow rate adjustment ---")
	
	// Second interval: flowRate should be reduced
	executed = 0
	blocked := 0
	for i := 0; i < 10; i++ {
		r := executeRequest()
		if r.wasExecuted {
			executed++
		} else {
			blocked++
		}
		
		// Log the first few to see the pattern
		if i < 5 {
			n.mut.RLock()
			allowRate := int64(0)
			if n.allowed > 0 {
				total := n.allowed + n.blocked
				if total > 0 {
					allowRate = (n.allowed * 100) / total
				}
			}
			flowRate := n.flowRate
			n.mut.RUnlock()
			
			status := "blocked"
			if r.wasExecuted {
				status = "executed"
			}
			t.Logf("  Request %d: allowRate=%d%%, flowRate=%d%%, status=%s",
				i+1, allowRate, flowRate, status)
		}
	}
	
	t.Logf("Totals: Executed=%d, Blocked=%d", executed, blocked)
	
	t.Log("\n=== THE REAL ISSUE ===")
	t.Log("The issue is NOT that the rate calculation is wrong.")
	t.Log("The issue is that the allowRate calculation CAN cause unexpected behavior:")
	t.Log("")
	t.Log("1. At the start of each interval, allowed=0 and blocked=0")
	t.Log("2. The first request sees allowRate=0% (0/0 treated as 0)")
	t.Log("3. If flowRate > 0, then allow = (0 < flowRate) = true")
	t.Log("4. So the first request is ALWAYS allowed if flowRate > 0")
	t.Log("")
	t.Log("However, in practice, this doesn't happen because of line 367:")
	t.Log("  if n.allowed != 0 { ... }")
	t.Log("When allowed=0, allowRate stays 0, and the check becomes:")
	t.Log("  allow = allowRate < n.flowRate")
	t.Log("  allow = 0 < flowRate")
	t.Log("")
	t.Log("So actually, the first request IS allowed when flowRate > 0!")
	t.Log("Let's verify this...")
}

// TestFirstRequestAlwaysAllowed proves the first request in each interval is always allowed
func TestFirstRequestAlwaysAllowed(t *testing.T) {
	t.Parallel()

	n, err := New(Options[string]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 0, // Very strict - no failures allowed
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	// Helper to check if request was executed
	wasExecuted := func() bool {
		result, _ := n.DoBool(func() (string, bool) {
			return "executed", false // Always fail to drive flowRate down
		})
		return result == "executed"
	}
	
	// Drive the flowRate down by having failures
	for interval := 0; interval < 5; interval++ {
		// First request of interval
		firstRequestExecuted := wasExecuted()
		
		// More requests in same interval
		for i := 1; i < 5; i++ {
			wasExecuted() // These might get blocked
		}
		
		n.Wait() // Process interval
		
		t.Logf("Interval %d: First request executed=%v, FlowRate after=%d%%",
			interval+1, firstRequestExecuted, n.FlowRate())
		
		if n.FlowRate() < 100 && firstRequestExecuted {
			t.Log("  ^ First request was allowed despite reduced flowRate!")
		}
	}
}

// TestActualRateCalculationIssue shows the REAL problem clearly
func TestActualRateCalculationIssue(t *testing.T) {
	t.Parallel()

	n, err := New(Options[int]{
		Interval:              50 * time.Millisecond,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	t.Log("THE ACTUAL ISSUE:")
	t.Log("================")
	t.Log("The allowed/blocked counters track throttling decisions.")
	t.Log("The successes/failures counters track callback outcomes.")
	t.Log("These are independent metrics that can diverge significantly.")
	t.Log("")
	
	// Set up a scenario where all allowed requests fail
	executionCount := 0
	
	for interval := 0; interval < 3; interval++ {
		t.Logf("\n--- Interval %d ---", interval+1)
		
		intervalExecutions := 0
		intervalBlocked := 0
		
		for i := 0; i < 10; i++ {
			value, _ := n.DoBool(func() (int, bool) {
				// Track that callback was executed
				return 1, false // Always fail
			})
			
			if value == 1 {
				intervalExecutions++
				executionCount++
			} else {
				intervalBlocked++
			}
		}
		
		n.mut.RLock()
		t.Logf("Before calculate: allowed=%d, blocked=%d, successes=%d, failures=%d",
			n.allowed, n.blocked, n.successes, n.failures)
		allowRate := int64(0)
		if n.allowed > 0 {
			total := n.allowed + n.blocked
			if total > 0 {
				allowRate = (n.allowed * 100) / total
			}
		}
		failureRate := int64(0)
		if n.failures > 0 || n.successes > 0 {
			failureRate = (n.failures * 100) / (n.failures + n.successes)
		}
		n.mut.RUnlock()
		
		t.Logf("Metrics: allowRate=%d%%, failureRate=%d%%", allowRate, failureRate)
		t.Logf("This interval: %d executed, %d blocked", intervalExecutions, intervalBlocked)
		
		n.Wait() // Process interval
		t.Logf("FlowRate after calculate: %d%%", n.FlowRate())
	}
	
	t.Log("\nCONCLUSION:")
	t.Log("===========")
	t.Log("The 'allowed' counter tracks requests that passed the throttle check.")
	t.Log("The 'failures' counter tracks callbacks that returned false/error.")
	t.Log("These measure different things!")
	t.Log("")
	t.Log("The throttling decision (based on allowed/blocked ratio) doesn't")
	t.Log("directly correlate with the failure rate (based on success/failure ratio).")
	t.Log("This can lead to:")
	t.Log("1. Continuing to allow requests when they're all failing")
	t.Log("2. Blocking requests even when allowed ones are succeeding")
	t.Log("")
	t.Log("The fix would be to use consistent metrics for both decisions.")
}