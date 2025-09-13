# OpenTelemetry Metrics Integration

This example shows how to connect Nozzle's `OnStateChange` callback to OpenTelemetry metrics for tracking state changes and flow rate.

## Setup Metrics

```go
import (
    "context"
    "github.com/justindfuller/nozzle"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

func setupNozzleWithMetrics[T any](opts nozzle.Options[T]) (*nozzle.Nozzle[T], error) {
    // Get meter from existing OpenTelemetry setup
    meter := otel.Meter("nozzle")
    
    // Create observable gauge for flow rate
    flowRateGauge, _ := meter.Float64ObservableGauge(
        "nozzle.flow_rate",
        metric.WithDescription("Current flow rate (0-100%)"),
        metric.WithUnit("%"),
    )
    
    // Counter for state changes
    stateChanges, _ := meter.Int64Counter(
        "nozzle.state_changes",
        metric.WithDescription("Count of state transitions"),
    )
    
    // Store latest flow rate for observable gauge
    var latestFlowRate float64
    
    // Register callback for observable gauge
    meter.RegisterCallback(
        func(_ context.Context, o metric.Observer) error {
            o.ObserveFloat64(flowRateGauge, latestFlowRate)
            return nil
        },
        flowRateGauge,
    )
    
    // Configure OnStateChange to update metrics
    opts.OnStateChange = func(snapshot nozzle.StateSnapshot) {
        // Update gauge value
        latestFlowRate = float64(snapshot.FlowRate)
        
        // Record state change with attributes
        stateChanges.Add(context.Background(), 1,
            metric.WithAttributes(
                attribute.String("state", string(snapshot.State)),
                attribute.Int64("flow_rate", snapshot.FlowRate),
            ))
    }
    
    return nozzle.New(opts)
}
```

## Usage

```go
// Create nozzle with metrics
n, err := setupNozzleWithMetrics(nozzle.Options[any]{
    Interval:              time.Second,
    AllowedFailurePercent: 50,
})
if err != nil {
    return err
}
defer n.Close()

// Use nozzle normally - metrics are automatically recorded
result, err := n.DoError(func() (any, error) {
    return someOperation()
})
```

## Metrics Produced

| Metric | Type | Description |
|--------|------|-------------|
| `nozzle.flow_rate` | Gauge | Current percentage of allowed operations (0-100) |
| `nozzle.state_changes` | Counter | Total state transitions with `state` and `flow_rate` attributes |

## Key Points

- The `StateSnapshot` passed to `OnStateChange` provides thread-safe access to all metrics
- Observable gauges are ideal for point-in-time values like flow rate
- The callback executes synchronously during state calculation, so avoid blocking operations
- Metrics are only recorded when state actually changes (at most once per interval)