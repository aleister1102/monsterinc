# MonsterInc Progress Display

This library provides a simple progress display manager for long-running tasks in Go applications. It can display progress for scans and monitoring tasks, including percentage complete, status icons, and ETA.

## Features

- **Dual Progress Tracking**: Track progress for two separate tasks (e.g., "scan" and "monitor") simultaneously.
- **Batch Progress**: Display progress for batch-oriented tasks.
- **ETA Estimation**: Provides a simple ETA estimation.
- **Configurable Display**: Control the display interval and enable/disable progress display.
- **Independent Module**: Designed as a standalone library.

## Installation

```bash
go get github.com/monsterinc/progress
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/monsterinc/progress"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(zerolog.NewConsoleWriter())
	
	config := &progress.ProgressDisplayConfig{
		DisplayInterval: 2 * time.Second,
		EnableProgress:  true,
	}

	pdm := progress.NewProgressDisplayManager(logger, config)
	pdm.Start()
	defer pdm.Stop()

	fmt.Println("Simulating a scan...")
	totalItems := int64(100)
	for i := int64(1); i <= totalItems; i++ {
		pdm.UpdateScanProgress(i, totalItems, "Processing", fmt.Sprintf("Item %d", i))
		time.Sleep(100 * time.Millisecond)
	}
	pdm.SetScanStatus(progress.ProgressStatusComplete, "Scan finished")

	time.Sleep(3 * time.Second) // Wait for final display
} 