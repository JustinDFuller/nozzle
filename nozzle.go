package nozzle

import (
	"sync"
	"time"
)

type nozzle struct {
	flowRate  int
	successes int64
	failures  int64
	allowed   int64
	blocked   int64
	start     time.Time
}

type Nozzle struct {
	previous nozzle
	current  nozzle
	options  Options

	mut    sync.RWMutex
	ticker chan struct{}
}

type Options struct {
	Interval              time.Duration
	AllowedFailurePercent int
}

func New(options Options) *Nozzle {
	n := Nozzle{
		current: nozzle{
			flowRate: 100,
			start:    time.Now(),
		},
		options: options,
		ticker:  make(chan struct{}),
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

	if n.current.allowed != 0 {
		allowRate = int((float64(n.current.allowed) / float64(n.current.allowed+n.current.blocked)) * 100)
	}

	allow := allowRate < n.current.flowRate

	if allow {
		n.current.allowed++
		fn(n.success, n.failure)
	} else {
		n.current.blocked++
	}
}

func (n *Nozzle) calculate() {
	n.mut.Lock()
	defer n.mut.Unlock()

	if n.current.start.Add(n.options.Interval).After(time.Now()) {
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
	if n.current.flowRate == 0 {
		return
	}

	n.current.flowRate = n.current.flowRate / 2
}

func (n *Nozzle) open() {
	if n.current.flowRate == 100 {
		return
	}

	n.current.flowRate += (100 - n.current.flowRate) / 2
}

func (n *Nozzle) reset() {
	n.previous = n.current
	n.current = nozzle{
		start:    time.Now(),
		flowRate: n.previous.flowRate,
	}
}

func (n *Nozzle) success() {
	n.current.successes++
}

func (n *Nozzle) failure() {
	n.current.failures++
}

func (n *Nozzle) FlowRate() int {
	n.mut.RLock()
	defer n.mut.RUnlock()

	return n.current.flowRate
}

func (n *Nozzle) failureRate() int {
	if n.current.failures == 0 && n.current.successes == 0 {
		return 0
	}

	// Ex: 500 failures, 500 successes
	// (500 / (500 + 500)) * 100 = 50
	return int((float64(n.current.failures) / float64(n.current.failures+n.current.successes)) * 100)
}

func (n *Nozzle) SuccessRate() int {
	n.mut.RLock()
	defer n.mut.RUnlock()

	if n.previous.failures == 0 && n.previous.successes == 0 {
		return 100
	}

	// Ex: 500 failures, 500 successes
	// (500 / (500 + 500)) * 100 = 50
	failureRate := int((float64(n.previous.failures) / float64(n.previous.failures+n.previous.successes)) * 100)

	return 100 - failureRate
}

func (n *Nozzle) Wait() {
	<-n.ticker
}
