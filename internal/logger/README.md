# go-logbook

A flexible and easy-to-use logging library for Go, built on top of [zerolog](https://github.com/rs/zerolog).

## Features

- **Fluent Builder API**: Simple and intuitive builder pattern for logger configuration.
- **Multiple Output Formats**: Supports JSON, human-readable console, and plain text formats.
- **File & Console Logging**: Log to standard output, files, or both.
- **Automatic Log Rotation**: Built-in support for log rotation using [lumberjack](https://github.com/natefinch/lumberjack).
- **Contextual Logging**: Organize logs into subdirectories based on `ScanID` or `CycleID`.
- **Dynamic Reconfiguration**: Change logger settings on the fly.

## Installation

```sh
go get github.com/monsterinc/go-logbook
```

## Usage

### Basic Example

Here's how to create a simple console logger:

```go
package main

import (
	"github.com/monsterinc/go-logbook"
	"github.com/rs/zerolog"
)

func main() {
	logger, err := logger.NewLoggerBuilder().
		WithLevel(zerolog.DebugLevel).
		WithFormat(logger.FormatConsole).
		Build()

	if err != nil {
		panic(err)
	}

	logger.GetZerolog().Info().Msg("Hello, go-logbook!")
	// Output: 10:00AM INF Hello, go-logbook!
}
```

### File Logging with Rotation

To log to a file with automatic rotation:

```go
package main

import (
	"github.com/monsterinc/go-logbook"
	"github.com/rs/zerolog"
)

func main() {
	logger, err := logger.NewLoggerBuilder().
		WithFile("app.log", 10, 3). // 10 MB max size, 3 backups
		WithFormat(logger.FormatJSON).
		WithConsole(false). // Disable console output
		Build()

	if err != nil {
		panic(err)
	}
	defer logger.Close() // Important: Close the file handle on exit

	logger.GetZerolog().Info().
		Str("service", "auth").
		Msg("User successfully logged in")
	// Output in app.log: {"level":"info","service":"auth","time":"...","message":"User successfully logged in"}
}
```

### Dynamic Reconfiguration

You can reconfigure the logger at runtime.

```go
// ... (initial logger setup)

// Reconfigure to change log level and output file
newConfig := logger.FileLogConfig{
    LogLevel:  "debug",
    LogFormat: "text",
    LogFile:   "new_app.log",
}

err := logger.Reconfigure(newConfig)
if err != nil {
    // handle error
}

logger.GetZerolog().Debug().Msg("This is a debug message after reconfiguration.")
```

## Configuration

The `LoggerBuilder` provides a fluent API for configuration:

- `WithLevel(zerolog.Level)`: Sets the minimum log level (e.g., `zerolog.DebugLevel`).
- `WithFormat(logger.LogFormat)`: Sets the output format (`FormatJSON`, `FormatConsole`, `FormatText`).
- `WithFile(path, maxSizeMB, maxBackups)`: Enables file logging with rotation.
- `WithConsole(enabled bool)`: Toggles console logging.
- `WithScanID(id string)`: Organizes logs in a `scans/{id}` subdirectory.
- `WithCycleID(id string)`: Organizes logs in a `monitors/{id}` subdirectory.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License.