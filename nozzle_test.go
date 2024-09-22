package nozzle //nolint:testpackage // meant to NOT be a blackbox test

import (
	"fmt"
	"sync/atomic"
	"testing"
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

	var fr atomic.Int64

	fr.Store(100)

	n := Nozzle{
		flowRate:  &fr,
		successes: &atomic.Int64{},
		failures:  &atomic.Int64{},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test=%d", i), func(t *testing.T) {
			t.Parallel()

			n.flowRate.Store(test.flowRate)
			n.failures.Store(test.failures)
			n.successes.Store(test.successes)

			if sr := n.SuccessRate(); sr != test.expected {
				t.Errorf("Expected SuccessRate=%d Got=%d", test.expected, sr)
			}
		})
	}
}
