## Relevant Files

- `internal/logger/logger.go` - Nơi định nghĩa interface `Logger` (nếu cần abstaction) và constructor `NewLogger` sử dụng `zerolog`.
- `internal/logger/console.go` - Console output handler with colorization.
- `internal/logger/file.go` - File output handler with rotation.
- `internal/config/config.go` - Chứa `LogConfig` struct để cấu hình logging.
- `internal/config/validator.go` - Có thể cần cập nhật để validate các giá trị mới của `LogConfig`.
- `cmd/monsterinc/main.go` - Nơi logger được khởi tạo và truyền vào các module khác. Hàm `setupZeroLogger` hiện có sẽ là cơ sở.
- `Makefile` or `scripts/` - May include commands to run the application with different log levels for testing.
- `config.example.yaml` - File cấu hình ví dụ, cần cập nhật các tùy chọn cho `LogConfig`.
- Các module khác trong `internal/` - Sẽ cần cập nhật để sử dụng `zerolog.Logger`.

### Notes

- `zerolog` đã hỗ trợ sẵn structured logging, JSON output, và log levels.
- Console output với màu sắc có thể được thực hiện với `zerolog.ConsoleWriter`.
- Log rotation và file naming theo mode sẽ được quản lý chủ yếu qua `LogConfig` và cách `setupZeroLogger` (hoặc `logger.NewLogger`) được triển khai.

## Tasks

- [x] **1.0 Define Logger Configuration and Interface**
  - [x] 1.1 Review and update `LogConfig` struct in `internal/config/config.go`:
    - [x] 1.1.1 Ensure `LogLevel` (string) field exists (e.g., "debug", "info", "warn", "error", "fatal").
    - [x] 1.1.2 Ensure `LogFormat` (string) field exists (e.g., "console", "json").
    - [x] 1.1.3 Ensure `LogFile` (string) field exists for optional file output path.
    - [x] 1.1.4 (Consider for future, based on PRD): Add `MaxLogSizeMB`, `MaxLogBackups`, `CompressOldLogs` if planning to implement custom rotation with `lumberjack` or similar. For now, focus on `zerolog`'s direct capabilities.
  - [x] 1.2 Review and update `NewDefaultLogConfig` in `internal/config/config.go`:
    - [x] 1.2.1 Set default `LogLevel` to "info".
    - [x] 1.2.2 Set default `LogFormat` to "console".
    - [x] 1.2.3 Set default `LogFile` to "" (empty, meaning stderr by default).
  - [x] 1.3 Update `logLevelValidator` and `logFormatValidator` in `internal/config/validator.go` if `LogConfig` fields or valid values changed.
  - [x] 1.4 (Decision Point) Decide if `internal/logger/logger.go` should define a new `Logger` interface to abstract `zerolog.Logger`, or if components will use `zerolog.Logger` directly.
    - [ ] 1.4.1 If using an interface: Define `type Logger interface` with common methods (`Debug()`, `Info()`, `Error()`, `Fatal()`, `Warn()`, `WithFields(fields map[string]interface{}) Logger`, etc.).
    - [x] 1.4.2 If not using an interface, this sub-task can be skipped, and subsequent tasks will assume direct `zerolog.Logger` usage.

