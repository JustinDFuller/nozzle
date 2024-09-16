# nozzle

[![Go Reference](https://pkg.go.dev/badge/github.com/justindfuller/nozzle.svg)](https://pkg.go.dev/github.com/justindfuller/nozzle)

The Hose Nozzle Pattern

## TL;DR

It allows/disallows actions gradually (like a hose nozzel) instead of totally, (like a switch).

## What?

Imagine these two control devices in your home:

1. A Light Switch
2. A Hose Nozzle

A light switch has two modes: off and on (generally; yes, I am aware of dimmers). A hose nozzle has many positions between fully off and fully on.

The [circuit breaker pattern](https://en.wikipedia.org/wiki/Circuit_breaker_design_pattern) mimics a part of your home similar to a light switch. A circuit breaker stays on at full power, then, when something goes wrong, it switches completely off. In the physical world, circuit breakers play an important role: they prevent surging electricity from destroying our electronics.

In technology, the circuit breaker pattern prevents one system from overloading another, giving it time to recover.

However, in many systems, particularly systems that experience errors due to extreme sudden scaling, it may not be necessary to shut things completely off. 

Imagine a scenario where an application is handing 1000 requests per second (RPS). Suddenly, it receives 10,000 requests per second. Now, assume the application takes somewhere between a few seconds and a minute to scale up. Until it scales up, it can only handle a bit more than 1000 requests per second, the rest return errors.

If you are using the circuit breaker pattern, and if you configured your circuit breaker to trip above a 10% error rate, you will likely go from 1000 RPS to 0 RPS. Then, once the application scales up, you may jump up to 10,000 RPS. Or, if the surge has passed, you will return to 1000 RPS.

This is not ideal. During this time, the system was able to handle the original 1000 RPS. In fact, as it scales up, it was likely able to handle increasingly (but gradually) higher amounts of traffic.

A better strategy would be to quickly (but gradually) scale the allowed traffic down until the system reaches the desired success rate. Then, to attempt to scale back up until the error threshold is passed.

Thus: we have the nozzle pattern. Like a hose nozzle, it gradually opens and closes in response to behaviors. Its goal is to stay 100% open, but it will only open as far as it can without passing the specified error threshold.

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
        n.Do(func(success, failure func()) {
            _, err := http.Get("https://google.com")
            if err == nil {
                success()
            } else {
                failure()
            }
        })
    }
}
```

The Nozzle will attempt to execute as many requests as possible.

