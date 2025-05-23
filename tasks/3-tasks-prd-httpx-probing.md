## Relevant Files

- `internal/httpxrunner/runner.go` - Wrapper for the `projectdiscovery/httpx` library, handles configuration, execution, and result parsing.
- `internal/config/httpx_config.go` - Defines configuration structures for `httpx` probing within MonsterInc.
- `internal/httpxrunner/result.go` - Defines the structured result object for `httpx` probes.
- `go.mod` - To add `github.com/projectdiscovery/httpx` dependency.
- `internal/httpxrunner/README.md` - Documentation of httpx library APIs and interfaces.
- `internal/urlhandler/urlhandler.go` - Các tiện ích xử lý URL chung.
- `internal/urlhandler/file.go` - Các tiện ích đọc URL từ file.
- `internal/core/target_manager.go` - Quản lý mục tiêu, sử dụng `urlhandler`.
- `internal/crawler/scope.go` - Quản lý phạm vi crawler, sử dụng `urlhandler`.
- `internal/crawler/crawler.go` - Crawler chính, sử dụng `urlhandler`.
- `internal/config/httpx_config_test.go` - Unit test cho `config/httpx_config.go`.
- `internal/urlhandler/urlhandler_test.go` - Unit test cho `urlhandler/urlhandler.go`.
- `internal/urlhandler/file_test.go` - Unit test cho `urlhandler/file.go`.

### Notes

- The existing files `internal/httpxrunner/client.go`, `internal/httpxrunner/probe.go`, and `internal/httpxrunner/techdetector.go` will likely be significantly refactored or removed as their functionality will be largely replaced by the direct integration of the `httpx` library.
- Unit test thường nên được đặt cùng với các file code mà chúng kiểm thử (ví dụ: `MyComponent.tsx` và `MyComponent.test.tsx` trong cùng một thư mục).
- Sử dụng `go test ./...` để chạy tất cả các test trong project.

## Tasks

- [x] 1.0 Setup and Integrate `httpx` Library
  - [x] 1.1 Add `github.com/projectdiscovery/httpx` dependency to `go.mod`
  - [x] 1.2 Create initial `internal/httpxrunner/runner.go` file with basic structure
  - [x] 1.3 Research and document the core `httpx` library APIs and interfaces
  - [x] 1.4 Implement basic initialization of the `httpx` runner
  - [x] 1.5 Write initial tests for the runner

- [x] 2.0 Implement `httpx` Configuration Mapping
  - [x] 2.1 Create `internal/config/httpx_config.go` with configuration structs
  - [x] 2.2 Implement data extraction flags configuration (status code, content length, etc.)
  - [x] 2.3 Implement control flags configuration (concurrency, timeout, retries, etc.)
  - [x] 2.4 Add configuration validation logic