- [x] **2.0 Implement Centralized Logger Initialization**
  - [x] 2.1 Refactor or move `setupZeroLogger` from `cmd/monsterinc/main.go` into a new function, e.g., `logger.New(cfg config.LogConfig) (zerolog.Logger, error)` in `internal/logger/logger.go`.
    - [x] 2.1.1 This function should take `config.LogConfig` as input.
    - [x] 2.1.2 It should parse `cfg.LogLevel` to `zerolog.Level`.
    - [x] 2.1.3 It should configure `zerolog.ConsoleWriter` for `cfg.LogFormat == "console"` (with `TimeFormat: time.RFC3339`, colorized output enabled by default with ConsoleWriter).
    - [x] 2.1.4 It should configure standard JSON output if `cfg.LogFormat == "json"`.
    - [x] 2.1.5 If `cfg.LogFile` is not empty, it should configure `zerolog` to write to this file. 
        - Note: `zerolog` can write to multiple writers using `zerolog.MultiLevelWriter`. Console and file output can be combined.
        - Consider log rotation if specified in `LogConfig` (e.g. using `lumberjack.Logger` as an `io.Writer`). For PRD, focus on naming and basic file output first.
    - [x] 2.1.6 The function should return the configured `zerolog.Logger` instance.
  - [x] 2.2 Modify `cmd/monsterinc/main.go`:
    - [x] 2.2.1 Call the new `logger.New()` (or the refactored `setupZeroLogger`) using `gCfg.LogConfig` to initialize the main application logger.
    - [x] 2.2.2 Handle any error returned from the logger initialization.

- [x] **3.0 Integrate New Logger Throughout Application**
  - [x] 3.1 Identify all modules currently using `log.Logger` (standard Go log) or `zerolog.Logger` instances initialized ad-hoc.
  - [x] 3.2 Update constructors and relevant functions in these modules to accept `zerolog.Logger` (or the custom `logger.Logger` interface if defined) as a dependency.
    - [x] 3.2.1 Example Module: `internal/datastore/parquet_reader.go` (`NewParquetReader`)
    - [x] 3.2.2 Example Module: `internal/datastore/parquet_writer.go` (`NewParquetWriter`)
    - [x] 3.2.3 Example Module: `internal/scheduler/scheduler.go` (`NewScheduler`, and its methods)
    - [x] 3.2.4 Example Module: `internal/scheduler/db.go` (`NewDB`)
    - [x] 3.2.5 Example Module: `internal/orchestrator/orchestrator.go` (`NewScanOrchestrator`)
    - [x] 3.2.6 Example Module: `internal/httpxrunner/runner.go` (Update internal logging)
    - [x] 3.2.7 Example Module: `internal/crawler/crawler.go` (Update internal logging)
    - [x] 3.2.8 Example Module: `internal/notifier/discord_notifier.go` (`NewDiscordNotifier`)
    - [x] 3.2.9 Example Module: `internal/notifier/notification_helper.go` (`NewNotificationHelper`)
    - [x] 3.2.10 Example Module: `internal/reporter/html_reporter.go` (`NewHtmlReporter`)
    - [x] 3.2.11 Example Module: `internal/differ/url_differ.go` (`NewUrlDiffer`)
    - [x] 3.2.12 Example Module: `internal/config/config.go` (If any logging exists there, e.g. in `LoadGlobalConfig`)
  - [x] 3.3 Replace old logging calls with `zerolog` methods:
    - [x] 3.3.1 `log.Printf("message: %v", var)` becomes `logger.Info().Msgf("message: %v", var)` or `logger.Debug()...` etc.
    - [x] 3.3.2 `log.Fatalf(...)` becomes `logger.Fatal().Msgf(...)` (which will also exit).
    - [x] 3.3.3 `log.Println(...)` becomes `logger.Info().Msg(...)`.
  - [x] 3.4 For adding module/component context to logs:
    - [x] 3.4.1 Use `baseLogger.With().Str("module", "ModuleName").Logger()` where `baseLogger` is passed from `main` or a higher-level component.
    - [x] 3.4.2 Ensure module-specific loggers are used for all logs within that module.

