# nozzle

[![Go Reference](https://pkg.go.dev/badge/github.com/justindfuller/nozzle.svg)](https://pkg.go.dev/github.com/justindfuller/nozzle)
[![Build Status](https://github.com/JustinDFuller/nozzle/actions/workflows/build.yml/badge.svg)](https://github.com/JustinDFuller/nozzle/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/justindfuller/nozzle)](https://goreportcard.com/report/github.com/justindfuller/nozzle)

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

func main() {}
    n := nozzle.New(nozzle.Options{
        Interval:              time.Second,
        AllowedFailurePercent: 50,
    })

    for i := 0; i < 1000; i++ {
        n.DoError(func() error {
            _, err := http.Get("https://google.com")
            
            return err
        })
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

func main() {}
    n := nozzle.New(nozzle.Options{
        Interval:              time.Second,
        AllowedFailurePercent: 50,
    })

    for i := 0; i < 1000; i++ {
        n.DoBool(func() bool {
            res, _ := http.Get("https://google.com")
            
            return res.Body == nil
        })
    }
}
```

## Observability

You may want to collect metrics to help you observe when your nozzle is opening and closing. You can accomplish this with `nozzle.OnStateChange`. `OnStateChange` will be called _at most_ once per `Interval` but only if a change occured.

```go
noz := nozzle.New(nozzle.Options{
    Interval:              time.Second,
    AllowedFailurePercent: 50,
    OnStateChange: func(s nozzle.State) {
        logger.Info(
            "Nozzle State Change", 
            "state", 
            s, 
            "flowRate", 
            noz.FlowRate(),
            "failureRate",
            noz.FailureRate(),
            "successRate",
            noz.SuccessRate(),
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

## Documentation

Please refer to the go documentatio hosted on [pkg.go.dev](https://pkg.go.dev/github.com/justindfuller/nozzle). You can see [all available types and methods](https://pkg.go.dev/github.com/justindfuller/nozzle#pkg-index) and [runnable examples](https://pkg.go.dev/github.com/justindfuller/nozzle#pkg-examples).
