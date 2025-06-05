package logger

import (
	"io"
	"os"

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

// CreateFileWriter creates a file writer with rotation
func (wf *WriterFactory) CreateFileWriter(config LoggerConfig) io.Writer {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   config.FilePath,
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
