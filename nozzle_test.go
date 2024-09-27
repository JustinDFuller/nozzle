package nozzle //nolint:testpackage // meant to NOT be a blackbox test

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSuccessRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expected  int64
		failures  int64
		successes int64
		flowRate  int64
	}{
		{
			expected:  100,
			failures:  0,
			successes: 0,
			flowRate:  100,
		},
		{
			expected:  100,
			failures:  0,
			successes: 100,
			flowRate:  100,
		},
		{
			expected:  0,
			failures:  100,
			successes: 0,
			flowRate:  100,
		},
		{
			expected:  50,
			failures:  50,
			successes: 50,
			flowRate:  100,
		},
		{
			expected:  0,
			failures:  50,
			successes: 50,
			flowRate:  0,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test=%d", i), func(t *testing.T) {
			t.Parallel()

			noz := Nozzle[any]{
				flowRate: 100,
			}

			noz.flowRate = test.flowRate
			noz.failures = test.failures
			noz.successes = test.successes

			if sr := noz.SuccessRate(); sr != test.expected {
				t.Errorf("Expected SuccessRate=%d Got=%d", test.expected, sr)
			}
		})
	}
}

func TestConcurrencyBool(t *testing.T) {
	t.Parallel()

	noz := New[any](Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	var mut sync.Mutex
	var last int

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		noz.DoBool(func() (any, bool) {
			defer wg.Done()

			time.Sleep(10 * time.Millisecond)

			mut.Lock()
			defer mut.Unlock()

			last = 1

			return nil, true
		})
	}()

	go func() {
		noz.DoBool(func() (any, bool) {
			defer wg.Done()

			mut.Lock()
			defer mut.Unlock()

			last = 2

			return nil, true
		})
	}()

	wg.Wait()

	if last != 1 {
		t.Errorf("Expected last=2 Got=%d", last)
	}
}

func TestConcurrencyError(t *testing.T) {
	t.Parallel()

	noz := New[any](Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	var mut sync.Mutex
	var last int

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		noz.DoError(func() (any, error) {
			defer wg.Done()

			time.Sleep(10 * time.Millisecond)

			mut.Lock()
			defer mut.Unlock()

			last = 1

			return nil, nil
		})
	}()

	go func() {
		noz.DoError(func() (any, error) {
			defer wg.Done()

			mut.Lock()
			defer mut.Unlock()

			last = 2

			return nil, nil
		})
	}()

	wg.Wait()

	if last != 1 {
		t.Errorf("Expected last=2 Got=%d", last)
	}
}
