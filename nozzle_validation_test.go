package nozzle_test

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

// TestNewValidation verifies that the New constructor properly validates options.
func TestNewValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		options     nozzle.Options[any]
		expectedErr error
	}{
		{
			name: "valid options",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: 50,
			},
			expectedErr: nil,
		},
		{
			name: "zero interval",
			options: nozzle.Options[any]{
				Interval:              0,
				AllowedFailurePercent: 50,
			},
			expectedErr: nozzle.ErrInvalidInterval,
		},
		{
			name: "negative interval",
			options: nozzle.Options[any]{
				Interval:              -1 * time.Second,
				AllowedFailurePercent: 50,
			},
			expectedErr: nozzle.ErrInvalidInterval,
		},
		{
			name: "negative failure percent",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: -1,
			},
			expectedErr: nozzle.ErrInvalidFailurePercent,
		},
		{
			name: "failure percent over 100",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: 101,
			},
			expectedErr: nozzle.ErrInvalidFailurePercent,
		},
		{
			name: "failure percent at 0",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: 0,
			},
			expectedErr: nil,
		},
		{
			name: "failure percent at 100",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: 100,
			},
			expectedErr: nil,
		},
		{
			name: "very small positive interval",
			options: nozzle.Options[any]{
				Interval:              1 * time.Nanosecond,
				AllowedFailurePercent: 50,
			},
			expectedErr: nil,
		},
		{
			name: "very large failure percent",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: 1000,
			},
			expectedErr: nozzle.ErrInvalidFailurePercent,
		},
		{
			name: "very negative failure percent",
			options: nozzle.Options[any]{
				Interval:              time.Second,
				AllowedFailurePercent: -100,
			},
			expectedErr: nozzle.ErrInvalidFailurePercent,
		},
	}

	for _, tt := range tests {
		// capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			noz, err := nozzle.New(tt.options)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}

				if noz != nil {
					t.Error("expected nil nozzle when error is returned")
				}

				return // Early return for error cases
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if noz == nil {
				t.Error("expected non-nil nozzle when no error")

				return
			}

			if err := noz.Close(); err != nil {
				t.Errorf("error closing nozzle: %v", err)
			}
		})
	}
}

// TestValidationErrorMessages verifies that validation errors have the expected messages.
func TestValidationErrorMessages(t *testing.T) {
	t.Parallel()

	if !errors.Is(nozzle.ErrInvalidInterval, nozzle.ErrInvalidInterval) {
		t.Error("ErrInvalidInterval should be comparable with errors.Is")
	}

	expectedMsg := "nozzle: interval must be positive"
	if nozzle.ErrInvalidInterval.Error() != expectedMsg {
		t.Errorf("ErrInvalidInterval message = %q, want %q", nozzle.ErrInvalidInterval.Error(), expectedMsg)
	}

	if !errors.Is(nozzle.ErrInvalidFailurePercent, nozzle.ErrInvalidFailurePercent) {
		t.Error("ErrInvalidFailurePercent should be comparable with errors.Is")
	}

	expectedMsg = "nozzle: allowed failure percent must be between 0 and 100"
	if nozzle.ErrInvalidFailurePercent.Error() != expectedMsg {
		t.Errorf("ErrInvalidFailurePercent message = %q, want %q", nozzle.ErrInvalidFailurePercent.Error(), expectedMsg)
	}
}

// TestValidationErrorsAreSentinel verifies that the validation errors are proper sentinel errors.
func TestValidationErrorsAreSentinel(t *testing.T) {
	t.Parallel()

	_, err := nozzle.New(nozzle.Options[any]{
		Interval:              0,
		AllowedFailurePercent: 50,
	})
	if !errors.Is(err, nozzle.ErrInvalidInterval) {
		t.Errorf("zero interval should return ErrInvalidInterval, got %v", err)
	}

	_, err = nozzle.New(nozzle.Options[any]{
		Interval:              time.Second,
		AllowedFailurePercent: -10,
	})
	if !errors.Is(err, nozzle.ErrInvalidFailurePercent) {
		t.Errorf("negative failure percent should return ErrInvalidFailurePercent, got %v", err)
	}
}

// countGoroutines returns the current number of goroutines.
func countGoroutines() int {
	return runtime.NumGoroutine()
}

// TestNewDoesNotLeakOnValidationError verifies that no goroutines are leaked when validation fails.
func TestNewDoesNotLeakOnValidationError(t *testing.T) { //nolint:paralleltest // Cannot run in parallel - counts goroutines
	time.Sleep(100 * time.Millisecond)

	initialGoroutines := countGoroutines()

	for range 10 {
		_, err := nozzle.New(nozzle.Options[any]{
			Interval:              0, // Invalid
			AllowedFailurePercent: 50,
		})
		if err == nil {
			t.Fatal("expected error for zero interval")
		}

		_, err = nozzle.New(nozzle.Options[any]{
			Interval:              time.Second,
			AllowedFailurePercent: -10, // Invalid
		})
		if err == nil {
			t.Fatal("expected error for negative failure percent")
		}
	}

	time.Sleep(10 * time.Millisecond)

	finalGoroutines := countGoroutines()

	if finalGoroutines > initialGoroutines {
		t.Errorf("goroutine leak detected: started with %d, ended with %d",
			initialGoroutines, finalGoroutines)
	}
}
