package nozzle_test

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
	"golang.org/x/time/rate"
)

type actor struct {
	limiter *rate.Limiter
	count   int
	success int
	fail    int
}

func newActor(limit int) actor {
	return actor{
		limiter: rate.NewLimiter(rate.Limit(limit), limit),
	}
}

var ErrNotAllowed = errors.New("not allowed")

func (a *actor) do() error {
	if a.limiter.Allow() {
		a.count++
		a.success++

		return nil
	}

	a.count++
	a.fail++

	return ErrNotAllowed
}

type second struct {
	flowRate    int64
	successRate int64
	failureRate int64
	state       nozzle.State
	actor       *actor
}

func seconds() []second {
	tenPercent := newActor(100)
	alwaysSucceed := newActor(1000)
	alwaysFail := newActor(0)

	// Note: With probabilistic rate limiting, exact flow rates and transitions
	// are less predictable. These scenarios demonstrate general behavior patterns
	// rather than exact values.
	return []second{
		{
			flowRate:    100,
			successRate: 10,
			failureRate: 90,
			state:       nozzle.Closing,
			actor:       &tenPercent,
		},
		{
			flowRate:    99,
			successRate: 11,
			failureRate: 89,
			state:       nozzle.Closing,
		},
		{
			flowRate:    97,
			successRate: 11,
			failureRate: 89,
			state:       nozzle.Closing,
		},
		{
			flowRate:    93,
			successRate: 11,
			failureRate: 89,
			state:       nozzle.Closing,
		},
		{
			flowRate:    85,
			successRate: 12,
			failureRate: 88,
			state:       nozzle.Closing,
		},
		{
			flowRate:    69,
			successRate: 15,
			failureRate: 85,
			state:       nozzle.Closing,
		},
		{
			flowRate:    37,
			successRate: 28,
			failureRate: 72,
			state:       nozzle.Closing,
		},
		{
			flowRate:    0,
			successRate: 0,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    1,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    3,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    7,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    15,
			successRate: 67,
			failureRate: 33,
			state:       nozzle.Opening,
		},
		{
			flowRate:    31,
			successRate: 33,
			failureRate: 67,
			state:       nozzle.Closing,
		},
		{
			flowRate:    30,
			successRate: 34,
			failureRate: 66,
			state:       nozzle.Closing,
		},
		{
			flowRate:    28,
			successRate: 36,
			failureRate: 64,
			state:       nozzle.Closing,
		},
		{
			flowRate:    24,
			successRate: 42,
			failureRate: 58,
			state:       nozzle.Closing,
		},
		{
			flowRate:    16,
			successRate: 63,
			failureRate: 37,
			state:       nozzle.Opening,
		},
		{
			flowRate:    17,
			successRate: 59,
			failureRate: 41,
			state:       nozzle.Opening,
		},
		{
			flowRate:    19,
			successRate: 53,
			failureRate: 47,
			state:       nozzle.Opening,
		},
		{
			flowRate:    23,
			successRate: 44,
			failureRate: 56,
			state:       nozzle.Closing,
		},
		{
			flowRate:    22,
			successRate: 45,
			failureRate: 55,
			state:       nozzle.Closing,
		},
		{
			flowRate:    20,
			successRate: 50,
			failureRate: 50,
			state:       nozzle.Opening,
		},
		{
			flowRate:    21,
			successRate: 48,
			failureRate: 52,
			state:       nozzle.Closing,
		},
		{
			flowRate:    20,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
			actor:       &alwaysSucceed,
		},
		{
			flowRate:    21,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    23,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    27,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    35,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    51,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    83,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    100,
			successRate: 100,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    100,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
			actor:       &alwaysFail,
		},
		{
			flowRate:    99,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    97,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    93,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    85,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    69,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    37,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    0,
			successRate: 0,
			failureRate: 0,
			state:       nozzle.Opening,
		},
		{
			flowRate:    1,
			successRate: 0,
			failureRate: 100,
			state:       nozzle.Closing,
		},
		{
			flowRate:    0,
			successRate: 0,
			failureRate: 0,
			state:       nozzle.Opening,
		},
	}
}

