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

			n := Nozzle{
				flowRate: 100,
			}

			n.flowRate = test.flowRate
			n.failures = test.failures
			n.successes = test.successes

			if sr := n.SuccessRate(); sr != test.expected {
				t.Errorf("Expected SuccessRate=%d Got=%d", test.expected, sr)
			}
		})
	}
}

func TestConcurrencyBool(t *testing.T) {
	n := New(Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	var mut sync.Mutex
	var last int

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		n.DoBool(func() bool {
			defer wg.Done()

			time.Sleep(10 * time.Millisecond)

			mut.Lock()
			defer mut.Unlock()

			last = 1

			return true
		})
	}()

	go func() {
		n.DoBool(func() bool {
			defer wg.Done()

			mut.Lock()
			defer mut.Unlock()

			last = 2

			return true
		})
	}()

	wg.Wait()

	if last != 1 {
		t.Errorf("Expected last=2 Got=%d", last)
	}
}

func TestConcurrencyError(t *testing.T) {
	n := New(Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	var mut sync.Mutex
	var last int

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		n.DoError(func() error {
			defer wg.Done()

			time.Sleep(10 * time.Millisecond)

			mut.Lock()
			defer mut.Unlock()

			last = 1

			return nil
		})
	}()

	go func() {
		n.DoError(func() error {
			defer wg.Done()

			mut.Lock()
			defer mut.Unlock()

			last = 2

			return nil
		})
	}()

	wg.Wait()

	if last != 1 {
		t.Errorf("Expected last=2 Got=%d", last)
	}
}
