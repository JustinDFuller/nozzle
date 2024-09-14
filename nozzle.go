package nozzle

import "time"

type Nozzle struct {
	flowRate    int
	successRate int

	successes int64
	failures  int64
	start     time.Time
}

type Options struct {
	Interval time.Duration

	Step int

	AllowedFailurePercent int
}

func New(options Options) Nozzle {
	return Nozzle{
		flowRate:    100,
		successRate: 100,
	}
}

func (n *Nozzle) Do(fn func(func(), func())) {
	if n.start.IsZero() {
		n.start = time.Now()
	}

	fn(n.success, n.failure)
}

func (n *Nozzle) success() {
	n.successes++
}

func (n *Nozzle) failure() {
	n.failures++
}

func (n *Nozzle) FlowRate() int {
	return n.flowRate
}

func (n *Nozzle) SuccessRate() int {
	return n.successRate
}
