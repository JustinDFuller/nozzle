// Package nozzle provides a gradual flow control mechanism (alternative to circuit breaker)
// that adjusts the rate of allowed operations based on success and failure rates.
// Unlike circuit breakers that operate in binary states (open/closed), the nozzle
// gradually opens and closes like a hose nozzle, providing more nuanced flow control.
package nozzle

import (
	"errors"
	"math"
	"sync"
	"time"
)

// ErrBlocked is returned when a call is blocked by the Nozzle.
// It indicates that the Nozzle has reached its limit and is not allowing any more calls.
// You can use this sentinel error to detect and handle this case separately.
//
// Example:
//
//	_, err := n.DoError(func() (any, error) {
//		res, err := someFuncThatCanFail()
//		return res, err
//	})
//
//	if errors.Is(err, nozzle.ErrBlocked) {
//		// handle blocked
//	}
//
//	if err != nil {
//		// handle error
//	}
var ErrBlocked = errors.New("nozzle: blocked")

// ErrClosed is returned when an operation is attempted on a closed Nozzle.
// After Close() is called, all operations will fail with this error (for DoError)
// or return false (for DoBool).
var ErrClosed = errors.New("nozzle: closed")

// Constants for overflow protection.
const (
	// maxDecreaseBy is the maximum absolute value for decreaseBy to prevent integer overflow.
	// Since decreaseBy doubles on each iteration, we limit it to prevent overflow.
	// With int64, max value is ~9.2e18. We set a conservative limit at 1 billion
	// which allows for ~30 doublings before hitting the cap.
	maxDecreaseBy int64 = 1000000000
)

// Nozzle manages the rate of allowed operations and adapts based on success and failure rates.
// It uses a flow rate to control the percentage of allowed operations and adjusts its state based on the observed failure rate.
// The Nozzle implements io.Closer for resource cleanup.
// see nozzle.New docs for how to create a Nozzle.
// see nozzle.Options docs for how to modify a Nozzle's behavior.
type Nozzle[T any] struct {
	// Options controls how the Nozzle works.
	// See the nozzle.Options docs for how it works.
	Options Options[T]

	// decreaseBy adjusts the rate at which the flowRate changes.
	// It determines how quickly the Nozzle opens or closes.
	// Example: If decreaseBy is -2, flowRate decreases faster than if decreaseBy is -1
	decreaseBy int64

	// flowRate indicates the percentage of allowed operations at any given time.
	// Example: A flowRate of 100 means all operations are allowed, while a flowRate of 0 means none are allowed.
	flowRate int64

	// successes counts the number of successful operations since the last reset.
	// Example: If 50 operations succeeded, successes will be 50.
	successes int64

	// failures counts the number of failed operations since the last reset.
	// Example: If 20 operations failed, failures will be 20.
	failures int64

	// allowed counts the number of operations that were allowed in the current interval.
	// Example: If 70 operations were allowed, allowed will be 70.
	allowed int64

	// blocked counts the number of operations that were blocked in the current interval.
	// Example: If 30 operations were blocked, blocked will be 30.
	blocked int64

	// start records the time when the current interval started.
	// Example: If the interval started at 10:00 AM, start will be the time corresponding to 10:00 AM.
	start time.Time

	// mut is a read-write mutex that ensures thread-safe access to Nozzle's state.
	// Example: It prevents concurrent read and write operations from causing inconsistencies when multiple goroutines interact with Nozzle.
	mut sync.RWMutex

	// state represents whether the Nozzle is currently opening or closing.
	// Example: If the Nozzle is adjusting to increase the flow rate, state will be Opening.
	state State

	// ticker is a channel used to signal the occurrence of a new tick.
	// Example: It allows other parts of the code to react to time-based events, such as triggering a status update.
	// See nozzle.Wait() for usage and nozzle.Calculate() for where it is called.
	ticker chan struct{}

	// done is a channel used to signal the ticker goroutine to stop.
	done chan struct{}

	// timeTicker stores the time.Ticker reference for proper cleanup.
	timeTicker *time.Ticker

	// once ensures that Close() is idempotent.
	once sync.Once

	// closed tracks whether the nozzle has been closed.
	closed bool
}

