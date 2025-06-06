package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// FileInfo contains metadata about a file
type FileInfo struct {
	Path        string      // Full file path
	Name        string      // File name only
	Size        int64       // File size in bytes
	IsDir       bool        // Whether it's a directory
	ModTime     time.Time   // Last modification time
	Permissions fs.FileMode // File permissions
}

// FileReadOptions configures file reading behavior
type FileReadOptions struct {
	MaxSize    int64           // Maximum file size to read (0 = no limit)
	BufferSize int             // Buffer size for reading
	LineBased  bool            // Whether to read line by line
	TrimLines  bool            // Whether to trim whitespace from lines
	SkipEmpty  bool            // Whether to skip empty lines
	Timeout    time.Duration   // Read timeout
	Context    context.Context // Context for cancellation
}

// FileWriteOptions configures file writing behavior
type FileWriteOptions struct {
	CreateDirs  bool            // Whether to create parent directories
	Permissions fs.FileMode     // File permissions
	Timeout     time.Duration   // Write timeout
	Context     context.Context // Context for cancellation
}

// DefaultFileReadOptions returns default file reading options
func DefaultFileReadOptions() FileReadOptions {
	return FileReadOptions{
		MaxSize:    50 * 1024 * 1024, // 50MB default
		BufferSize: 64 * 1024,        // 64KB buffer
		LineBased:  false,
		TrimLines:  true,
		SkipEmpty:  true,
		Timeout:    30 * time.Second,
		Context:    context.Background(),
	}
}

// DefaultFileWriteOptions returns default file writing options
func DefaultFileWriteOptions() FileWriteOptions {
	return FileWriteOptions{
		CreateDirs:  true,
		Permissions: 0644,
		Timeout:     30 * time.Second,
		Context:     context.Background(),
	}
}

// FileManager provides high-level file operations with standardized error handling and logging
type FileManager struct {
	logger zerolog.Logger
}

// NewFileManager creates a new FileManager instance
func NewFileManager(logger zerolog.Logger) *FileManager {
	return &FileManager{
		logger: logger.With().Str("component", "FileManager").Logger(),
	}
}

// FileExists checks if a file or directory exists
func (fm *FileManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileInfo returns information about a file
func (fm *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, WrapError(ErrNotFound, fmt.Sprintf("file not found: %s", path))
		}
		return nil, WrapError(err, fmt.Sprintf("failed to get file info for: %s", path))
	}

	info := &FileInfo{
		Path:        path,
		Name:        stat.Name(),
		Size:        stat.Size(),
		IsDir:       stat.IsDir(),
		ModTime:     stat.ModTime(),
		Permissions: stat.Mode(),
	}

	return info, nil
}

// ReadFile reads a file with the given options
func (fm *FileManager) ReadFile(path string, opts FileReadOptions) ([]byte, error) {
	// Validate file and options
	_, err := fm.validateFileForReading(path, opts)
	if err != nil {
		return nil, err
	}

	// Set up context with timeout if specified
	ctx, cancel := fm.setupContextWithTimeout(opts)
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
			fm.logger.Error().Err(err).Str("path", path).Msg("Failed to close file.")
		}
	}()

	// Create buffered reader if buffer size is specified
	reader := fm.createBufferedReader(file, opts.BufferSize)

	// Perform file read with context support
	content, err := fm.performFileRead(path, reader, opts, ctx)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// validateFileForReading validates a file path and options before reading
