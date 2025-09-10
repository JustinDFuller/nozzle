package nozzle_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

func TestDoErrorContext(t *testing.T) {
	t.Parallel()

	t.Run("returns context error when context is already cancelled", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before calling

		called := false
		result, err := n.DoErrorContext(ctx, func(_ context.Context) (string, error) {
			called = true

			return "should not be called", nil
		})

		if called {
			t.Error("callback should not have been called when context is cancelled")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}

		if result != "" {
			t.Errorf("expected empty result, got %s", result)
		}
	})

	t.Run("returns context timeout error when context times out", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		time.Sleep(time.Millisecond) // Ensure timeout

		called := false
		result, err := n.DoErrorContext(ctx, func(_ context.Context) (string, error) {
			called = true

			return "should not be called", nil
		})

		if called {
			t.Error("callback should not have been called when context is timed out")
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded error, got %v", err)
		}

		if result != "" {
			t.Errorf("expected empty result, got %s", result)
		}
	})

	t.Run("executes callback when context is valid", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx := context.Background()

		called := false
		expectedResult := "success"
		result, err := n.DoErrorContext(ctx, func(_ context.Context) (string, error) {
			called = true

			return expectedResult, nil
		})

		if !called {
			t.Error("callback should have been called")
		}

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result != expectedResult {
			t.Errorf("expected result %s, got %s", expectedResult, result)
		}
	})

	t.Run("handles context cancellation during callback execution", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())

		result, err := n.DoErrorContext(ctx, func(ctx context.Context) (string, error) {
			cancel() // Cancel during callback

			return "partial", ctx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}

		if result != "partial" {
			t.Errorf("expected partial result, got %s", result)
		}
	})

	t.Run("returns ErrClosed when nozzle is closed", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		if err := n.Close(); err != nil {
			t.Fatalf("failed to close nozzle: %v", err)
		}

		ctx := context.Background()

		called := false
		result, err := n.DoErrorContext(ctx, func(_ context.Context) (string, error) {
			called = true

			return "should not be called", nil
		})

		if called {
			t.Error("callback should not have been called when nozzle is closed")
		}

		if !errors.Is(err, nozzle.ErrClosed) {
			t.Errorf("expected nozzle.ErrClosed error, got %v", err)
		}

		if result != "" {
			t.Errorf("expected empty result, got %s", result)
		}
	})

	t.Run("returns ErrBlocked when flow rate is zero", func(t *testing.T) {
		t.Parallel()

		noz := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 0,
		})

		defer func() {
			if err := noz.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		// Force failures to close the nozzle
		for range 10 {
			_, _ = noz.DoError(func() (string, error) {
				return "", errors.New("fail")
			})
		}

		// Wait for nozzle to process and close
		time.Sleep(time.Millisecond * 50)

		ctx := context.Background()

		// After enough failures, nozzle should block
		var blockedCount int

		for range 100 {
			_, err := noz.DoErrorContext(ctx, func(_ context.Context) (string, error) {
				return "success", nil
			})
			if errors.Is(err, nozzle.ErrBlocked) {
				blockedCount++
			}
		}

		if blockedCount == 0 {
			t.Error("expected at least some calls to be blocked")
		}
	})
}

