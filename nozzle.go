package nozzle

import (
	"sync"
	"time"
)

type Nozzle struct {
	Options Options

	multiplier int
	flowRate   int
	successes  int64
	failures   int64
	allowed    int64
	blocked    int64
	start      time.Time
	mut        sync.RWMutex
	state      State
	ticker     chan struct{}
}

// Options controls the behavior of the Nozzle.
type Options struct {
	// Interval controls how often the Nozzle will process.
	//
	// Example:
	//
	//	Interval: time.Second
	//	Interval: time.Millisecond * 100
	//
	// The best interval depends on the needs of your application.
	// If you are unsure, start with 1 second.
	Interval time.Duration
	// AllowedFailurePercent determines when the Nozzle should close.
	// It is an integer that represents a percentage.
	//
	// Example:
	//
	// 	0 means 0% failure rate, no failures allowed.
	// 	100 means 100% failure rate, all failures allowed.
	//	50 means 50% failure rate, 50% failures allowed.
	//
	// At or above this failure percent, the Nozzle will attempt to open as far as possible.
	// Below this failure percent, the Nozzle will close until it is no longer below this percent.
	// If it never rises above this failure percent, the Nozzle will completely close.
	// If it never falls below this failure percent, the Nozzle will completely open.
	//
	// The best FailurePercent depends on the needs of your application.
	// If you are unsure, start with 50%.
	AllowedFailurePercent int
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
//		AllowedFailureRate: 50,
//	})
//
// See docs of nozzle.Options for details about each Option field.
func New(options Options) *Nozzle {
	n := Nozzle{
		flowRate: 100,
		Options:  options,
		state:    Opening,
	}

	go func() {
		for range time.Tick(options.Interval) {
			n.calculate()
		}
	}()

	return &n
}

// Do executes a callback function while respecting the Nozzle's state.
// It monitors how many calls have been allowed and compares it with the flowRate to determine if this particular call will be allowed.
//
// The callback function receives two function arguments: success and failure.
// You must call success if your callback function succeeded.
// You must call failure if your callback function failed.
//
// These functions will always be non-nil.
//
// Example:
//
//	var n nozzle.Nozzle
//
//	n.Do(func(success, failure func()) {
//		err := someFuncThatCanFail()
//		if err == nil {
//			success()
//		} else {
//			failure()
//		}
//	})
//
// If you do not call success/failure, Nozzle will have no effect.
func (n *Nozzle) Do(fn func(func(), func())) {
	n.mut.Lock()
	defer n.mut.Unlock()

	var allowRate int

	if n.allowed != 0 {
		allowRate = int((float64(n.allowed) / float64(n.allowed+n.blocked)) * 100)
	}

	allow := allowRate < n.flowRate

	if allow {
		n.allowed++
		fn(n.success, n.failure)
	} else {
		n.blocked++
	}
}

func (n *Nozzle) calculate() {
	n.mut.Lock()
	defer n.mut.Unlock()

	if time.Since(n.start) < n.Options.Interval {
		return
	}

	if n.failureRate() > n.Options.AllowedFailurePercent {
		n.close()
		n.state = Closing
	} else {
		n.open()
		n.state = Opening
	}

	n.reset()

	if n.ticker != nil {
		n.ticker <- struct{}{}
	}
}

func (n *Nozzle) close() {
	if n.flowRate == 0 {
		return
	}

	mult := n.multiplier
	if mult > -1 {
		mult = -1
	}

	n.flowRate = clamp(n.flowRate + mult)
	n.multiplier = mult * 2
}

func (n *Nozzle) open() {
	if n.flowRate == 100 {
		return
	}

	mult := n.multiplier
	if mult < 1 {
		mult = 1
	}

	n.flowRate = clamp(n.flowRate + mult)
	n.multiplier = mult * 2
}

func (n *Nozzle) reset() {
	n.start = time.Now()
	n.successes = 0
	n.failures = 0
	n.allowed = 0
	n.blocked = 0
}

func (n *Nozzle) success() {
	n.successes++
}

func (n *Nozzle) failure() {
	n.failures++
}

// FlowRate reports the current flow rate.
// The flow rate determines how many calls will be allowed.
// Ex: A flow rate of 100 will allow all calls.
//
//	A flow rate of 0 will allow no calls.
//	A flow rate of 50 will allow 50% of calls.
func (n *Nozzle) FlowRate() int {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.flowRate
}

func (n *Nozzle) failureRate() int {
	if n.failures == 0 && n.successes == 0 {
		return 0
	}

	// Ex: 500 failures, 500 successes
	// (500 / (500 + 500)) * 100 = 50
	return int((float64(n.failures) / float64(n.failures+n.successes)) * 100)
}

// SuccessRate reports the success rate of nozzle calls.
// If 100% of nozzle.Do calls are reporting success, the success rate will be 100.
// If 0% of calls are reporting success, success rate will be 0.
func (n *Nozzle) SuccessRate() int {
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

// FailureRate reports the failure rate of nozzle calls.
// If 100% of nozzle.Do calls are reporting failure, the failure rate will be 100.
// If 0% of calls are reporting failure, failure rate will be 0.
func (n *Nozzle) FailureRate() int {
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

// State reports the current state of the nozzle.
// Except for an un-initialized Nozzle, the state is always opening or closing.
func (n *Nozzle) State() State {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.state
}

// Wait will block until the Nozzle processes the next tick.
// You should most likely not use this in production, but it is useful for testing.
func (n *Nozzle) Wait() {
	n.mut.Lock()

	if n.ticker == nil {
		n.ticker = make(chan struct{})
	}

	n.mut.Unlock()

	<-n.ticker
}

func clamp(i int) int {
	if i < 0 {
		return 0
	}

	if i > 100 {
		return 100
	}

	return i
}
