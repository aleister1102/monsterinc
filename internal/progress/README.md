# Odometer

Odometer is a flexible Go library for tracking and displaying the progress of long-running tasks. It's designed to provide meaningful, visual feedback for operations like data processing, downloads, or any time-consuming task, with support for concurrent multi-task tracking.

This library is built on top of [zerolog](https://github.com/rs/zerolog) for efficient logging.

## Features

- **Flexible Progress Tracking**: Track single tasks or multiple concurrent tasks with distinct progress types (e.g., `Scan`, `Monitor`).
- **Batch Tracking**: Supports tasks that are split into batches, showing progress for both the current batch and the overall process.
- **ETA Estimation**: Automatically calculates and displays the Estimated Time of Arrival (ETA) for tasks.
- **Customizable Status**: Set custom statuses (e.g., `Running`, `Complete`, `Error`) and messages for each task.
- **Automated Display**: The `ProgressDisplayManager` handles displaying progress automatically at a configurable interval.
- **Structured Output**: Uses icons and progress bars to provide a clear and concise overview.

## Installation

```bash
go get github.com/aleister1102/go-odometer
```

## Usage

### Core Concepts

- **`Progress`**: Represents a single unit of progress. It holds information about status, current/total values, ETA, and more. You can interact with this object directly.
- **`ProgressDisplayManager`**: A higher-level manager that contains specialized `Progress` objects (`ScanProgress` and `MonitorProgress`). It automatically displays their status periodically, making integration into applications straightforward.

### Example 1: Tracking with `ProgressDisplayManager`

This is the most common approach. The `ProgressDisplayManager` handles the display loop for you.

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aleister1102/go-odometer"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	
	pdm := odometer.NewProgressDisplayManager(
		context.Background(),
		logger,
		&odometer.ProgressDisplayConfig{
			DisplayInterval:   500 * time.Millisecond,
			EnableProgress:    true,
			ShowETAEstimation: true,
		},
	)
	pdm.Start()
	defer pdm.Stop()

	// --- Simulate a "Scan" task ---
	fmt.Println("Starting scan simulation...")
	scanTotal := int64(150)
	for i := int64(1); i <= scanTotal; i++ {
		pdm.UpdateScanProgress(i, scanTotal, "Scanning files", "")
		time.Sleep(20 * time.Millisecond)
	}
	pdm.SetScanStatus(odometer.ProgressStatusComplete, "Scan complete")

	// --- Simulate a "Monitor" task with batches ---
	fmt.Println("\nStarting batched monitor simulation...")
	totalBatches := 5
	itemsPerBatch := 30
	for b := 1; b <= totalBatches; b++ {
		// Reset progress for the new batch
		pdm.ResetMonitorBatch(b, totalBatches, "Processing batch", "")
		for i := 1; i <= itemsPerBatch; i++ {
			// Update progress for the item within the batch
			pdm.UpdateMonitorWorkflow(int64(i), int64(itemsPerBatch), "", fmt.Sprintf("Item %d", i))
			time.Sleep(10 * time.Millisecond)
		}
	}
	pdm.SetMonitorStatus(odometer.ProgressStatusComplete, "All batches monitored")

	time.Sleep(1 * time.Second)
	fmt.Println("\nDone.")
}
```

### Example 2: Using a Standalone `Progress`

You can also manage a `Progress` object manually if you just need to track data without automated display.

```go
package main

import (
	"fmt"
	"time"
	"github.com/aleister1102/go-odometer"
)

func main() {
	p := odometer.NewProgress(odometer.ProgressTypeScan)

	totalItems := int64(100)
	p.Update(0, totalItems, "Starting", "")

	for i := int64(1); i <= totalItems; i++ {
		p.Update(i, totalItems, "Processing", fmt.Sprintf("Item #%d", i))
		info := p.Info() // Get a copy of the progress data
		
		// Display it yourself however you want
		fmt.Printf(
			"\rProgress: %.1f%% (%d/%d), ETA: %s", 
			info.GetPercentage(), 
			info.Current, 
			info.Total, 
			info.EstimatedETA.Round(time.Second),
		)
		time.Sleep(50 * time.Millisecond)
	}
	
	p.SetStatus(odometer.ProgressStatusComplete, "Finished!")
	fmt.Printf("\nFinal status: %s\n", p.Info().Message)
}
```

## Data Structures

The core object is `ProgressInfo`, which contains all the state data for a task:

- `Type`: `ProgressTypeScan` or `ProgressTypeMonitor`.
- `Status`: `Idle`, `Running`, `Complete`, `Error`, `Cancelled`.
- `Current`, `Total`: The progress values.
- `Stage`, `Message`: Textual descriptions for the current state.
- `StartTime`, `LastUpdateTime`: Timestamps for tracking.
- `EstimatedETA`: A `time.Duration` for the estimated completion time.
- `BatchInfo`: A pointer to `BatchProgressInfo` if the task is batched.
- `MonitorInfo`: A pointer to `MonitorProgressInfo` for monitor-specific data.

## License

This project is licensed under the MIT License. 