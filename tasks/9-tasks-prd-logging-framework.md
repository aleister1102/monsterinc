## Relevant Files

- `internal/logger/logger.go` - Defines the `Logger` interface and the main logger implementation (e.g., using `zerolog` or `zap`).
- `internal/logger/console.go` - Console output handler with colorization.
- `internal/logger/file.go` - File output handler with rotation.
- `internal/config/config.go` - Contains `LogConfig` struct with settings like `LogLevel`, `LogFormat`, `LogFile`.
- `cmd/monsterinc/main.go` - Where the logger is initialized and passed to other components.
- `Makefile` or `scripts/` - May include commands to run the application with different log levels for testing.

### Notes

- Choose a well-established logging library (e.g., `rs/zerolog`, `uber-go/zap`) to handle low-level details like JSON formatting, leveling, and performance.
- Logging should be non-blocking or have minimal performance impact on the application.
- Ensure log messages are structured and provide sufficient context (e.g., module, function, relevant IDs).

## Tasks

- [ ] 1.0 Setup Core Logging Framework (in `internal/logger/logger.go`)
  - [ ] 1.1 Choose and add a Go logging library (e.g., `go get github.com/rs/zerolog/log` or `go.uber.org/zap`).
  - [ ] 1.2 Define a simple `Logger` interface (e.g., `Debugf`, `Infof`, `Warnf`, `Errorf`, `Fatalf`, `WithFields(fields map[string]interface{}) Logger`).
  - [ ] 1.3 Implement a wrapper struct around the chosen library that satisfies the `Logger` interface.
        *This allows swapping the underlying library later if needed.*
  - [ ] 1.4 Implement `NewLogger(cfg config.LogConfig) Logger` constructor.
  - [ ] 1.5 Configure log level (Debug, Info, Warn, Error, Fatal) based on `LogConfig.LogLevel`.
        *Implement a mapping from string config values ("debug", "info") to library-specific level constants.*
  - [ ] 1.6 Configure output format (text/console or JSON) based on `LogConfig.LogFormat`.
        *   Console format should be human-readable, possibly colorized (the library might support this).
        *   JSON format for structured logging, suitable for log management systems.*
  - [ ] 1.7 Configure output destination: `stdout` by default, or a file if `LogConfig.LogFile` is specified.

- [ ] 2.0 Implement File Logging (enhancement to `internal/logger/logger.go` or handled by chosen library)
  - [ ] 2.1 If `LogConfig.LogFile` is provided, configure the logger to write to this file.
  - [ ] 2.2 Implement log file rotation (if not handled by the chosen library directly or via an add-on like `lumberjack`).
        *   Rotation based on size (e.g., `MaxLogSizeMB` in `LogConfig`).
        *   Rotation based on time (e.g., daily - less critical for this app but good practice).
        *   Configure max old log files to keep (e.g., `MaxLogBackups` in `LogConfig`).
  - [ ] 2.3 Ensure file logging is resilient to errors (e.g., disk full, permission issues) and logs a message to `stderr` if file logging fails.

- [ ] 3.0 Implement Contextual Logging
  - [ ] 3.1 Implement the `WithFields(fields map[string]interface{}) Logger` method to add structured context to log messages.
        *Example: `logger.WithFields(log.Fields{"userID": 123, "module": "parser"}).Info("Processing item")`*
  - [ ] 3.2 Encourage consistent use of contextual fields across the application (e.g., `module`, `scan_id`, `target_url`).

- [ ] 4.0 Integrate Logging Configuration (Covered by `7-tasks-prd-configuration-management.md`)
  - [ ] 4.1 Ensure `LogConfig` in `internal/config/config.go` includes:
        *   `LogLevel string` (e.g., "debug", "info", "warn", "error", "fatal")
        *   `LogFormat string` (e.g., "console", "json")
        *   `LogFile string` (optional path to log file)
        *   `MaxLogSizeMB int` (for rotation)
        *   `MaxLogBackups int` (for rotation)
        *   `CompressOldLogs bool` (for rotation)
  - [ ] 4.2 Ensure these fields are in `config.example.json` with explanations and sensible defaults.

- [ ] 5.0 Initialize and Use Logger in Application (`cmd/monsterinc/main.go` and other packages)
  - [ ] 5.1 In `main.go`, initialize the logger using `logger.NewLogger(cfg.LogConfig)` after configuration is loaded.
  - [ ] 5.2 Make the logger instance available to other packages/modules, either by passing it as a dependency or using a global accessor (dependency injection is preferred).
  - [ ] 5.3 Replace all existing `fmt.Println`, `log.Println` calls throughout the codebase with the new logger methods.
  - [ ] 5.4 Add informative log messages at key points in the application lifecycle (e.g., app start/stop, module initialization, significant operations, errors).

- [ ] 6.0 Unit Tests
  - [ ] 6.1 Write unit tests for logger initialization with different configurations (levels, formats, file output).
        *   May involve capturing log output (e.g., to a buffer) and asserting its content/format.*
  - [ ] 6.2 Test contextual field logging.
  - [ ] 6.3 Test log rotation if implemented manually (can be complex to unit test reliably, might require integration tests or manual verification).

- [ ] 7.0 Documentation
  - [ ] 7.1 Document logging conventions (e.g., when to use which log level, common contextual fields) in a `CONTRIBUTING.md` or development guide. 