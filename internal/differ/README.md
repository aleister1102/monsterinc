# Package differ

This package is responsible for comparing current scan results with historical data to identify changes (new, old, existing URLs).

## Components

### `UrlDiffer`

- **`NewUrlDiffer(pr *datastore.ParquetReader, logger *log.Logger) *UrlDiffer`**: Constructor for `UrlDiffer`. It takes a `datastore.ParquetReader` to fetch historical data and a `log.Logger`.
- **`Compare(currentScanResults []models.ProbeResult, rootTargetURL string) (*models.URLDiffResult, error)`**: This is the main method that performs the comparison.
    1. It calls `parquetReader.FindMostRecentScanURLs(rootTargetURL)` to get a list of URLs from the most recent previous scan for the given `rootTargetURL`.
    2. If `FindMostRecentScanURLs` returns an error or an empty list (no historical data), it logs this and treats the current scan as a "first scan" (all current URLs will likely be marked as new or existing based on context, typically new).
    3. It uses maps for efficient comparison of current URLs (from `currentScanResults`, normalized using `FinalURL` or `InputURL` as fallback) against the historical URLs.
    4. It populates a `models.URLDiffResult` struct with:
        - `RootTargetURL`: The target URL for which the diff was performed.
        - `Results`: A slice of `models.DiffedURL` structs, where each struct contains:
            - `NormalizedURL`: The URL being reported.
            - `Status`: `models.StatusNew`, `models.StatusOld`, or `models.StatusExisting`.
            - `LastSeenData`: For `StatusNew` and `StatusExisting` URLs, this is the `models.ProbeResult` from the `currentScanResults`. For `StatusOld` URLs, this field might be minimally populated or require further enhancement if detailed historical data is needed for old URLs in the report.
    5. Logs a summary of the diff operation (counts of new, old, existing URLs).

## Data Models

- Uses `models.ProbeResult` to represent current scan data.
- Produces `models.URLDiffResult` which contains `models.DiffedURL` items, each with a `models.URLStatus`.

## Dependencies

- `internal/datastore`: To read historical scan data via `ParquetReader`.
- `internal/models`: For data structures like `ProbeResult`, `URLDiffResult`, `DiffedURL`, and `URLStatus`.

## Logging

- `UrlDiffer` uses a `log.Logger` for its operations, including the start and completion of diffing, errors encountered while fetching historical data, and counts of different URL statuses. 