// StateSnapshot represents an immutable snapshot of the Nozzle's state at a specific point in time.
// This struct is passed to OnStateChange callbacks to provide thread-safe access to all
// observable state without requiring mutex locks. All fields represent consistent state
// from the same instant, ensuring atomic access to related metrics.
//
// Example usage:
//
//	OnStateChange: func(snapshot nozzle.StateSnapshot) {
//	    if snapshot.FlowRate < 50 {
//	        log.Printf("WARNING: Flow restricted to %d%%", snapshot.FlowRate)
//	    }
//	    metrics.SetGauge("nozzle.flow_rate", float64(snapshot.FlowRate))
//	    metrics.SetGauge("nozzle.failure_rate", float64(snapshot.FailureRate))
//	}
type StateSnapshot struct {
	// FlowRate is the current percentage of operations allowed through (0-100).
	// When FlowRate is 100, all operations are allowed. When 0, all operations are blocked.
	// The rate adjusts dynamically based on success/failure ratios.
	FlowRate int64

	// State indicates whether the Nozzle is currently opening or closing.
	// Opening means the flow rate is increasing (reducing restrictions).
	// Closing means the flow rate is decreasing (increasing restrictions).
	State State

	// FailureRate is the percentage of failed operations in the current interval (0-100).
	// This is calculated from recent operation outcomes within the interval window.
	// A higher failure rate causes the nozzle to close (reduce flow).
	FailureRate int64

	// SuccessRate is the percentage of successful operations in the current interval (0-100).
	// This is calculated as (100 - FailureRate) for convenience.
	// A higher success rate causes the nozzle to open (increase flow).
	SuccessRate int64

	// Allowed is the cumulative count of operations that have been allowed through
	// since the nozzle was created. This counter never resets.
	Allowed int64

	// Blocked is the cumulative count of operations that have been blocked
	// since the nozzle was created. This counter never resets.
	Blocked int64
}

// Options controls the behavior of the Nozzle.
// See each field for explanations.
type Options[T any] struct {
	// Interval controls how often the Nozzle will process its state.
	// Example:
	//
	//	Interval: time.Second      // Processes state every second
	//	Interval: time.Millisecond * 100  // Processes state every 100 milliseconds
	//
	// The best interval depends on the needs of your application.
	// If you are unsure, start with 1 second.
	Interval time.Duration

	// AllowedFailurePercent sets the threshold for the failure rate at which the Nozzle should open or close.
	// Example:
	//
	//	AllowedFailurePercent: 0    // No failures allowed
	//	AllowedFailurePercent: 50   // Allows up to 50% failure rate
	//	AllowedFailurePercent: 100  // Allows all failures
	//
	// The best FailurePercent depends on the needs of your application.
	// If you are unsure, start with 50%.
	AllowedFailurePercent int64

	// OnStateChange is an optional callback function invoked whenever the nozzle's
	// state changes. The callback receives a StateSnapshot containing an immutable copy
	// of the nozzle's state at the time of the change.
	//
	// Execution guarantees:
	//  - Called at most once per Interval, only when state actually changes
	//  - Called sequentially (never concurrently) even with multiple nozzles
	//  - Called while holding the nozzle's mutex (thread-safe but avoid blocking operations)
	//  - Panics in callbacks are recovered and don't affect nozzle operation
	//
	// Performance considerations:
	//  - The callback executes synchronously during state calculation
	//  - Long-running callbacks may delay subsequent state calculations
	//  - StateSnapshot is passed by value (zero allocations, minimal overhead)
	//  - Avoid heavy operations; consider queueing work for async processing
	//
	// Example - Basic logging:
	//
	//	OnStateChange: func(snapshot nozzle.StateSnapshot) {
	//	    log.Printf("State: %s, Flow: %d%%, Failures: %d%%",
	//	        snapshot.State, snapshot.FlowRate, snapshot.FailureRate)
	//	}
	//
	// Example - Metrics integration:
	//
	//	OnStateChange: func(snapshot nozzle.StateSnapshot) {
	//	    metrics.SetGauge("nozzle.flow_rate", float64(snapshot.FlowRate))
	//	    metrics.SetGauge("nozzle.failure_rate", float64(snapshot.FailureRate))
	//	    if snapshot.State == nozzle.Closing {
	//	        metrics.Increment("nozzle.closing_events")
	//	    }
	//	}
	//
	// Example - Alerting on degradation:
	//
	//	OnStateChange: func(snapshot nozzle.StateSnapshot) {
	//	    if snapshot.FlowRate < 25 {
	//	        // Queue alert asynchronously to avoid blocking
	//	        go alerting.Send("Critical: Flow rate at %d%%", snapshot.FlowRate)
	//	    }
	//	}
	OnStateChange func(StateSnapshot)
}

