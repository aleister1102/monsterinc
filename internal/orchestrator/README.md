# Orchestrator Package (`internal/orchestrator`)

The `orchestrator` package is responsible for managing the overall scan workflow. It coordinates the actions of various modules like the crawler, HTTPX runner, differ, Parquet writer, and reporter.

## Key Responsibilities

-   **Workflow Execution**: Manages the sequence of operations for a scan, typically:
    1.  Crawling (optional, based on configuration and seed URLs).
    2.  HTTP/S Probing of discovered/provided URLs.
    3.  Diffing current scan results against historical data.
    4.  Storing probe results to Parquet files.
    5.  Generating an HTML report.
-   **Module Initialization**: Initializes and uses other internal modules (`crawler`, `httpxrunner`, `differ`, `datastore.ParquetReader`, `datastore.ParquetWriter`) by passing them the necessary configurations and logger.
-   **Data Flow Management**: Passes data between modules (e.g., discovered URLs from crawler to httpxrunner, probe results to differ and reporter).
-   **Configuration Handling**: Uses `config.GlobalConfig` to configure itself and other modules.
-   **Logging**: Integrates with the application's `zerolog` logger for detailed operational logging of the orchestration process.

## Initialization

The `ScanOrchestrator` is initialized using `NewScanOrchestrator(cfg *config.GlobalConfig, logger zerolog.Logger, pqReader *datastore.ParquetReader, pqWriter *datastore.ParquetWriter)`.

-   `cfg`: The global application configuration (`config.GlobalConfig`).
-   `logger`: A `zerolog.Logger` instance.
-   `pqReader`: An instance of `datastore.ParquetReader` for reading historical scan data (used by the differ).
-   `pqWriter`: An instance of `datastore.ParquetWriter` for writing current scan results.

## Main Workflow

The primary method is `ExecuteScanWorkflow(ctx context.Context, seedURLs []string, scanSessionID string) ([]models.ProbeResult, map[string]models.URLDiffResult, error)`.

-   `ctx`: A `context.Context` for managing cancellation and timeouts throughout the workflow.
-   `seedURLs`: A slice of initial URLs to start the scan. These might be used directly for probing or as seeds for the crawler.
-   `scanSessionID`: A unique identifier for the current scan session, used for logging and potentially for naming output files/directories.

### Steps in `ExecuteScanWorkflow`:

1.  **Crawler Phase (Optional)**:
    -   Determines the primary root target URL from seed URLs (used for organizing Parquet storage and diffing).
    -   If `CrawlerConfig.SeedURLs` are provided in the configuration (or if `seedURLs` to `ExecuteScanWorkflow` are to be used for crawling), it initializes and runs the `crawler.Crawler`.
    -   Collects all URLs discovered by the crawler.
    -   These discovered URLs are combined with the initial `seedURLs` (if any were not used for crawling) to form the list of targets for the HTTPX runner.

2.  **HTTPX Runner Phase**:
    -   Initializes the `httpxrunner.Runner` with the list of URLs to probe (from seeds and/or crawler output).
    -   Executes the HTTP/S probes.
    -   Collects `models.ProbeResult` for each successfully or unsuccessfully probed URL.
    -   Ensures `RootTargetURL` is set for each `ProbeResult`.

3.  **URL Differ Phase**:
    -   Initializes the `differ.UrlDiffer`.
    -   For each primary root target (typically one per `ExecuteScanWorkflow` call based on current logic), it calls `differ.Compare()` to compare the current `ProbeResult`s against historical data (read via `ParquetReader`).
    -   Collects `models.URLDiffResult` which contains new, old, and existing URLs.

4.  **Parquet Writer Phase**:
    -   If a `ParquetWriter` is available, it writes the collected `models.ProbeResult`s to a Parquet file. The path is typically structured using the `StorageConfig.ParquetBasePath`, the `primaryRootTargetURL`, and the `scanSessionID`.

5.  **Return Results**: The method returns the collected `probeResults`, `urlDiffResults`, and any error encountered during the workflow.

    *Note: HTML Report generation is typically handled by the caller of `ExecuteScanWorkflow` (e.g., in `cmd/monsterinc/main.go` or `internal/scheduler/scheduler.go`) using the results from this method.*

## Dependencies

-   `monsterinc/internal/config`: For global and module-specific configurations.
-   `monsterinc/internal/crawler`: For web crawling capabilities.
-   `monsterinc/internal/datastore`: For Parquet reading and writing.
-   `monsterinc/internal/differ`: For comparing scan results.
-   `monsterinc/internal/httpxrunner`: For HTTP/S probing.
-   `monsterinc/internal/models`: For data structures like `ProbeResult` and `URLDiffResult`.
-   `monsterinc/internal/urlhandler`: For URL utility functions.
-   `github.com/rs/zerolog`: For logging.
-   `net/http`: For `http.DefaultClient` passed to the crawler.

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