# MonsterInc Resource Limiter

This library provides a resource limiter to monitor and control memory, CPU, and goroutine usage in Go applications. It's designed to help prevent resource exhaustion and can trigger graceful shutdowns when limits are exceeded.

## Features

- **Monitor Memory, CPU, and Goroutines**: Keep track of key application resources.
- **Configurable Limits**: Set limits for memory usage, goroutine count, and CPU usage.
- **Graceful Shutdown**: Trigger a callback function to gracefully shut down your application when limits are breached.
- **Independent Module**: Designed as a standalone library.

## Installation

```bash
go get github.com/monsterinc/limiter
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/monsterinc/limiter"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.Nop()

	// Configure the resource limiter
	config := limiter.ResourceLimiterConfig{
		MaxMemoryMB:        512,  // 512 MB
		MaxGoroutines:      1000,
		CheckInterval:      15 * time.Second,
		SystemMemThreshold: 0.8, // 80%
		CPUThreshold:       0.9, // 90%
		EnableAutoShutdown: true,
	}

	resourceLimiter := limiter.NewResourceLimiter(config, logger)

	// Set a shutdown callback
	resourceLimiter.SetShutdownCallback(func() {
		fmt.Println("Shutdown callback triggered due to resource limits!")
		// Perform graceful shutdown logic here
	})

	// Start the limiter
	resourceLimiter.Start()
	defer resourceLimiter.Stop()

	fmt.Println("Resource limiter is running. Application logic would go here.")

	// Example: simulate high resource usage
	// In a real application, this would be your actual workload.
	time.Sleep(60 * time.Second)
} 