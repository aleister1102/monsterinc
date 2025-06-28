package logger

import (
	"io"
	stdlog "log" // Standard Go log package, aliased to avoid conflict with zerolog field

	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// LoggerBuilder provides fluent interface for building loggers
type LoggerBuilder struct {
	config    LoggerConfig
	factory   *WriterFactory
	converter *ConfigConverter
}

// NewLoggerBuilder creates a new logger builder
func NewLoggerBuilder() *LoggerBuilder {
	return &LoggerBuilder{
		config:    DefaultLoggerConfig(),
		factory:   NewWriterFactory(),
		converter: NewConfigConverter(),
	}
}

// WithConfig sets the logger configuration
func (lb *LoggerBuilder) WithConfig(cfg config.LogConfig) *LoggerBuilder {
	loggerConfig, _ := lb.converter.ConvertConfig(cfg)
	lb.config = loggerConfig
	return lb
}

// WithScanID sets the scan ID for organizing logs by scan session
func (lb *LoggerBuilder) WithScanID(scanID string) *LoggerBuilder {
	lb.config.ScanID = scanID
	return lb
}

// Build creates the logger instance
func (lb *LoggerBuilder) Build() (*Logger, error) {
	if err := lb.validateConfig(); err != nil {
		return nil, err
	}

	writers := lb.createWriters()
	if len(writers) == 0 {
		return nil, errors.NewError("no output writers configured")
	}

	multiWriter := zerolog.MultiLevelWriter(writers...)
	zerologInstance := zerolog.New(multiWriter).
		Level(lb.config.Level).
		With().
		Timestamp().
		Logger()

	// Configure global settings
	zerolog.SetGlobalLevel(lb.config.Level)
	lb.configureStandardLog(zerologInstance)

	logger := &Logger{
		zerolog: zerologInstance,
		config:  lb.config,
	}

	return logger, nil
}

// validateConfig validates the logger configuration
func (lb *LoggerBuilder) validateConfig() error {
	if lb.config.EnableFile && lb.config.FilePath == "" {
		return errors.NewValidationError("file_path", lb.config.FilePath, "file path required when file logging enabled")
	}

	if lb.config.MaxSizeMB <= 0 {
		return errors.NewValidationError("max_size_mb", lb.config.MaxSizeMB, "max size must be positive")
	}

	return nil
}

// createWriters creates the appropriate writers based on configuration
func (lb *LoggerBuilder) createWriters() []io.Writer {
	var writers []io.Writer

	if lb.config.EnableConsole {
		consoleWriter := lb.factory.CreateConsoleWriter(lb.config.Format)
		writers = append(writers, consoleWriter)
	}

	if lb.config.EnableFile {
		fileWriter := lb.factory.CreateFileWriter(lb.config)
		writers = append(writers, fileWriter)
	}

	return writers
}

// configureStandardLog configures standard Go log package
func (lb *LoggerBuilder) configureStandardLog(logger zerolog.Logger) {
	stdlog.SetOutput(logger)
	stdlog.SetFlags(0)
}
