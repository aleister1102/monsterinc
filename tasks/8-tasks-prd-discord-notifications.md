## Relevant Files

- `internal/notifier/discord_notifier.go` - Core Discord notification logic, including sending messages via webhook.
- `internal/notifier/discord_formatter.go` - Logic for formatting messages and constructing Discord embed objects.
- `internal/config/config.go` - Contains `NotificationConfig` struct with Discord webhook URL and other notification settings.
- `cmd/monsterinc/main.go` - Where the `DiscordNotifier` will be initialized and used.
- `internal/models/scan_summary.go` (New or existing) - Struct to hold summary data for notifications (e.g., `ScanSummaryData`).

### Notes

- Focus on clear, concise, and informative notifications.
- Adhere to Discord message limits (e.g., embed field limits, total message length).
- Ensure robust error handling for network issues or invalid webhook URLs.

## Tasks

- [ ] 1.0 Setup Discord Notifier Core (in `internal/notifier/discord_notifier.go`)
  - [ ] 1.1 Define `DiscordNotifier` struct (dependencies: `config.NotificationConfig`, `logger.Logger`).
  - [ ] 1.2 Implement `NewDiscordNotifier(cfg config.NotificationConfig, logger logger.Logger) *DiscordNotifier` constructor.
  - [ ] 1.3 Implement `SendNotification(ctx context.Context, title string, message string, embed *discordEmbed) error` method.
        *   This method will be generic for sending various types of notifications.*
  - [ ] 1.4 Implement the actual HTTP POST request to the Discord webhook URL.
  - [ ] 1.5 Add basic retry logic for transient network errors (e.g., using a simple backoff).
  - [ ] 1.6 Handle Discord API rate limits gracefully (log a warning, or implement more sophisticated backoff if needed).
  - [ ] 1.7 Ensure `DiscordWebhookURL` is validated (e.g., not empty, looks like a URL) during notifier initialization or in config validation.

- [ ] 2.0 Implement Message Formatting (in `internal/notifier/discord_formatter.go`)
  - [ ] 2.1 Define `discordEmbed`, `discordEmbedField`, `discordEmbedAuthor`, `discordEmbedFooter` structs mirroring Discord's embed structure.
  - [ ] 2.2 Implement `FormatScanCompleteMessage(summaryData models.ScanSummaryData, reportPath string) (string, *discordEmbed)`.
        *   `summaryData` should include: Total targets, successful probes, new URLs, old URLs, errors, scan duration.*
        *   The embed should be well-structured, possibly color-coded (e.g., green for success).
        *   Include a link to the generated HTML report if available (FR4).
  - [ ] 2.3 Implement `FormatScanStartMessage(inputFile string, totalTargets int) (string, *discordEmbed)` (FR1.1).
  - [ ] 2.4 Implement `FormatCriticalErrorMessage(errorMessage string, component string) (string, *discordEmbed)` (FR1.3).
        *   The embed should be color-coded (e.g., red for errors).
  - [ ] 2.5 Implement helper functions for common formatting tasks (e.g., bolding text, creating links).
  - [ ] 2.6 Handle message/embed field length limitations. If content is too long, truncate with an indicator (e.g., "...").

- [ ] 3.0 Integrate Notifications into Application Flow (`cmd/monsterinc/main.go` and relevant services)
  - [ ] 3.1 Initialize `DiscordNotifier` in `main.go` if `DiscordWebhookURL` is configured.
  - [ ] 3.2 Call `DiscordNotifier.SendNotification` with `FormatScanStartMessage` at the beginning of a scan (FR1.1).
  - [ ] 3.3 After HTML report generation (if successful), gather `ScanSummaryData`.
  - [ ] 3.4 Call `DiscordNotifier.SendNotification` with `FormatScanCompleteMessage` when a scan finishes successfully (FR1.2, FR2 - conditional on `NotifyOnSuccess`).
  - [ ] 3.5 If a critical, unrecoverable error occurs during the application lifecycle (e.g., config load failure, major module failure), call `DiscordNotifier.SendNotification` with `FormatCriticalErrorMessage` (FR1.3, FR2 - conditional on `NotifyOnFailure`).
  - [ ] 3.6 Implement logic to include mentions (`<@&ROLE_ID>` or `<@USER_ID>`) in messages based on `MentionRoles` from `NotificationConfig` (FR3).

- [ ] 4.0 Configuration (Covered by `7-tasks-prd-configuration-management.md` but ensure these fields are present)
  - [ ] 4.1 Ensure `NotificationConfig` in `internal/config/config.go` includes:
        *   `DiscordWebhookURL string`
        *   `NotifyOnScanStart bool`
        *   `NotifyOnScanComplete bool`
        *   `NotifyOnCriticalError bool`
        *   `MentionRoleIDs []string`
  - [ ] 4.2 Ensure these fields are included in `config.example.json` with explanations.

- [ ] 5.0 Logging and Error Handling
  - [ ] 5.1 Add logging in `DiscordNotifier` for: sending notification, successful delivery, webhook errors (include status code if available).
  - [ ] 5.2 Log errors from message formatting if they occur.
  - [ ] 5.3 If `DiscordWebhookURL` is not set, the notifier methods should do nothing and not error (or log an Info message once at startup).

- [ ] 6.0 Unit Tests
  - [ ] 6.1 Write unit tests for `DiscordNotifier`:
        *   Mock the HTTP client to test successful sending and error conditions (e.g., 4xx, 5xx responses).
        *   Test retry logic.
  - [ ] 6.2 Write unit tests for `discord_formatter.go` functions:
        *   Test `FormatScanCompleteMessage`, `FormatScanStartMessage`, `FormatCriticalErrorMessage` for correct content and embed structure based on various inputs.
        *   Test truncation logic for long messages/fields.
        *   Test mention formatting.
  - [ ] 6.3 Test scenarios where webhook URL is missing or invalid. 