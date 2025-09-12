package nozzle

import (
	"math"
	"testing"
	"time"
)

// TestFirstRequestRespectsFlowRate verifies that all requests, including the first
// request of each interval, respect the flow rate using pure probabilistic decisions.
// With unified probabilistic rate limiting, every request has exactly flowRate% chance.
func TestFirstRequestRespectsFlowRate(t *testing.T) {
	t.Parallel()

	// Test with multiple flow rates to ensure probabilistic behavior works correctly
	testCases := []struct {
		name                 string
		flowRate             int64
		iterations           int
		expectedAllowedRatio float64
		tolerance            float64 // Statistical tolerance for randomness
	}{
		{
			name:                 "Very low flow rate (5%)",
			flowRate:             5,
			iterations:           200,
			expectedAllowedRatio: 0.05,
			tolerance:            0.03, // ±3% for statistical variance
		},
		{
			name:                 "Low flow rate (20%)",
			flowRate:             20,
			iterations:           100,
			expectedAllowedRatio: 0.20,
			tolerance:            0.07, // Slightly wider for probabilistic variance
		},
		{
			name:                 "Medium flow rate (50%)",
			flowRate:             50,
			iterations:           100,
			expectedAllowedRatio: 0.50,
			tolerance:            0.08,
		},
		{
			name:                 "High flow rate (80%)",
			flowRate:             80,
			iterations:           100,
			expectedAllowedRatio: 0.80,
			tolerance:            0.08,
		},
	}

	for _, tc := range testCases {
		// Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			n, err := New(Options[string]{
				Interval:              10 * time.Millisecond,
				AllowedFailurePercent: 50,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer n.Close()

			// Set the flow rate to the desired test value
			// We need to manipulate the flow rate by having the right success/failure ratio
			// Start with some failures to reduce flow rate
			if tc.flowRate < 100 {
				for range 10 {
					n.DoBool(func() (string, bool) {
						return "setup", false // Fail to reduce flowRate
					})
				}

				n.Wait() // Process interval

				// Now manually set the flowRate for testing
				// This is a bit of a hack, but necessary for controlled testing
				n.mut.Lock()
				n.flowRate = tc.flowRate
				n.mut.Unlock()
			}

			// Test first requests across multiple intervals
			firstRequestsAllowed := 0

			for range tc.iterations {
				// Reset counters for new interval
				n.Wait()

				// Reset flowRate to desired value (it changes based on success/failure)
				n.mut.Lock()
				n.flowRate = tc.flowRate
				n.mut.Unlock()

				// Make the first request of the interval
				result, _ := n.DoBool(func() (string, bool) {
					return "first", false // Return false to avoid changing flowRate
				})

				if result == "first" {
					firstRequestsAllowed++
				}
			}

			// Calculate the actual ratio of first requests allowed
			actualRatio := float64(firstRequestsAllowed) / float64(tc.iterations)

			// Check if the actual ratio is within the expected range
			lowerBound := tc.expectedAllowedRatio - tc.tolerance
			upperBound := tc.expectedAllowedRatio + tc.tolerance

			if actualRatio < lowerBound || actualRatio > upperBound {
				t.Errorf("First request allow ratio out of range.\n"+
					"Flow rate: %d%%\n"+
					"Expected ratio: %.2f ± %.2f\n"+
					"Actual ratio: %.2f (%d/%d allowed)\n"+
					"This should be close to the flow rate due to probabilistic decisions",
					tc.flowRate,
					tc.expectedAllowedRatio, tc.tolerance,
					actualRatio, firstRequestsAllowed, tc.iterations)
			} else {
				t.Logf("✓ Flow rate %d%%: First requests allowed %.1f%% (expected %.1f%% ± %.1f%%)",
					tc.flowRate,
					actualRatio*100,
					tc.expectedAllowedRatio*100,
					tc.tolerance*100)
			}
		})
	}
}

// TestFirstRequestProbabilisticVsDeterministic demonstrates the difference between
// the old deterministic behavior (always allow) and new probabilistic behavior.
func TestFirstRequestProbabilisticVsDeterministic(t *testing.T) {
	t.Parallel()

	n, err := New(Options[string]{
		Interval:              20 * time.Millisecond,
		AllowedFailurePercent: 0, // Strict - drives flow rate down quickly
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	// Drive flow rate down
	for range 10 {
		n.DoBool(func() (string, bool) {
			return "fail", false
		})
	}

	n.Wait()

	// Flow rate should be very low now
	if n.FlowRate() >= 50 {
		t.Skip("Could not reduce flow rate sufficiently for test")
	}

	lowFlowRate := n.FlowRate()
	t.Logf("Flow rate reduced to %d%%", lowFlowRate)

	// Test pattern: first request followed by more requests in same interval
	intervals := 20
	firstRequestResults := make([]bool, 0, intervals)

	for range intervals {
		// First request of interval
		result, _ := n.DoBool(func() (string, bool) {
			return "first", false // Continue failing to keep flow rate low
		})
		firstAllowed := result == "first"
		firstRequestResults = append(firstRequestResults, firstAllowed)

		// Make more requests in same interval (these follow normal logic)
		for range 5 {
			n.DoBool(func() (string, bool) {
				return "other", false
			})
		}

		n.Wait() // Move to next interval
	}

	// Count how many first requests were allowed
	allowedCount := 0

	for _, allowed := range firstRequestResults {
		if allowed {
			allowedCount++
		}
	}

	// With probabilistic behavior, we expect approximately flowRate% of first requests allowed
	// With old deterministic behavior, this would be 100%
	allowedPercentage := float64(allowedCount) * 100 / float64(intervals)

	t.Logf("First requests allowed: %d/%d (%.1f%%)", allowedCount, intervals, allowedPercentage)
	t.Logf("Flow rate: %d%%", lowFlowRate)

	// The allowed percentage should be roughly equal to flow rate, not 100%
	// Allow for statistical variance
	expectedMax := math.Min(float64(lowFlowRate)*2, 100) // At most 2x flow rate
	if allowedPercentage > expectedMax {
		t.Errorf("Too many first requests allowed: %.1f%% (expected ≤ %.1f%% based on flow rate %d%%)",
			allowedPercentage, expectedMax, lowFlowRate)
	}
}
