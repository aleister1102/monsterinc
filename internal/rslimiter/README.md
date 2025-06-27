# go-rslimiter: Go Resource Limiter

[![Go Reference](https://pkg.go.dev/badge/github.com/aleister1102/go-rslimiter.svg)](https://pkg.go.dev/github.com/aleister1102/go-rslimiter)

A robust, standalone Go library for monitoring and controlling application resource usage (memory, CPU, goroutines). `go-rslimiter` helps prevent resource exhaustion and enables graceful shutdowns for enhanced application stability.

## Quick Start

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aleister1102/go-rslimiter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// 1. Define your resource limits
	config := rslimiter.ResourceLimiterConfig{
		MaxMemoryMB:        256,  // Max app memory: 256 MB
		MaxGoroutines:      500,
		CheckInterval:      10 * time.Second,
		SystemMemThreshold: 0.9, // Shutdown if system memory reaches 90%
		CPUThreshold:       0.9, // Shutdown if CPU usage reaches 90%
		EnableAutoShutdown: true,
	}

	// 2. Create a new limiter instance
	resourceLimiter := rslimiter.NewResourceLimiter(config, logger)

	// 3. Register a callback for graceful shutdown
	resourceLimiter.SetShutdownCallback(func() {
		log.Info().Msg("Shutdown callback triggered! Cleaning up...")
		// Simulate cleanup (e.g., close DB connections, finish requests)
		time.Sleep(2 * time.Second)
		log.Info().Msg("Cleanup finished. Exiting.")
		os.Exit(1)
	})

	// 4. Start the monitoring loop
	resourceLimiter.Start()
	defer resourceLimiter.Stop()

	log.Info().Msg("Resource limiter is running. Application logic goes here.")
	log.Info().Msg("Press Ctrl+C to exit.")

	// Wait for an interrupt signal to keep the application running
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	<-sc
}
```

## Installation

```bash
go get github.com/aleister1102/go-rslimiter
```

## Features

- **Comprehensive Monitoring**: Tracks application memory (`runtime.MemStats`), system-wide memory, and CPU usage (`gopsutil`), alongside goroutine counts.
- **Configurable Limits**: Define hard limits that trigger a shutdown and warning thresholds for proactive logging.
- **Graceful Shutdown**: Execute custom cleanup logic before termination via a simple callback mechanism.
- **Standalone & Lightweight**: Designed to be a simple, dependency-minimal library.
- **Flexible**: Can be configured to only log limit breaches without triggering a shutdown, allowing for custom handling.

## How It Works

The limiter runs a background goroutine that periodically checks resource usage.

- **Application Metrics**: It uses the standard `runtime` package to get Go-specific stats like heap allocation (`Alloc`) and goroutine count.
- **System Metrics**: It uses `gopsutil` to get system-wide metrics like total CPU and memory usage, providing a more holistic view of the machine's state.

When a configured "hard" limit is breached and `EnableAutoShutdown` is true, the limiter invokes the registered shutdown callback. When a "warning" threshold is breached, it logs a message, allowing you to observe trends before they become critical issues.

## Configuration

The limiter is configured via the `rslimiter.ResourceLimiterConfig` struct. Any fields left as zero-values will use sensible defaults.

| Field                | Type          | Description                                                                                                                              | Default |
| -------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| `MaxMemoryMB`        | `int64`       | The maximum application memory (heap allocation) in MB. Triggers a shutdown.                                                             | `0`     |
| `MaxGoroutines`      | `int`         | The maximum number of concurrent goroutines. Triggers a shutdown.                                                                        | `0`     |
| `CheckInterval`      | `time.Duration` | The frequency at which the limiter checks resource usage.                                                                                | `30s`   |
| `SystemMemThreshold` | `float64`     | System-wide memory usage percentage (0.0-1.0) that triggers a shutdown. `0.9` means 90%.                                                   | `0.9`   |
| `CPUThreshold`       | `float64`     | System-wide CPU usage percentage (0.0-1.0) that triggers a shutdown.                                                                     | `0.9`   |
| `EnableAutoShutdown` | `bool`        | If `true`, the shutdown callback is triggered on limit breach. If `false`, only an error is logged.                                       | `false` |
| `MemoryThreshold`    | `float64`     | Percentage of `MaxMemoryMB` (0.0-1.0) to use as a warning threshold. Logs a warning when breached.                                         | `0.8`   |
| `GoroutineWarning`   | `float64`     | Percentage of `MaxGoroutines` (0.0-1.0) to use as a warning threshold. Logs a warning when breached.                                       | `0.8`   |

## Advanced Usage

### Manual Resource Checks

You can fetch the current resource snapshot at any time by calling `rslimiter.GetResourceUsage()`.

```go
usage := rslimiter.GetResourceUsage()

log.Info().
    Int64("alloc_mb", usage.AllocMB).
    Int("goroutines", usage.Goroutines).
    Float64("cpu_percent", usage.CPUUsagePercent).
    Msg("Current resource snapshot")

// You can also manually perform checks against the limiter's configured thresholds.
// Note: This is handled automatically by the background monitoring loop.
if err := resourceLimiter.CheckMemoryLimit(); err != nil {
    log.Warn().Err(err).Msg("Manual check failed: memory limit exceeded")
}
```
This is useful for exporting metrics or for making decisions at critical points in your application's lifecycle.