package nozzle_test

import (
	"fmt"
	"time"

	"github.com/justindfuller/nozzle"
)

func ExampleNew() {
	noz := nozzle.New(nozzle.Options[any]{
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
	noz := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())

	for i := range 2 {
		res, ok := noz.DoBool(func() (int, bool) {
			return i, false
		})

		fmt.Printf("Result=%d OK=%v Success=%d Failure=%d\n", res, ok, noz.SuccessRate(), noz.FailureRate())
	}

	for i := range 2 {
		res, ok := noz.DoBool(func() (int, bool) {
			return i, true
		})

		fmt.Printf("Result=%v OK=%v Success=%d Failure=%d\n", res, ok, noz.SuccessRate(), noz.FailureRate())
	}

	// Output:
	// Success=100 Failure=0
	// Result=0 OK=false Success=0 Failure=100
	// Result=1 OK=false Success=0 Failure=100
	// Result=0 OK=true Success=34 Failure=66
	// Result=1 OK=true Success=50 Failure=50
}

func ExampleNozzle_DoError() {
	noz := nozzle.New(nozzle.Options[string]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})

	fmt.Printf("Success=%d Failure=%d\n", noz.SuccessRate(), noz.FailureRate())

	for range 2 {
		res, err := noz.DoError(func() (string, error) {
			return "fail", ErrNotAllowed
		})

		fmt.Printf("Result=\"%s\" Error=\"%v\" Success=%d Failure=%d\n", res, err, noz.SuccessRate(), noz.FailureRate())
	}

	for range 2 {
		res, err := noz.DoError(func() (string, error) {
			return "succeed", nil
		})

		fmt.Printf("Result=\"%s\" Error=\"%v\" Success=%d Failure=%d\n", res, err, noz.SuccessRate(), noz.FailureRate())
	}

	// Output:
	// Success=100 Failure=0
	// Result="fail" Error="not allowed" Success=0 Failure=100
	// Result="fail" Error="not allowed" Success=0 Failure=100
	// Result="succeed" Error="<nil>" Success=34 Failure=66
	// Result="succeed" Error="<nil>" Success=50 Failure=50
}

func ExampleNozzle_State() {
	type example struct {
		name string
	}

	noz := nozzle.New(nozzle.Options[*example]{
		Interval:              time.Second,
		AllowedFailurePercent: 0,
	})

	fmt.Println(noz.State())

	// Simulate some operations
	res, _ := noz.DoBool(func() (*example, bool) {
		return &example{name: "failed bool"}, false
	})

	fmt.Printf("Result=%v\n", res.name)

	noz.Wait()

	fmt.Println(noz.State())

	res, _ = noz.DoBool(func() (*example, bool) {
		return &example{name: "succeed bool"}, true
	})

	fmt.Printf("Result=%v\n", res.name)

	noz.Wait()

	fmt.Println(noz.State())
	// Output:
	// opening
	// Result=failed bool
	// closing
	// Result=succeed bool
	// opening
}

func ExampleNozzle_FlowRate() {
	noz := nozzle.New(nozzle.Options[any]{
		Interval:              time.Millisecond * 50,
		AllowedFailurePercent: 10,
	})

	for range 7 {
		for range 10 {
			noz.DoBool(func() (any, bool) {
				return nil, false
			})
		}

		noz.Wait()
		fmt.Println(noz.FlowRate())
	}

	for range 7 {
		for range 10 {
			noz.DoBool(func() (any, bool) {
				return nil, true
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
	noz := nozzle.New(nozzle.Options[map[string]any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	for range 2 {
		noz.DoBool(func() (map[string]any, bool) {
			return nil, false
		})
	}

	fmt.Printf("State Before Wait = %s\n", noz.State())

	noz.Wait()

	fmt.Printf("State After Wait = %s\n", noz.State())

	// Output:
	// State Before Wait = opening
	// State After Wait = closing
}

func ExampleOptions() {
	noz := nozzle.New(nozzle.Options[[]string]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
		OnStateChange: func(n *nozzle.Nozzle[[]string]) {
			fmt.Printf("New State: %s\n", n.State())
			fmt.Printf("Failure Rate: %d\n", n.FailureRate())
			fmt.Printf("Success Rate: %d\n", n.SuccessRate())
			fmt.Printf("Flow Rate: %d\n", n.FlowRate())
		},
	})

	for range 10 {
		noz.DoBool(func() ([]string, bool) {
			return nil, false
		})
	}

	noz.Wait()

	for range 100 {
		noz.DoBool(func() ([]string, bool) {
			return nil, true
		})
	}

	noz.Wait()

	// Output:
	// New State: closing
	// Failure Rate: 100
	// Success Rate: 0
	// Flow Rate: 99
	// New State: opening
	// Failure Rate: 0
	// Success Rate: 100
	// Flow Rate: 100
}

// This example demonstrates the proper cleanup pattern to prevent goroutine leaks.
// Always use defer n.Close() after creating a Nozzle to ensure resources are released.
func Example_cleanup() {
	// Create a nozzle
	n := nozzle.New(nozzle.Options[string]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	// Important: Always close the nozzle when done to prevent goroutine leaks
	defer n.Close()

	// Use the nozzle for operations
	result, ok := n.DoBool(func() (string, bool) {
		return "Hello, World!", true
	})

	if ok {
		fmt.Printf("Operation succeeded: %s\n", result)
	}

	// The deferred Close() will be called when the function exits
	// Output:
	// Operation succeeded: Hello, World!
}

// This example demonstrates that operations on a closed Nozzle return the zero value
// and ErrClosed without executing the callback function.
func Example_closedBehavior() {
	// Create a nozzle
	n := nozzle.New(nozzle.Options[int]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})

	// Close the nozzle
	n.Close()

	// DoBool on closed nozzle returns zero value and false
	resultBool, ok := n.DoBool(func() (int, bool) {
		// This callback will not be executed
		return 42, true
	})
	fmt.Printf("DoBool result: %d, ok: %v\n", resultBool, ok)

	// DoError on closed nozzle returns zero value and ErrClosed
	resultError, err := n.DoError(func() (int, error) {
		// This callback will not be executed
		return 42, nil
	})
	fmt.Printf("DoError result: %d, error: %v\n", resultError, err)

	// Output:
	// DoBool result: 0, ok: false
	// DoError result: 0, error: nozzle: closed
}