func (fm *FileManager) validateFileForReading(path string, opts FileReadOptions) (*FileInfo, error) {
	// Check if file exists and validate
	info, err := fm.GetFileInfo(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir {
		return nil, NewValidationError("path", path, "is a directory, not a file")
	}

	if opts.MaxSize > 0 && info.Size > opts.MaxSize {
		return nil, NewValidationError("file_size", info.Size, fmt.Sprintf("exceeds maximum size of %d bytes", opts.MaxSize))
	}

	return info, nil
}

// setupContextWithTimeout sets up context with timeout if specified
func (fm *FileManager) setupContextWithTimeout(opts FileReadOptions) (context.Context, context.CancelFunc) {
	ctx := opts.Context
	if opts.Timeout > 0 {
		return context.WithTimeout(ctx, opts.Timeout)
	}
	return ctx, nil
}

// createBufferedReader creates a buffered reader if buffer size is specified
func (fm *FileManager) createBufferedReader(file *os.File, bufferSize int) io.Reader {
	var reader io.Reader = file
	if bufferSize > 0 {
		reader = bufio.NewReaderSize(file, bufferSize)
	}
	return reader
}

// performFileRead performs the actual file reading operation with context support
func (fm *FileManager) performFileRead(path string, reader io.Reader, opts FileReadOptions, ctx context.Context) ([]byte, error) {
	done := make(chan struct{})
	var content []byte
	var readErr error

	go func() {
		defer close(done)
		if opts.LineBased {
			content, readErr = fm.readFileLines(reader, opts)
		} else {
			content, readErr = fm.readFileContent(reader, opts.MaxSize)
		}
	}()

	select {
	case <-ctx.Done():
		fm.logger.Warn().Str("path", path).Msg("File read cancelled due to context timeout")
		return nil, WrapError(ctx.Err(), "file read operation cancelled")
	case <-done:
		if readErr != nil {
			return nil, WrapError(readErr, fmt.Sprintf("failed to read file content: %s", path))
		}
	}

	return content, nil
}

// readFileLines reads file content line by line
func (fm *FileManager) readFileLines(reader io.Reader, opts FileReadOptions) ([]byte, error) {
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
func (fm *FileManager) readFileContent(reader io.Reader, maxSize int64) ([]byte, error) {
	if maxSize > 0 {
		reader = io.LimitReader(reader, maxSize)
	}
	return io.ReadAll(reader)
}

// ReadLines reads a file and returns it as a slice of lines
func (fm *FileManager) ReadLines(path string, opts FileReadOptions) ([]string, error) {
	opts.LineBased = true
	content, err := fm.ReadFile(path, opts)
	if err != nil {
		return nil, err
	}

	if len(content) == 0 {
		return []string{}, nil
	}

	lines := strings.Split(string(content), "\n")

	// Remove empty last line if present (common with text files)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

// EnsureDirectory creates a directory and its parents if they don't exist
func (fm *FileManager) EnsureDirectory(path string, perm fs.FileMode) error {
	if fm.FileExists(path) {
		info, err := fm.GetFileInfo(path)
		if err != nil {
			return WrapError(err, fmt.Sprintf("failed to check directory: %s", path))
		}
		if !info.IsDir {
			return NewValidationError("path", path, "exists but is not a directory")
		}
		return nil
	}

	if err := os.MkdirAll(path, perm); err != nil {
		return WrapError(err, fmt.Sprintf("failed to create directory: %s", path))
	}

	fm.logger.Debug().Str("path", path).Msg("Created directory")
	return nil
}

// WriteFile writes data to a file with the given options
func (fm *FileManager) WriteFile(path string, data []byte, opts FileWriteOptions) error {
	// Create parent directories if required
	if opts.CreateDirs {
		dir := filepath.Dir(path)
		if err := fm.EnsureDirectory(dir, 0755); err != nil {
			return WrapError(err, fmt.Sprintf("failed to create parent directories for: %s", path))
		}
	}

	// Set up context with timeout if specified
	ctx, cancel := fm.setupContextWithTimeout(FileReadOptions{
		Timeout: opts.Timeout,
		Context: opts.Context,
	})
	if cancel != nil {
		defer cancel()
	}

	// Perform file write with context support
	done := make(chan error, 1)
	go func() {
		done <- fm.performFileWrite(path, data, opts)
	}()

	select {
	case <-ctx.Done():
		fm.logger.Warn().Str("path", path).Msg("File write cancelled due to context timeout")
		return WrapError(ctx.Err(), "file write operation cancelled")
	case err := <-done:
		if err != nil {
			return WrapError(err, fmt.Sprintf("failed to write file: %s", path))
		}
	}

	fm.logger.Debug().Str("path", path).Int("bytes", len(data)).Msg("File written successfully")
	return nil
}

// performFileWrite performs the actual file writing operation
func (fm *FileManager) performFileWrite(path string, data []byte, opts FileWriteOptions) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, opts.Permissions)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fm.logger.Error().Err(closeErr).Str("path", path).Msg("Failed to close file after writing")
		}
	}()

	_, err = file.Write(data)
	return err
}
