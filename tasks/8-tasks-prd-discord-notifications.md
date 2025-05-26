## Relevant Files

- `internal/notifier/discord_notifier.go` - Core Discord notification logic, including sending messages via webhook.
- `internal/notifier/discord_formatter.go` - Logic for formatting messages and constructing Discord embed objects.
- `internal/config/config.go` - Contains `NotificationConfig` struct with Discord webhook URL and other notification settings.
- `cmd/monsterinc/main.go` - Where the `DiscordNotifier` will be initialized and used.
- `internal/models/notification_models.go` - Struct to hold summary data for notifications (e.g., `ScanSummaryData`).
- `internal/notifier/notification_helper.go` - Helper service to simplify sending different types of notifications.

### Notes

- Focus on clear, concise, and informative notifications.
- Adhere to Discord message limits (e.g., embed field limits, total message length).
- Ensure robust error handling for network issues or invalid webhook URLs.
- `ScanSessionID` refers to the timestamped ID of a specific scan run (e.g., `20230101-123000`).
- `TargetSource` refers to the origin of the targets for a scan (e.g., `targets.txt`, `config_input_urls`, or `Signal Interrupt`).

## Tasks

- [x] 1.0 Setup Discord Notifier Core (in `internal/notifier/discord_notifier.go`)
  - [x] 1.1 Define `DiscordNotifier` struct (dependencies: `config.NotificationConfig`, `zerolog.Logger`, `*http.Client`).
  - [x] 1.2 Implement `NewDiscordNotifier(cfg config.NotificationConfig, logger zerolog.Logger, httpClient *http.Client) (*DiscordNotifier, error)` constructor.
  - [x] 1.3 Implement `SendNotification(ctx context.Context, payload models.DiscordMessagePayload, reportFilePath string) error` method.
        *   This method handles constructing and sending the multipart/form-data request if `reportFilePath` is provided, otherwise sends JSON.
  - [x] 1.4 Implement the actual HTTP POST request to the Discord webhook URL (handling both JSON and multipart).
  - [x] 1.5 Add basic retry logic for transient network errors (e.g., using a simple backoff). (Implemented: 1 retry after 5s)
  - [x] 1.6 Handle Discord API rate limits gracefully (log a warning, or implement more sophisticated backoff if needed). (Implemented: logs error, retry might help for minor limits)
  - [x] 1.7 Ensure `DiscordWebhookURL` is validated (e.g., not empty, looks like a URL) during notifier initialization. Notifier is disabled if URL is missing.

- [x] 2.0 Implement Message Formatting (in `internal/notifier/discord_formatter.go`)
  - [x] 2.1 Define `DiscordMessagePayload`, `DiscordEmbed`, `DiscordEmbedField`, etc. structs in `internal/models/notification_models.go`.
  - [x] 2.2 Implement `FormatScanCompleteMessage(summaryData models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload`.
        *   `summaryData` includes: `ScanSessionID`, `TargetSource`, `TotalTargets`, `ProbeStats`, `DiffStats`, `ErrorMessages`, `ScanDuration`, `ReportPath`, `Status`.
        *   The embed is well-structured, color-coded based on `Status`.
        *   Indicates if HTML report is attached (actual attachment handled by `DiscordNotifier`).
  - [x] 2.3 Implement `FormatScanStartMessage(summaryData models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload`.
        *   Uses `summaryData.TargetSource` for the main title, `summaryData.ScanSessionID` in description.
  - [x] 2.4 Implement `FormatCriticalErrorMessage(summaryData models.ScanSummaryData, cfg config.NotificationConfig) models.DiscordMessagePayload`.
        *   Uses `summaryData.Component`, `summaryData.ErrorMessages`, `summaryData.ScanSessionID`, `summaryData.TargetSource`.
        *   The embed is color-coded red.
  - [x] 2.5 Implement helper functions for `buildMentions`, `truncateString`, `formatDuration`.
  - [x] 2.6 Handle message/embed field length limitations by truncating content with an indicator ("...").

- [x] 3.0 Integrate Notifications via NotificationHelper (in `cmd/monsterinc/main.go`, `internal/scheduler/scheduler.go`)
  - [x] 3.1 Create `internal/notifier/notification_helper.go` with `NotificationHelper` struct.
  - [x] 3.2 Implement `NewNotificationHelper` and methods like `SendScanStartNotification(ctx context.Context, summary models.ScanSummaryData)`, `SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData)`, `SendCriticalErrorNotification(ctx context.Context, componentName string, summary models.ScanSummaryData)`.
  - [x] 3.3 Initialize `DiscordNotifier` and then `NotificationHelper` in `main.go`.
  - [x] 3.4 In `main.go` (`runOnetimeScan` and signal handler) and `internal/scheduler/scheduler.go` (`runScanCycle`, `runScanCycleWithRetries`):
        *   Populate `models.ScanSummaryData` with `ScanSessionID` and `TargetSource` appropriately.
        *   Call the relevant `NotificationHelper` methods.

- [x] 4.0 Configuration (Covered by `7-tasks-prd-configuration-management.md`)
  - [x] 4.1 `NotificationConfig` in `internal/config/config.go` includes:
        *   `DiscordWebhookURL string`
        *   `NotifyOnSuccess bool`
        *   `NotifyOnFailure bool`
        *   `NotifyOnScanStart bool`
        *   `NotifyOnCriticalError bool`
        *   `MentionRoleIDs []string`
  - [x] 4.2 These fields are in `config.example.yaml`.

- [x] 5.0 Logging and Error Handling in Notifier
  - [x] 5.1 `DiscordNotifier` logs: sending notification, successful delivery, webhook errors (status code).
  - [x] 5.2 `NotificationHelper` logs: preparation of notifications, success/failure of sending through `DiscordNotifier`.
  - [x] 5.3 `DiscordNotifier` is disabled if `DiscordWebhookURL` is not set; methods return early.

- [ ] 6.0 Unit Tests (SKIPPED)
  - [ ] 6.1 Write unit tests for `DiscordNotifier`.
  - [ ] 6.2 Write unit tests for `discord_formatter.go` functions.
  - [ ] 6.3 Write unit tests for `NotificationHelper`. 