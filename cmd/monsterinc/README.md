# Command: monsterinc

The `monsterinc` command is the main entry point for the MonsterInc application.

## Overview

`main.go` orchestrates the application's workflow based on the specified mode (`onetime` or `automated`). Key responsibilities include:

-   **Configuration Management**: Parses command-line flags, loads global configuration (from `config.yaml` or `config.json` via `internal/config`), and validates it.
-   **Logging Initialization**: Sets up the `zerolog` logger based on `LogConfig` (via `internal/logger`).
-   **Notification Setup**: Initializes the `DiscordNotifier` and `NotificationHelper` (via `internal/notifier`) for sending alerts.
-   **Signal Handling**: Listens for SIGINT and SIGTERM to enable graceful shutdown, sending an interruption notification if a scan is active.
-   **Mode Dispatching**:
    -   **Onetime Mode**: Executes `runOnetimeScan`.
    -   **Automated Mode**: Initializes and starts the `Scheduler` (from `internal/scheduler`).

## `runOnetimeScan` Workflow (for `--mode onetime`)

1.  **Initialization**:
    -   Initializes `datastore.ParquetReader`.
    -   Initializes `datastore.ParquetWriter`, providing it with the `ParquetReader` instance (this is crucial for the new merge logic where the writer needs to read existing data).
    -   Initializes `orchestrator.ScanOrchestrator` with the reader and writer.
2.  **Target Acquisition**:
    -   Determines seed URLs: Prioritizes `-urlfile`, then `input_config.input_urls` from config.
    -   Identifies `onetimeTargetSource` (e.g., filename or "config_input_urls").
    -   Sends a "Scan Start" notification via `NotificationHelper`, including `TargetSource`.
3.  **Scan Execution**:
    -   Generates a `scanSessionID` (timestamp, e.g., `YYYYMMDD-HHMMSS`).
    -   Calls `ScanOrchestrator.ExecuteScanWorkflow` with the seed URLs and `scanSessionID`.
    -   This orchestrator call internally manages: Crawling -> HTTPX Probing -> Diffing (against historical Parquet data read via `ParquetReader` and `FindAllProbeResultsForTarget`) -> Storing new/updated probe results by merging with historical data and overwriting the single `data.parquet` file for the target (via `ParquetWriter`).
4.  **Result Processing & Reporting**:
    -   Populates `models.ScanSummaryData` with results (probe stats, diff stats), `ScanSessionID`, and `TargetSource`.
    -   If an error occurred in the workflow, sends a "Scan Failed" notification and exits.
    -   Initializes `reporter.HtmlReporter`.
    -   Generates an HTML report (e.g., `reports/YYYYMMDD-HHMMSS_onetime_report.html`).
    -   If report generation fails, updates summary and sends a "Scan Failed" (or partial) notification.
5.  **Completion Notification**: Sends a "Scan Completed" notification via `NotificationHelper` with the final `ScanSummaryData` (including report path if successful).

## `automated` Mode Workflow

1.  Initializes `scheduler.Scheduler` with global config, logger, notification helper. The `Scheduler` itself initializes its own `ParquetReader` and `ParquetWriter` (passing the reader to the writer) for use by its internal `ScanOrchestrator`.
2.  Calls `scheduler.Start(ctx)`, which blocks until the scheduler is stopped (e.g., by OS signal).
3.  The `Scheduler` internally manages:
    -   Calculating next scan times based on `SchedulerConfig.CycleMinutes` and scan history from an SQLite DB.
    -   Loading/refreshing targets for each cycle via `TargetManager`.
    -   Calling `ScanOrchestrator.ExecuteScanWorkflow` for each scan cycle, which follows the same data handling logic as in "onetime" mode (reading historical, merging, overwriting single Parquet file per target).
    -   Managing scan history (start, end, status, diffs, report path) in SQLite.
    -   Handling retries for failed scan cycles (`SchedulerConfig.RetryAttempts`).
    -   Sending "Scan Start" and "Scan Completion" (success/failure) notifications for each cycle via `NotificationHelper`.

## Command-Line Flags

-   **`--mode <onetime|automated>`**: (Required) Execution mode.
-   **`-u <path/to/urls.txt>` or `--urlfile <path/to/urls.txt>`**: (Optional) Path to a text file with seed URLs. Used by `onetime` mode and as the initial/configurable source for `automated` mode.
-   **`--globalconfig <path/to/config.yaml>`**: (Optional) Path to configuration. Defaults to `config.yaml`, then `config.json` etc. (see `internal/config/loader.go`).

## Configuration

Relies on `config.yaml` (preferred) or `config.json`. See `internal/config/README.md` and `config.example.yaml`.

## Logging & Notifications

-   Uses `zerolog` for structured logging (see `internal/logger/README.md`).
-   Uses `internal/notifier` package for Discord notifications for scan lifecycle events and critical errors. `ScanSummaryData` (with `ScanSessionID` and `TargetSource`) is central to formatting these notifications.

## Key Data Structures for Notifications

-   `models.ScanSummaryData`:
    -   `ScanSessionID`: Timestamped ID of the specific scan run (e.g., `20230101-123000`).
    -   `TargetSource`: Origin of targets (e.g., `urls.txt`, `config_input_urls`).
    -   Other fields: `Targets`, `TotalTargets`, `ProbeStats`, `DiffStats`, `ScanDuration`, `ReportPath`, `Status`, `ErrorMessages`. 