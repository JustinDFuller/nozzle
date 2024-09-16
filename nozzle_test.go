package nozzle

import (
	"fmt"
	"testing"
)

func TestSuccessRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expected  int
		failures  int
		successes int
		flowRate  int
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

	n := Nozzle{
		flowRate: 100,
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test=%d", i), func(t *testing.T) {
			t.Parallel()

			n.flowRate = test.flowRate
			n.failures = int64(test.failures)
			n.successes = int64(test.successes)

			if sr := n.SuccessRate(); sr != test.expected {
				t.Errorf("Expected SuccessRate=%d Got=%d", test.expected, sr)
			}
		})
	}
}
