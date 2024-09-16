package nozzle

import (
	"sync"
	"time"
)

type nozzle struct {
	multiplier int
	flowRate   int
	successes  int64
	failures   int64
	allowed    int64
	blocked    int64
	start      time.Time
	mut        sync.RWMutex
	options    Options
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
func New(options Options) *nozzle {
	n := nozzle{
		flowRate: 100,
		start:    time.Now(),
		options:  options,
		ticker:   make(chan struct{}),
		state:    Opening,
	}

	go func() {
		for range time.Tick(options.Interval) {
			n.calculate()
		}
	}()

	return &n
}

func (n *nozzle) Do(fn func(func(), func())) {
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

func (n *nozzle) calculate() {
	n.mut.Lock()
	defer n.mut.Unlock()

	if time.Since(n.start) < n.options.Interval {
		return
	}

	if n.failureRate() > n.options.AllowedFailurePercent {
		n.close()
		n.state = Closing
	} else {
		n.open()
		n.state = Opening
	}

	n.reset()

	n.ticker <- struct{}{}
}

func (n *nozzle) close() {
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

func (n *nozzle) open() {
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

func (n *nozzle) reset() {
	n.start = time.Now()
	n.successes = 0
	n.failures = 0
	n.allowed = 0
	n.blocked = 0
}

func (n *nozzle) success() {
	n.successes++
}

func (n *nozzle) failure() {
	n.failures++
}

func (n *nozzle) FlowRate() int {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.flowRate
}

func (n *nozzle) failureRate() int {
	if n.failures == 0 && n.successes == 0 {
		return 0
	}

	// Ex: 500 failures, 500 successes
	// (500 / (500 + 500)) * 100 = 50
	return int((float64(n.failures) / float64(n.failures+n.successes)) * 100)
}

func (n *nozzle) SuccessRate() int {
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

func (n *nozzle) FailureRate() int {
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

func (n *nozzle) State() State {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.state
}

func (n *nozzle) Wait() {
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
