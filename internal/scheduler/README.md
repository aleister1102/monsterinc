# Scheduler Package (`internal/scheduler`)

## Overview

The `scheduler` package is responsible for managing and orchestrating periodic (automated) scan operations within MonsterInc. When the application is run in `automated` mode, this package takes control of the execution flow after initial configuration loading and validation. It ensures that scans are performed at regular intervals, retries are attempted upon failure, and the history of scans is maintained.

## Key Responsibilities

1.  **Task Scheduling & Main Loop:**
    *   Reads scheduling parameters from `SchedulerConfig` (e.g., `CycleMinutes`).
    *   Manages the main application loop in `automated` mode, sleeping between scan cycles.
    *   Calculates the `nextScanTime` based on the last completed scan's end time and the configured interval.
    *   Triggers an initial scan immediately if no previous scan history exists or if a scan is due.
    *   Handles graceful shutdown via context cancellation signals from `main.go`.

2.  **Scan Cycle Management:**
    *   Initiates and manages the lifecycle of each scan cycle, identified by a `scanSessionID` (YYYYMMDD-HHMMSS format).
    *   Utilizes `TargetManager` to load and normalize target URLs at the beginning of each cycle. Target sources can be a file specified via command-line (`-urlfile`), a file path from `InputConfig.InputFile`, or a list from `InputConfig.InputURLs`.
    *   Invokes `ScanOrchestrator` to execute the complete scan workflow (Crawl -> HTTPX Probe -> Diff -> Parquet Storage).
    *   Generates an HTML report for each scan cycle with a filename like `YYYYMMDD-HHMMSS_automated_report.html`.

3.  **State and History Persistence (SQLite):**
    *   Uses `db.go` to interact with an SQLite database (path defined in `SchedulerConfig.SQLiteDBPath`).
    *   `db.go` handles schema initialization (`scan_history` table) and CRUD operations.
    *   Records details for each scan attempt: `scanSessionID`, start time, end time, status (e.g., STARTED, COMPLETED, FAILED), target source, number of targets, diff statistics (new, old, existing), report file path, and error summaries.
    *   `GetLastScanTime()` is used to determine when the next scan is due.

4.  **Error Handling and Retries:**
    *   Implements a retry mechanism for failed scan cycles via `runScanCycleWithRetries`.
    *   Number of retries is configured by `SchedulerConfig.RetryAttempts`.
    *   A fixed delay (currently 5 minutes) is applied between retry attempts. This delay is interruptible by context cancellation.
    *   Logs detailed error information and updates the scan history in the database with the final status after all retries.

5.  **Notifications:**
    *   Integrates with the `notifier.NotificationHelper` (passed during initialization).
    *   Sends notifications for scan start and scan completion (both success and final failure after retries).
    *   Notifications include key details like scan ID, target summary, duration, status, and a link to the report if generated.

## Core Components

