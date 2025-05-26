# Scheduler Package (`internal/scheduler`)

## Overview

The `scheduler` package is responsible for managing and orchestrating periodic (automated) scan operations within MonsterInc. When the application is run in `automated` mode, this package takes control of the execution flow after initial configuration loading and validation.

## Key Responsibilities

1.  **Task Scheduling:**
    *   Reads scheduling parameters from `SchedulerConfig` (e.g., `ScanIntervalDays`).
    *   Determines when scan cycles are due based on the last run time and the configured interval.
    *   Manages the main application loop in `automated` mode, putting the application to sleep between scan cycles.
    *   Triggers an initial scan immediately when `automated` mode starts for the first time or if a scan is due.

2.  **Scan Cycle Management:**
    *   Initiates and manages the lifecycle of each scan cycle.
    *   Reloads target URLs from the specified input file (`InputConfig.InputFile` or `-urlfile` argument) at the beginning of each cycle.
    *   Orchestrates the sequence of operations: crawling, HTTPX probing, URL diffing, and report generation by calling relevant modules.

3.  **State and History Persistence (SQLite):**
    *   Interacts with an SQLite database (defined by `SchedulerConfig.SQLiteDBPath`) to store scan history.
    *   Records details for each scan attempt: start time, end time, status (e.g., success, failure, retrying), target source, report file path, and error summaries.
    *   Initializes the database and required tables if they don't exist.
    *   Uses this history to determine when the next scan is due.

4.  **Error Handling and Retries:**
    *   Implements a retry mechanism for failed scan cycles, based on `SchedulerConfig.RetryAttempts`.
    *   Manages delays between retries.

5.  **Notifications:**
    *   Integrates with the `notifier` package (or directly with `NotificationConfig`) to send alerts upon successful completion of a scan cycle or after all retries for a cycle have failed.

## Core Components (Planned)

*   `scheduler.go`: Main logic for the scheduler, including the main loop, cycle management, and interaction with other components.
*   `db.go`: Handles all database interactions (SQLite), including schema creation, and CRUD operations for scan history.
*   `target_loader.go`: (May replace or absorb `target_manager.go`) Logic for loading and refreshing target lists for each scan cycle.

## Interactions

-   **`config`**: Reads `SchedulerConfig` and `GlobalConfig.Mode`.
-   **`crawler`, `httpxrunner`, `differ`, `reporter`**: Invokes these modules to perform the actual scanning and reporting tasks during a cycle.
-   **`logger`**: Uses the application logger for verbose output and error logging.
-   **`notifier`**: Triggers notifications based on scan outcomes.

## Future Considerations

-   More sophisticated scheduling options (e.g., cron expressions).
-   Ability to manage multiple independent schedules for different target sets. 