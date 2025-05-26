# Differ Package (`internal/differ`)

This package is responsible for comparing current scan results against historical data to identify new, existing, and old URLs.

## Core Component

-   **`url_differ.go`**: Defines the `UrlDiffer` struct and its methods.
    -   `NewUrlDiffer(pr *datastore.ParquetReader, logger zerolog.Logger) *UrlDiffer`: Constructor that takes a `datastore.ParquetReader` (to access historical data) and a logger.
    -   `Compare(currentScanProbes []*models.ProbeResult, rootTarget string) (*models.URLDiffResult, error)`: This is the main method for performing the comparison.
        1.  It calls `ud.parquetReader.FindAllProbeResultsForTarget(rootTarget)` to fetch all historical `models.ProbeResult` records from the single `data.parquet` file associated with the `rootTarget`.
        2.  It compares the `currentScanProbes` with the `historicalProbes` based on `InputURL`.
        3.  It populates a `models.URLDiffResult` struct with:
            *   `RootTargetURL`: The target being diffed.
            *   `Results`: A slice of `models.DiffedURL` objects. Each `DiffedURL` contains a `ProbeResult` which includes a `URLStatus` field set to `models.StatusNew`, `models.StatusExisting`, or `models.StatusOld`.
            *   Counts for `New`, `Old`, and `Existing` URLs.
            *   An `Error` field if any issues occurred during the diffing process (e.g., failure to read historical data).
        4.  The `ProbeResult` within `DiffedURL` for "old" URLs will be the data from the historical scan. For "new" and "existing" URLs, it will be the data from the `currentScanProbes` (with `URLStatus` and potentially `OldestScanTimestamp` updated).

## Logic Overview

1.  **Fetch Historical Data**: The `UrlDiffer` uses the `ParquetReader` to load all probe results from the specific target's `data.parquet` file. This file contains the consolidated history for that target.
2.  **Map Creation**: For efficient lookup, both current and historical probe results are temporarily placed into maps keyed by their `InputURL`.
3.  **Comparison & Status Assignment**:
    *   **Existing URLs**: If a URL from the current scan is found in the historical data map, its status is marked as `models.StatusExisting`.
    *   **New URLs**: If a URL from the current scan is *not* found in the historical data map, its status is marked as `models.StatusNew`.
    *   **Old URLs**: If a URL from the historical data is *not* found in the current scan map, its status is marked as `models.StatusOld`. The historical data for this URL is preserved in the diff result.
4.  **Result Aggregation**: The `URLDiffResult` structure is populated with all diffed URLs and summary counts.

## Dependencies

-   `internal/datastore`: Specifically, `ParquetReader` to fetch historical scan data.
-   `internal/models`: For data structures like `ProbeResult`, `URLDiffResult`, `DiffedURL`, and status constants (`StatusNew`, `StatusOld`, `StatusExisting`).
-   `github.com/rs/zerolog`: For logging.

## Usage Example (Conceptual)

```go
// Assuming pqReader is an initialized *datastore.ParquetReader
// and logger is an initialized zerolog.Logger
differ := differ.NewUrlDiffer(pqReader, logger)

// currentScanResults would come from the orchestrator after a scan
currentScanResults := []*models.ProbeResult{
    // ... populated probe results ...
}
rootTarget := "example.com"

diffReport, err := differ.Compare(currentScanResults, rootTarget)
if err != nil {
    // Handle error
    logger.Error().Err(err).Msg("Failed to compare URL results")
    return
}

logger.Info().Int("new_urls", diffReport.New).Int("old_urls", diffReport.Old).Msg("Diff complete")
// Process diffReport.Results which contains all DiffedURL entries
``` 