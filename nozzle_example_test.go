package nozzle_test

import (
	"fmt"
	"time"

	"github.com/justindfuller/nozzle"
)

func ExampleNew() {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	fmt.Println(n.FlowRate())
	// Output: 100
}

func ExampleNozzle_DoBool() {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	// Simulate success
	n.DoBool(func() bool {
		return true
	})
	fmt.Println(n.SuccessRate())

	// Simulate failure
	n.DoBool(func() bool {
		return false
	})

	n.DoBool(func() bool {
		return false
	})

	fmt.Println(n.SuccessRate())
	// Output:
	// 100
	// 50
}

func ExampleNozzle_DoError() {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	// Simulate no error
	n.DoError(func() error {
		return nil
	})
	fmt.Println(n.SuccessRate())

	// Simulate error
	n.DoError(func() error {
		return ErrNotAllowed
	})

	n.DoError(func() error {
		return ErrNotAllowed
	})

	fmt.Println(n.FailureRate())
	// Output:
	// 100
	// 50
}

func ExampleNozzle_State() {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	// Simulate some operations
	n.DoBool(func() bool {
		return true
	})

	fmt.Println(n.State())
	// Output: opening
}

func ExampleNozzle_FlowRate() {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Millisecond * 50,
		AllowedFailurePercent: 10,
	})

	// Simulate a few operations to potentially alter flow rate
	for i := range 10 {
		n.DoBool(func() bool {
			return i%2 == 0 // Alternates between true and false
		})
	}

	fmt.Println(n.FlowRate())
	// Output: 100
}

func ExampleNozzle_Wait() {
	n := nozzle.New(nozzle.Options{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	go func() {
		n.Wait() // Wait for the next tick
		fmt.Println("Tick processed")
	}()

	// Simulate some operations and wait
	time.Sleep(time.Second)
	fmt.Println("Wait completed")
	// Output:
	// Tick processed
	// Wait completed
}