func TestDoBoolContext(t *testing.T) {
	t.Parallel()

	t.Run("returns false when context is already cancelled", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before calling

		called := false
		result, ok := n.DoBoolContext(ctx, func(_ context.Context) (string, bool) {
			called = true

			return "should not be called", true
		})

		if called {
			t.Error("callback should not have been called when context is cancelled")
		}

		if ok {
			t.Error("expected false when context is cancelled")
		}

		if result != "" {
			t.Errorf("expected empty result, got %s", result)
		}
	})

	t.Run("returns false when context times out", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		time.Sleep(time.Millisecond) // Ensure timeout

		called := false
		result, ok := n.DoBoolContext(ctx, func(_ context.Context) (string, bool) {
			called = true

			return "should not be called", true
		})

		if called {
			t.Error("callback should not have been called when context is timed out")
		}

		if ok {
			t.Error("expected false when context times out")
		}

		if result != "" {
			t.Errorf("expected empty result, got %s", result)
		}
	})

	t.Run("executes callback when context is valid", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx := context.Background()

		called := false
		expectedResult := "success"
		result, ok := n.DoBoolContext(ctx, func(_ context.Context) (string, bool) {
			called = true

			return expectedResult, true
		})

		if !called {
			t.Error("callback should have been called")
		}

		if !ok {
			t.Error("expected true for successful callback")
		}

		if result != expectedResult {
			t.Errorf("expected result %s, got %s", expectedResult, result)
		}
	})

	t.Run("handles context cancellation during callback execution", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())

		result, ok := n.DoBoolContext(ctx, func(ctx context.Context) (string, bool) {
			cancel() // Cancel during callback

			select {
			case <-ctx.Done():
				return "partial", false
			default:
				return "partial", true
			}
		})

		// The callback may return either true or false depending on timing
		// What matters is that it was executed
		if result != "partial" {
			t.Errorf("expected partial result, got %s", result)
		}

		// The ok value depends on what the callback returned
		_ = ok
	})

	t.Run("returns false when nozzle is closed", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		if err := n.Close(); err != nil {
			t.Fatalf("failed to close nozzle: %v", err)
		}

		ctx := context.Background()

		called := false
		result, ok := n.DoBoolContext(ctx, func(_ context.Context) (string, bool) {
			called = true

			return "should not be called", true
		})

		if called {
			t.Error("callback should not have been called when nozzle is closed")
		}

		if ok {
			t.Error("expected false when nozzle is closed")
		}

		if result != "" {
			t.Errorf("expected empty result, got %s", result)
		}
	})

	t.Run("returns false when flow rate is zero", func(t *testing.T) {
		t.Parallel()

		noz := nozzle.New[string](nozzle.Options[string]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 0,
		})

		defer func() {
			if err := noz.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		// Force failures to close the nozzle
		for range 10 {
			_, _ = noz.DoBool(func() (string, bool) {
				return "", false
			})
		}

		// Wait for nozzle to process and close
		time.Sleep(time.Millisecond * 50)

		ctx := context.Background()

		// After enough failures, nozzle should block
		var blockedCount int

		for range 100 {
			_, ok := noz.DoBoolContext(ctx, func(_ context.Context) (string, bool) {
				return "success", true
			})
			if !ok {
				blockedCount++
			}
		}

		if blockedCount == 0 {
			t.Error("expected at least some calls to be blocked")
		}
	})
}

func TestContextMethodsConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("DoErrorContext handles concurrent calls", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[int](nozzle.Options[int]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx := context.Background()

		var wg sync.WaitGroup

		const numGoroutines = 100

		for i := range numGoroutines {
			wg.Add(1)

			go func(val int) {
				defer wg.Done()

				result, err := n.DoErrorContext(ctx, func(_ context.Context) (int, error) {
					return val, nil
				})
				if err != nil && !errors.Is(err, nozzle.ErrBlocked) {
					t.Errorf("unexpected error: %v", err)
				}

				if err == nil && result != val {
					t.Errorf("expected result %d, got %d", val, result)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("DoBoolContext handles concurrent calls", func(t *testing.T) {
		t.Parallel()

		n := nozzle.New[int](nozzle.Options[int]{
			Interval:              time.Millisecond * 10,
			AllowedFailurePercent: 50,
		})

		defer func() {
			if err := n.Close(); err != nil {
				t.Errorf("failed to close nozzle: %v", err)
			}
		}()

		ctx := context.Background()

		var wg sync.WaitGroup

		const numGoroutines = 100

		for i := range numGoroutines {
			wg.Add(1)

			go func(val int) {
				defer wg.Done()

				result, ok := n.DoBoolContext(ctx, func(_ context.Context) (int, bool) {
					return val, true
				})
				if ok && result != val {
					t.Errorf("expected result %d, got %d", val, result)
				}
			}(i)
		}

		wg.Wait()
	})
}
