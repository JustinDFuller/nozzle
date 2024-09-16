package nozzle_test

import (
	"errors"
	"fmt"
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

func (a *actor) do() error {
	if a.limiter.Allow() {
		a.count++
		a.success++

		return nil
	}

	a.count++
	a.fail++

	return errors.New("not allowed") //nolint:err113 // Just a testing error
}

func TestNozzleBlackbox(t *testing.T) { //nolint:tparallel // sub-tests should NOT be parallel (order matters)
	t.Parallel()

	noz := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	if fr := noz.FlowRate(); fr != 100 {
		t.Fatalf("Expected FlowRate=100 but got %d", fr)
	}

	if sr := noz.SuccessRate(); sr != 100 {
		t.Fatalf("Expected SuccessRate=100 but got %d", sr)
	}

	// FIRST TEST:
	// set up an actor that allows 100 RPS.
	// send it 1000 RPS.
	// nozzle allows a 50% error rate.
	// nozzle has an interval of 1s.
	// nozzle has a step of 10.
	//
	// EXPECTATIONS:
	// We should get down to a 20% flow rate.
	// It should get there after 8 seconds.
	// It should then continue to try to go up to 30%
	// Then go back to 20% when it determines that opens the error rate.

	tenPercent := newActor(100)
	alwaysSucceed := newActor(1000)
	alwaysFail := newActor(0)

	seconds := []struct {
		flowRate    int
		successRate int
		failureRate int
		calls       int
		state       nozzle.State
		actor       *actor
	}{
		{
			flowRate:    100,
			successRate: 11,
			failureRate: 89,
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
			successRate: 46,
			failureRate: 54,
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

	t.Logf("Warning: This test will take at least %d seconds to run.", len(seconds))

	var act *actor

	for i, second := range seconds { //nolint:paralleltest // meant to NOT be parallel
		if second.actor != nil {
			act = second.actor
		}

		t.Run(fmt.Sprintf("Second %d rate=%d", i, second.flowRate), func(t *testing.T) {
			if fr := noz.FlowRate(); fr != second.flowRate {
				t.Errorf("FlowRate want=%d got=%d", second.flowRate, fr)
			}

			var calls int

			for range 1000 {
				noz.Do(func(success, failure func()) {
					calls++

					if success == nil {
						t.Errorf("Got nil success function")
					}

					if failure == nil {
						t.Errorf("Got nil failure function")
					}

					err := act.do()
					if err == nil {
						success()
					} else {
						failure()
					}
				})
			}

			if expected := int(1000 * (float64(second.flowRate) / 100)); calls-expected > 1 || calls-expected < -1 {
				t.Errorf("Calls want=%d got=%d", expected, calls)
			}

			if sr := noz.SuccessRate(); sr != second.successRate {
				t.Errorf("SuccessRate want=%d got=%d", second.successRate, sr)
			}

			if fr := noz.FailureRate(); fr != second.failureRate {
				t.Errorf("failureRate want=%d got=%d", second.failureRate, fr)
			}

			noz.Wait()

			if noz.State() != second.state {
				t.Errorf("Expected state=%s got=%s", second.state, noz.State())
			}
		})
	}
}
