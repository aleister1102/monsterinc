package logger

import (
	"strings"

	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/rs/zerolog"
)

// LogLevelParser handles parsing of log levels
type LogLevelParser struct{}

// NewLogLevelParser creates a new log level parser
func NewLogLevelParser() *LogLevelParser {
	return &LogLevelParser{}
}

// ParseLevel parses string log level to zerolog.Level
func (llp *LogLevelParser) ParseLevel(levelStr string) (zerolog.Level, error) {
	level, err := zerolog.ParseLevel(strings.ToLower(levelStr))
	if err != nil {
		return zerolog.InfoLevel, errors.WrapError(err, "invalid log level")
	}
	return level, nil
}

// LogFormatParser handles parsing of log formats
type LogFormatParser struct{}

// NewLogFormatParser creates a new log format parser
func NewLogFormatParser() *LogFormatParser {
	return &LogFormatParser{}
}

// ParseFormat parses string format to LogFormat
func (lfp *LogFormatParser) ParseFormat(formatStr string) LogFormat {
	switch strings.ToLower(formatStr) {
	case "json":
		return FormatJSON
	case "console":
		return FormatConsole
	case "text":
		return FormatText
	default:
		return FormatConsole
	}
}
