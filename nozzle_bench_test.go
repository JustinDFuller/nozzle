package nozzle_test

import (
	"errors"
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

func BenchmarkNozzle_DoBool_Open(b *testing.B) {
	noz := nozzle.New(nozzle.Options[any]{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})

	b.Cleanup(func() {
		err := noz.Close()
		if err != nil {
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
		err := noz.Close()
		if err != nil {
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
		err := noz.Close()
		if err != nil {
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
		err := noz.Close()
		if err != nil {
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
		err := noz.Close()
		if err != nil {
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
		err := noz.Close()
		if err != nil {
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
		err := act.do()
		if err != nil {
			continue
		}

		continue
	}
}
