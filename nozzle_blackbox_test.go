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

func TestNozzleDoBoolBlackbox(t *testing.T) { //nolint:tparallel // sub-tests should NOT be parallel (order matters)
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

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

	t.Logf("Warning: This test will take at least %d seconds to run.", len(seconds()))

	var act *actor

	for i, second := range seconds() { //nolint:paralleltest // meant to NOT be parallel
		if second.actor != nil {
			act = second.actor
		}

		t.Run(fmt.Sprintf("Second %d rate=%d", i, second.flowRate), func(t *testing.T) {
			if fr := noz.FlowRate(); fr != second.flowRate {
				t.Errorf("FlowRate want=%d got=%d", second.flowRate, fr)
			}

			var calls int

			for range 1000 {
				noz.DoBool(func() bool {
					calls++

					err := act.do()

					return err == nil
				})
			}

			if expected := int(1000 * (float64(second.flowRate) / 100)); calls-expected > 1 || calls-expected < -1 {
				t.Errorf("Calls want=%d got=%d", expected, calls)
			}

			if diff, ok := within(noz.SuccessRate(), second.successRate, 1); !ok {
				t.Errorf("SuccessRate want=%d got=%d diff=%d", second.successRate, noz.SuccessRate(), diff)
			}

			if diff, ok := within(noz.FailureRate(), second.failureRate, 1); !ok {
				t.Errorf("failureRate want=%d got=%d diff=%d", second.failureRate, noz.FailureRate(), diff)
			}

			noz.Wait()

			if noz.State() != second.state {
				t.Errorf("Expected state=%s got=%s", second.state, noz.State())
			}
		})
	}
}

func TestNozzleDoErrorBlackbox(t *testing.T) { //nolint:tparallel // sub-tests should NOT be parallel (order matters)
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

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

	t.Logf("Warning: This test will take at least %d seconds to run.", len(seconds()))

	var act *actor

	for i, second := range seconds() { //nolint:paralleltest // meant to NOT be parallel
		if second.actor != nil {
			act = second.actor
		}

		t.Run(fmt.Sprintf("Second %d rate=%d", i, second.flowRate), func(t *testing.T) {
			if fr := noz.FlowRate(); fr != second.flowRate {
				t.Errorf("FlowRate want=%d got=%d", second.flowRate, fr)
			}

			var calls int

			for range 1000 {
				noz.DoError(func() error {
					calls++

					err := act.do()

					return err
				})
			}

			if expected := int(1000 * (float64(second.flowRate) / 100)); calls-expected > 1 || calls-expected < -1 {
				t.Errorf("Calls want=%d got=%d", expected, calls)
			}

			if diff, ok := within(noz.SuccessRate(), second.successRate, 1); !ok {
				t.Errorf("SuccessRate want=%d got=%d diff=%d", second.successRate, noz.SuccessRate(), diff)
			}

			if diff, ok := within(noz.FailureRate(), second.failureRate, 1); !ok {
				t.Errorf("failureRate want=%d got=%d diff=%d", second.failureRate, noz.FailureRate(), diff)
			}

			noz.Wait()

			if noz.State() != second.state {
				t.Errorf("Expected state=%s got=%s", second.state, noz.State())
			}
		})
	}
}

func within(a, b, tolerance int64) (int64, bool) {
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
