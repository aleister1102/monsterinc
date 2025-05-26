# Notifier Package (`internal/notifier`)

This package is responsible for sending notifications to various services based on scan events and application status.

## Core Components

1.  **`discord_notifier.go`**: 
    *   Contains the `DiscordNotifier` struct which handles the direct communication with the Discord webhook API.
    *   Responsible for constructing and sending HTTP requests (JSON payloads or multipart/form-data if attaching files like reports).
    *   Includes basic retry logic for network issues and handles disabling if the webhook URL is not configured.

2.  **`discord_formatter.go`**: 
    *   Provides functions to format raw scan data (`models.ScanSummaryData`) into structured `models.DiscordMessagePayload` objects, including Discord embeds.
    *   Handles message content, titles, descriptions, color-coding based on status, and field generation for scan statistics, errors, etc.
    *   Manages string truncation to adhere to Discord limitations.

3.  **`notification_helper.go`**:
    *   Defines the `NotificationHelper` struct, which acts as a higher-level service.
    *   It uses `DiscordNotifier` and formatters to simplify sending specific types of notifications (scan start, completion, critical errors).
    *   This decouples the main application logic (`cmd/monsterinc/main.go`, `internal/scheduler/scheduler.go`) from the specifics of Discord message formatting and sending.

4.  **`internal/models/notification_models.go`**:
    *   Defines the `ScanSummaryData` struct which is a crucial data structure passed around to gather all necessary information for a notification.
    *   Also defines `DiscordMessagePayload`, `DiscordEmbed`, and related structs that map to the Discord webhook API's expected JSON structure.

## Key Features

- Send notifications for:
  - Scan Start: Includes target source, total targets, and session ID.
  - Scan Completion (Success, Failure, Partial, Interrupted): Includes target source, session ID, status, duration, probe stats, diff stats, error messages, and attaches the HTML report if generated.
  - Critical Application Errors: Includes component where error occurred, error messages, and related scan session/target source if available.
- Support for Discord webhooks.
- Configurable notification settings via `config.yaml` (`NotificationConfig` section in `internal/config/config.go`):
  - `discord_webhook_url`
  - `notify_on_success`
  - `notify_on_failure`
  - `notify_on_scan_start`
  - `notify_on_critical_error`
  - `mention_role_ids` (for Discord role pings)
- Attaches HTML reports to Discord messages for completed scans.
- Graceful handling of missing webhook URL (disables notifier).
- Structured logging for notification attempts, successes, and failures.

## Usage Flow

1.  In `main.go`, `DiscordNotifier` is initialized using `config.NotificationConfig` and an HTTP client.
2.  `NotificationHelper` is then initialized with the `DiscordNotifier` instance.
3.  Modules like `runOnetimeScan` (in `main.go`) or `Scheduler` (in `internal/scheduler/scheduler.go`) populate a `models.ScanSummaryData` struct.
    *   `ScanSessionID`: Typically a timestamp like `YYYYMMDD-HHMMSS`.
    *   `TargetSource`: Indicates where targets came from (e.g., `urls.txt`, `config_input_urls`).
4.  The relevant method on `NotificationHelper` is called (e.g., `SendScanStartNotification`, `SendScanCompletionNotification`).
5.  `NotificationHelper` calls the appropriate formatting function from `discord_formatter.go` to create the `DiscordMessagePayload`.
6.  `NotificationHelper` then calls `DiscordNotifier.SendNotification` with the payload and any report file to be attached.
7.  `DiscordNotifier` sends the HTTP request to the Discord webhook.

## Example Initialization (Conceptual)

```go
// In main.go or similar setup area

// Assuming gCfg is *config.GlobalConfig and zLogger is zerolog.Logger
discordNotifier, err := notifier.NewDiscordNotifier(gCfg.NotificationConfig, zLogger, &http.Client{Timeout: 20 * time.Second})
if err != nil {
    // Handle error (e.g., log, but notifier will be disabled if webhook is missing)
}
notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

// Later, when a scan starts:
summary := models.GetDefaultScanSummaryData()
summary.ScanSessionID = "20230527-100000"
summary.TargetSource = "my_targets.txt"
summary.Targets = []string{"http://example.com"}
summary.TotalTargets = 1
summary.Status = string(models.ScanStatusStarted)
notificationHelper.SendScanStartNotification(context.Background(), summary)

// After scan completion:
summary.Status = string(models.ScanStatusCompleted)
summary.ScanDuration = time.Minute * 5
summary.ReportPath = "reports/20230527-100000_report.html"
notificationHelper.SendScanCompletionNotification(context.Background(), summary)
``` 