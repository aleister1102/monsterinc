# PRD: Periodic Crawl and Scan Feature

## 1. Introduction/Overview

This document outlines the requirements for the "Periodic Crawl and Scan" feature for MonsterInc. This feature will enable users to configure the application to automatically repeat crawling and scanning tasks at defined intervals, supporting the `automated` mode in the global configuration. The primary goal is to allow continuous monitoring of web targets over time to track changes and identify new potential vulnerabilities or information.

## 2. Goals

-   Enable users to schedule recurring crawl and scan operations based on a configurable interval (in days).
-   Automate the process of re-scanning targets, including reloading target lists from files, to identify changes (URL diffing) over time.
-   Provide a mechanism for continuous monitoring without manual intervention for each scan.
-   Integrate seamlessly with the existing `automated` mode configuration.
-   Store scan history (start time, end time, status) in an SQLite database.
-   Implement a configurable retry mechanism for failed scans within a cycle.
-   Notify users (e.g., Discord) upon scan completion or failure.

## 3. User Stories

-   As a security professional, I want to configure MonsterInc to re-scan my target websites every X days (e.g., 7 days), reloading the target list each time, so that I can stay updated on any new URLs or changes, and receive notifications on completion/failure.
-   As a bug bounty hunter, I want the tool to periodically check for new assets on my monitored targets by re-crawling and re-scanning them based on a configured schedule (interval in days), with scan history saved to a database, and retry failed scans a few times before waiting for the next cycle.
-   As an application administrator, I want to set up automated scans that run immediately on first activation and then every N days, to track changes on my web applications over time, focusing on URL differences, with reports named consistently (e.g., `YYYYMMDD-HHMMSS_automated_report.html`).

## 4. Functional Requirements

1.  **Scheduling Configuration (`SchedulerConfig`):
    1.1. A new configuration struct, `SchedulerConfig`, shall be added to `internal/config/config.go` and included in `GlobalConfig`.
    1.2. `SchedulerConfig` shall include:
        1.2.1. `ScanIntervalDays` (int): The number of days between scan cycles. (e.g., 7). Must be a positive integer.
        1.2.2. `RetryAttempts` (int): Number of times to retry a failed scan within a cycle before waiting for the next full interval (e.g., 2-3). Must be a non-negative integer.
        1.2.3. `SQLiteDBPath` (string): Path to the SQLite database file (e.g., `database/scheduler_history.db`). Defaults to a path within the `database` directory.
    1.3. Default values shall be provided for `SchedulerConfig`.
    1.4. `config.example.yaml` must be updated to include `SchedulerConfig` with explanations.
    1.5. Validation rules must be added for `ScanIntervalDays` (e.g., `min=1`) and `RetryAttempts` (e.g., `min=0`) in `internal/config/validator.go`.
2.  **Scheduling Logic (within a new `scheduler` package, refactored from `core`):
    2.1. When `GlobalConfig.Mode` is "automated":
        2.1.1. The application shall initialize the scheduling mechanism.
        2.1.2. On the first run in `automated` mode, a scan cycle shall be triggered immediately.
        2.1.3. After a scan cycle completes (successfully or after exhausting retries), the scheduler calculates the next scan time based on the *completion time* of the last attempt and `SchedulerConfig.ScanIntervalDays`.
        2.1.4. The main application process must remain running, sleeping/waiting until the next scheduled scan time.
3.  **Scan Cycle Execution:
    3.1. At the beginning of each scan cycle (initial or subsequent):
        3.1.1. Target URLs must be reloaded from the source specified in `InputConfig.InputFile` or the `-urlfile` command-line argument.
        3.1.2. The full crawl, probe, diff, and report generation process (similar to `onetime` mode) is executed.
    3.2. **Error Handling & Retries:**
        3.2.1. If a scan cycle fails, it will be retried up to `SchedulerConfig.RetryAttempts` times.
        3.2.2. A short, fixed delay should occur between retries (e.g., 5-10 minutes, not configurable initially).
        3.2.3. If all retries fail, the scheduler logs the failure and waits for the next scheduled `ScanIntervalDays`.