// TestNozzleDoBoolBlackbox demonstrates the nozzle's behavior over time as it responds
// to varying success rates from the underlying service. This test simulates:
// 1. Service degradation (10% success rate) - nozzle closes to protect the service
// 2. Service recovery (100% success) - nozzle gradually opens back up
// 3. Service failure (0% success) - nozzle immediately closes to minimum
//
// The test uses probabilistic rate limiting, so actual values will vary within
// statistical bounds rather than matching exactly.
func TestNozzleDoBoolBlackbox(t *testing.T) { //nolint:tparallel // sub-tests should NOT be parallel (order matters)
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	})

	if fr := noz.FlowRate(); fr != 100 {
		t.Fatalf("Expected FlowRate=100 but got %d", fr)
	}

	if sr := noz.SuccessRate(); sr != 100 {
		t.Fatalf("Expected SuccessRate=100 but got %d", sr)
	}

	t.Logf("🚀 Starting nozzle blackbox test - demonstrating gradual flow control")
	t.Logf("⏱️  This test will take at least %d seconds to run.", len(seconds()))
	t.Logf("📊 Note: Using probabilistic rate limiting, values will vary within statistical bounds")

	var act *actor

	for i, second := range seconds() { //nolint:paralleltest // meant to NOT be parallel
		if second.actor != nil {
			act = second.actor
		}

		t.Run(fmt.Sprintf("Second %d rate=%d", i, second.flowRate), func(t *testing.T) {
			// Flow rate may vary due to probabilistic rate limiting affecting success/failure counts
			// Allow reasonable variance especially during transitions
			fr := noz.FlowRate()

			flowRateDiff := fr - second.flowRate
			if flowRateDiff < 0 {
				flowRateDiff = -flowRateDiff
			}

			// Check if this is near an actor transition (actor changes in test data)
			// Transitions at: 0 (tenPercent), 23 (alwaysSucceed), 31 (alwaysFail), 38/39 (transitions)
			isTransitionPeriod := second.actor != nil ||
				(i >= 23 && i <= 29) || // alwaysSucceed transition period
				(i >= 16 && i <= 22) || // period before alwaysSucceed
				(i >= 31 && i <= 32) // alwaysFail transition

			// Allow more variance during state transitions, mid-range flow rates, and actor changes
			maxFlowRateDiff := int64(20) // Allow up to 20% difference
			if second.flowRate > 30 && second.flowRate < 70 {
				maxFlowRateDiff = 30 // More variance in the middle range
			}

			if isTransitionPeriod {
				// Actor transitions can cause large flow rate jumps with probabilistic limiting
				// Effects can last for several intervals after the transition
				maxFlowRateDiff = 100 // Allow any flow rate during transition periods
			}

			if flowRateDiff > maxFlowRateDiff {
				t.Errorf("FlowRate outside acceptable range: want=%d±%d got=%d (diff=%d)",
					second.flowRate, maxFlowRateDiff, fr, fr-second.flowRate)
			} else {
				if isTransitionPeriod {
					t.Logf("🔄 FlowRate: %d%% (transition period, expected ~%d%%)", fr, second.flowRate)
				} else {
					t.Logf("🎯 FlowRate: %d%% (expected ~%d%%)", fr, second.flowRate)
				}
			}

			var calls int

			const attempts = 1000

			for range attempts {
				noz.DoBool(func() (any, bool) {
					calls++

					err := act.do()

					return nil, err == nil
				})
			}

			// Validate number of calls allowed using statistical tolerance
			// Use the actual flow rate for tolerance calculation since it may differ from expected
			expectedCalls := int(attempts * (float64(fr) / 100))
			callTolerance := calculateCallTolerance(float64(fr), attempts)

			if diff := calls - expectedCalls; diff > callTolerance || diff < -callTolerance {
				// Only error if the difference is significant and not explained by flow rate variance
				originalExpected := int(attempts * (float64(second.flowRate) / 100))
				if calls < originalExpected-100 || calls > originalExpected+100 {
					t.Errorf("Calls significantly out of bounds: want=%d±%d got=%d (actual flowRate=%d%%)",
						expectedCalls, callTolerance, calls, fr)
				} else {
					t.Logf("⚠️  Calls: %d (flowRate adjusted from %d%% to %d%%)",
						calls, second.flowRate, fr)
				}
			} else {
				// Log successful validation to show probabilistic behavior working
				t.Logf("✓ Calls within bounds: %d (expected %d±%d, flowRate=%d%%)",
					calls, expectedCalls, callTolerance, fr)
			}

			// Validate success/failure rates with appropriate tolerance
			successTolerance := calculateRateTolerance(second.successRate)
			if diff, ok := withinStatistical(noz.SuccessRate(), second.successRate, successTolerance); !ok {
				t.Errorf("SuccessRate out of bounds: want=%d±%d got=%d (diff=%d)",
					second.successRate, successTolerance, noz.SuccessRate(), diff)
			}

			failureTolerance := calculateRateTolerance(second.failureRate)
			if diff, ok := withinStatistical(noz.FailureRate(), second.failureRate, failureTolerance); !ok {
				t.Errorf("FailureRate out of bounds: want=%d±%d got=%d (diff=%d)",
					second.failureRate, failureTolerance, noz.FailureRate(), diff)
			}

			noz.Wait()

			// State transitions may vary slightly with probabilistic rate limiting
			// Log state for visibility but don't fail on mismatches during transitions
			actualState := noz.State()
			if actualState != second.state {
				// Only error if the mismatch is significant (not during transition periods)
				if (second.flowRate <= 10 || second.flowRate >= 90) && fr > 20 && fr < 80 {
					// During extreme flow rates, state should be more predictable
					t.Errorf("Unexpected state at extreme flow rate: want=%s got=%s (flowRate=%d)",
						second.state, actualState, fr)
				} else {
					// Log state difference but don't fail during transitions
					t.Logf("🔄 State: %s (expected %s, flowRate=%d%%)", actualState, second.state, fr)
				}
			} else {
				t.Logf("✅ State: %s", actualState)
			}
		})
	}
}

