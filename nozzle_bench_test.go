package nozzle_test

import (
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

func BenchmarkNozzle_DoBool_Open(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(b.N)

	for i := 0; i < b.N; i++ {
		noz.DoBool(func() bool {
			err := act.do()

			return err == nil
		})
	}
}

func BenchmarkNozzle_DoBool_Closed(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(0)

	for i := 0; i < b.N; i++ {
		noz.DoBool(func() bool {
			err := act.do()

			return err == nil
		})
	}
}

func BenchmarkNozzle_DoBool_Half(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(b.N / 2)

	for i := 0; i < b.N; i++ {
		noz.DoBool(func() bool {
			err := act.do()

			return err == nil
		})
	}
}

func BenchmarkNozzle_DoError_Open(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(b.N)

	for i := 0; i < b.N; i++ {
		noz.DoError(func() error {
			err := act.do()

			return err
		})
	}
}

func BenchmarkNozzle_DoError_Closed(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(0)

	for i := 0; i < b.N; i++ {
		noz.DoError(func() error {
			err := act.do()

			return err
		})
	}
}

func BenchmarkNozzle_DoError_Half(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(b.N / 2)

	for i := 0; i < b.N; i++ {
		noz.DoError(func() error {
			err := act.do()

			return err
		})
	}
}

func BenchmarkNozzle_DoBool_Control(b *testing.B) {
	act := newActor(b.N / 2)

	for i := 0; i < b.N; i++ {
		if err := act.do(); err != nil {
			continue
		}

		continue
	}
}
