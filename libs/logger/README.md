# MonsterInc Logger

This library provides a flexible and powerful logging solution for Go applications, built on top of the excellent [zerolog](https://github.com/rs/zerolog) library.

## Features

- **Multiple Log Formats**: Supports JSON, console-friendly, and simple text formats.
- **File and Console Output**: Log to both console and rotating log files simultaneously.
- **Fluent Builder API**: Easily construct logger instances with a chained builder pattern.
- **Contextual Logging**: Automatically add contextual information like timestamps.
- **Log Rotation**: Built-in support for log rotation based on size.
- **Independent Module**: Designed as a standalone library with no dependencies on the main `monsterinc` application.

## Installation

```bash
go get github.com/monsterinc/logger
```

## Usage

### Basic Initialization

```go
package main

import (
	"github.com/monsterinc/logger"
	"github.com/rs/zerolog"
)

func main() {
	// Create a new logger with default settings (console output, info level)
	log, err := logger.NewLoggerBuilder().Build()
	if err != nil {
		panic(err)
	}

	log.GetZerolog().Info().Msg("Hello, from the MonsterInc logger!")
}
```

### Advanced Configuration

```go
package main

import (
	"github.com/monsterinc/logger"
	"github.com/rs/zerolog"
)

func main() {
	// Use the builder to configure the logger
	log, err := logger.NewLoggerBuilder().
		WithLevel(zerolog.DebugLevel).
		WithFormat(logger.FormatJSON).
		WithFile("app.log", 100, 3). // Log to app.log, max size 100MB, 3 backups
		WithConsole(true).
		Build()

	if err != nil {
		panic(err)
	}

	log.GetZerolog().Debug().
		Str("component", "main").
		Msg("This is a debug message.")
}
```

### Configuration from File

You can also configure the logger from a configuration file.

```go
package main

import (
	"github.com/monsterinc/logger"
)

func main() {
	fileConfig := logger.FileLogConfig{
		LogLevel:      "info",
		LogFormat:     "json",
		LogFile:       "app.log",
		MaxLogSizeMB:  50,
		MaxLogBackups: 5,
	}

	log, err := logger.NewLoggerBuilder().
		WithFileConfig(fileConfig).
		Build()

	if err != nil {
		panic(err)
	}

	log.GetZerolog().Info().Msg("Logger configured from file config.")
}
```

## Configuration

### YAML Configuration

```yaml
log_config:
  log_level: "info"         # trace, debug, info, warn, error, fatal, panic
  log_format: "json"        # json, console, text
  log_file: "app.log"       # File path for log output
  max_log_size_mb: 100      # Maximum log file size in MB
  max_log_backups: 5        # Number of backup files to keep
```

### Configuration Options

- **`log_level`**: Minimum log level (trace, debug, info, warn, error, fatal, panic)
- **`log_format`**: Output format (json, console, text)
- **`log_file`**: File path for log output (optional)
- **`max_log_size_mb`**: Maximum size before rotation
- **`max_log_backups`**: Number of backup files to retain

## Components

### 1. LoggerBuilder

Fluent interface for constructing loggers:

```go
builder := logger.NewLoggerBuilder()

logger, err := builder.
    WithLevel(zerolog.InfoLevel).
    WithFormat(logger.ConsoleFormat).
    WithFile("app.log", 100, 5).
    WithConsole(true).
    Build()
```

**Methods:**
- `WithConfig(cfg config.LogConfig)` - Apply configuration
- `WithLevel(level zerolog.Level)` - Set log level
- `WithFormat(format LogFormat)` - Set output format
- `WithFile(path, sizeMB, backups)` - Enable file output
- `WithConsole(enabled)` - Enable/disable console output
- `Build()` - Create the logger instance

### 2. ConfigConverter

Converts configuration to internal types:

```go
converter := logger.NewConfigConverter()
internalConfig, err := converter.ConvertConfig(configLogConfig)
```

**Features:**
- Log level parsing and validation
- Format parsing and validation
- Default value application
- Configuration validation

### 3. WriterFactory

Creates output writers based on format:

```go
factory := logger.NewWriterFactory()

// Console writer
consoleWriter := factory.CreateConsoleWriter(logger.JSONFormat)

// File writer with rotation
fileWriter := factory.CreateFileWriter(loggerConfig)
```

### 4. Writer Strategies

Different output formatting strategies:

#### JSONWriterStrategy
```go
// Produces structured JSON logs
{"level":"info","time":"2024-01-01T12:00:00Z","message":"Application started"}
```

#### ConsoleWriterStrategy
```go
// Produces colored, human-readable console output
12:00:00 INF Application started component=main
```

#### TextWriterStrategy
```go
// Produces plain text output
2024-01-01T12:00:00Z INF Application started component=main
```

## Advanced Usage

### Custom Writer Strategies

```go
// Implement custom writer strategy
type CustomWriterStrategy struct{}

func (cws *CustomWriterStrategy) CreateWriter(output io.Writer) io.Writer {
    // Custom formatting logic
    return zerolog.ConsoleWriter{
        Out:        output,
        TimeFormat: "15:04:05",
        // Custom formatting
    }
}

// Register with factory
factory := logger.NewWriterFactory()
factory.RegisterStrategy("custom", &CustomWriterStrategy{})
```

### Multiple Outputs

```go
// Log to both console and file
logger, err := logger.NewLoggerBuilder().
    WithLevel(zerolog.InfoLevel).
    WithFormat(logger.JSONFormat).
    WithFile("app.log", 100, 5).
    WithConsole(true).
    Build()
```

### Runtime Reconfiguration

```go
// Change logger configuration at runtime
newConfig := config.LogConfig{
    LogLevel: "debug",
    LogFormat: "console",
}

err := logger.Reconfigure(newConfig)
if err != nil {
    log.Printf("Failed to reconfigure logger: %v", err)
}
```

## Integration Examples

### With Scanner Service

```go
// Initialize logger for scanner
scannerLogger, err := logger.New(cfg.LogConfig)
if err != nil {
    return fmt.Errorf("logger init failed: %w", err)
}

scanner := scanner.NewScanner(cfg, scannerLogger)
```

### With Monitor Service

```go
// Monitor-specific logger with file output
monitorLogger, err := logger.NewLoggerBuilder().
    WithConfig(cfg.LogConfig).
    WithFile("monitor.log", 50, 3).
    Build()

monitor := monitor.NewMonitoringService(cfg, monitorLogger.GetZerolog(), helper)
```

### With Error Handling

```go
logger.Error().
    Err(err).
    Str("component", "scanner").
    Str("operation", "target_processing").
    Msg("Failed to process target")
```

## Log Levels

### Level Hierarchy
1. **trace** - Very detailed debugging information
2. **debug** - Debugging information
3. **info** - General information
4. **warn** - Warning messages
5. **error** - Error conditions
6. **fatal** - Fatal errors (calls os.Exit)
7. **panic** - Panic level (calls panic())

### Usage Guidelines

```go
// Trace - Very detailed debugging
logger.Trace().Str("url", url).Msg("Processing URL")

// Debug - Development debugging
logger.Debug().Int("count", count).Msg("Processing batch")

// Info - General information
logger.Info().Str("mode", "scan").Msg("Starting operation")

// Warn - Concerning but not critical
logger.Warn().Str("file", path).Msg("File not found, using default")

// Error - Error conditions
logger.Error().Err(err).Msg("Operation failed")

// Fatal - Application should exit
logger.Fatal().Err(err).Msg("Critical system failure")
```

## Performance Considerations

### Structured Logging
- Use structured fields instead of string formatting
- Pre-allocate context loggers for components
- Use appropriate log levels to control overhead

### File I/O
- File rotation handled automatically
- Buffered writes for performance
- Atomic file operations during rotation

### Memory Usage
- Minimal allocation overhead with zerolog
- Efficient JSON marshaling
- Context reuse for repeated fields

## Error Handling

### Configuration Errors
```go
logger, err := logger.New(cfg)
if err != nil {
    // Handle invalid configuration
    log.Fatal("Logger configuration invalid:", err)
}
```

### Runtime Errors
- File permission issues handled gracefully
- Automatic fallback to stderr if file unavailable
- Non-blocking error reporting

## Dependencies

- **github.com/rs/zerolog** - Core logging library
- **gopkg.in/natefinch/lumberjack.v2** - Log rotation
- **github.com/aleister1102/monsterinc/internal/config** - Configuration types

## Thread Safety

- All logger operations are thread-safe
- Concurrent access to log files handled properly
- Factory and builder patterns support concurrent usage
- File rotation is atomic and thread-safe

## Logger với Scan ID và Cycle ID

Logger hiện tại hỗ trợ tổ chức log files theo scan ID và monitor cycle ID để tránh conflict khi nhiều process ghi vào cùng file log.

### Cấu trúc thư mục

```
logs/
├── scans/
│   ├── 20250607-132708/
│   │   └── monsterinc.log
│   ├── 20250607-140520/
│   │   └── monsterinc.log
│   └── ...
├── monitors/
│   ├── monitor-20250607-132708/
│   │   └── monsterinc.log
│   ├── monitor-20250607-140530/
│   │   └── monsterinc.log
│   └── ...
└── monsterinc.log  # fallback nếu không có scan/cycle ID
```

### Sử dụng

#### Tạo logger với Scan ID

```go
import "github.com/aleister1102/monsterinc/internal/logger"

// Tạo logger cho scan session
scanLogger, err := logger.NewWithScanID(config.LogConfig, "20250607-132708")
if err != nil {
    // Handle error
}

// Hoặc sử dụng builder pattern
logger, err := logger.NewLoggerBuilder().
    WithConfig(cfg).
    WithScanID("20250607-132708").
    Build()
```

#### Tạo logger với Cycle ID

```go
// Tạo logger cho monitor cycle
cycleLogger, err := logger.NewWithCycleID(config.LogConfig, "monitor-20250607-132708")
if err != nil {
    // Handle error
}

// Hoặc sử dụng builder pattern
logger, err := logger.NewLoggerBuilder().
    WithConfig(cfg).
    WithCycleID("monitor-20250607-132708").
    Build()
```

#### Tạo logger với context

```go
// Tạo logger dựa trên context có sẵn scan ID hoặc cycle ID
logger, err := logger.NewWithContext(config.LogConfig, scanID, cycleID)
```

### Cấu hình

Để enable/disable tính năng subdirectory:

```go
logger, err := logger.NewLoggerBuilder().
    WithConfig(cfg).
    WithScanID("scan-123").
    WithSubdirs(true). // hoặc false để disable
    Build()
```

### Tích hợp với Scheduler và Monitor

#### Scheduler sử dụng scan ID
Trong scheduler, mỗi scan cycle sẽ tự động tạo logger riêng:

```go
func (s *Scheduler) executeScanCycle(ctx context.Context, scanSessionID string, ...) {
    // Tự động tạo logger với scan ID
    scanLogger, err := logger.NewWithScanID(s.globalConfig.LogConfig, scanSessionID)
    // Log files sẽ được lưu trong logs/scans/{scanSessionID}/
}
```

#### Monitor sử dụng cycle ID
Trong monitor service, mỗi cycle sẽ có logger riêng:

```go
func (s *MonitoringService) GenerateNewCycleID() string {
    newCycleID := s.createCycleID()
    // Tự động tạo logger với cycle ID
    cycleLogger, err := logger.NewWithCycleID(s.gCfg.LogConfig, newCycleID)
    // Log files sẽ được lưu trong logs/monitors/{cycleID}/
}
```

### Lợi ích

1. **Tránh conflict**: Mỗi scan/cycle có file log riêng
2. **Dễ debug**: Log được tổ chức theo session/cycle
3. **Không bị ghi đè**: Không có vấn đề với log rotation khi nhiều process chạy song song
4. **Dễ quản lý**: Có thể dễ dàng xóa log của session/cycle cụ thể