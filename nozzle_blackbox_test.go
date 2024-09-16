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
	return errors.New("not allowed")
}

func TestNozzleBlackbox(t *testing.T) {
	t.Parallel()

	n := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	if fr := n.FlowRate(); fr != 100 {
		t.Fatalf("Expected FlowRate=100 but got %d", fr)
	}

	if sr := n.SuccessRate(); sr != 100 {
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

	a := newActor(100)
	b := newActor(1000)
	c := newActor(0)

	seconds := []struct {
		flowRate    int
		successRate int
		failureRate int
		calls       int
		actor       *actor
	}{
		{
			flowRate:    100,
			successRate: 11,
			failureRate: 89,
			actor:       &a,
		},
		{
			flowRate:    99,
			successRate: 11,
			failureRate: 89,
		},
		{
			flowRate:    97,
			successRate: 11,
			failureRate: 89,
		},
		{
			flowRate:    93,
			successRate: 11,
			failureRate: 89,
		},
		{
			flowRate:    85,
			successRate: 12,
			failureRate: 88,
		},
		{
			flowRate:    69,
			successRate: 15,
			failureRate: 85,
		},
		{
			flowRate:    37,
			successRate: 28,
			failureRate: 72,
		},
		{
			flowRate:    0,
			successRate: 0,
			failureRate: 0,
		},
		{
			flowRate:    1,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    3,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    7,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    15,
			successRate: 67,
			failureRate: 33,
		},
		{
			flowRate:    31,
			successRate: 33,
			failureRate: 67,
		},
		{
			flowRate:    30,
			successRate: 34,
			failureRate: 66,
		},
		{
			flowRate:    28,
			successRate: 36,
			failureRate: 64,
		},
		{
			flowRate:    24,
			successRate: 42,
			failureRate: 58,
		},
		{
			flowRate:    16,
			successRate: 63,
			failureRate: 37,
		},
		{
			flowRate:    17,
			successRate: 59,
			failureRate: 41,
		},
		{
			flowRate:    19,
			successRate: 53,
			failureRate: 47,
		},
		{
			flowRate:    23,
			successRate: 44,
			failureRate: 56,
		},
		{
			flowRate:    22,
			successRate: 46,
			failureRate: 54,
		},
		{
			flowRate:    20,
			successRate: 50,
			failureRate: 50,
		},
		{
			flowRate:    21,
			successRate: 48,
			failureRate: 52,
		},
		{
			flowRate:    20,
			successRate: 100,
			failureRate: 0,
			actor:       &b,
		},
		{
			flowRate:    21,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    23,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    27,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    35,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    51,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    83,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    100,
			successRate: 100,
			failureRate: 0,
		},
		{
			flowRate:    100,
			successRate: 0,
			failureRate: 100,
			actor:       &c,
		},
		{
			flowRate:    99,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    97,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    93,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    85,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    69,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    37,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    0,
			successRate: 0,
			failureRate: 0,
		},
		{
			flowRate:    1,
			successRate: 0,
			failureRate: 100,
		},
		{
			flowRate:    0,
			successRate: 0,
			failureRate: 0,
		},
	}

	t.Logf("Warning: This test will take at least %d seconds to run.", len(seconds))

	var act *actor

	for i, second := range seconds {
		if second.actor != nil {
			act = second.actor
		}

		t.Run(fmt.Sprintf("Second %d rate=%d", i, second.flowRate), func(t *testing.T) {
			if fr := n.FlowRate(); fr != second.flowRate {
				t.Errorf("FlowRate want=%d got=%d", second.flowRate, fr)
			}

			var calls int

			for i := 0; i < 1000; i++ {
				n.Do(func(success, failure func()) {
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

			if sr := n.SuccessRate(); sr != second.successRate {
				t.Errorf("SuccessRate want=%d got=%d", second.successRate, sr)
			}

			if fr := n.FailureRate(); fr != second.failureRate {
				t.Errorf("failureRate want=%d got=%d", second.failureRate, fr)
			}
		})

		n.Wait()
	}
}
