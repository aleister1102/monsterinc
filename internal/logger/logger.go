package logger

import (
	"github.com/aleister1102/monsterinc/internal/config"

	"github.com/rs/zerolog"
)

// Logger represents the main logger with configuration
type Logger struct {
	zerolog zerolog.Logger
	config  LoggerConfig
}

// GetZerolog returns the underlying zerolog instance
func (l *Logger) GetZerolog() *zerolog.Logger {
	return &l.zerolog
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
