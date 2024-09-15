package nozzle

import (
	"fmt"
	"testing"
)

func TestSuccessRate(t *testing.T) {
	tests := []struct {
		expected  int
		failures  int
		successes int
	}{
		{
			expected:  100,
			failures:  0,
			successes: 0,
		},
		{
			expected:  100,
			failures:  0,
			successes: 100,
		},
		{
			expected:  0,
			failures:  100,
			successes: 0,
		},
		{
			expected:  50,
			failures:  50,
			successes: 50,
		},
	}

	n := Nozzle{}

	for i, test := range tests {
		t.Run(fmt.Sprintf("test=%d", i), func(t *testing.T) {
			n.failures = int64(test.failures)
			n.successes = int64(test.successes)

			if sr := n.SuccessRate(); sr != test.expected {
				t.Errorf("Expected SuccessRate=%d Got=%d", test.expected, sr)
			}
		})
	}
}
