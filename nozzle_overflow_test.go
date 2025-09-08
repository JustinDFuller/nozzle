package nozzle

import (
	"math"
	"testing"
	"time"
)

// TestNozzleDecreaseByOverflow verifies that decreaseBy doesn't overflow even after many iterations.
func TestNozzleDecreaseByOverflow(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	// Simulate many failures to trigger exponential growth of decreaseBy
	for i := range 100 {
		// Force a close operation by setting high failure rate
		n.mut.Lock()
		n.failures = 100
		n.successes = 0
		n.close() // This will double decreaseBy each time

		// Verify decreaseBy is capped and doesn't overflow
		if n.decreaseBy > 0 || n.decreaseBy < -maxDecreaseBy {
			t.Errorf("iteration %d: decreaseBy out of bounds: %d (expected between -%d and 0)",
				i, n.decreaseBy, maxDecreaseBy)
		}

		n.mut.Unlock()
	}

	// Verify that after many iterations, decreaseBy is at the cap
	n.mut.Lock()

	if n.decreaseBy != -maxDecreaseBy {
		t.Errorf("decreaseBy should be capped at -%d, got %d", maxDecreaseBy, n.decreaseBy)
	}

	n.mut.Unlock()
}

// TestDecreaseByBounds verifies exponential growth limits for both positive and negative values.
func TestDecreaseByBounds(t *testing.T) {
	tests := []struct {
		name       string
		initial    int64
		iterations int
		isClosing  bool
		wantCapped bool
	}{
		{
			name:       "closing reaches negative cap",
			initial:    -1,
			iterations: 50, // 2^50 would overflow without protection
			isClosing:  true,
			wantCapped: true,
		},
		{
			name:       "opening reaches positive cap",
			initial:    1,
			iterations: 50, // 2^50 would overflow without protection
			isClosing:  false,
			wantCapped: true,
		},
		{
			name:       "small iterations don't cap",
			initial:    -1,
			iterations: 10, // 2^10 = 1024, well below cap
			isClosing:  true,
			wantCapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New[any](Options[any]{
				Interval:              10 * time.Millisecond,
				AllowedFailurePercent: 10,
			})
			defer n.Close()

			n.mut.Lock()
			n.decreaseBy = tt.initial

			for range tt.iterations {
				if tt.isClosing {
					n.close()
				} else {
					n.flowRate = 50 // Ensure we're not at 100% so open() doesn't return early
					n.open()
				}
			}

			if tt.wantCapped {
				if tt.isClosing && n.decreaseBy != -maxDecreaseBy {
					t.Errorf("expected decreaseBy to be capped at -%d, got %d", maxDecreaseBy, n.decreaseBy)
				} else if !tt.isClosing && n.decreaseBy != maxDecreaseBy {
					t.Errorf("expected decreaseBy to be capped at %d, got %d", maxDecreaseBy, n.decreaseBy)
				}
			} else {
				if tt.isClosing && n.decreaseBy <= -maxDecreaseBy {
					t.Errorf("decreaseBy should not be capped for small iterations, got %d", n.decreaseBy)
				} else if !tt.isClosing && n.decreaseBy >= maxDecreaseBy {
					t.Errorf("decreaseBy should not be capped for small iterations, got %d", n.decreaseBy)
				}
			}

			// Always verify no overflow occurred
			if n.decreaseBy > maxDecreaseBy || n.decreaseBy < -maxDecreaseBy {
				t.Errorf("decreaseBy overflowed bounds: %d", n.decreaseBy)
			}

			n.mut.Unlock()
		})
	}
}

// TestSafeMultiply verifies the overflow protection in the safeMultiply function.
func TestSafeMultiply(t *testing.T) {
	tests := []struct {
		name string
		a    int64
		b    int64
		want int64
	}{
		{
			name: "normal multiplication",
			a:    100,
			b:    2,
			want: 200,
		},
		{
			name: "negative multiplication",
			a:    -100,
			b:    2,
			want: -200,
		},
		{
			name: "zero multiplication",
			a:    0,
			b:    1000,
			want: 0,
		},
		{
			name: "positive overflow",
			a:    math.MaxInt64/2 + 1,
			b:    2,
			want: math.MaxInt64,
		},
		{
			name: "negative overflow",
			a:    math.MinInt64/2 - 1,
			b:    2,
			want: math.MinInt64,
		},
		{
			name: "large but safe multiplication",
			a:    1000000,
			b:    1000,
			want: 1000000000,
		},
		{
			name: "both negative resulting in positive",
			a:    -100,
			b:    -2,
			want: 200,
		},
		{
			name: "near max positive multiplication",
			a:    math.MaxInt64 / 3,
			b:    2,
			want: (math.MaxInt64 / 3) * 2,
		},
		{
			name: "extreme positive overflow",
			a:    math.MaxInt64,
			b:    2,
			want: math.MaxInt64,
		},
		{
			name: "extreme negative overflow",
			a:    math.MinInt64,
			b:    2,
			want: math.MinInt64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeMultiply(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("safeMultiply(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestNozzleLongRunningNoOverflow ensures the nozzle can run for extended periods without overflow.
func TestNozzleLongRunningNoOverflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	n := New[any](Options[any]{
		Interval:              5 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	// Run for 1000 intervals with alternating failure patterns
	for i := range 1000 {
		// Simulate varying failure rates
		n.mut.Lock()

		if i%10 < 3 {
			// High failure rate to trigger closing
			n.failures = 90
			n.successes = 10
		} else {
			// Low failure rate to trigger opening
			n.failures = 5
			n.successes = 95
		}

		n.calculate()

		// Verify bounds are maintained
		if n.decreaseBy > maxDecreaseBy || n.decreaseBy < -maxDecreaseBy {
			t.Fatalf("iteration %d: decreaseBy out of bounds: %d", i, n.decreaseBy)
		}

		n.mut.Unlock()

		// Small delay to simulate real-world timing
		time.Sleep(1 * time.Millisecond)
	}
}

// TestOverflowProtectionConcurrent verifies overflow protection works correctly under concurrent access.
func TestOverflowProtectionConcurrent(t *testing.T) {
	n := New[any](Options[any]{
		Interval:              10 * time.Millisecond,
		AllowedFailurePercent: 10,
	})
	defer n.Close()

	done := make(chan bool)

	// Start multiple goroutines that trigger close operations
	for range 10 {
		go func() {
			for range 100 {
				n.mut.Lock()
				n.failures = 100
				n.successes = 0
				n.close()

				// Verify bounds
				if n.decreaseBy > 0 || n.decreaseBy < -maxDecreaseBy {
					t.Errorf("decreaseBy out of bounds in goroutine: %d", n.decreaseBy)
				}

				n.mut.Unlock()
				time.Sleep(1 * time.Microsecond)
			}

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}

	// Final verification
	n.mut.Lock()

	if n.decreaseBy < -maxDecreaseBy || n.decreaseBy > maxDecreaseBy {
		t.Errorf("final decreaseBy out of bounds: %d", n.decreaseBy)
	}

	n.mut.Unlock()
}
