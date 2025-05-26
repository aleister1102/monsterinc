# Command: monsterinc

The `monsterinc` command is the main entry point for the MonsterInc application.

## Overview

It orchestrates the various modules of the application, including:
- Configuration loading
- Target input (from file or config)
- Web crawling (via `internal/crawler`)
- HTTP/HTTPS probing (via `internal/httpxrunner`)
- Data storage to Parquet files (via `internal/datastore`)
- URL diffing against historical data (via `internal/differ`)
- HTML report generation (via `internal/reporter`)

## Execution Flow

1.  **Initialization**:
    -   Prints a startup message.
    -   Parses command-line flags (`-urlfile`, `-globalconfig`, `-mode`).
    -   Loads the global configuration (`config.json` by default) into `config.GlobalConfig`.
    -   Initializes a logger (currently standard Go `log.Default()`).
    -   Initializes `datastore.ParquetReader` (for `UrlDiffer`).
    -   Initializes `datastore.ParquetWriter` (for storing scan results).

2.  **Target Acquisition & Crawler Module**:
    -   Determines seed URLs: Prioritizes URLs from the file specified by `-urlfile`. If not provided, uses `InputConfig.InputURLs` from the global config.
    -   Determines a `primaryRootTargetURL` for the scan session, typically the first seed URL. This is used for naming Parquet files and as the key for diffing.
    -   If seed URLs are available, initializes and starts the `crawler.Crawler`.
    -   Collects `discoveredURLs` from the crawler.

3.  **HTTPX Probing Module**:
    -   If `discoveredURLs` are available, initializes `httpxrunner.Runner` with these URLs and settings from `gCfg.HttpxRunnerConfig`.
    -   Runs the probes.
    -   Collects `models.ProbeResult` from the runner.
    -   Ensures all `discoveredURLs` have a corresponding `ProbeResult` in `finalResults`, creating fallback entries with errors for URLs that didn't yield a result from `httpxrunner`. The `RootTargetURL` field in each `ProbeResult` is set to the `primaryRootTargetURL`.

4.  **URL Diffing Module**:
    -   If `finalResults` are available and `primaryRootTargetURL` is valid, initializes `differ.UrlDiffer`.
    -   Calls `differ.Compare(finalResults, primaryRootTargetURL)` to get `models.URLDiffResult`.
    -   Stores the diff result in a map `urlDiffResults` keyed by `primaryRootTargetURL` (currently supports one primary target per run for diffing).

5.  **HTML Report Generation Module**:
    -   If `finalResults` are available, initializes `reporter.HtmlReporter`.
    -   Determines the output HTML file path based on `gCfg.Mode` (onetime/automated) and timestamp, or uses `ReporterConfig.DefaultOutputHTMLPath`.
    -   Calls `htmlReporter.GenerateReport(finalResults, urlDiffResults, outputPath)` to generate the report, passing both probe results and the diff results.

6.  **Parquet Storage Module**:
    -   If `parquetWriter` is initialized, `finalResults` are available, and `primaryRootTargetURL` is valid, calls `parquetWriter.Write(finalResults, scanSessionID, primaryRootTargetURL)` to store results.

7.  **Completion**:
    -   Logs a completion message.

## Command-Line Flags

-   **`-urlfile <path>` (alias `-u <path>`)**: Path to a text file containing seed URLs (one URL per line). Overrides seed URLs from the global config.
-   **`-globalconfig <path>`**: Path to the global JSON configuration file (default: `config.json`).
-   **`-mode <mode>`**: (Required) Mode to run the tool. Common values might be `onetime` or `automated`. This affects output file naming and potentially other behaviors.

## Configuration

Relies heavily on `config.json` for detailed configuration of all modules (crawler, httpxrunner, reporter, storage, etc.). See `internal/config/config.go` for all available options.

## Logging

Uses the standard Go `log` package for logging informational messages, warnings, errors, and fatal errors throughout its execution. 