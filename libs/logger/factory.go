package logger

import (
	"io"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// WriterFactory creates writers based on format
type WriterFactory struct {
	strategies map[LogFormat]WriterStrategy
}

// NewWriterFactory creates a new writer factory
func NewWriterFactory() *WriterFactory {
	return &WriterFactory{
		strategies: map[LogFormat]WriterStrategy{
			FormatJSON:    &JSONWriterStrategy{},
			FormatConsole: &ConsoleWriterStrategy{NoColor: false},
			FormatText:    &TextWriterStrategy{},
		},
	}
}

// CreateConsoleWriter creates a console writer
func (wf *WriterFactory) CreateConsoleWriter(format LogFormat) io.Writer {
	strategy, exists := wf.strategies[format]
	if !exists {
		strategy = &ConsoleWriterStrategy{NoColor: false}
	}
	return strategy.CreateWriter(os.Stderr)
}

// CreateFileWriter creates a file writer with rotation and directory organization
func (wf *WriterFactory) CreateFileWriter(config LoggerConfig) io.Writer {
	finalPath := wf.buildLogPath(config)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		// If directory creation fails, use original path
		finalPath = config.FilePath
	}

	lumberjackLogger := &lumberjack.Logger{
		Filename:   finalPath,
		MaxSize:    config.MaxSizeMB,
		LocalTime:  true,
		MaxBackups: config.MaxBackups,
	}

	strategy, exists := wf.strategies[config.Format]
	if !exists {
		strategy = &JSONWriterStrategy{}
	}

	if config.Format == FormatConsole {
		return (&ConsoleWriterStrategy{NoColor: true}).CreateWriter(lumberjackLogger)
	}

	return strategy.CreateWriter(lumberjackLogger)
}

// buildLogPath constructs the final log file path with subdirectories if enabled
func (wf *WriterFactory) buildLogPath(config LoggerConfig) string {
	if !config.UseSubdirs {
		return config.FilePath
	}

	baseDir := filepath.Dir(config.FilePath)
	fileName := filepath.Base(config.FilePath)

	// Determine subdirectory based on scan ID or cycle ID
	var subDir string
	if config.ScanID != "" {
		subDir = filepath.Join(baseDir, "scans", config.ScanID)
	} else if config.CycleID != "" {
		subDir = filepath.Join(baseDir, "monitors", config.CycleID)
	} else {
		// No ID specified, use original path
		return config.FilePath
	}

	return filepath.Join(subDir, fileName)
}