- [x] **4.0 Verify Log Content and Output**
  - [x] 4.1 Run the application with `log_format: "console"` and verify:
    - [x] 4.1.1 Timestamps, log levels, module names (if implemented via `With().Str()`), and messages are present.
    - [x] 4.1.2 Output is colorized by level.
  - [x] 4.2 Run the application with `log_format: "json"` and verify:
    - [x] 4.2.1 Output is valid JSON.
    - [x] 4.2.2 JSON includes fields like "time", "level", "module" (if added), "message".
  - [x] 4.3 Run the application with `log_file` set to a specific path and verify:
    - [x] 4.3.1 The log file is created.
    - [x] 4.3.2 Log messages are written to the file without color codes.
    - [x] 4.3.3 (PRD FR5.3) Verify file naming: The current `setupZeroLogger` may need adjustment or this task might be deferred if file naming based on *application mode* (monitor vs scan) is complex with a single `LogFile` config. The PRD suggests `projectmonsterinc_YYYY-MM-DD.log` for monitor and `projectmonsterinc_scan_YYYYMMDD_HHMMSS.log` for scan. This might require logic outside `zerolog`'s basic file output, or two separate logger instances/configurations based on `gCfg.Mode`.
        - For now, ensure single file output as per `LogFile` works. Complex naming can be a follow-up.

- [x] **5.0 Test Log Output Under Error Conditions (e.g., unwritable log file)**
  - [x] 5.1 Modify `config.yaml` to set `log_file` to an unwritable path (e.g., `C:\\Windows\\System32\\NonExistentDir\\monsterinc.log` or `/dev/full` on Linux if permissions allow, or a path with no write access).
  - [x] 5.2 Confirm that `zerolog` (or the file writer wrapper like `lumberjack`) handles these errors gracefully, ideally by printing an error to `stderr` without crashing the application. (This is often default behavior for `zerolog` if the writer returns an error).

- [x] **6.0 Documentation Updates**
  - [x] 6.1 Update `config.example.yaml` with detailed explanations for `log_level`, `log_format`, `log_file`, and the log rotation settings (`max_log_size_mb`, `max_log_backups`, `compress_old_logs`).
  - [x] 6.2 Create/Update `internal/logger/README.md`:
    - [x] 6.2.1 Describe the package's purpose (centralized zerolog initialization).
    - [x] 6.2.2 Explain how to use the `logger.New(config.LogConfig)` function.
    - [x] 6.2.3 Briefly mention supported formats, levels, and file/rotation capabilities.
  - [x] 6.3 Update `cmd/monsterinc/README.md` (Logging Section):
    - [x] 6.3.1 Briefly explain how logging is configured (via `config.yaml` -> `log_config`).
    - [x] 6.3.2 Mention key features: structured logging, levels, formats, file output, rotation.
    - [x] 6.3.3 Point to `internal/logger/README.md` and `config.example.yaml` for more details.

- [x] **7.0 Refactor and Cleanup**
  - [x] 7.1 If a custom `stdLogger` or old `logger.Logger` interface existed in `internal/logger/logger.go`, remove it if it's no longer used.
  - [x] 7.2 Perform a final search through the codebase for any remaining old `log.Print/Fatal/Panic` calls (from the standard `log` package) and migrate them to the new `zerolog.Logger`. Ensure `main.go` only uses standard `log` for messages *before* `zerolog` is initialized or if `zerolog` initialization fails.
  - [x] 7.3 Ensure all `zerolog.Logger` instances are passed consistently (e.g., from `main` to orchestrator, then to sub-modules like crawler, httpxrunner) and not re-initialized unnecessarily in sub-modules. Modules should derive their specific logger using `baseLogger.With().Str("module", "ModuleName").Logger()`.

- [ ] **8.0 Unit Tests (SKIPPED)**
  - [ ] 8.1 Write unit tests for logger initialization with different configurations (levels, formats, file output).
        *   May involve capturing log output (e.g., to a buffer) and asserting its content/format.*
  - [ ] 8.2 Test contextual field logging.
  - [ ] 8.3 Test log rotation if implemented manually (can be complex to unit test reliably, might require integration tests or manual verification).

- [ ] **9.0 Documentation (SKIPPED)**
  - [ ] 9.1 Document logging conventions (e.g., when to use which log level, common contextual fields) in a `CONTRIBUTING.md` or development guide. 