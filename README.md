# nozzle

[![Go Reference](https://pkg.go.dev/badge/github.com/justindfuller/nozzle.svg)](https://pkg.go.dev/github.com/justindfuller/nozzle)
[![Build Status](https://github.com/JustinDFuller/nozzle/actions/workflows/build.yml/badge.svg)](https://github.com/JustinDFuller/nozzle/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/justindfuller/nozzle)](https://goreportcard.com/report/github.com/justindfuller/nozzle)
![Go Test Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/justindfuller/63d4999a653a0555c9806062b40c0139/raw/coverage.json)

The Hose Nozzle Pattern

## TL;DR

It allows/disallows actions gradually (like a hose nozzel) instead of totally, (like a switch).

## 📖 Table of Contents 📖
* [Explanation](#what)
* [Usage](#usage)
* [Observability](#observability)
* [Performance](#performance)
* [Documentation](#documentation)

## What?

Imagine these two control devices in your home:

1. A Light Switch
2. A Hose Nozzle

A light switch has two modes: off and on (generally; yes, I am aware of dimmers). A hose nozzle has many positions between fully off and fully on.

The [circuit breaker pattern](https://en.wikipedia.org/wiki/Circuit_breaker_design_pattern) mimics a part of your home similar to a light switch. A circuit breaker stays on at full power, then, when something goes wrong, it switches completely off. In the physical world, circuit breakers play an important role: they prevent surging electricity from destroying our electronics.

In technology, the circuit breaker pattern prevents one system from overloading another, giving it time to recover.

However, in many systems, particularly systems that experience errors due to extreme sudden scaling, it may not be necessary to shut things completely off.

### Example Scenario

Imagine a scenario where an application is handing 1000 requests per second (RPS). Suddenly, it receives 10,000 requests per second. Now, assume the application takes somewhere between a few seconds and a minute to scale up. Until it scales up, it can only handle a bit more than 1000 requests per second, the rest return errors.

If you are using the circuit breaker pattern, and if you configured your circuit breaker to trip above a 10% error rate, you will likely go from 1000 RPS to 0 RPS. Then, once the application scales up, you may jump up to 10,000 RPS. Or, if the surge has passed, you will return to 1000 RPS.

This is not ideal. During this time, the system was able to handle the original 1000 RPS. In fact, as it scales up, it was likely able to handle increasingly (but gradually) higher amounts of traffic.

### Alternative

A better strategy would be to quickly (but gradually) scale the allowed traffic down until the system reaches the desired success rate. Then, to attempt to scale back up until the error threshold is passed.

Thus: we have the nozzle pattern. Like a hose nozzle, it gradually opens and closes in response to behaviors. Its goal is to stay 100% open, but it will only open as far as it can without passing the specified error threshold.

### Illustration

The following images illustrate the difference in behavior.

First, the circuit breaker: once the threshold of 25% is crossed, the circuit breaker engages and fully shuts off requests. This results in a total loss of traffic. After a few seconds, the half-open step begins. Once it sees the half-open requests succeed, it fully re-opens.

<p align="center">
    <img src="https://github.com/user-attachments/assets/7dbc3c30-158a-45c0-91de-4c0d46e6c7de" alt="Circuit Breaker Illustration" height="300px" />
</p>

Second, the nozzle: once the threshold of 25% is crossed, the nozzle begins closing. First, slowly, then increasingly more quickly. Once it notices the failure rate has decreased below the threshold, it begins to open again.

In this case, you should notice it takes longer to return to full throughput, but overall loses fewer requests.

<p align="center">
    <img src="https://github.com/user-attachments/assets/074c024c-63bc-40e4-8953-214d2c9f69cc" alt="Nozzle Illustration" height="300px" />
</p>

## Usage

First, install the package:

```
go get github.com/justindfuller/nozzle
```

Then, create a nozzle:

```go
package main

import (
    "net/http"

    "github.com/justindfuller/nozzle"
)

func main() {
    n := nozzle.New(nozzle.Options[*http.Response]{
        Interval:              time.Second,
        AllowedFailurePercent: 50,
    })
    defer n.Close()

    for i := 0; i < 1000; i++ {
        res, err := n.DoError(func() (*http.Response, error) {
            res, err := http.Get("https://google.com")

            return res, err
        })
        if err != nil {
            log.Println(err)

            continue
        }

        log.Println(res)
    }
}
```

The Nozzle will attempt to execute as many requests as possible.

If you are not working with errors, you can use a Boolean Nozzle.

```go
package main

import (
    "net/http"

    "github.com/justindfuller/nozzle"
)

func main() {
    n := nozzle.New(nozzle.Options[*http.Response]{
        Interval:              time.Second,
        AllowedFailurePercent: 50,
    })
    defer n.Close()

    for i := 0; i < 1000; i++ {
        res, ok := n.DoBool(func() (*http.Response, bool) {
            res, err := http.Get("https://google.com")

            return res, err != nil && res.StatusCode == http.StatusOK
        })
        if !ok {
            log.Println("Request failed")

            continue
        }

        log.Println(res)
    }
}
```

### Resource Cleanup

Always close the nozzle when done to prevent goroutine leaks:

```go
n := nozzle.New(nozzle.Options[any]{
    Interval:              time.Second,
    AllowedFailurePercent: 50,
})
defer n.Close()

// Use the nozzle...
```

The `Close()` method:
- Like closing a literal nozzle, the flow stops completely
- Stops the internal ticker goroutine
- Is idempotent (safe to call multiple times)
- Should be called when the nozzle is no longer needed

After closing:
- `DoBool` returns the zero value and `false` without executing the callback
- `DoError` returns the zero value and `nozzle.ErrClosed` without executing the callback
- No callbacks are executed once the nozzle is closed
- The nozzle becomes completely non-functional to prevent resource usage

### Generics

As you can see, this package uses generics. This allows the Nozzle's methods to return the same type as the function you pass to it. This allows the Nozzle to perform its work without interrupting the control-flow of your application.

## Migration Guide (v1 to v2)

### Breaking Change: OnStateChange Callback

The `OnStateChange` callback signature has changed to fix a critical race condition. The new API provides an immutable `StateSnapshot` instead of a direct reference to the `Nozzle`.

#### Old API (v1.x)
```go
OnStateChange: func(noz *nozzle.Nozzle[string]) {
    // Potential race condition: mutex unlocked during callback
    fmt.Printf("Flow rate: %d%%\n", noz.FlowRate())
    fmt.Printf("State: %s\n", noz.State())
}
```

#### New API (v2.x)
```go
OnStateChange: func(snapshot nozzle.StateSnapshot) {
    // Thread-safe: immutable snapshot with no lock contention
    fmt.Printf("Flow rate: %d%%\n", snapshot.FlowRate)
    fmt.Printf("State: %s\n", snapshot.State)
}
```

#### Migration Steps

1. **Update callback signature** from `func(*Nozzle[T])` to `func(StateSnapshot)`
2. **Replace method calls with field access**:
   - `noz.FlowRate()` → `snapshot.FlowRate`
   - `noz.State()` → `snapshot.State`
   - `noz.FailureRate()` → `snapshot.FailureRate`
   - `noz.SuccessRate()` → `snapshot.SuccessRate`
3. **Access new cumulative counters** (not available in v1):
   - `snapshot.Allowed` - total operations allowed
   - `snapshot.Blocked` - total operations blocked

#### Benefits of the New API

- **Eliminates race conditions** - No mutex unlocking during callback execution
- **Atomic state view** - All fields represent the same instant in time
- **Better performance** - No lock contention between callback and operations
- **Zero allocations** - StateSnapshot is passed by value
- **Additional metrics** - Access to cumulative counters

## Observability

You may want to collect metrics to help you observe when your nozzle is opening and closing. You can accomplish this with `nozzle.OnStateChange`. `OnStateChange` will be called _at most_ once per `Interval` but only if a change occurred.

The callback receives a `StateSnapshot` containing an immutable copy of the nozzle's state, ensuring thread-safe access to state information:

```go
nozzle.New(nozzle.Options[*example]{
    Interval:              time.Second,
    AllowedFailurePercent: 50,
    OnStateChange: func(snapshot nozzle.StateSnapshot) {
        logger.Info(
            "Nozzle State Change",
            "state",
            snapshot.State,
            "flowRate",
            snapshot.FlowRate,
            "failureRate",
            snapshot.FailureRate,
            "successRate",
            snapshot.SuccessRate,
        )
        /**
         Example output:
         {
            "message": "Nozzle State Change",
            "state": "opening",
            "flowRate": 50,
            "failureRate": 20,
            "successRate": 80
         }
        **/
    },
}
```

### Real-World Examples

#### Prometheus Metrics Integration

```go
var (
    nozzleFlowRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nozzle_flow_rate",
            Help: "Current flow rate percentage (0-100)",
        },
        []string{"service"},
    )
    nozzleFailureRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "nozzle_failure_rate",
            Help: "Current failure rate percentage (0-100)",
        },
        []string{"service"},
    )
    nozzleStateChanges = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "nozzle_state_changes_total",
            Help: "Total number of state changes",
        },
        []string{"service", "state"},
    )
)

noz := nozzle.New(nozzle.Options[*Response]{
    Interval:              time.Second,
    AllowedFailurePercent: 30,
    OnStateChange: func(snapshot nozzle.StateSnapshot) {
        serviceName := "api-gateway"
        
        nozzleFlowRate.WithLabelValues(serviceName).Set(float64(snapshot.FlowRate))
        nozzleFailureRate.WithLabelValues(serviceName).Set(float64(snapshot.FailureRate))
        nozzleStateChanges.WithLabelValues(serviceName, string(snapshot.State)).Inc()
        
        // Track cumulative stats
        prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "nozzle_total_allowed",
        }).Set(float64(snapshot.Allowed))
        
        prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "nozzle_total_blocked",
        }).Set(float64(snapshot.Blocked))
    },
})
```

#### Alerting Integration

```go
type AlertManager struct {
    client *alerting.Client
}

noz := nozzle.New(nozzle.Options[*Result]{
    Interval:              5 * time.Second,
    AllowedFailurePercent: 25,
    OnStateChange: func(snapshot nozzle.StateSnapshot) {
        switch {
        case snapshot.FlowRate == 0:
            // Critical: Circuit fully closed
            go alertManager.SendCritical(
                "Service completely blocked",
                map[string]interface{}{
                    "flow_rate":    snapshot.FlowRate,
                    "failure_rate": snapshot.FailureRate,
                    "blocked_ops":  snapshot.Blocked,
                },
            )
            
        case snapshot.FlowRate < 25:
            // Warning: Severe throttling
            go alertManager.SendWarning(
                fmt.Sprintf("Service severely throttled at %d%%", snapshot.FlowRate),
                map[string]interface{}{
                    "flow_rate":    snapshot.FlowRate,
                    "failure_rate": snapshot.FailureRate,
                },
            )
            
        case snapshot.State == nozzle.Closing && snapshot.FlowRate < 50:
            // Info: Service degrading
            go alertManager.SendInfo(
                "Service entering degraded state",
                map[string]interface{}{
                    "flow_rate":    snapshot.FlowRate,
                    "state":        snapshot.State,
                },
            )
        }
    },
})
```

#### Testing with StateSnapshot

```go
func TestNozzleStateChanges(t *testing.T) {
    var snapshots []nozzle.StateSnapshot
    var mu sync.Mutex
    
    noz := nozzle.New(nozzle.Options[string]{
        Interval:              100 * time.Millisecond,
        AllowedFailurePercent: 50,
        OnStateChange: func(snapshot nozzle.StateSnapshot) {
            mu.Lock()
            snapshots = append(snapshots, snapshot)
            mu.Unlock()
        },
    })
    defer noz.Close()
    
    // Simulate failures to trigger closing
    for i := 0; i < 100; i++ {
        noz.DoBool(func() (string, bool) {
            return "test", false // All failures
        })
    }
    
    // Wait for state calculation
    time.Sleep(200 * time.Millisecond)
    
    // Verify state changed to closing
    mu.Lock()
    defer mu.Unlock()
    
    require.NotEmpty(t, snapshots, "Expected state changes")
    lastSnapshot := snapshots[len(snapshots)-1]
    
    assert.Equal(t, nozzle.Closing, lastSnapshot.State)
    assert.Less(t, lastSnapshot.FlowRate, int64(100))
    assert.Greater(t, lastSnapshot.FailureRate, int64(50))
}
```

#### Distributed Tracing Integration

```go
noz := nozzle.New(nozzle.Options[*Response]{
    Interval:              time.Second,
    AllowedFailurePercent: 40,
    OnStateChange: func(snapshot nozzle.StateSnapshot) {
        // Add state change as a trace event
        span := opentracing.StartSpan("nozzle.state_change")
        defer span.Finish()
        
        span.SetTag("flow_rate", snapshot.FlowRate)
        span.SetTag("state", snapshot.State)
        span.SetTag("failure_rate", snapshot.FailureRate)
        span.SetTag("success_rate", snapshot.SuccessRate)
        
        if snapshot.FlowRate < 50 {
            span.SetTag("alert", "degraded")
            span.LogKV(
                "event", "flow_restricted",
                "flow_rate", snapshot.FlowRate,
                "blocked_total", snapshot.Blocked,
            )
        }
    },
})
```

## Performance

The performance is excellent. 0 bytes per operation, 0 allocations per operation. It works with concurrent goroutines without any race conditions.

```go
@JustinDFuller ➜ /workspaces/nozzle (main) $ make bench
goos: linux
goarch: amd64
pkg: github.com/justindfuller/nozzle
cpu: AMD EPYC 7763 64-Core Processor
BenchmarkNozzle_DoBool_Open-2             908032            1316 ns/op               0 B/op       0 allocs/op
BenchmarkNozzle_DoBool_Closed-2          2301523             445.2 ns/op             0 B/op       0 allocs/op
BenchmarkNozzle_DoBool_Half-2             981314            1313 ns/op               0 B/op       0 allocs/op
BenchmarkNozzle_DoError_Open-2            892647            1446 ns/op               0 B/op       0 allocs/op
BenchmarkNozzle_DoError_Closed-2         2554688             452.1 ns/op             0 B/op       0 allocs/op
BenchmarkNozzle_DoError_Half-2            964617            1311 ns/op               0 B/op       0 allocs/op
BenchmarkNozzle_DoBool_Control-2         1292871             960.8 ns/op             0 B/op       0 allocs/op
PASS
ok      github.com/justindfuller/nozzle 11.410s
```

### StateSnapshot Performance

The `StateSnapshot` approach maintains zero allocations while providing additional benefits:

- **Zero Allocations**: StateSnapshot is a value type copied by value
- **No Lock Contention**: Callback execution doesn't block nozzle operations
- **Atomic State View**: All state fields reflect the same point in time
- **Reduced Latency**: No mutex acquisition during callback execution

## Thread Safety

The nozzle library is designed for safe concurrent use across multiple goroutines.

### Safe Concurrent Operations

All public methods are thread-safe and can be called concurrently:

```go
// Safe: Multiple goroutines can call DoBool/DoError concurrently
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        result, ok := noz.DoBool(func() (string, bool) {
            return "data", true
        })
    }()
}
wg.Wait()
```

### StateSnapshot Pattern

The `StateSnapshot` eliminates race conditions in callbacks:

```go
// Thread-Safe (Recommended)
OnStateChange: func(snapshot nozzle.StateSnapshot) {
    // All fields are immutable and safe to access
    // No locks required, no race conditions possible
    processMetrics(snapshot.FlowRate, snapshot.FailureRate)
}
```

Compare with direct method calls which acquire locks:

```go
// Thread-Safe but may cause lock contention
currentRate := noz.FlowRate() // Acquires mutex
currentState := noz.State()   // Acquires mutex again
// Note: These values may be from different points in time
```

### Callback Execution Guarantees

- Callbacks are called **sequentially** (never concurrently)
- Callbacks are called **at most once per interval**
- Panics in callbacks are recovered and don't affect nozzle operation
- Long-running callbacks may delay subsequent state calculations

### Best Practices

1. **Use StateSnapshot in callbacks** - Avoids lock contention
2. **Keep callbacks fast** - Queue heavy work for async processing
3. **Always call Close()** - Ensures proper resource cleanup
4. **Use defer for cleanup** - Guarantees Close() is called

```go
noz := nozzle.New(options)
defer noz.Close() // Always clean up resources
```

## Documentation

Please refer to the go documentatio hosted on [pkg.go.dev](https://pkg.go.dev/github.com/justindfuller/nozzle). You can see [all available types and methods](https://pkg.go.dev/github.com/justindfuller/nozzle#pkg-index) and [runnable examples](https://pkg.go.dev/github.com/justindfuller/nozzle#pkg-examples).
