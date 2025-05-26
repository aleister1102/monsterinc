# Logger Package (`internal/logger`)

## Overview

The `logger` package is responsible for initializing and configuring the application-wide logger using the `zerolog` library. It provides a centralized way to set up logging based on the application's configuration (`config.LogConfig`).

## Features

-   **Structured Logging**: Utilizes `zerolog` for fast, structured JSON logging.
-   **Configurable Log Levels**: Supports standard log levels (debug, info, warn, error, fatal, panic) configurable via `log_config.log_level`.
-   **Configurable Output Formats**:
    -   `console`: Human-readable, colorized output suitable for development and interactive terminals.
    -   `json`: Machine-readable JSON output, ideal for log aggregation and processing systems.
    -   `text`: Plain text output, similar to `console` but without color codes.
-   **File Logging**: Optionally logs to a specified file (`log_config.log_file`).
-   **Log Rotation**: Implements log rotation for file-based logging using `gopkg.in/natefinch/lumberjack.v2`, with configurable:
    -   `max_log_size_mb`: Maximum size of a log file before rotation.
    -   `max_log_backups`: Maximum number of old log files to retain.
    -   `compress_old_logs`: Whether to compress rotated log files.
-   **Multi-Writer Support**: Can log to both console (stderr) and a file simultaneously.
-   **Standard Log Redirection**: Redirects Go's standard `log` package output to `zerolog`, ensuring all logs go through the configured pipeline.

## Initialization

The main application logger is initialized in `cmd/monsterinc/main.go` by calling `logger.New(gCfg.LogConfig)`.

```go
// In cmd/monsterinc/main.go
import (
    "monsterinc/internal/config"
    "monsterinc/internal/logger"
    "github.com/rs/zerolog"
)

func main() {
    // ... load gCfg *config.GlobalConfig ...

    zLogger, err := logger.New(gCfg.LogConfig)
    if err != nil {
        log.Fatalf("[FATAL] Main: Could not initialize logger: %v", err)
    }
    zLogger.Info().Msg("Logger initialized successfully.")

    // ... rest of the application ...
}
```

## Usage in Modules

Modules should accept a `zerolog.Logger` instance (typically the one initialized in `main.go` or a derivative) through their constructor or as a parameter to functions that require logging.

To create a module-specific logger with added context (e.g., module name), use `zerolog`'s `With()` method:

```go
package mymodule

import (
    "github.com/rs/zerolog"
)

type MyModule struct {
    logger zerolog.Logger
    // ... other fields
}

func NewMyModule(appLogger zerolog.Logger /*, other dependencies */) *MyModule {
    moduleLogger := appLogger.With().Str("module", "MyModule").Logger()
    // Now use moduleLogger for logging within MyModule
    moduleLogger.Info().Msg("MyModule initialized")
    return &MyModule{
        logger: moduleLogger,
        // ...
    }
}

func (m *MyModule) DoSomething() {
    m.logger.Debug().Str("action", "DoSomething").Msg("Performing an action...")
    // ... logic ...
    m.logger.Info().Msg("Action completed.")
}
```

This practice ensures that log messages from different parts of the application can be easily identified and filtered based on their context.

## Configuration

Refer to the `log_config` section in `config.example.yaml` for all available logging configuration options. 