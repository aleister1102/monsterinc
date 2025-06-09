package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// FileReader handles file reading operations
type FileReader struct {
	logger    zerolog.Logger
	validator *FileValidator
}

// NewFileReader creates a new FileReader instance
func NewFileReader(logger zerolog.Logger) *FileReader {
	componentLogger := logger.With().Str("component", "FileReader").Logger()
	return &FileReader{
		logger:    componentLogger,
		validator: NewFileValidator(componentLogger),
	}
}

// ReadFile reads a file with the given options
func (fr *FileReader) ReadFile(path string, opts FileReadOptions) ([]byte, error) {
	// Validate file and options
	_, err := fr.validator.ValidateFileForReading(path, opts)
	if err != nil {
		return nil, err
	}

	// Set up context with timeout if specified
	ctx, cancel := fr.setupContextWithTimeout(opts)
	if cancel != nil {
		defer cancel()
	}

	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, WrapError(err, fmt.Sprintf("failed to open file: %s", path))
	}
	defer func() {
		err := file.Close()
		if err != nil {
			fr.logger.Error().Err(err).Str("path", path).Msg("Failed to close file.")
		}
	}()

	// Create buffered reader if buffer size is specified
	reader := fr.createBufferedReader(file, opts.BufferSize)

	// Perform file read with context support
	content, err := fr.performFileRead(path, reader, opts, ctx)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// setupContextWithTimeout sets up context with timeout if specified
func (fr *FileReader) setupContextWithTimeout(opts FileReadOptions) (context.Context, context.CancelFunc) {
	ctx := opts.Context
	if opts.Timeout > 0 {
		return context.WithTimeout(ctx, opts.Timeout)
	}
	return ctx, nil
}

// createBufferedReader creates a buffered reader if buffer size is specified
func (fr *FileReader) createBufferedReader(file *os.File, bufferSize int) io.Reader {
	var reader io.Reader = file
	if bufferSize > 0 {
		reader = bufio.NewReaderSize(file, bufferSize)
	}
	return reader
}

// performFileRead performs the actual file reading operation with context support
func (fr *FileReader) performFileRead(path string, reader io.Reader, opts FileReadOptions, ctx context.Context) ([]byte, error) {
	done := make(chan struct{})
	var content []byte
	var readErr error

	go func() {
		defer close(done)
		if opts.LineBased {
			content, readErr = fr.readFileLines(reader, opts)
		} else {
			content, readErr = fr.readFileContent(reader, opts.MaxSize)
		}
	}()

	select {
	case <-ctx.Done():
		fr.logger.Warn().Str("path", path).Msg("File read cancelled due to context timeout")
		return nil, WrapError(ctx.Err(), "file read operation cancelled")
	case <-done:
		if readErr != nil {
			return nil, WrapError(readErr, fmt.Sprintf("failed to read file content: %s", path))
		}
	}

	return content, nil
}

// readFileLines reads file content line by line
func (fr *FileReader) readFileLines(reader io.Reader, opts FileReadOptions) ([]byte, error) {
	scanner := bufio.NewScanner(reader)

	// Use string slice pool to reduce allocations
	lines := DefaultStringSlicePool.Get()
	defer DefaultStringSlicePool.Put(lines)

	for scanner.Scan() {
		line := scanner.Text()

		if opts.TrimLines {
			line = strings.TrimSpace(line)
		}

		if opts.SkipEmpty && line == "" {
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// readFileContent reads file content into bytes
func (fr *FileReader) readFileContent(reader io.Reader, maxSize int64) ([]byte, error) {
	if maxSize > 0 {
		reader = io.LimitReader(reader, maxSize)
	}
	return io.ReadAll(reader)
}
