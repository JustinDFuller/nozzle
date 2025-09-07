package nozzle_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

func BenchmarkNozzle_DoBool_Open(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	act := newActor(b.N)

	for range b.N {
		noz.DoBool(func() (any, bool) {
			err := act.do()

			return nil, err == nil
		})
	}
}

func BenchmarkNozzle_DoBool_Closed(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	act := newActor(0)

	for range b.N {
		noz.DoBool(func() (any, bool) {
			err := act.do()

			return nil, err == nil
		})
	}
}

func BenchmarkNozzle_DoBool_Half(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	act := newActor(b.N / 2)

	for range b.N {
		noz.DoBool(func() (any, bool) {
			err := act.do()

			return nil, err == nil
		})
	}
}

func BenchmarkNozzle_DoError_Open(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	act := newActor(b.N)

	for range b.N {
		_, err := noz.DoError(func() (any, error) {
			err := act.do()

			return nil, err
		})
		// In benchmarks, we expect both ErrBlocked and actual errors from act.do()
		// Both are valid outcomes for flow control testing
		if err != nil && !errors.Is(err, nozzle.ErrBlocked) && !errors.Is(err, ErrNotAllowed) {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkNozzle_DoError_Closed(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	act := newActor(0)

	for range b.N {
		_, err := noz.DoError(func() (any, error) {
			err := act.do()

			return nil, err
		})
		// In benchmarks, we expect both ErrBlocked and actual errors from act.do()
		// Both are valid outcomes for flow control testing
		if err != nil && !errors.Is(err, nozzle.ErrBlocked) && !errors.Is(err, ErrNotAllowed) {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkNozzle_DoError_Half(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	act := newActor(b.N / 2)

	for range b.N {
		_, err := noz.DoError(func() (any, error) {
			err := act.do()

			return nil, err
		})
		// In benchmarks, we expect both ErrBlocked and actual errors from act.do()
		// Both are valid outcomes for flow control testing
		if err != nil && !errors.Is(err, nozzle.ErrBlocked) && !errors.Is(err, ErrNotAllowed) {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkNozzle_DoBool_Control(b *testing.B) {
	act := newActor(b.N / 2)

	for range b.N {
		if err := act.do(); err != nil {
			continue
		}

		continue
	}
}

// BenchmarkNozzle_StateSnapshot measures the performance of creating state snapshots
// during OnStateChange callbacks.
func BenchmarkNozzle_StateSnapshot(b *testing.B) {
	var snapshotCount atomic.Int64

	noz := nozzle.New(nozzle.Options[any]{
		Interval:              time.Millisecond * 10,
		AllowedFailurePercent: 30,
		OnStateChange: func(snapshot nozzle.StateSnapshot) {
			// Simulate accessing snapshot fields as would happen in real usage
			// Access fields to ensure they're included in benchmark measurements
			if snapshot.FlowRate < 0 || snapshot.State == "" || 
				snapshot.FailureRate < 0 || snapshot.SuccessRate < 0 ||
				snapshot.Allowed < 0 || snapshot.Blocked < 0 {
				// This should never happen but ensures fields are accessed
				return
			}
			snapshotCount.Add(1)
		},
	})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}

		b.Logf("Created %d snapshots during benchmark", snapshotCount.Load())
	})

	// Create varying success/failure patterns to trigger state changes
	for i := range b.N {
		// Vary success rate to trigger state changes
		shouldSucceed := i%10 < 7 // 70% success rate initially
		if i%100 > 50 {
			shouldSucceed = i%10 < 3 // Then 30% success rate
		}

		noz.DoBool(func() (any, bool) {
			return nil, shouldSucceed
		})

		// Periodically force state calculation to trigger snapshots
		if i%100 == 0 {
			noz.Wait()
		}
	}
}

// BenchmarkNozzle_StateSnapshot_NoCallback measures baseline performance without callback.
func BenchmarkNozzle_StateSnapshot_NoCallback(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{
		Interval:              time.Millisecond * 10,
		AllowedFailurePercent: 30,
		// No OnStateChange callback
	})

	b.Cleanup(func() {
		if err := noz.Close(); err != nil {
			b.Errorf("Failed to close nozzle: %v", err)
		}
	})

	// Same pattern as above but without callback
	for i := range b.N {
		shouldSucceed := i%10 < 7
		if i%100 > 50 {
			shouldSucceed = i%10 < 3
		}

		noz.DoBool(func() (any, bool) {
			return nil, shouldSucceed
		})

		if i%100 == 0 {
			noz.Wait()
		}
	}
}
