package nozzle

import (
	"sync"
	"time"
)

type Nozzle struct {
	multiplier int

	flowRate  int
	successes int64
	failures  int64
	allowed   int64
	blocked   int64
	start     time.Time
	mut       sync.RWMutex
	options   Options
	ticker    chan struct{}
}

type Options struct {
	Interval              time.Duration
	AllowedFailurePercent int
}

func New(options Options) *Nozzle {
	n := Nozzle{
		flowRate: 100,
		start:    time.Now(),
		options:  options,
		ticker:   make(chan struct{}),
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

	if time.Since(n.start) < n.options.Interval {
		return
	}

	if n.failureRate() > n.options.AllowedFailurePercent {
		n.close()
	} else {
		n.open()
	}

	n.reset()

	n.ticker <- struct{}{}
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

	if n.flowRate == 0 {
		return 0
	}

	if n.failures == 0 && n.successes == 0 {
		return 100
	}

	return 100 - n.failureRate()
}

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

func (n *Nozzle) Wait() {
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