// State describes the current direction the Nozzle is moving.
// The Nozzle is always moving, so there are only two states: Opening and Closing.
// If the Nozzle is fully open and below the allowed error rate, it will continue to try to open, but this is a no-op.
// If the Nozzle is fully closed, it will revert to trying to open. This allows it to continually check for opportunities to re-open.
// If the Nozzle is on the edge of the AllowedFailurePercent, you will observe it toggle between opening/closing as it explores if it can re-open.
type State string

const (
	// Opening means the FlowRate is increasing.
	Opening State = "opening"

	// Closing means the FlowRate is decreasing.
	Closing State = "closing"
)

// New creates a new Nozzle with Options.
//
// A Nozzle starts fully open.
// A Nozzle begins with no errors.
// A Nozzle is safe for use by multiple goroutines.
//
// The returned Nozzle must be closed with Close() when no longer needed to prevent goroutine leaks.
//
// The Nozzle contains a mutex, so it must not be copied after first creation.
// If you do, you will receive an error from `go vet`.
//
// Example:
//
//	n := nozzle.New(nozzle.Options[any]{
//		Interval: time.Second,
//		AllowedFailurePercent: 50,
//	})
//	defer n.Close()
//
// See docs of nozzle.Options for details about each Option field.
func New[T any](options Options[T]) *Nozzle[T] {
	n := Nozzle[T]{
		flowRate:   100,
		Options:    options,
		state:      Opening,
		done:       make(chan struct{}),
		timeTicker: time.NewTicker(options.Interval),
	}

	go n.tick()

	return &n
}

// tick periodically invokes the calculate method based on the Nozzle's interval.
// It ensures the Nozzle processes its state updates at regular intervals.
func (n *Nozzle[T]) tick() {
	for {
		select {
		case <-n.timeTicker.C:
			n.calculate()
		case <-n.done:
			return
		}
	}
}

// Close gracefully shuts down the Nozzle and releases all resources.
// It stops the internal ticker goroutine and can be called multiple times safely.
//
// After Close is called:
//   - DoBool will return (zero value, false) without calling the callback
//   - DoError will return (zero value, ErrClosed) without calling the callback
//   - The ticker goroutine will be stopped
//   - All resources will be released
//
// Close is idempotent - calling it multiple times has no additional effect.
//
// Example:
//
//	n := nozzle.New(nozzle.Options[any]{
//		Interval: time.Second,
//		AllowedFailurePercent: 50,
//	})
//	defer n.Close() // Ensure cleanup
//
//	// Use the nozzle...
func (n *Nozzle[T]) Close() error {
	n.once.Do(func() {
		n.mut.Lock()
		n.closed = true
		n.mut.Unlock()

		close(n.done)
		n.timeTicker.Stop()
	})

	return nil
}

// DoBool executes a callback function while respecting the Nozzle's state.
// It monitors how many calls have been allowed and compares this with the flowRate to determine if this particular call will be permitted.
//
// If the Nozzle is closed, DoBool returns (zero value, false) immediately without calling the callback.
//
// The callback function receives no arguments and should return a boolean value.
// If the callback returns true, the success method will be called, otherwise the failure method will be called.
//
// Example:
//
//	var n nozzle.Nozzle[*example]
//
//	res, ok := n.DoBool(func() (*example, bool) {
//		result, err := someFuncThatCanFail()
//		return result, err == nil
//	})
//	if !ok {
//		// handle failure or closed nozzle.
//	}
//
//	fmt.Println(res) // use res.
//
// If the callback function does not return true or false, Nozzle's behavior will not be affected.
func (n *Nozzle[T]) DoBool(callback func() (T, bool)) (T, bool) {
	n.mut.Lock()

	// Check if nozzle is closed
	if n.closed {
		n.mut.Unlock()

		return *new(T), false
	}

	var allowRate int64

	if n.allowed != 0 {
		allowRate = int64((float64(n.allowed) / float64(n.allowed+n.blocked)) * 100)
	}

	var allow bool

	if n.flowRate == 100 {
		allow = true
	} else if n.flowRate > 0 {
		allow = allowRate < n.flowRate
	}

	if !allow {
		n.blocked++
		n.mut.Unlock()

		return *new(T), false
	}

	n.allowed++

	n.mut.Unlock()

	res, ok := callback()

	if ok {
		n.success()
	} else {
		n.failure()
	}

	return res, ok
}