4.  **State & History Management (SQLite):
    4.1. A new SQLite database will be used, located at `SchedulerConfig.SQLiteDBPath`.
    4.2. A table (e.g., `scan_history`) will store records for each scan *attempt*:
        4.2.1. `id` (Primary Key, AutoIncrement)
        4.2.2. `scan_start_time` (TEXT/DATETIME)
        4.2.3. `scan_end_time` (TEXT/DATETIME, nullable if ongoing or crashed)
        4.2.4. `status` (TEXT: e.g., "STARTED", "COMPLETED_SUCCESS", "COMPLETED_FAILURE", "RETRYING")
        4.2.5. `target_source` (TEXT: path to the urlfile or "config_input_urls")
        4.2.6. `report_file_path` (TEXT, path to the generated HTML report, if successful)
        4.2.7. `log_summary` (TEXT, brief summary of errors if failed)
    4.3. The scheduler package will be responsible for database initialization (creating table if not exists) and all CRUD operations on `scan_history`.
    4.4. The timestamp of the last *successful* scan completion (or last attempt if all retries failed) is used to determine the next scan time.
5.  **Target Management (within `scheduler` package):
    5.1. The logic for loading URLs from `InputConfig.InputFile` or the `-urlfile` argument will be managed or invoked by the scheduler at the start of each cycle.
6.  **Diffing & Reporting:
    6.1. URL diffing shall be performed as in `onetime` mode for each scan cycle.
    6.2. Report filenames for `automated` mode will be `YYYYMMDD-HHMMSS_automated_report.html` (e.g., `20231027-153000_automated_report.html`). The `scanSessionID` in `main.go` can be adapted for this.
7.  **Notifications:
    7.1. Notifications (leveraging `NotificationConfig`) shall be sent when a scan cycle completes successfully or when all retry attempts for a cycle have failed.
8.  **Package Refactoring:**
    8.1. The existing `monsterinc/internal/core` package shall be refactored/renamed to `monsterinc/internal/scheduler`.
    8.2. This new `scheduler` package will house the primary logic for managing scheduled tasks, interacting with the SQLite DB, and orchestrating scan cycles in `automated` mode. `target_manager.go`'s responsibilities will be absorbed or adapted within this package.

## 5. Non-Goals (Out of Scope)

-   Complex cron-like scheduling expressions (only interval in days).
-   User interface for managing schedules (configuration will be via `config.yaml`).
-   Real-time, instant change detection (scans are periodic).
-   Detailed content diffing (only URL list diffing is in scope for this iteration).
-   Running multiple independent scheduled jobs for different target sets simultaneously with different schedules (initially, one global schedule for all targets defined in the active config when in `automated` mode).
-   Configurable delay between retries (will be fixed initially).

## 6. Design Considerations (Optional)

-   **SQLite Driver:** A Go SQLite driver like `github.com/mattn/go-sqlite3` will be needed.
-   **Process Management:** The main application process will run persistently in `automated` mode, primarily sleeping between scan cycles. Graceful shutdown (handling SIGINT, SIGTERM) is important to ensure the current state can be saved.
-   **Concurrency (Database):** SQLite typically handles concurrent writes by locking. If multiple MonsterInc instances were accidentally pointed to the same DB (not recommended), this could lead to contention. The design assumes a single MonsterInc scheduler process managing the DB.

## 7. Technical Considerations (Optional)

-   **Database Migrations:** For simplicity in the first iteration, schema changes might require manual DB adjustments or dropping/recreating the DB. Future iterations could consider a migration system.
-   **Time Zones:** Ensure all timestamps stored and used for calculations are consistent (preferably UTC).
-   **Resource Usage:** The application should be efficient with resources while in its sleep/wait state between scan cycles.

## 8. Success Metrics

-   The application, in `automated` mode, successfully executes scans at user-defined N-day intervals, starting immediately on first activation.
-   Scan history, including start/end times and status, is correctly recorded in the SQLite database.
-   Target lists are reloaded correctly at the start of each scan cycle.
-   The retry mechanism for failed scans functions as specified.
-   Notifications for scan completion/failure are delivered.
-   URL diffing and reporting work correctly for each scheduled scan.

## 9. Open Questions

-   If the `input_file` (or `-urlfile`) is not found or is empty at the start of a new cycle, should the cycle be skipped with a warning, or should it be treated as a failure prompting retries?

---
*This PRD is based on user feedback and requirements for the periodic scanning feature.* 