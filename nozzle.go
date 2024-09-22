package nozzle

import (
	"sync"
	"sync/atomic"
	"time"
)

// Nozzle manages the rate of allowed operations and adapts based on success and failure rates.
// It uses a flow rate to control the percentage of allowed operations and adjusts its state based on the observed failure rate.
// see nozzle.New docs for how to create a Nozzle.
// see nozzle.Options docs for how to modify a Nozzle's behavior.
type Nozzle struct {
	// Options controls how the Nozzle works.
	// See the nozzle.Options docs for how it works.
	Options Options

	// decreaseBy adjusts the rate at which the flowRate changes.
	// It determines how quickly the Nozzle opens or closes.
	// Example: If decreaseBy is -2, flowRate decreases faster than if decreaseBy is -1
	decreaseBy *atomic.Int64

	// flowRate indicates the percentage of allowed operations at any given time.
	// Example: A flowRate of 100 means all operations are allowed, while a flowRate of 0 means none are allowed.
	flowRate *atomic.Int64

	// successes counts the number of successful operations since the last reset.
	// Example: If 50 operations succeeded, successes will be 50.
	successes *atomic.Int64

	// failures counts the number of failed operations since the last reset.
	// Example: If 20 operations failed, failures will be 20.
	failures *atomic.Int64

	// allowed counts the number of operations that were allowed in the current interval.
	// Example: If 70 operations were allowed, allowed will be 70.
	allowed *atomic.Int64

	// blocked counts the number of operations that were blocked in the current interval.
	// Example: If 30 operations were blocked, blocked will be 30.
	blocked *atomic.Int64

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
	ticker chan struct{}
}

