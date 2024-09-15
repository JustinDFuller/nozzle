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
	t.Log("Warning: This test will take at least 10 seconds to run.")

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

	seconds := []struct {
		flowRate    int
		successRate int
		calls       int
	}{
		{
			flowRate:    100,
			successRate: 11,
		},
		{
			flowRate:    49,
			successRate: 21,
		},
		{
			flowRate:    23,
			successRate: 44,
		},
		{
			flowRate:    10,
			successRate: 100,
		},
		{
			flowRate:    17,
			successRate: 59,
		},
		{
			flowRate:    21,
			successRate: 48,
		},
		{
			flowRate:    18,
			successRate: 56,
		},
		{
			flowRate:    20,
			successRate: 50,
		},
		{
			flowRate:    21,
			successRate: 48,
		},
		{
			flowRate:    19,
			successRate: 53,
		},
		{
			flowRate:    21,
			successRate: 48,
		},
		{
			flowRate:    19,
			successRate: 53,
		},
	}

	a := newActor(100)

	for i, second := range seconds {
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

					err := a.do()
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
		})

		n.Wait()
	}
}
