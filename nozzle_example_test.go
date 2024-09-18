package nozzle_test

import (
	"fmt"
	"time"

	"github.com/justindfuller/nozzle"
)

func ExampleNew() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	fmt.Printf("FlowRate=%d\n", noz.FlowRate())
	fmt.Printf("SuccessRate=%d\n", noz.SuccessRate())
	fmt.Printf("FailureRate=%d\n", noz.FailureRate())
	fmt.Printf("State=%s", noz.State())
	// Output:
	// FlowRate=100
	// SuccessRate=100
	// FailureRate=0
	// State=opening
}

func ExampleNozzle_DoBool() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())

	for range 2 {
		noz.DoError(func() error {
			return ErrNotAllowed
		})

		fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())
	}

	for range 2 {
		noz.DoError(func() error {
			return nil
		})

		fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())
	}

	// Output:
	// Success=100 Failure=0
	// Success=0 Failure=100
	// Success=0 Failure=100
	// Success=50 Failure=50
	// Success=67 Failure=33
}

func ExampleNozzle_DoError() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())

	for range 2 {
		noz.DoError(func() error {
			return ErrNotAllowed
		})

		fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())
	}

	for range 2 {
		noz.DoError(func() error {
			return nil
		})

		fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())
	}

	// Output:
	// Success=100 Failure=0
	// Success=0 Failure=100
	// Success=0 Failure=100
	// Success=50 Failure=50
	// Success=67 Failure=33
}

func ExampleNozzle_State() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 0,
	})

	fmt.Println(noz.State())

	// Simulate some operations
	noz.DoBool(func() bool {
		return false
	})

	noz.Wait()

	fmt.Println(noz.State())

	noz.DoBool(func() bool {
		return true
	})

	noz.Wait()

	fmt.Println(noz.State())
	// Output:
	// opening
	// closing
	// opening
}

func ExampleNozzle_FlowRate() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Millisecond * 50,
		AllowedFailurePercent: 10,
	})

	for range 7 {
		for range 10 {
			noz.DoBool(func() bool {
				return false
			})
		}

		noz.Wait()
		fmt.Println(noz.FlowRate())
	}

	for range 7 {
		for range 10 {
			noz.DoBool(func() bool {
				return true
			})
		}

		noz.Wait()
		fmt.Println(noz.FlowRate())
	}

	// Output:
	// 99
	// 97
	// 93
	// 85
	// 69
	// 37
	// 0
	// 1
	// 3
	// 7
	// 15
	// 31
	// 63
	// 100
}

func ExampleNozzle_Wait() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	for range 2 {
		noz.DoBool(func() bool {
			return false
		})
	}

	fmt.Printf("State Before Wait = %s\n", noz.State())

	noz.Wait()

	fmt.Printf("State After Wait = %s\n", noz.State())

	// Output:
	// State Before Wait = opening
	// State After Wait = closing
}

func ExampleOptions_OnStateChange() {
	noz := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
		OnStateChange: func(s nozzle.State) {
			fmt.Printf("New State: %s\n", s)
		},
	})

	for range 10 {
		noz.DoBool(func() bool {
			return false
		})
	}

	noz.Wait()

	for range 100 {
		noz.DoBool(func() bool {
			return true
		})
	}

	noz.Wait()

	// Output:
	// New State: closing
	// New State: opening
}
