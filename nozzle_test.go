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

func newActor(limit rate.Limit) actor {
	return actor{
		limiter: rate.NewLimiter(limit, 1),
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

func TestNozzle(t *testing.T) {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		Step:                  5,
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
		second      int
		flowRate    int
		successRate int
		calls       int
	}{
		{
			second:      0,
			flowRate:    100,
			successRate: 100,
		},
		{
			second:      1,
			flowRate:    90,
			successRate: 10,
		},
		{
			second:      2,
			flowRate:    80,
			successRate: 11,
		},
		{
			second:      3,
			flowRate:    70,
			successRate: 12,
		},
		{
			second:      4,
			flowRate:    60,
			successRate: 14,
		},
		{
			second:      5,
			flowRate:    50,
			successRate: 17,
		},
		{
			second:      6,
			flowRate:    40,
			successRate: 20,
		},
		{
			second:      7,
			flowRate:    30,
			successRate: 25,
		},
		{
			second:      8,
			flowRate:    20,
			successRate: 33,
		},
		{
			second:      9,
			flowRate:    30,
			successRate: 50,
		},
		{
			second:      10,
			flowRate:    20,
			successRate: 33,
		},
	}

	a := newActor(rate.Limit(100))

	for _, second := range seconds {
		t.Run(fmt.Sprintf("Second %d", second.second), func(t *testing.T) {
			if fr := n.FlowRate(); fr != second.flowRate {
				t.Errorf("Expected FlowRate=%d but got %d", second.flowRate, fr)
			}

			if sr := n.SuccessRate(); sr != second.successRate {
				t.Errorf("Expected SuccessRate=%d but got %d", second.successRate, sr)
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

			if expected := int(1000 * (float64(second.flowRate) / 100)); calls != expected {
				t.Errorf("Expected %d calls but got %d", expected, calls)
			}
		})

		time.Sleep(time.Second)
	}
}