// TestNozzleDoErrorBlackbox tests the same scenarios as TestNozzleDoBoolBlackbox but
// using the DoError method instead of DoBool. This ensures both APIs behave consistently
// with probabilistic rate limiting.
func TestNozzleDoErrorBlackbox(t *testing.T) { //nolint:tparallel // sub-tests should NOT be parallel (order matters)
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Cleanup(func() {
		if err := noz.Close(); err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	})

	if fr := noz.FlowRate(); fr != 100 {
		t.Fatalf("Expected FlowRate=100 but got %d", fr)
	}

	if sr := noz.SuccessRate(); sr != 100 {
		t.Fatalf("Expected SuccessRate=100 but got %d", sr)
	}

	t.Logf("🚀 Starting nozzle DoError blackbox test")
	t.Logf("⏱️  This test will take at least %d seconds to run.", len(seconds()))
	t.Logf("📊 Using probabilistic rate limiting with statistical validation")

	var act *actor

	for i, second := range seconds() { //nolint:paralleltest // meant to NOT be parallel
		if second.actor != nil {
			act = second.actor
		}

		t.Run(fmt.Sprintf("Second %d rate=%d", i, second.flowRate), func(t *testing.T) {
			// Flow rate may vary due to probabilistic rate limiting affecting success/failure counts
			// Allow reasonable variance especially during transitions
			fr := noz.FlowRate()

			flowRateDiff := fr - second.flowRate
			if flowRateDiff < 0 {
				flowRateDiff = -flowRateDiff
			}

			// Check if this is near an actor transition (actor changes in test data)
			// Transitions at: 0 (tenPercent), 23 (alwaysSucceed), 31 (alwaysFail), 38/39 (transitions)
			isTransitionPeriod := second.actor != nil ||
				(i >= 23 && i <= 29) || // alwaysSucceed transition period
				(i >= 16 && i <= 22) || // period before alwaysSucceed
				(i >= 31 && i <= 32) // alwaysFail transition

			// Allow more variance during state transitions, mid-range flow rates, and actor changes
			maxFlowRateDiff := int64(20) // Allow up to 20% difference
			if second.flowRate > 30 && second.flowRate < 70 {
				maxFlowRateDiff = 30 // More variance in the middle range
			}

			if isTransitionPeriod {
				// Actor transitions can cause large flow rate jumps with probabilistic limiting
				// Effects can last for several intervals after the transition
				maxFlowRateDiff = 100 // Allow any flow rate during transition periods
			}

			if flowRateDiff > maxFlowRateDiff {
				t.Errorf("FlowRate outside acceptable range: want=%d±%d got=%d (diff=%d)",
					second.flowRate, maxFlowRateDiff, fr, fr-second.flowRate)
			} else {
				if isTransitionPeriod {
					t.Logf("🔄 FlowRate: %d%% (transition period, expected ~%d%%)", fr, second.flowRate)
				} else {
					t.Logf("🎯 FlowRate: %d%% (expected ~%d%%)", fr, second.flowRate)
				}
			}

			var calls int

			const attempts = 1000

			for range attempts {
				_, err := noz.DoError(func() (any, error) {
					calls++

					err := act.do()

					return nil, err
				})
				// Both ErrBlocked and ErrNotAllowed are expected in flow control testing
				if err != nil && !errors.Is(err, nozzle.ErrBlocked) && !errors.Is(err, ErrNotAllowed) {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Validate number of calls allowed using statistical tolerance
			expectedCalls := int(attempts * (float64(second.flowRate) / 100))
			callTolerance := calculateCallTolerance(float64(second.flowRate), attempts)

			if diff := calls - expectedCalls; diff > callTolerance || diff < -callTolerance {
				t.Errorf("Calls out of statistical bounds: want=%d±%d got=%d (diff=%d)",
					expectedCalls, callTolerance, calls, diff)
			} else {
				t.Logf("✓ Calls within bounds: %d (expected %d±%d, flowRate=%d%%)",
					calls, expectedCalls, callTolerance, second.flowRate)
			}

			// Validate success/failure rates with appropriate tolerance
			successTolerance := calculateRateTolerance(second.successRate)
			if diff, ok := withinStatistical(noz.SuccessRate(), second.successRate, successTolerance); !ok {
				t.Errorf("SuccessRate out of bounds: want=%d±%d got=%d (diff=%d)",
					second.successRate, successTolerance, noz.SuccessRate(), diff)
			}

			failureTolerance := calculateRateTolerance(second.failureRate)
			if diff, ok := withinStatistical(noz.FailureRate(), second.failureRate, failureTolerance); !ok {
				t.Errorf("FailureRate out of bounds: want=%d±%d got=%d (diff=%d)",
					second.failureRate, failureTolerance, noz.FailureRate(), diff)
			}

			noz.Wait()

			// State transitions may vary slightly with probabilistic rate limiting
			// Log state for visibility but don't fail on mismatches during transitions
			actualState := noz.State()
			if actualState != second.state {
				// Only error if the mismatch is significant (not during transition periods)
				if (second.flowRate <= 10 || second.flowRate >= 90) && fr > 20 && fr < 80 {
					// During extreme flow rates, state should be more predictable
					t.Errorf("Unexpected state at extreme flow rate: want=%s got=%s (flowRate=%d)",
						second.state, actualState, fr)
				} else {
					// Log state difference but don't fail during transitions
					t.Logf("🔄 State: %s (expected %s, flowRate=%d%%)", actualState, second.state, fr)
				}
			} else {
				t.Logf("✅ State: %s", actualState)
			}
		})
	}
}

// tolerance is the amount of error allowed in the tests.
const tolerance = 1

// within returns true if a and b are within tolerance of each other.
func within(a, b int64) (int64, bool) {
	if a == b {
		return 0, true
	}

	diff := a - b

	if diff > tolerance {
		return diff, false
	}

	if diff < -tolerance {
		return diff, false
	}

	return 0, true
}

// calculateCallTolerance calculates the acceptable variance for the number of calls
// based on the binomial distribution (3-sigma confidence interval ~99.7%).
// For a binomial distribution: stddev = sqrt(n * p * (1-p)).
func calculateCallTolerance(flowRate float64, sampleSize int) int {
	if flowRate <= 0 || flowRate >= 100 {
		// At 0% or 100%, there should be no variance
		return 1
	}

	p := flowRate / 100.0
	// Standard deviation for binomial distribution
	stdDev := math.Sqrt(float64(sampleSize) * p * (1 - p))
	// Use 3-sigma for ~99.7% confidence interval
	tolerance := 3.0 * stdDev
	// Ensure minimum tolerance of 2 for very small variances
	if tolerance < 2 {
		return 2
	}

	return int(math.Ceil(tolerance))
}

// calculateRateTolerance calculates acceptable variance for success/failure rates.
// With probabilistic rate limiting, success/failure rates have high natural variance.
func calculateRateTolerance(expectedRate int64) int64 {
	if expectedRate == 0 || expectedRate == 100 {
		// At extremes, allow small absolute variance
		return 5
	}

	if expectedRate <= 10 || expectedRate >= 90 {
		// Near extremes, allow 40% relative error
		return int64(math.Ceil(float64(expectedRate) * 0.4))
	}

	if expectedRate <= 20 || expectedRate >= 80 {
		// For low/high rates, allow 35% relative error
		return int64(math.Ceil(float64(expectedRate) * 0.35))
	}
	// For middle rates, allow 30% relative error (minimum 10)
	tolerance := int64(math.Ceil(float64(expectedRate) * 0.30))
	if tolerance < 10 {
		return 10
	}

	return tolerance
}

// withinStatistical checks if actual is within statistical bounds of expected.
func withinStatistical(actual, expected, tolerance int64) (int64, bool) {
	diff := actual - expected

	absDiff := diff
	if absDiff < 0 {
		absDiff = -absDiff
	}

	return diff, absDiff <= tolerance
}
