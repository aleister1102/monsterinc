package logger

import (
	"io"
	stdlog "log" // Standard Go log package, aliased to avoid conflict with zerolog field
	"os"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"

	"github.com/rs/zerolog"
	// "github.com/rs/zerolog/log" // Removed to avoid conflict
	"gopkg.in/natefinch/lumberjack.v2"
)

// New initializes a new zerolog.Logger instance based on the provided LogConfig.
func New(cfg config.LogConfig) (zerolog.Logger, error) {
	var finalLogger zerolog.Logger
	var outputWriters []io.Writer

	// Preliminary logger for setup issues (before finalLogger is fully configured)
	prelimLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Set log level
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		prelimLogger.Warn().Str("provided_level", cfg.LogLevel).Err(err).Msg("Invalid log level, defaulting to 'info'")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level) // This affects the global level for all zerolog instances if not overridden

	// Configure console writer (always to stderr for primary console output)
	var consoleWriter io.Writer
	switch strings.ToLower(cfg.LogFormat) {
	case "console":
		consoleWriter = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	case "json":
		consoleWriter = os.Stderr // Raw JSON to stderr
	case "text":
		consoleWriter = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			NoColor:    true, // Plain text is console writer without color
			// PartsExclude: []string{zerolog.CallerFieldName}, // Optionally exclude caller for cleaner text
		}
	default:
		prelimLogger.Warn().Str("provided_format", cfg.LogFormat).Msg("Unknown log format, defaulting to 'console'")
		cfg.LogFormat = "console" // Correct the format for subsequent logic
		consoleWriter = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}
	// Only add consoleWriter to outputWriters if LogFile is not set OR if LogFile is set but we want duplicate console output.
	// Current behavior: if LogFile is set, console output still happens based on cfg.LogFormat.
	// If cfg.LogFile is "" (empty), then consoleWriter is the ONLY writer unless LogFormat was json (raw to stderr).
	// This logic seems to ensure consoleWriter is always considered for stderr.
	outputWriters = append(outputWriters, consoleWriter)

	// Configure file writer if LogFile is specified
	if cfg.LogFile != "" {
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    cfg.MaxLogSizeMB,
			MaxBackups: cfg.MaxLogBackups,
			LocalTime:  true,
		}

		var fileLogTargetWriter io.Writer = lumberjackLogger // Default to lumberjack for JSON
		switch strings.ToLower(cfg.LogFormat) {
		case "console":
			fileLogTargetWriter = zerolog.ConsoleWriter{Out: lumberjackLogger, TimeFormat: time.RFC3339, NoColor: true}
		case "text":
			fileLogTargetWriter = zerolog.ConsoleWriter{Out: lumberjackLogger, TimeFormat: time.RFC3339, NoColor: true /* PartsExclude: []string{zerolog.CallerFieldName} */}
			// case "json": // Default, lumberjackLogger is already set for JSON output by zerolog.New()
		}
		outputWriters = append(outputWriters, fileLogTargetWriter)
	}

	if len(outputWriters) == 0 {
		// This should ideally not be reached if consoleWriter is always added to outputWriters initially.
		// However, as a safeguard:
		prelimLogger.Error().Msg("No output writers configured for logger, defaulting to stderr for finalLogger.")
		outputWriters = append(outputWriters, os.Stderr)
	}

	multiWriter := zerolog.MultiLevelWriter(outputWriters...)
	finalLogger = zerolog.New(multiWriter).Level(level).With().Timestamp().Logger() // Ensure level is set on the final logger

	// Replace standard log package's output
	stdlog.SetOutput(finalLogger) // Redirect Go's standard log to zerolog
	stdlog.SetFlags(0)            // Disable standard log's prefix and timestamp, as zerolog handles it

	return finalLogger, nil
}
