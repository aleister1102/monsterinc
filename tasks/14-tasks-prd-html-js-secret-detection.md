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

- [ ] 1.0 Setup Core Secret Detection Components
  - [ ] 1.1 In `internal/models/secret_finding.go`, define the `SecretFinding` struct to hold all required info (SourceURL, RuleID, SecretText, LineNumber, Context, etc.).
  - [ ] 1.2 In `internal/secretscanner/patterns.go`, define a struct for regex rules and create a default list of patterns inspired by `mantra`.
  - [ ] 1.3 In `internal/secretscanner/scanner.go`, implement a `RegexScanner` with a `Scan()` method that applies the regex patterns to input content and returns findings.
- [ ] 2.0 Develop Detector Service and Data Storage
  - [ ] 2.1 In `internal/secretscanner/detector.go`, define the `Detector` service struct with its dependencies (config, datastore, notifier, logger).
  - [ ] 2.2 Implement `NewDetector(...)` constructor.
  - [ ] 2.3 Implement the main `ScanAndProcess(sourceURL string, content []byte)` method to orchestrate scanning, storage, and notification.
  - [ ] 2.4 In `internal/datastore/secrets_store.go`, create a `SecretsStore` to write findings to a Parquet file. Define the schema and implement a `StoreFindings` method.
- [ ] 3.0 Integrate with Crawler for Real-time Scanning
  - [ ] 3.1 Modify `internal/crawler/crawler.go` to accept the `Detector` service as a dependency.
  - [ ] 3.2 In the crawler's `OnResponse` handler, check the response's `Content-Type`.
  - [ ] 3.3 If the content type matches `text/html` or JavaScript variants, call `detector.ScanAndProcess` in a new goroutine to avoid blocking the crawler.
- [ ] 4.0 Implement Reporting and Notifications
  - [ ] 4.1 In the `Detector` service, upon finding a secret, prepare a payload and call the existing `DiscordNotifier` to send an immediate alert.
  - [ ] 4.2 Modify `internal/reporter/html_reporter.go` to accept secret findings as an input for the `GenerateReport` method.
  - [ ] 4.3 Before generating the report, load all secrets for the current scan from the Parquet store.
  - [ ] 4.4 Add a new "Secrets" section to the `report.html.tmpl` template and render the findings in a table.
- [ ] 5.0 Add Configuration and Finalize
  - [ ] 5.1 Add a `SecretsConfig` struct to `internal/config/config.go` with fields like `Enabled` and `NotifyOnFound`.
  - [ ] 5.2 Use these configuration values in the `Detector` and `Crawler` to conditionally enable/disable scanning and notifications.
  - [ ] 5.3 Update `config.example.yaml` with the new `secrets` configuration section and add explanatory comments.