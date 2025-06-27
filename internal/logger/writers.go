package logger

import (
	"io"
	"time"

	"github.com/rs/zerolog"
)

// WriterStrategy defines interface for creating log writers
type WriterStrategy interface {
	CreateWriter(output io.Writer) io.Writer
}

// JSONWriterStrategy creates JSON formatted writers
type JSONWriterStrategy struct{}

// CreateWriter creates a JSON writer
func (jws *JSONWriterStrategy) CreateWriter(output io.Writer) io.Writer {
	return output
}

// ConsoleWriterStrategy creates console formatted writers
type ConsoleWriterStrategy struct {
	NoColor bool
}

// CreateWriter creates a console writer
func (cws *ConsoleWriterStrategy) CreateWriter(output io.Writer) io.Writer {
	return zerolog.ConsoleWriter{
		Out:        output,
		TimeFormat: time.RFC3339,
		NoColor:    cws.NoColor,
	}
}

// TextWriterStrategy creates text formatted writers
type TextWriterStrategy struct{}

// CreateWriter creates a text writer
func (tws *TextWriterStrategy) CreateWriter(output io.Writer) io.Writer {
	return zerolog.ConsoleWriter{
		Out:        output,
		TimeFormat: time.RFC3339,
		NoColor:    true,
	}
}
