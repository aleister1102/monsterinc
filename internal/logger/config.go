package logger

import "github.com/rs/zerolog"

// LoggerConfig holds configuration for logger setup
type LoggerConfig struct {
	Level         zerolog.Level
	Format        LogFormat
	EnableConsole bool
	EnableFile    bool
	FilePath      string
	MaxSizeMB     int
	MaxBackups    int
}

// LogFormat represents available log formats
type LogFormat int

const (
	FormatJSON LogFormat = iota
	FormatConsole
	FormatText
)

// String returns string representation of LogFormat
func (lf LogFormat) String() string {
	switch lf {
	case FormatJSON:
		return "json"
	case FormatConsole:
		return "console"
	case FormatText:
		return "text"
	default:
		return "console"
	}
}

// DefaultLoggerConfig returns default logger configuration
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:         zerolog.InfoLevel,
		Format:        FormatConsole,
		EnableConsole: true,
		EnableFile:    false,
		MaxSizeMB:     100,
		MaxBackups:    3,
	}
}
