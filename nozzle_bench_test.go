package nozzle_test

import (
	"testing"
	"time"

	"github.com/justindfuller/nozzle"
)

func BenchmarkNozzle_Do_Open(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(b.N)

	for i := 0; i < b.N; i++ {
		noz.DoBool(func() bool {
			err := act.do()

			return err == nil
		})
	}
}

func BenchmarkNozzle_Do_Closed(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(0)

	for i := 0; i < b.N; i++ {
		noz.DoBool(func() bool {
			err := act.do()

			return err == nil
		})
	}
}

func BenchmarkNozzle_Do_Half(b *testing.B) {
	noz := nozzle.New(nozzle.Options{Interval: time.Millisecond * 10, AllowedFailurePercent: 50})
	act := newActor(b.N / 2)

	for i := 0; i < b.N; i++ {
		noz.DoBool(func() bool {
			err := act.do()

			return err == nil
		})
	}
}

func BenchmarkNozzle_Do_Control(b *testing.B) {
	act := newActor(b.N / 2)

	for i := 0; i < b.N; i++ {
		if err := act.do(); err != nil {
			continue
		}

		continue
	}
}