1.  **`scheduler.go`**:
    *   Defines the `Scheduler` struct, which is the main engine for automated scanning.
    *   `NewScheduler(...)`: Initializes the scheduler. This involves:
        *   Setting up a SQLite database (`db.go`) for scan history tracking.
        *   Initializing a `TargetManager` (`target_manager.go`) to handle loading and selection of scan targets for each cycle.
        *   Initializing `datastore.ParquetReader` and `datastore.ParquetWriter`. Note: `NewParquetWriter` is called without a `ParquetReader` argument, aligning with a simpler writer that overwrites data rather than merging internally.
        *   Initializing a `ScanOrchestrator` which will be used to execute the scan workflow for each cycle.
    *   `Start(ctx context.Context)`: Begins the main scheduling loop. It calculates the time for the next scan based on `SchedulerConfig.CycleMinutes` and the last scan time from the database. It then waits until the next scan time and invokes `runScanCycleWithRetries`.
    *   `Stop()`: Gracefully stops the scheduler loop and cleans up resources, including closing the database connection.
    *   `runScanCycleWithRetries(ctx context.Context)`: Manages the execution of a single scan cycle with retry logic based on `SchedulerConfig.RetryAttempts`.
    *   `runScanCycle(ctx context.Context, scanSessionID string, predeterminedTargetSource string)`: Executes a full scan workflow for one cycle:
        1.  Loads targets using `TargetManager`.
        2.  Sends a "Scan Start" notification using `NotificationHelper`.
        3.  Records the scan start in the SQLite database.
        4.  Calls `ScanOrchestrator.ExecuteScanWorkflow` with the seed URLs and `scanSessionID`. The orchestrator handles probing, diffing (comparing against the previous scan's data in the target's `data.parquet` file), and writing results (overwriting the target's `data.parquet` file with current results).
        5.  Generates an HTML report via `reporter.NewHtmlReporter().GenerateReport()`.
        6.  Updates the scan record in the SQLite database with completion status, report path, and diff statistics.
        7.  Sends a "Scan Completion" notification.

2.  **`db.go`**:
    *   Manages the SQLite database connection and schema.
    *   Stores scan history: start time, end time, status, target source, report path, and a summary of diff results (new, old, existing counts).
    *   `InitSchema()`: Creates the necessary `scan_history` table if it doesn't exist.
    *   `RecordScanStart(...)`, `UpdateScanCompletion(...)`: Methods to manage scan records.
    *   `GetLastScanTime()`: Retrieves the end time of the most recently completed scan to help schedule the next one.

3.  **`target_manager.go`**:
    *   `NewTargetManager(...)`: Initializes the target manager.
    *   `LoadAndSelectTargets(...)`: Determines the list of target URLs to be scanned in a cycle. It prioritizes targets from the command-line `-urlfile` option, then `input_config.input_urls`, and finally `input_config.input_file` from the global configuration.
    *   Returns the selected targets (`[]models.Target`) and the source name (e.g., filename, "config_input_urls").

## Interactions

-   **`config`**: Reads `GlobalConfig`, specifically `SchedulerConfig`, `InputConfig`, `ReporterConfig`, and `NotificationConfig`.
-   **`orchestrator.ScanOrchestrator`**: Invoked by `runScanCycle` to perform the actual scanning and data processing tasks.
-   **`datastore`**: The `ScanOrchestrator` uses `ParquetReader` and `ParquetWriter` (initialized by `NewScheduler`) for diffing and storing probe results.
-   **`reporter`**: Used by `runScanCycle` (via `generateReport` helper) to create HTML reports.
-   **`logger`**: Uses the application-wide `zerolog.Logger` instance, creating module-specific contexts for its logs.
-   **`notifier.NotificationHelper`**: Used to send notifications about scan lifecycle events.
-   **`cmd/monsterinc/main.go`**: Initializes and starts the `Scheduler` when the application is run in `automated` mode. Handles context cancellation for graceful shutdown.

## Database Schema (`scan_history` table)

(Generated by `db.go` -> `InitSchema()`)

| Column             | Type      | Constraints        | Description                                      |
| ------------------ | --------- | ------------------ | ------------------------------------------------ |
| `id`                 | INTEGER   | PRIMARY KEY AUTOINCR | Unique ID for the scan history entry             |
| `scan_session_id`  | TEXT      | NOT NULL           | Unique ID for the scan session (e.g., YYYYMMDD-HHMMSS) |
| `target_source`    | TEXT      |                    | Source of targets (e.g., file path, config)      |
| `num_targets`      | INTEGER   |                    | Number of targets for this scan                  |
| `scan_start_time`  | TIMESTAMP | NOT NULL           | Time when the scan cycle started                 |
| `scan_end_time`    | TIMESTAMP |                    | Time when the scan cycle ended (or last attempt) |
| `status`             | TEXT      | NOT NULL           | e.g., STARTED, COMPLETED, FAILED, RETRYING       |
| `report_file_path` | TEXT      |                    | Path to the generated HTML report (if any)       |
| `log_summary`        | TEXT      |                    | Summary of errors or key events                  |
| `diff_new`         | INTEGER   |                    | Count of new URLs found                          |
| `diff_old`         | INTEGER   |                    | Count of URLs that disappeared                   |
| `diff_existing`    | INTEGER   |                    | Count of URLs that remained unchanged            |

## Future Considerations

-   More sophisticated scheduling options (e.g., cron-like expressions).
-   Ability to manage multiple independent schedules for different target sets if the application evolves to support that.
-   Dynamic adjustment of retry delays or strategies.

## Workflow in Automated Mode

1.  `main.go` initializes and starts the `Scheduler`.
2.  The `Scheduler` enters a loop, waiting for the next scheduled scan time.
3.  When it's time to scan:
    *   `TargetManager` provides the list of URLs.
    *   A unique `scanSessionID` (timestamp) is generated.
    *   `NotificationHelper` sends a "Scan Started" notification.
    *   `ScanOrchestrator.ExecuteScanWorkflow` is called.
        *   The orchestrator runs the full scan (crawling, probing).
        *   `UrlDiffer` compares current results with the data in `database/<target>/data.parquet` (which contains results from the *previous* scan cycle for that target).
        *   `ParquetWriter` overwrites `database/<target>/data.parquet` with the *current* scan's results.
    *   An HTML report is generated.
    *   Scan details (including diff counts) are saved to `scheduler_history.db`.
    *   `NotificationHelper` sends a "Scan Completed" notification.
4.  The cycle repeats.

## Configuration

Relies on `SchedulerConfig` from the global configuration:
-   `CycleMinutes`: Interval between scans.
-   `RetryAttempts`: How many times to retry a failed scan cycle.
-   `SQLiteDBPath`: Path to the SQLite database file (e.g., `database/scheduler_history.db`).

Also uses `InputConfig` (for target sources) and other configurations indirectly via the `ScanOrchestrator`. 