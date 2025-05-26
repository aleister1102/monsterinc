## Relevant Files

- `internal/secrets/detector_service.go` - Main service orchestrating secret detection.
- `internal/secrets/trufflehog_adapter.go` - Adapter for integrating with TruffleHog (either as a library or CLI).
- `internal/secrets/regex_scanner.go` - Implements scanning using custom regex patterns.
- `internal/secrets/patterns.go` - Defines default and custom regex patterns for secrets.
- `internal/models/secret_finding.go` (New) - Struct defining a secret finding (e.g., `SourceURL`, `FilePathInArchive` (if applicable), `RuleID`, `Description`, `Severity`, `SecretText`, `LineNumber`).
- `internal/datastore/secrets_store.go` (New or part of `parquet_writer.go`) - For storing detected secrets.
- `internal/reporter/html_reporter.go` - To integrate secret findings into the main HTML report.
- `internal/config/config.go` - May include `SecretsConfig` (e.g., `EnableTruffleHog`, `CustomRegexPatternsFile`, `MaxFileSizeScanMB`).

### Notes

- Secret detection can be resource-intensive. Optimize for performance.
- False positives are common. Provide context and clear descriptions for findings.
- TruffleHog is a good starting point; custom regexes can augment it.
- Security: Be careful when logging or storing parts of detected secrets.

## Tasks

- [ ] 1.0 Setup Secret Detector Service (in `internal/secrets/detector_service.go`)
  - [ ] 1.1 Define `SecretDetectorService` struct (dependencies: `config.SecretsConfig`, `datastore.SecretsStore`, `logger.Logger`, `notifier.DiscordNotifier`).
  - [ ] 1.2 Implement `NewSecretDetectorService(...)` constructor.
  - [ ] 1.3 Implement `ScanContent(sourceURL string, content []byte, contentType string) ([]models.SecretFinding, error)` method.
        *   This service will be called by other components (e.g., Crawler, Monitor) when new HTML/JS content is fetched.

- [ ] 2.0 Integrate TruffleHog (in `internal/secrets/trufflehog_adapter.go`)
  - [ ] 2.1 Evaluate TruffleHog integration: Go library (if a stable one exists and is suitable) vs. shelling out to TruffleHog CLI (FR1.1).
        *   CLI might be easier to keep updated but adds external dependency management.
  - [ ] 2.2 Implement `TruffleHogAdapter` struct/functions.
  - [ ] 2.3 Implement `ScanWithTruffleHog(content []byte, filenameHint string) ([]models.SecretFinding, error)` (FR1.2).
        *   `filenameHint` helps TruffleHog apply relevant rules if it uses filename patterns.
        *   Parse TruffleHog output (JSON if CLI) into `models.SecretFinding` structs.
        *   Map TruffleHog severities/types to internal `Severity` and `Description`.
  - [ ] 2.4 Handle TruffleHog execution errors, timeouts.

- [ ] 3.0 Implement Custom Regex Scanning (in `internal/secrets/regex_scanner.go` and `internal/secrets/patterns.go`)
  - [ ] 3.1 In `patterns.go`, define a struct for regex patterns (e.g., `RuleID`, `Pattern string`, `Description string`, `Severity string`).
  - [ ] 3.2 Populate `patterns.go` with a default set of regexes (inspired by `gitleaks`, `trufflehog` regexes, or `mantra` - FR2.1).
        *   Categorize them (API keys, private keys, tokens, PII placeholders etc.).
  - [ ] 3.3 (Optional) Implement loading additional custom regex patterns from a file specified in `SecretsConfig.CustomRegexPatternsFile` (FR2.2).
  - [ ] 3.4 In `regex_scanner.go`, implement `ScanWithRegexes(content []byte, patterns []Pattern) ([]models.SecretFinding, error)`.
        *   Iterate through patterns, compile regex, and search content.
        *   For each match, create a `models.SecretFinding` (include line number, matched text snippet - carefully consider how much to include).

- [ ] 4.0 Combine Results and Store (in `SecretDetectorService.ScanContent` and `internal/datastore/secrets_store.go`)
  - [ ] 4.1 In `ScanContent`:
        *   Call `ScanWithTruffleHog` if enabled.
        *   Call `ScanWithRegexes`.
        *   Combine and deduplicate findings (a secret found by both TruffleHog and a regex should ideally be one finding).
  - [ ] 4.2 In `secrets_store.go`, define Parquet schema for `models.SecretFinding` (FR3.1).
  - [ ] 4.3 Implement `StoreSecretFindings(findings []models.SecretFinding) error` to write to Parquet (FR3.2).

- [ ] 5.0 Integrate Findings into HTML Report and Notifications
  - [ ] 5.1 Modify `internal/reporter/html_reporter.go` and `report.html.tmpl`:
        *   Add a new section to the HTML report for "Secret Detection Findings" (FR4.1, FR4.2).
        *   Display findings in a table: Source URL, Description/Rule, Severity, Snippet (masked or limited), Line.
  - [ ] 5.2 Modify `ProbeResultDisplay` or `ReportPageData` in `internal/models/report_data.go` to include `[]models.SecretFinding`.
  - [ ] 5.3 When `SecretDetectorService` finds high-severity secrets, consider immediate notification via `DiscordNotifier` (FR4.3 - needs a new formatter in `discord_formatter.go`).
        *   `FormatHighSeveritySecretNotification(finding models.SecretFinding) (string, *discordEmbed)`.

- [ ] 6.0 Configuration (as part of `GlobalConfig`)
  - [ ] 6.1 Add `SecretsConfig` to `internal/config/config.go`:
        *   `Enabled bool`
        *   `EnableTruffleHog bool`
        *   `TruffleHogPath string` (if using CLI)
        *   `EnableCustomRegex bool`
        *   `CustomRegexPatternsFile string` (optional)
        *   `MaxFileSizeToScanMB int` (to avoid scanning huge files - FR5.1)
        *   `NotifyOnHighSeveritySecret bool`
  - [ ] 6.2 Ensure these are in `config.example.json`.

- [ ] 7.0 Performance and Error Handling
  - [ ] 7.1 In `SecretDetectorService.ScanContent`, check file size against `MaxFileSizeToScanMB` before scanning (FR5.1).
  - [ ] 7.2 Add robust error handling for TruffleHog execution, regex compilation, and file I/O for custom patterns.
  - [ ] 7.3 Add comprehensive logging for the secret detection process.

- [ ] 8.0 Unit Tests
  - [ ] 8.1 Test `TruffleHogAdapter` (mock CLI execution or library calls) with sample outputs.
  - [ ] 8.2 Test `RegexScanner` with various patterns and content (matching and non-matching cases).
  - [ ] 8.3 Test `SecretDetectorService.ScanContent` for combining and deduplicating results.
  - [ ] 8.4 Test `SecretsStore` for Parquet writing/reading (if applicable).
  - [ ] 8.5 Test exclusion of oversized files. 