- [x] 3.0 Implement `httpx` Execution and Result Parsing
  - [x] 3.1 Create `internal/httpxrunner/result.go` with `ProbeResult` struct
  - [x] 3.2 Implement URL input handling and validation (chuyển sang `urlhandler`)
  - [x] 3.3 Implement core probing execution logic using `httpx` runner
    - [x] 3.3.1 Ensure `httpxrunner.Runner.Run()` correctly calls `httpxRunner.RunEnumeration()` of the `httpx` library.
    - [x] 3.3.2 Verify that the `OnResult` callback in `NewRunner` is correctly configured to process results asynchronously.
  - [x] 3.4 Implement result parsing from `httpx` output to `ProbeResult` in `OnResult` callback
    - [x] 3.4.1 Map `runner.Result.URL` to `ProbeResult.InputURL`.
    - [x] 3.4.2 Map `runner.Result.FinalURL` to `ProbeResult.FinalURL` (if available).
    - [x] 3.4.3 Map `runner.Result.StatusCode` to `ProbeResult.StatusCode`.
    - [x] 3.4.4 Map `runner.Result.ContentLength` to `ProbeResult.ContentLength`.
    - [x] 3.4.5 Map `runner.Result.ContentType` to `ProbeResult.ContentType`.
    - [x] 3.4.6 Map `runner.Result.Title` to `ProbeResult.Title` (if available).
    - [x] 3.4.7 Map `runner.Result.WebServer` to `ProbeResult.WebServer` (if available).
    - [x] 3.4.8 Map `runner.Result.Error` to `ProbeResult.Error` string.
    - [x] 3.4.9 Map `runner.Result.Duration` to `ProbeResult.Duration` (in seconds).
    - [x] 3.4.10 Map `runner.Result.Headers` (map[string][]string or similar) to `ProbeResult.Headers` (map[string]string, potentially concatenating values).
    - [x] 3.4.11 Map `runner.Result.ResponseBody` (or similar if body extraction is enabled) to `ProbeResult.Body`.
    - [x] 3.4.12 Map `runner.Result.TLSData` to `ProbeResult` TLS fields.
    - [x] 3.4.13 (Optional) Store raw JSON from `httpx` in `ProbeResult.JSONOutput` if available and configured.
  - [x] 3.5 Add support for technology detection parsing
    - [x] 3.5.1 Ensure `httpx.Options` in `runner.go` (e.g., `Options.TechDetect`) allows enabling technology detection.
    - [x] 3.5.2 Map `runner.Result.Technologies` (list of strings or structs) to `ProbeResult.Technologies` (list of `Technology` structs: Name. Version/Category tạm thời bỏ qua).
  - [x] 3.6 Add support for DNS information parsing (IP)
    - [x] 3.6.1 Ensure `httpx.Options` in `runner.go` allows enabling IP extraction.
    - [x] 3.6.2 Map `runner.Result.A` (IPs) to `ProbeResult.IPs` ([]string).
    - [x] 3.6.3 Map `runner.Result.CNAMEs` to `ProbeResult.CNAMEs`.
    - [x] 3.6.4 Map `runner.Result.ASN` to `ProbeResult.ASN` and `ProbeResult.ASNOrg`.

- [x] 4.0 Integrate `httpx` Runner into MonsterInc's Probing Workflow
  - [x] 4.1 Review existing MonsterInc probing interfaces (e.g., in `core` package or services).
  - [x] 4.2 Define how `httpxrunner.Runner` will be invoked (e.g., create a new service or adapt `TargetManager`).
  - [x] 4.3 Implement logic to create `httpxrunner.Config` from MonsterInc's global/job configuration.
  - [x] 4.4 Implement the main probing loop: initialize runner, set targets, run, collect results, handle errors.
  - [x] 4.5 Identify and plan for deprecation/removal of old probing code (e.g., `internal/httpxrunner/client.go`, `probe.go`).
  - [x] 4.6 (Optional) Update internal developer documentation regarding the new probing workflow.

- [x] 5.0 Implement Error Handling and Logging for `httpx` Integration
  - [x] 5.1 Ensure errors from `httpxrunner.NewRunner()` and `httpxrunner.Runner.Initialize()` are properly handled and logged by the calling module.
  - [x] 5.2 Confirm `ProbeResult.Error` is populated for individual probe failures (already handled by `OnResult`).
  - [x] 5.3 Implement structured logging (using MonsterInc's standard logger or `log` package) for key events:
    - [x] 5.3.1 Log runner initialization (success/failure).
    - [x] 5.3.2 Log start and end of a batch probing operation.
    - [x] 5.3.3 Log significant errors reported by the `httpx` library during enumeration.
    - [x] 5.3.4 Log a summary of probing results (total probed, successes, failures).
  - [x] 5.4 Implement metrics collection for probe success/failure rates.
    - [x] 5.4.1 Add counters for total URLs attempted, successful probes, and failed probes.
    - [x] 5.4.2 Log these metrics or make them available for other system components. 