# Orchestrator Package

`monsterinc/internal/orchestrator`

This package provides the `ScanOrchestrator` type, which is responsible for managing and executing the main scan workflow of the MonsterInc application. It centralizes the logic for crawling, HTTP/HTTPS probing, URL differencing, and Parquet data storage, making it reusable by different execution modes (e.g., `onetime` and `automated`).

## Responsibilities

-   Initializes and runs the web crawler based on provided seed URLs and configuration.
-   Takes the discovered URLs and performs HTTP/HTTPS probing using the `httpxrunner`.
-   Processes and normalizes probe results, assigning root target URLs.
-   Groups probe results by their respective root targets.
-   Performs URL differencing against historical data (read via `ParquetReader`) for each root target.
-   Writes the current scan's probe results to Parquet files (via `ParquetWriter`), organized by scan session and root target.

## Usage

The `ScanOrchestrator` is typically initialized with the global application configuration, a logger, and instances of `ParquetReader` and `ParquetWriter`.

```go
import (
    "monsterinc/internal/config"
    "monsterinc/internal/datastore"
    "monsterinc/internal/orchestrator"
    "log"
)

// Example Initialization:
cfg := &config.GlobalConfig{ /* ... */ }
logger := log.Default()
pqReader := datastore.NewParquetReader(&cfg.StorageConfig, logger)
pqWriter, _ := datastore.NewParquetWriter(&cfg.StorageConfig, logger)

orchestrator := orchestrator.NewScanOrchestrator(cfg, logger, pqReader, pqWriter)

// Example Execution:
seedURLs := []string{"http://example.com"}
scanSessionID := "20231027-120000" // A unique ID for the scan

probeResults, urlDiffResults, err := orchestrator.ExecuteScanWorkflow(seedURLs, scanSessionID)
if err != nil {
    logger.Fatalf("Scan workflow failed: %v", err)
}

// Process probeResults and urlDiffResults (e.g., for reporting)
```

The `ExecuteScanWorkflow` method returns the raw probe results and the URL diff results, which can then be used for report generation or other post-processing tasks. 