## Relevant Files

- `internal/secretscanner/detector.go` - Service chính điều phối việc phát hiện bí mật.
- `internal/secretscanner/scanner.go` - Logic quét sử dụng regex.
- `internal/secretscanner/patterns.go` - Định nghĩa các pattern regex.
- `internal/models/secret_finding.go` - Struct định nghĩa một phát hiện bí mật.
- `internal/datastore/secrets_store.go` - Logic lưu trữ kết quả vào Parquet.
- `internal/crawler/crawler_core.go` - Chứa định nghĩa struct `Crawler` và builder.
- `internal/crawler/crawler_initializer.go` - Chứa logic khởi tạo cho crawler, bao gồm cả việc thiết lập collector và các callback.
- `internal/crawler/crawler_executor.go` - Chứa logic thực thi các batch crawl.
- `internal/crawler/handlers.go` - Chứa các hàm xử lý callback của colly (OnResponse, OnError, OnRequest).
- `internal/reporter/html_reporter.go` - Sửa đổi để hiển thị kết quả quét bí mật.
- `internal/notifier/discord_notifier.go` - Gửi thông báo khi có phát hiện.
- `internal/config/config.go` - Thêm cấu hình cho secret scanner.

### Notes

- Cần tối ưu hóa hiệu năng để việc quét không làm chậm quá trình crawl.
- Xử lý lỗi cẩn thận, đặc biệt là khi gửi thông báo hoặc ghi vào file.

## Tasks

- [x] 1.0 Set up `gitleaks` dependency and initial `Detector` service structure.
  - [x] 1.1 Add the `gitleaks` library (`github.com/gitleaks/gitleaks/v8`) to `go.mod`.
  - [x] 1.2 Create a new package `internal/secretscanner`.
  - [x] 1.3 Inside `secretscanner`, create `detector.go` with a `Detector` struct. This struct should have fields for configuration and a logger.
- [x] 2.0 Implement Core Secret Scanning Logic
  - [x] 2.1 The `Detector` struct should have a method `ScanAndProcess(content []byte, sourceURL string) ([]models.SecretFinding, error)`.
  - [x] 2.2 Inside `ScanAndProcess`, use the `gitleaks` detector to scan the input `content`.
  - [x] 2.3 For each finding from `gitleaks`, create a `models.SecretFinding` struct. The model should include fields like `RuleID`, `Description`, `SecretText`, and `SourceURL`.
  - [x] 2.4 Store the findings in a Parquet file using a new `SecretsStore` in `internal/datastore`.
- [x] 3.0 Integrate with Crawler for Real-time Scanning
  - [x] 3.1 The `Detector` service should be a dependency in `internal/crawler/crawler_core.go`.
  - [x] 3.2 In the crawler's `OnResponse` handler, get the response body.
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
- [x] 6.0 Implement Unit Tests for Secret Scanner
  - [x] 6.1 Write unit tests for `internal/secretscanner/detector.go` to test scanning logic and notification triggers.
  - [x] 6.2 Write unit tests for `internal/datastore/secrets_store.go` to test storing and loading findings from Parquet files.
  - [x] 6.3 Write unit tests for the reporting logic to ensure secrets are correctly processed and associated with the right URLs.