## Relevant Files

- `internal/secretscanner/detector.go` - Service chính điều phối việc phát hiện bí mật.
- `internal/secretscanner/scanner.go` - Logic quét sử dụng regex.
- `internal/secretscanner/patterns.go` - Định nghĩa các pattern regex.
- `internal/models/secret_finding.go` - Struct định nghĩa một phát hiện bí mật.
- `internal/datastore/secrets_store.go` - Logic lưu trữ kết quả vào Parquet.
- `internal/crawler/crawler.go` - Nơi tích hợp để gọi secret scanner.
- `internal/reporter/html_reporter.go` - Sửa đổi để hiển thị kết quả quét bí mật.
- `internal/notifier/discord_notifier.go` - Gửi thông báo khi có phát hiện.
- `internal/config/config.go` - Thêm cấu hình cho secret scanner.

### Notes

- Cần tối ưu hóa hiệu năng để việc quét không làm chậm quá trình crawl.
- Xử lý lỗi cẩn thận, đặc biệt là khi gửi thông báo hoặc ghi vào file.

## Tasks

- [x] 1.0 Setup Core Secret Detection Components
  - [x] 1.1 In `internal/models/secret_finding.go`, define the `SecretFinding` struct to hold all required info (SourceURL, RuleID, SecretText, LineNumber, Context, etc.).
  - [x] 1.2 In `internal/secretscanner/patterns.go`, define a struct for regex rules and create a default list of patterns inspired by `mantra`.
  - [x] 1.3 In `internal/secretscanner/scanner.go`, implement a `RegexScanner` with a `Scan()` method that applies the regex patterns to input content and returns findings.
- [x] 2.0 Develop Detector Service and Data Storage
  - [x] 2.1 In `internal/secretscanner/detector.go`, define the `Detector` service struct with its dependencies (config, datastore, notifier, logger).
  - [x] 2.2 Implement `NewDetector(...)` constructor.
  - [x] 2.3 Implement the main `ScanAndProcess(sourceURL string, content []byte)` method to orchestrate scanning, storage, and notification.
  - [x] 2.4 In `internal/datastore/secrets_store.go`, create a `SecretsStore` to write findings to a Parquet file. Define the schema and implement a `StoreFindings` method.
- [x] 3.0 Integrate with Crawler for Real-time Scanning
  - [x] 3.1 Modify `internal/crawler/crawler.go` to accept the `Detector` service as a dependency.
  - [x] 3.2 In the crawler's `OnResponse` handler, check the response's `Content-Type`.
  - [x] 3.3 If the content type matches `text/html` or JavaScript variants, call `detector.ScanAndProcess` in a new goroutine to avoid blocking the crawler.
- [x] 4.0 Implement Reporting and Notifications
  - [x] 4.1 In the `Detector` service, upon finding a secret, prepare a payload and call the existing `DiscordNotifier` to send an immediate alert.
  - [x] 4.2 Modify `internal/reporter/html_reporter.go` to accept secret findings as an input for the `GenerateReport` method.
  - [x] 4.3 Before generating the report, load all secrets for the current scan from the Parquet store.
  - [x] 4.4 Add a new "Secrets" section to the `report.html.tmpl` template and render the findings in a table.
- [x] 5.0 Add Configuration and Finalize
  - [x] 5.1 Add a `SecretsConfig` struct to `internal/config/config.go` with fields like `Enabled` and `NotifyOnFound`.
  - [x] 5.2 Use these configuration values in the `Detector` and `Crawler` to conditionally enable/disable scanning and notifications.
  - [x] 5.3 Update `config.example.yaml` with the new `secrets` configuration section and add explanatory comments.

## Summary

All major tasks for secret detection have been completed:

✅ **Core Infrastructure**: SecretDetectorService, TruffleHogAdapter, RegexScanner, ParquetSecretsStore
✅ **Configuration**: SecretsConfig with all necessary options
✅ **Monitor Service Integration**: Secret detection moved from scan service to monitor service
✅ **Diff Report Integration**: Secret findings displayed in diff reports with statistics and masked secrets
✅ **Notifications**: High-severity secret notifications via Discord (using MonitorServiceNotification)
✅ **Comprehensive Logging**: Detailed logging throughout the secret detection process
✅ **Mantra Patterns**: Extensive regex patterns from open-source databases
✅ **Integration**: Full integration with monitor service and diff reporting

**Important Changes Made:**
- **Secret detection removed from scan service/orchestrator** - no longer runs during main scans
- **Secret detection integrated into monitor service** - runs when monitoring file changes
- **Secret findings stored in ContentDiffResult** - displayed in diff reports instead of main scan reports
- **Notifications use MonitorServiceNotification** - since secrets are only detected in monitor service
- **Diff reports enhanced** - now show secret findings with statistics, masked secret text, and severity-based styling

The secret detection feature is now fully functional and integrated with the monitor service and diff reporting system. 