// Options controls the behavior of the Nozzle.
// See each field for explanations.
type Options struct {
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

	// OnStateChange is a callback function that will be called whenever the Nozzle's state changes.
	// This function will be called at most once per Interval.
	OnStateChange func(State)
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
// The Nozzle contains a mutex, so it must not be copied after first creation.
// If you do, you will receive an error from `go vet`.
//
// Example:
//
//	nozzle.New(nozzle.Options{
//		Interval: time.Second,
//		AllowedFailurePercent: 50,
//	})
//
// See docs of nozzle.Options for details about each Option field.
func New(options Options) *Nozzle {
	var fr atomic.Int64

	fr.Store(100)

	n := Nozzle{
		decreaseBy: &atomic.Int64{},
		successes:  &atomic.Int64{},
		failures:   &atomic.Int64{},
		allowed:    &atomic.Int64{},
		blocked:    &atomic.Int64{},
		flowRate:   &fr,
		Options:    options,
		state:      Opening,
	}

	go tick(&n)

	return &n
}

// tick periodically invokes the calculate method based on the Nozzle's interval.
// It ensures the Nozzle processes its state updates at regular intervals.
func tick(n *Nozzle) {
	for range time.Tick(n.Options.Interval) {
		n.calculate()
	}
}

// DoBool executes a callback function while respecting the Nozzle's state.
// It monitors how many calls have been allowed and compares this with the flowRate to determine if this particular call will be permitted.
//
// The callback function receives no arguments and should return a boolean value.
// If the callback returns true, the success method will be called, otherwise the failure method will be called.
//
// Example:
//
//	var n nozzle.Nozzle
//
//	n.DoBool(func() bool {
//		err := someFuncThatCanFail()
//		return err == nil
//	})
//
// If the callback function does not return true or false, Nozzle's behavior will not be affected.
func (n *Nozzle) DoBool(fn func() bool) {
	allowed := n.allowed.Load()
	blocked := n.blocked.Load()

	var allowRate int64

	if allowed != 0 {
		allowRate = int64((float64(allowed) / float64(allowed+blocked)) * 100)
	}

	allow := allowRate < n.flowRate.Load()

	if allow {
		n.allowed.Add(1)

		if fn() {
			n.success()
		} else {
			n.failure()
		}
	} else {
		n.blocked.Add(1)
	}
}

// DoError executes a callback function while respecting the Nozzle's state.
// It monitors how many calls have been allowed and compares this with the flowRate to determine if this particular call will be permitted.
//
// The callback function receives no arguments and should return an error.
// If the callback returns nil, the success method will be called. If the callback returns an error, the failure method will be called.
//
// Example:
//
//	var n nozzle.Nozzle
//
//	n.DoError(func() error {
//		err := someFuncThatCanFail()
//		return err
//	})
//
// If the callback function does not return an error, Nozzle's behavior will be affected according to the success method.
func (n *Nozzle) DoError(fn func() error) {
	allowed := n.allowed.Load()
	blocked := n.blocked.Load()

	var allowRate int64

	if allowed != 0 {
		allowRate = int64((float64(allowed) / float64(allowed+blocked)) * 100)
	}

	allow := allowRate < n.flowRate.Load()

	if allow {
		n.allowed.Add(1)

		if err := fn(); err != nil {
			n.failure()
		} else {
			n.success()
		}
	} else {
		n.blocked.Add(1)
	}
}

// calculate updates the Nozzle's state based on the elapsed time and failure rate.
// It determines whether to open or close the Nozzle and triggers the ticker if necessary.
func (n *Nozzle) calculate() {
	n.mut.Lock()
	defer n.mut.Unlock()

	if time.Since(n.start) < n.Options.Interval {
		return
	}

	originalFlowRate := n.flowRate

	if n.failureRate() > n.Options.AllowedFailurePercent {
		n.close()
		n.state = Closing
	} else {
		n.open()
		n.state = Opening
	}

	if n.flowRate != originalFlowRate && n.Options.OnStateChange != nil {
		n.Options.OnStateChange(n.state)
	}

	n.reset()

	if n.ticker != nil {
		n.ticker <- struct{}{}
	}
}

// close reduces the flow rate and increases the multiplier to speed up the closing process.
// It is called when the failure rate exceeds the allowed threshold.
func (n *Nozzle) close() {
	fr := n.flowRate.Load()

	if fr == 0 {
		return
	}

	mult := n.decreaseBy.Load()
	if mult > -1 {
		mult = -1
	}

	n.flowRate.Store(clamp(fr + mult))
	n.decreaseBy.Store(mult * 2)
}

// open increases the flow rate and doubles the multiplier to speed up the opening process.
// It is called when the failure rate is within the allowed threshold.
func (n *Nozzle) open() {
	fr := n.flowRate.Load()
	if fr == 100 {
		return
	}

	mult := n.decreaseBy.Load()
	if mult < 1 {
		mult = 1
	}

	n.flowRate.Store(clamp(fr + mult))
	n.decreaseBy.Store(mult * 2)
}

// reset reinitializes the Nozzle's state for the next interval.
// It sets the start time to now and clears the counters for successes, failures, allowed, and blocked operations.
func (n *Nozzle) reset() {
	n.start = time.Now()
	n.successes.Store(0)
	n.failures.Store(0)
	n.allowed.Store(0)
	n.blocked.Store(0)
}

// success increments the count of successful operations.
// This contributes to calculating the success rate.
func (n *Nozzle) success() {
	n.successes.Add(1)
}

// failure increments the count of failed operations.
// This contributes to calculating the failure rate.
func (n *Nozzle) failure() {
	n.failures.Add(1)
}

// FlowRate reports the current flow rate.
// The flow rate determines how many calls will be allowed.
// Example: A flow rate of 100 will allow all calls, while a flow rate of 50 will allow 50% of calls.
func (n *Nozzle) FlowRate() int64 {
	return n.flowRate.Load()
}

// failureRate calculates the percentage of failed operations out of the total operations.
// Example: With 500 failures and 500 successes, the failure rate will be 50%.
func (n *Nozzle) failureRate() int64 {
	successes := n.successes.Load()
	failures := n.failures.Load()

	if failures == 0 && successes == 0 {
		return 0
	}

	// Ex: 500 failures, 500 successes
	// (500 / (500 + 500)) * 100 = 50
	return int64((float64(failures) / float64(failures+successes)))
}

// SuccessRate reports the success rate of Nozzle calls.
// It calculates the percentage of successful operations out of the total operations.
// Example: With 90 successes and 10 failures, the success rate will be 90%.
func (n *Nozzle) SuccessRate() int64 {
	failures := n.failures.Load()

	if n.flowRate.Load() == 0 {
		return 0
	}

	if failures == 0 && n.successes.Load() == 0 {
		return 100
	}

	return 100 - n.failureRate()
}

// FailureRate reports the failure rate of Nozzle calls.
// It calculates the percentage of failed operations out of the total operations.
// Example: With 10 failures and 90 successes, the failure rate will be 10%.
func (n *Nozzle) FailureRate() int64 {
	failures := n.failures.Load()

	if n.flowRate.Load() == 0 {
		return 0
	}

	if failures == 0 && n.successes.Load() == 0 {
		return 0
	}

	return n.failureRate()
}

// State reports the current state of the Nozzle.
// It reflects whether the Nozzle is currently in the process of opening or closing.
// Example: If the Nozzle is increasing its flow rate, the state will be Opening.
func (n *Nozzle) State() State {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.state
}

// Wait blocks until the Nozzle processes the next tick.
// This is useful for testing but should be avoided in production code.
func (n *Nozzle) Wait() {
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
