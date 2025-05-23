# Product Requirements Document: Logging Framework

## 1. Introduction/Overview

This document specifies the requirements for the Logging Framework feature of Project MonsterInc. A comprehensive logging mechanism is essential for debugging, monitoring application behavior, auditing actions, and diagnosing issues. This framework will provide a standardized way to record events and messages from all parts of the application.

The goal is to implement a configurable, flexible, and robust logging framework that supports multiple log levels, structured log messages, and output to both console and files, with considerations for different operational modes (continuous monitoring vs. one-time scans) and log rotation.

## 2. Goals

*   Provide a centralized logging facility for the entire application.
*   Support standard log levels (DEBUG, INFO, WARN, ERROR, FATAL).
*   Produce structured and informative log messages.
*   Enable logging to both console (with colorized output) and text files (plain text).
*   Allow per-module and global log level configuration via the main JSON configuration file.
*   Implement log rotation for file-based logs to manage disk space.
*   Tailor log file naming conventions based on the operational mode of the tool (monitor vs. scan).

## 3. User Stories

*   **As a Developer, I want to be able to emit DEBUG messages from my module while developing, and see them clearly (e.g., color-coded) on the console without affecting the log level of other modules, so I can effectively troubleshoot my code.**
*   **As an Operator running the tool in continuous monitoring mode, I want logs to be written to daily files (e.g., `app_YYYY-MM-DD.log`) with automatic log rotation, so I can review historical activity and manage disk space.**
*   **As an Analyst running a one-time scan, I want the scan logs to be saved to a uniquely named file based on the scan's start timestamp (e.g., `scan_YYYYMMDD_HHMMSS.log`), so I can easily associate logs with specific scan sessions.**
*   **As a System Administrator, I want critical ERROR and FATAL messages to be clearly distinguishable in both console and file logs, including timestamps, module names, and detailed messages, to quickly identify and address critical failures.**

## 4. Functional Requirements

1.  The logging framework **must** be usable by all modules and components within the application.
2.  The framework **must** support the following log levels, in increasing order of severity:
    *   DEBUG
    *   INFO
    *   WARN
    *   ERROR
    *   FATAL (A FATAL log message implies the application may be in an unstable state and might terminate after logging).
3.  Each log message **must** include at least the following structured information:
    *   Timestamp (ISO 8601 format preferred, e.g., `YYYY-MM-DDTHH:MM:SS.sssZ`).
    *   Log Level (e.g., "INFO", "ERROR").
    *   Module/Component Name (source of the log message).
    *   The log message string.
    *   Request/Task ID (if applicable and available in the current context).
4.  **Log Output - Console:**
    *   The framework **must** support logging to standard output (`stdout`) and standard error (`stderr`) (e.g., INFO/DEBUG to `stdout`, WARN/ERROR/FATAL to `stderr`).
    *   Console output **should** be color-coded based on log level for improved readability (e.g., DEBUG: gray, INFO: green, WARN: yellow, ERROR: red, FATAL: bold red).
5.  **Log Output - File:**
    *   The framework **must** support logging to text files.
    *   Log messages written to files **must not** contain color codes.
    *   File naming convention **must** depend on the operational mode:
        *   **Monitor Mode (continuous operation):** Logs **must** be written to files named by date (e.g., `projectmonsterinc_YYYY-MM-DD.log`).
        *   **Scan Mode (one-time execution):** Logs **must** be written to a file named with a timestamp of the scan's initiation (e.g., `projectmonsterinc_scan_YYYYMMDD_HHMMSS.log`).
    *   The base directory for log files **should** be configurable.
6.  **Log Configuration:**
    *   The global default log level **must** be configurable via the main JSON configuration file (`configuration-management` feature).
    *   The log level for individual modules/components **should** also be configurable via the JSON configuration file, allowing for more granular control (e.g., `logging.levels.module_name: DEBUG`).
    *   Configuration for log file paths and rotation settings **must** be managed through the JSON configuration file.
7.  **Log Rotation:**
    *   The framework **must** implement log rotation for file logs to prevent excessive disk usage.
    *   Rotation **should** be configurable based on:
        *   Maximum file size (e.g., rotate when file exceeds 100MB).
        *   Time interval (especially for monitor mode, e.g., create a new file daily - already covered by naming convention, but also consider keeping N recent files).
        *   Number of backup files to keep.
8.  If an error occurs during the logging process itself (e.g., unable to write to a log file due to permissions or disk space), the framework **should** attempt to report this error to `stderr` (if possible) and continue application execution without crashing the main application.

## 5. Non-Goals (Out of Scope)

*   Direct integration with external centralized logging systems (e.g., ELK, Splunk) in this version. The framework should produce logs in a format that *could* be ingested by such systems.
*   A dedicated UI for viewing or searching logs.
*   Remote log streaming capabilities.
*   Logging to Parquet files (as per user specification: "không cần ghi vào file parquet").

## 6. Design Considerations (Optional)

*   Choose a well-established Go logging library (e.g., `logrus`, `zap`, `zerolog`) as the foundation to avoid reinventing the wheel, and then customize its output and hooks as needed.
*   Ensure the logging format is parseable for potential future ingestion into log analysis tools.
*   Logging calls should have minimal performance impact on the application, especially for disabled log levels.

## 7. Technical Considerations (Optional)

*   Logging configuration (levels, output paths) should be applied early in the application startup sequence.
*   Ensure thread safety for all logging operations, as multiple goroutines will likely be writing logs concurrently.
*   For file logging, consider asynchronous logging or buffered I/O to reduce performance overhead, while ensuring logs are flushed appropriately on error/exit.

## 8. Success Metrics

*   Log messages are generated accurately, with correct timestamps, levels, and content, to the configured outputs (console and files).
*   Log levels and output destinations are correctly controlled by the JSON configuration.
*   Colorized console output functions as expected and improves readability.
*   File log naming conventions for different modes (monitor, scan) are implemented correctly.
*   Log rotation functions as specified, preventing uncontrolled disk space consumption.
*   The logging framework is easily adoptable by developers across all application modules.
*   Performance overhead of logging is within acceptable limits.

## 9. Open Questions

*   What are the specific default settings for log rotation (max size, max age, number of backups)?
*   Should there be a distinct log file for audit-type logs if needed in the future, or will all logs go to the same files based on mode/level?
*   How should the application behave if the configured log directory is not writable at startup?
*   For the `Request/Task ID`, what will be the mechanism to generate and propagate this ID through different parts of the application to ensure it's available for logging contexts?
*   Is there a preferred existing Go logging library to base this framework on? 