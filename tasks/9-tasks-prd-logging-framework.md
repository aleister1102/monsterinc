## Relevant Files

- `internal/logger/logger.go` - Core logging interface and implementation.
- `internal/logger/console.go` - Console output handler with colorization.
- `internal/logger/file.go` - File output handler with rotation.
- `internal/config/config.go` - Configuration structure and loading logic (from configuration-management feature).
- `main.go` - Application entry point, initializes logging based on config.

### Notes

- All code should be placed in the `internal` directory to maintain proper Go package organization.

## Tasks

- [ ] 1.0 Initialize Logging Framework Core Components
  - [ ] 1.1 Define the core `Logger` interface in `internal/logger/logger.go`.
  - [ ] 1.2 Implement the main `Logger` struct in `internal/logger/logger.go`.
  - [ ] 1.3 Create log level constants and helper functions in `internal/logger/logger.go`.
  - [ ] 1.4 Implement basic log message formatting (timestamp, level, module, message) in `internal/logger/logger.go`.

- [ ] 2.0 Implement Console Logging Output
  - [ ] 2.1 Create a `ConsoleHandler` struct in `internal/logger/console.go`.
  - [ ] 2.2 Implement colorization logic for different log levels in `internal/logger/console.go`.
  - [ ] 2.3 Implement `Write` method for `ConsoleHandler` to output to `stdout`/`stderr` in `internal/logger/console.go`.

  - [ ] 2.5 Integrate `ConsoleHandler` with the core `Logger` in `internal/logger/logger.go`.
- [ ] 3.0 Implement File Logging Output with Mode-Based Naming
  - [ ] 3.1 Create a `FileHandler` struct in `internal/logger/file.go`.
  - [ ] 3.2 Implement logic to determine log file name based on operational mode (monitor/scan) in `internal/logger/file.go`.
  - [ ] 3.3 Implement file opening and writing logic in `internal/logger/file.go`.

  - [ ] 3.5 Integrate `FileHandler` with the core `Logger` in `internal/logger/logger.go`.
- [ ] 4.0 Integrate Configuration Management for Logging Settings
  - [ ] 4.1 Define logging configuration structure in `internal/config/config.go`.
  - [ ] 4.2 Implement configuration loading logic for logging settings in `internal/config/config.go`.

  - [ ] 4.4 Modify `Logger` initialization in `main.go` to use settings from the loaded configuration.
- [ ] 5.0 Implement Log Rotation Mechanism
  - [ ] 5.1 Implement logic to check log file size in `internal/logger/file.go`.
  - [ ] 5.2 Implement logic to rename/rotate log files based on size in `internal/logger/file.go`.
  - [ ] 5.3 Implement logic to delete old log files based on configuration in `internal/logger/file.go`.

- [ ] 6.0 Ensure Thread Safety and Error Handling for Logging Operations
  - [ ] 6.1 Add mutex locks to `Logger` and handlers to ensure thread safety.
  - [ ] 6.2 Implement error handling for file operations (e.g., permission errors, disk full) in `internal/logger/file.go`.
  - [ ] 6.3 Implement fallback logging to `stderr` if primary logging fails. 