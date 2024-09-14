package nozzle

import (
	"sync"
	"time"
)

type Nozzle struct {
	flowRate  int
	successes int64
	failures  int64
	allowed   int64
	blocked   int64
	start     time.Time
	options   Options

	mut sync.RWMutex
}

type Options struct {
	Interval              time.Duration
	AllowedFailurePercent int
}

func New(options Options) *Nozzle {
	n := Nozzle{
		options:  options,
		flowRate: 100,
		start:    time.Now(),
	}

	go func() {
		for range time.Tick(options.Interval) {
			n.calculate()
		}
	}()

	return &n
}

func (n *Nozzle) Do(fn func(func(), func())) {
	n.mut.Lock()
	defer n.mut.Unlock()

	var allowRate int

	if n.allowed != 0 {
		allowRate = int((float64(n.allowed) / float64(n.allowed+n.blocked)) * 100)
	}

	allow := allowRate <= n.flowRate

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

	if n.start.Add(n.options.Interval).After(time.Now()) {
		return
	}

	if n.failureRate() > n.options.AllowedFailurePercent {
		n.close()
	} else {
		n.open()
	}

	n.reset()
}

func (n *Nozzle) close() {
	if n.flowRate == 0 {
		return
	}

	n.flowRate = n.flowRate / 2
}

func (n *Nozzle) open() {
	if n.flowRate == 100 {
		return
	}
}

func (n *Nozzle) reset() {
	n.successes = 0
	n.failures = 0
	n.allowed = 0
	n.blocked = 0
	n.start = time.Now()
}

func (n *Nozzle) success() {
	n.successes++
}

func (n *Nozzle) failure() {
	n.failures++
}

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

func (n *Nozzle) SuccessRate() int {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return 100 - n.failureRate()
}
