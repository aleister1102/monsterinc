package logger

import (
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// ConfigConverter converts config.LogConfig to LoggerConfig
type ConfigConverter struct {
	levelParser  *LogLevelParser
	formatParser *LogFormatParser
}

// NewConfigConverter creates a new config converter
func NewConfigConverter() *ConfigConverter {
	return &ConfigConverter{
		levelParser:  NewLogLevelParser(),
		formatParser: NewLogFormatParser(),
	}
}

// ConvertConfig converts application config to logger config
func (cc *ConfigConverter) ConvertConfig(cfg config.LogConfig) (LoggerConfig, error) {
	level, err := cc.levelParser.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel // fallback to default
	}

	format := cc.formatParser.ParseFormat(cfg.LogFormat)

	return LoggerConfig{
		Level:         level,
		Format:        format,
		EnableConsole: true,
		EnableFile:    cfg.LogFile != "",
		FilePath:      cfg.LogFile,
		MaxSizeMB:     cc.getMaxSizeMB(cfg.MaxLogSizeMB),
		MaxBackups:    cc.getMaxBackups(cfg.MaxLogBackups),
		// New fields default to empty/false - will be set by builder methods
		ScanID:     "",
		CycleID:    "",
		UseSubdirs: true, // Enable by default
	}, nil
}

// getMaxSizeMB returns max size with default fallback
func (cc *ConfigConverter) getMaxSizeMB(maxSize int) int {
	if maxSize <= 0 {
		return 100
	}
	return maxSize
}

// getMaxBackups returns max backups with default fallback
func (cc *ConfigConverter) getMaxBackups(maxBackups int) int {
	if maxBackups <= 0 {
		return 3
	}
	return maxBackups
}
