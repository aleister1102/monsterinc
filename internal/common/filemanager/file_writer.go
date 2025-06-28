package filemanager

import (
	"context"
	"fmt"
	"os"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/rs/zerolog"
)

// FileWriter handles file writing operations
type FileWriter struct {
	logger zerolog.Logger
}

// NewFileWriter creates a new FileWriter instance
func NewFileWriter(logger zerolog.Logger) *FileWriter {
	return &FileWriter{
		logger: logger.With().Str("component", "FileWriter").Logger(),
	}
}

// WriteFile writes data to a file with the given options
func (fw *FileWriter) WriteFile(path string, data []byte, opts FileWriteOptions) error {
	// Set up context with timeout if specified
	ctx, cancel := fw.setupContextWithTimeout(opts)
	if cancel != nil {
		defer cancel()
	}

	// Perform file write with context support
	done := make(chan error, 1)
	go func() {
		done <- fw.performFileWrite(path, data, opts)
	}()

	select {
	case <-ctx.Done():
		fw.logger.Warn().Str("path", path).Msg("File write cancelled due to context timeout")
		return errorwrapper.WrapError(ctx.Err(), "file write operation cancelled")
	case err := <-done:
		if err != nil {
			return errorwrapper.WrapError(err, fmt.Sprintf("failed to write file: %s", path))
		}
	}

	fw.logger.Debug().Str("path", path).Int("bytes", len(data)).Msg("File written successfully")
	return nil
}

// setupContextWithTimeout sets up context with timeout if specified
func (fw *FileWriter) setupContextWithTimeout(opts FileWriteOptions) (context.Context, context.CancelFunc) {
	ctx := opts.Context
	if opts.Timeout > 0 {
		return context.WithTimeout(ctx, opts.Timeout)
	}
	return ctx, nil
}

// performFileWrite performs the actual file writing operation
func (fw *FileWriter) performFileWrite(path string, data []byte, opts FileWriteOptions) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, opts.Permissions)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fw.logger.Error().Err(closeErr).Str("path", path).Msg("Failed to close file after writing")
		}
	}()

	_, err = file.Write(data)
	return err
}