// DoError executes a callback function while respecting the Nozzle's state.
// It monitors how many calls have been allowed and compares this with the flowRate to determine if this particular call will be permitted.
//
// If the Nozzle is closed, DoError returns (zero value, ErrClosed) immediately without calling the callback.
//
// The callback function receives no arguments and should return an error.
// If the callback returns nil, the success method will be called. If the callback returns an error, the failure method will be called.
//
// Example:
//
//	var n nozzle.Nozzle[*example]
//
//	res, err := n.DoError(func() (*example, error) {
//		ex, err := someFuncThatCanFail()
//		return ex, err
//	})
//	if errors.Is(err, nozzle.ErrClosed) {
//		// nozzle is closed
//	} else if err != nil {
//		// handle other error
//	}
//
//	fmt.Print(res) // Use the result
//
// If the callback function does not return an error, Nozzle's behavior will be affected according to the success method.
func (n *Nozzle[T]) DoError(callback func() (T, error)) (T, error) {
	n.mut.Lock()

	// Check if nozzle is closed
	if n.closed {
		n.mut.Unlock()

		return *new(T), ErrClosed
	}

	var allowRate int64

	if n.allowed != 0 {
		allowRate = int64((float64(n.allowed) / float64(n.allowed+n.blocked)) * 100)
	}

	var allow bool

	if n.flowRate == 100 {
		allow = true
	} else if n.flowRate > 0 {
		allow = allowRate < n.flowRate
	}

	if !allow {
		n.blocked++
		n.mut.Unlock()

		return *new(T), ErrBlocked
	}

	n.allowed++
	n.mut.Unlock()

	res, err := callback()
	if err != nil {
		n.failure()
	} else {
		n.success()
	}

	return res, err
}

// calculate updates the Nozzle's state based on the elapsed time and failure rate.
// It determines whether to open or close the Nozzle and triggers the ticker if necessary.
func (n *Nozzle[T]) calculate() {
	n.mut.Lock()
	defer n.mut.Unlock()

	if time.Since(n.start) < n.Options.Interval {
		return
	}

	originalFlowRate := n.flowRate
	originalState := n.state

	if n.failureRate() > n.Options.AllowedFailurePercent {
		n.close()
		n.state = Closing
	} else {
		n.open()
		n.state = Opening
	}

	var changed bool

	if n.flowRate != originalFlowRate {
		changed = true
	}

	if n.state != originalState {
		changed = true
	}

	if changed && n.Options.OnStateChange != nil {
		// Create an immutable snapshot of the current state.
		// This is safe to pass to the callback without unlocking the mutex.
		snapshot := StateSnapshot{
			FlowRate:    n.flowRate,
			State:       n.state,
			FailureRate: n.failureRate(),
			SuccessRate: n.successRate(),
			Allowed:     n.allowed,
			Blocked:     n.blocked,
		}

		// Call the callback with the snapshot.
		// The mutex remains locked, preventing race conditions.
		n.Options.OnStateChange(snapshot)
	}

	n.reset()

	if n.ticker != nil {
		select {
		case n.ticker <- struct{}{}:
		default:
		}
	}
}

// close reduces the flow rate and increases the multiplier to speed up the closing process.
// It is called when the failure rate exceeds the allowed threshold.
func (n *Nozzle[T]) close() {
	mult := n.decreaseBy
	if mult > -1 {
		mult = -1
	}

	n.flowRate = clamp(n.flowRate + mult)

	// Safe multiplication with overflow protection
	nextDecrease := safeMultiply(mult, 2)
	// Apply cap to prevent unbounded growth
	if nextDecrease < -maxDecreaseBy {
		nextDecrease = -maxDecreaseBy
	}

	n.decreaseBy = nextDecrease
}

// open increases the flow rate and doubles the multiplier to speed up the opening process.
// It is called when the failure rate is within the allowed threshold.
func (n *Nozzle[T]) open() {
	if n.flowRate == 100 {
		return
	}

	mult := n.decreaseBy
	if mult < 1 {
		mult = 1
	}

	n.flowRate = clamp(n.flowRate + mult)

	// Safe multiplication with overflow protection
	nextDecrease := safeMultiply(mult, 2)
	// Apply cap to prevent unbounded growth
	if nextDecrease > maxDecreaseBy {
		nextDecrease = maxDecreaseBy
	}

	n.decreaseBy = nextDecrease
}

