package logger

import (
	"io"
	stdlog "log" // Standard Go log package, aliased to avoid conflict with zerolog field

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"

	"github.com/rs/zerolog"
)

// Logger represents the main logger with configuration
type Logger struct {
	zerolog   zerolog.Logger
	config    LoggerConfig
	factory   *WriterFactory
	converter *ConfigConverter
}

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

// WithLevel sets the log level
func (lb *LoggerBuilder) WithLevel(level zerolog.Level) *LoggerBuilder {
	lb.config.Level = level
	return lb
}

// WithFormat sets the log format
func (lb *LoggerBuilder) WithFormat(format LogFormat) *LoggerBuilder {
	lb.config.Format = format
	return lb
}

// WithFile enables file logging
func (lb *LoggerBuilder) WithFile(filePath string, maxSizeMB, maxBackups int) *LoggerBuilder {
	lb.config.EnableFile = true
	lb.config.FilePath = filePath
	lb.config.MaxSizeMB = maxSizeMB
	lb.config.MaxBackups = maxBackups
	return lb
}

// WithScanID sets the scan ID for organizing logs by scan session
func (lb *LoggerBuilder) WithScanID(scanID string) *LoggerBuilder {
	lb.config.ScanID = scanID
	return lb
}

// WithCycleID sets the cycle ID for organizing logs by monitor cycle
func (lb *LoggerBuilder) WithCycleID(cycleID string) *LoggerBuilder {
	lb.config.CycleID = cycleID
	return lb
}

// WithSubdirs enables/disables subdirectory organization
func (lb *LoggerBuilder) WithSubdirs(enabled bool) *LoggerBuilder {
	lb.config.UseSubdirs = enabled
	return lb
}

// WithConsole enables/disables console logging
func (lb *LoggerBuilder) WithConsole(enabled bool) *LoggerBuilder {
	lb.config.EnableConsole = enabled
	return lb
}

// Build creates the logger instance
func (lb *LoggerBuilder) Build() (*Logger, error) {
	if err := lb.validateConfig(); err != nil {
		return nil, err
	}

	writers := lb.createWriters()
	if len(writers) == 0 {
		return nil, common.NewError("no output writers configured")
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
		zerolog:   zerologInstance,
		config:    lb.config,
		factory:   lb.factory,
		converter: lb.converter,
	}

	return logger, nil
}

// validateConfig validates the logger configuration
func (lb *LoggerBuilder) validateConfig() error {
	if lb.config.EnableFile && lb.config.FilePath == "" {
		return common.NewValidationError("file_path", lb.config.FilePath, "file path required when file logging enabled")
	}

	if lb.config.MaxSizeMB <= 0 {
		return common.NewValidationError("max_size_mb", lb.config.MaxSizeMB, "max size must be positive")
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

// GetZerolog returns the underlying zerolog instance
func (l *Logger) GetZerolog() *zerolog.Logger {
	return &l.zerolog
}

// GetConfig returns the logger configuration
func (l *Logger) GetConfig() LoggerConfig {
	return l.config
}

// Reconfigure reconfigures the logger with new settings
func (l *Logger) Reconfigure(cfg config.LogConfig) error {
	newConfig, err := l.converter.ConvertConfig(cfg)
	if err != nil {
		return common.WrapError(err, "failed to convert config")
	}

	builder := NewLoggerBuilder().
		WithLevel(newConfig.Level).
		WithFormat(newConfig.Format).
		WithConsole(newConfig.EnableConsole)

	if newConfig.EnableFile {
		builder = builder.WithFile(newConfig.FilePath, newConfig.MaxSizeMB, newConfig.MaxBackups)
	}

	newLogger, err := builder.Build()
	if err != nil {
		return common.WrapError(err, "failed to build new logger")
	}

	// Update current logger
	l.zerolog = *newLogger.GetZerolog()
	l.config = newLogger.config

	return nil
}

// New creates a new logger instance - maintains backward compatibility
func New(cfg config.LogConfig) (zerolog.Logger, error) {
	logger, err := NewLoggerBuilder().WithConfig(cfg).Build()
	if err != nil {
		return zerolog.Logger{}, err
	}
	return *logger.GetZerolog(), nil
}

// NewWithScanID creates a new logger instance with scan ID for organizing logs
func NewWithScanID(cfg config.LogConfig, scanID string) (zerolog.Logger, error) {
	logger, err := NewLoggerBuilder().
		WithConfig(cfg).
		WithScanID(scanID).
		Build()
	if err != nil {
		return zerolog.Logger{}, err
	}
	return *logger.GetZerolog(), nil
}

// NewWithCycleID creates a new logger instance with cycle ID for organizing logs
func NewWithCycleID(cfg config.LogConfig, cycleID string) (zerolog.Logger, error) {
	logger, err := NewLoggerBuilder().
		WithConfig(cfg).
		WithCycleID(cycleID).
		Build()
	if err != nil {
		return zerolog.Logger{}, err
	}
	return *logger.GetZerolog(), nil
}

// NewWithContext creates a new logger instance with context-based ID
func NewWithContext(cfg config.LogConfig, scanID, cycleID string) (zerolog.Logger, error) {
	builder := NewLoggerBuilder().WithConfig(cfg)

	if scanID != "" {
		builder = builder.WithScanID(scanID)
	}
	if cycleID != "" {
		builder = builder.WithCycleID(cycleID)
	}

	logger, err := builder.Build()
	if err != nil {
		return zerolog.Logger{}, err
	}
	return *logger.GetZerolog(), nil
}
