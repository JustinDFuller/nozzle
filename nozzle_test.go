package nozzle //nolint:testpackage // meant to NOT be a blackbox test

import (
	"errors"
	"fmt"
	"runtime"
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

	noz := New(Options[any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	t.Cleanup(func() {
		err := noz.Close()
		if err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	})

	var (
		mut  sync.Mutex
		last int
	)

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

	noz := New(Options[any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	t.Cleanup(func() {
		err := noz.Close()
		if err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	})

	var (
		mut  sync.Mutex
		last int
	)

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		_, err := noz.DoError(func() (any, error) {
			defer wg.Done()

			time.Sleep(10 * time.Millisecond)

			mut.Lock()
			defer mut.Unlock()

			last = 1

			return nil, nil
		})
		if err != nil && !errors.Is(err, ErrBlocked) {
			t.Errorf("Unexpected error in goroutine 1: %v", err)
		}
	}()

	go func() {
		_, err := noz.DoError(func() (any, error) {
			defer wg.Done()

			mut.Lock()
			defer mut.Unlock()

			last = 2

			return nil, nil
		})
		if err != nil && !errors.Is(err, ErrBlocked) {
			t.Errorf("Unexpected error in goroutine 2: %v", err)
		}
	}()

	wg.Wait()

	if last != 1 {
		t.Errorf("Expected last=2 Got=%d", last)
	}
}

// TestNozzleNoGoroutineLeak ensures that closing nozzles properly cleans up goroutines.
func TestNozzleNoGoroutineLeak(t *testing.T) {
	t.Parallel()

	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	baseline := runtime.NumGoroutine()

	// Create multiple nozzles
	nozzles := make([]*Nozzle[any], 10)
	for i := range nozzles {
		nozzles[i] = New(Options[any]{
			Interval:              100 * time.Millisecond,
			AllowedFailurePercent: 50,
		})
	}

	// Verify goroutines were created
	time.Sleep(100 * time.Millisecond)

	withNozzles := runtime.NumGoroutine()

	if withNozzles <= baseline {
		t.Errorf("Expected goroutines to be created, baseline=%d, with nozzles=%d", baseline, withNozzles)
	}

	// Close all nozzles
	for _, n := range nozzles {
		err := n.Close()
		if err != nil {
			t.Errorf("Failed to close nozzle: %v", err)
		}
	}

	// Wait for goroutines to exit
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check that goroutine count returned to baseline (with some tolerance)
	afterClose := runtime.NumGoroutine()
	if afterClose > baseline+2 { // Allow small variance
		t.Errorf("Goroutine leak detected: baseline=%d, after close=%d", baseline, afterClose)
	}
}

// TestCloseIdempotent ensures Close can be called multiple times safely.
func TestCloseIdempotent(t *testing.T) {
	t.Parallel()

	n := New(Options[any]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 50,
	})

	// Call Close multiple times
	for i := range 5 {
		err := n.Close()
		if err != nil {
			t.Errorf("Close() call %d returned error: %v", i, err)
		}
	}
}

// TestConcurrentClose ensures Close is thread-safe.
func TestConcurrentClose(t *testing.T) {
	t.Parallel()

	n := New(Options[any]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 50,
	})

	var wg sync.WaitGroup
	// Launch multiple goroutines to close concurrently
	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := n.Close()
			if err != nil {
				t.Errorf("Concurrent Close() returned error: %v", err)
			}
		}()
	}

	wg.Wait()
}

// TestOperationsAfterClose ensures operations handle closed state gracefully.
func TestOperationsAfterClose(t *testing.T) {
	t.Parallel()

	nozzle := New(Options[any]{
		Interval:              100 * time.Millisecond,
		AllowedFailurePercent: 50,
	})

	// Close the nozzle
	if err := nozzle.Close(); err != nil {
		t.Fatalf("Failed to close nozzle: %v", err)
	}

	// Wait a bit to ensure close completes
	time.Sleep(100 * time.Millisecond)

	// Test DoBool after close - should return (zero value, false)
	result, ok := nozzle.DoBool(func() (any, bool) {
		t.Error("Callback should not be called on closed nozzle")

		return "test", true
	})
	if ok {
		t.Error("DoBool should return false for closed nozzle")
	}

	if result != nil {
		t.Errorf("DoBool should return zero value for closed nozzle, got: %v", result)
	}

	// Test DoError after close - should return (zero value, ErrClosed)
	result2, err := nozzle.DoError(func() (any, error) {
		t.Error("Callback should not be called on closed nozzle")

		return "test", nil
	})
	if !errors.Is(err, ErrClosed) {
		t.Errorf("DoError should return ErrClosed for closed nozzle, got: %v", err)
	}

	if result2 != nil {
		t.Errorf("DoError should return zero value for closed nozzle, got: %v", result2)
	}
}