// reset reinitializes the Nozzle's state for the next interval.
// It sets the start time to now and clears the counters for successes, failures, allowed, and blocked operations.
func (n *Nozzle[T]) reset() {
	n.start = time.Now()
	n.successes = 0
	n.failures = 0
	n.allowed = 0
	n.blocked = 0
}

// success increments the count of successful operations.
// This contributes to calculating the success rate.
func (n *Nozzle[T]) success() {
	n.mut.Lock()
	defer n.mut.Unlock()

	n.successes++
}

// failure increments the count of failed operations.
// This contributes to calculating the failure rate.
func (n *Nozzle[T]) failure() {
	n.mut.Lock()
	defer n.mut.Unlock()

	n.failures++
}

// FlowRate reports the current flow rate.
// The flow rate determines how many calls will be allowed.
// Example: A flow rate of 100 will allow all calls, while a flow rate of 50 will allow 50% of calls.
func (n *Nozzle[T]) FlowRate() int64 {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.flowRate
}

// failureRate calculates the percentage of failed operations out of the total operations.
// Example: With 500 failures and 500 successes, the failure rate will be 50%.
func (n *Nozzle[T]) failureRate() int64 {
	if n.failures == 0 && n.successes == 0 {
		return 0
	}

	// Ex: 500 failures, 500 successes
	// (500 / (500 + 500)) * 100 = 50
	return int64((float64(n.failures) / float64(n.failures+n.successes)) * 100)
}

// successRate is an internal helper method that calculates the success rate without acquiring the lock.
// It assumes the caller already holds the lock. This method is not exported and is used internally
// by the calculate() method when creating StateSnapshots.
func (n *Nozzle[T]) successRate() int64 {
	if n.flowRate == 0 {
		return 0
	}

	if n.failures == 0 && n.successes == 0 {
		return 100
	}

	return 100 - n.failureRate()
}

// SuccessRate reports the success rate of Nozzle calls.
// It calculates the percentage of successful operations out of the total operations.
// Example: With 90 successes and 10 failures, the success rate will be 90%.
func (n *Nozzle[T]) SuccessRate() int64 {
	n.mut.RLock()
	defer n.mut.RUnlock()

	if n.flowRate == 0 {
		return 0
	}

	if n.failures == 0 && n.successes == 0 {
		return 100
	}

	return 100 - n.failureRate()
}

// FailureRate reports the failure rate of Nozzle calls.
// It calculates the percentage of failed operations out of the total operations.
// Example: With 10 failures and 90 successes, the failure rate will be 10%.
func (n *Nozzle[T]) FailureRate() int64 {
	n.mut.RLock()
	defer n.mut.RUnlock()

	if n.flowRate == 0 {
		return 0
	}

	if n.failures == 0 && n.successes == 0 {
		return 0
	}

	return n.failureRate()
}

// State reports the current state of the Nozzle.
// It reflects whether the Nozzle is currently in the process of opening or closing.
// Example: If the Nozzle is increasing its flow rate, the state will be Opening.
func (n *Nozzle[T]) State() State {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.state
}

// Wait blocks until the Nozzle processes the next tick.
// This is useful for testing but should be avoided in production code.
func (n *Nozzle[T]) Wait() {
	n.mut.Lock()

	if n.ticker == nil {
		n.ticker = make(chan struct{})
	}

	n.mut.Unlock()

	<-n.ticker
}

// clamp constrains the flowRate to be within the range [0, 100].
// It ensures the flowRate stays within valid bounds to prevent unexpected behavior.
func clamp(i int64) int64 {
	if i < 0 {
		return 0
	}

	if i > 100 {
		return 100
	}

	return i
}

// safeMultiply performs multiplication with overflow detection.
// If the multiplication would overflow, it returns the maximum or minimum int64 value
// depending on the sign of the result.
func safeMultiply(a, b int64) int64 {
	// Handle special cases
	if a == 0 || b == 0 {
		return 0
	}

	// Check if multiplication would overflow
	// For positive result (both positive)
	if a > 0 && b > 0 && a > math.MaxInt64/b {
		return math.MaxInt64
	}
	// For positive result (both negative)
	if a < 0 && b < 0 && a < math.MaxInt64/b {
		return math.MaxInt64
	}
	// For negative result (a positive, b negative)
	if a > 0 && b < 0 && b < math.MinInt64/a {
		return math.MinInt64
	}
	// For negative result (a negative, b positive)
	if a < 0 && b > 0 && a < math.MinInt64/b {
		return math.MinInt64
	}

	return a * b
}
