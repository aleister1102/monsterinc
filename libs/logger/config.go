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
	// New fields for organizing logs by scan/cycle
	ScanID     string // Scan session ID for organizing scan logs
	CycleID    string // Monitor cycle ID for organizing monitor logs
	UseSubdirs bool   // Whether to create subdirectories based on scan/cycle ID
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
		UseSubdirs:    true, // Enable subdirectories by default
	}
}
