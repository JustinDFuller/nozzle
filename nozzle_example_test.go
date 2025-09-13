package nozzle_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/justindfuller/nozzle"
)

func ExampleNew() {
	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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
	noz, err := nozzle.New(nozzle.Options[int]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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
	noz, err := nozzle.New(nozzle.Options[string]{
		Interval:              time.Millisecond * 100,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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

	noz, err := nozzle.New(nozzle.Options[*example]{
		Interval:              time.Second,
		AllowedFailurePercent: 0,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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
	noz, err := nozzle.New(nozzle.Options[any]{
		Interval:              time.Millisecond * 50,
		AllowedFailurePercent: 10,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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
	noz, err := nozzle.New(nozzle.Options[map[string]any]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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
	// Use a synchronous approach for deterministic example output
	var (
		outputs []string
		mutex   sync.Mutex
	)

	noz, err := nozzle.New(nozzle.Options[[]string]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
		OnStateChange: func(_ context.Context, snapshot nozzle.StateSnapshot) {
			output := fmt.Sprintf("New State: %s\nFailure Rate: %d\nSuccess Rate: %d\nFlow Rate: %d",
				snapshot.State, snapshot.FailureRate, snapshot.SuccessRate, snapshot.FlowRate)
			mutex.Lock()
			outputs = append(outputs, output)
			mutex.Unlock()
		},
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	defer func() {
		if err := noz.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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

	// Wait a bit for callbacks to complete
	time.Sleep(100 * time.Millisecond)

	// Print collected outputs
	mutex.Lock()

	for _, output := range outputs {
		fmt.Println(output)
	}

	mutex.Unlock()

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
	n, err := nozzle.New(nozzle.Options[string]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	// Always close the nozzle when done
	defer func() {
		if err := n.Close(); err != nil {
			fmt.Printf("Error closing nozzle: %v\n", err)
		}
	}()

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
	noz, err := nozzle.New(nozzle.Options[int]{
		Interval:              time.Second,
		AllowedFailurePercent: 50,
	})
	if err != nil {
		fmt.Printf("Error creating nozzle: %v\n", err)

		return
	}

	// Close the nozzle
	if err := noz.Close(); err != nil {
		fmt.Printf("Error closing nozzle: %v\n", err)
	}

	// DoBool on closed nozzle returns zero value and false
	resultBool, ok := noz.DoBool(func() (int, bool) {
		// This callback will not be executed
		return 42, true
	})
	fmt.Printf("DoBool result: %d, ok: %v\n", resultBool, ok)

	// DoError on closed nozzle returns zero value and ErrClosed
	resultError, err := noz.DoError(func() (int, error) {
		// This callback will not be executed
		return 42, nil
	})
	fmt.Printf("DoError result: %d, error: %v\n", resultError, err)

	// Output:
	// DoBool result: 0, ok: false
	// DoError result: 0, error: nozzle: closed
}
