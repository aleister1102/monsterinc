## Relevant Files

- `internal/monitor/fetcher.go` - Logic for fetching content of HTML and JavaScript files from URLs.
- `internal/monitor/processor.go` - Logic for processing fetched content (e.g., hashing, initial analysis if any before diffing).
- `internal/monitor/scheduler.go` - Manages the scheduling of monitoring tasks for different target files.
- `internal/monitor/service.go` - The main service orchestrating the monitoring process (uses fetcher, processor, scheduler, datastore).
- `internal/datastore/file_history_store.go` (New or part of `parquet_writer.go`/`parquet_reader.go`) - Stores and retrieves historical versions of monitored files (content or hashes).
- `internal/models/monitored_file.go` (New) - Struct defining a monitored file (URL, last hash, last checked, content type etc.).
- `internal/config/config.go` - Contains `MonitorConfig` with settings like check interval, target JS/HTML file extensions/patterns.
- `cmd/monsterinc/main.go` - Where the monitoring service is initialized and started (potentially as a separate mode or goroutine).

### Notes

- This feature focuses on *detecting changes* in specified HTML/JS files, not full crawling or discovery.
- Efficiency is key for fetching and comparing files. Use `HEAD` requests and `ETag`/`Last-Modified` headers where possible before full GET.
- Storage of historical versions needs to be considered carefully (full content vs. hashes vs. diffs).

## Tasks

- [ ] 1.0 Setup Monitoring Service Core (in `internal/monitor/service.go`)
  - [ ] 1.1 Define `MonitoringService` struct (dependencies: `config.MonitorConfig`, `datastore.FileHistoryStore`, `logger.Logger`, `notifier.DiscordNotifier`).
  - [ ] 1.2 Implement `NewMonitoringService(...)` constructor.
  - [ ] 1.3 Implement `Start()` method to begin the monitoring loop/scheduler.
  - [ ] 1.4 Implement `Stop()` method for graceful shutdown.
  - [ ] 1.5 Define how target URLs for monitoring are provided (e.g., from main input, a separate file, or discovered URLs matching patterns from `CrawlerConfig`).

- [ ] 2.0 Implement File Fetching and Pre-processing (in `internal/monitor/fetcher.go` and `internal/monitor/processor.go`)
  - [ ] 2.1 In `fetcher.go`, implement `FetchFileContent(url string) ([]byte, string, error)` returning content, content-type, and error.
        *   Use `http.Client` with appropriate timeout from config.
        *   Handle HTTP errors, non-200 status codes.
  - [ ] 2.2 (Optimization) Implement conditional fetching using `ETag` and `Last-Modified` headers. Store these with the file history.
        *   Send `If-None-Match` (for ETag) or `If-Modified-Since`.
        *   If server returns 304 Not Modified, skip download and hashing.
  - [ ] 2.3 In `processor.go`, implement `ProcessContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error)`.
        *   Calculate a hash of the content (e.g., SHA256) (FR1).
        *   Potentially extract metadata if needed for later stages (e.g., JS comments, HTML structure - though this might be out of scope for simple change detection).
        *   Return a struct like `MonitoredFileUpdate {URL, NewHash, ContentType, FetchedAt}`.

- [ ] 3.0 Implement Change Detection and Storage (in `internal/monitor/service.go` and `internal/datastore/file_history_store.go`)
  - [ ] 3.1 In `FileHistoryStore`, implement `GetLastKnownHash(url string) (string, error)`.
  - [ ] 3.2 In `FileHistoryStore`, implement `StoreFileRecord(url string, hash string, content []byte, contentType string, timestamp time.Time) error`.
        *   This will store the current version (FR2). Decide if full content is always stored or only on change, or only hashes.
        *   Parquet can be used for this, with schema: URL, Timestamp, Hash, ContentType, Content (optional, or path to content).
  - [ ] 3.3 In `MonitoringService` loop: for each target URL:
        *   Fetch content using `Fetcher`.
        *   Process content using `Processor` to get new hash.
        *   Get last known hash from `FileHistoryStore`.
        *   If hash is different or file is new: 
            *   Log the change (FR3.1).
            *   Store the new version/hash using `FileHistoryStore` (FR2).
            *   Trigger notification (FR3.2, via `DiscordNotifier`).
            *   This detected change will be the input for `11-tasks-prd-html-js-content-diff-reporting.md`.

- [ ] 4.0 Implement Scheduling (in `internal/monitor/scheduler.go` or within `service.go`)
  - [ ] 4.1 Implement a scheduler to periodically check monitored files based on `MonitorConfig.CheckIntervalSeconds` (FR4).
        *   Use a library like `robfig/cron` or a simple `time.Ticker` loop.
  - [ ] 4.2 Manage a list of URLs to monitor. This list might come from initial configuration or be dynamically updated (though dynamic updates are more complex).
  - [ ] 4.3 Ensure concurrent checks are managed if many files are monitored (e.g., using a worker pool limited by `MonitorConfig.MaxConcurrentChecks`).

- [ ] 5.0 Configuration (Covered by `7-tasks-prd-configuration-management.md`)
  - [ ] 5.1 Ensure `MonitorConfig` in `internal/config/config.go` includes:
        *   `Enabled bool`
        *   `CheckIntervalSeconds int`
        *   `TargetJSFilePatterns []string` (regex or glob for URLs that are JS - FR5)
        *   `TargetHTMLFilePatterns []string` (regex or glob for URLs that are HTML - FR5)
        *   `MaxConcurrentChecks int`
        *   `StoreFullContentOnChange bool` (config whether to store full content or just hash/metadata)
        *   `HTTPTimeoutSeconds int`
  - [ ] 5.2 Ensure these are in `config.example.json`.

- [ ] 6.0 Notifications (Integration with `DiscordNotifier`)
  - [ ] 6.1 In `discord_formatter.go`, implement `FormatFileChangeNotification(url string, oldHash string, newHash string, contentType string) (string, *discordEmbed)` (FR3.2).
  - [ ] 6.2 When `MonitoringService` detects a change, call `DiscordNotifier.SendNotification`.

- [ ] 7.0 Unit Tests
  - [ ] 7.1 Test `Fetcher` for successful fetch, error handling, and conditional GET logic.
  - [ ] 7.2 Test `Processor` for correct hashing.
  - [ ] 7.3 Test `FileHistoryStore` for storing and retrieving records.
  - [ ] 7.4 Test `MonitoringService` for change detection logic (new file, changed file, unchanged file).
  - [ ] 7.5 Test scheduler logic (if custom implemented) or integration with cron library.

- [ ] 8.0 (Future) Integration with Crawler
    *   Consider how newly discovered JS/HTML files from the crawler (`2-tasks-prd-target-crawling.md`) that match `TargetJSFilePatterns`/`TargetHTMLFilePatterns` can be added to the monitoring list. 