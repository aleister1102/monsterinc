package common

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// FileManager provides high-level file operations with standardized error handling and logging
type FileManager struct {
	logger    zerolog.Logger
	reader    *FileReader
	writer    *FileWriter
	validator *FileValidator
}

// NewFileManager creates a new FileManager instance
func NewFileManager(logger zerolog.Logger) *FileManager {
	componentLogger := logger.With().Str("component", "FileManager").Logger()

	return &FileManager{
		logger:    componentLogger,
		reader:    NewFileReader(componentLogger),
		writer:    NewFileWriter(componentLogger),
		validator: NewFileValidator(componentLogger),
	}
}

// FileExists checks if a file or directory exists
func (fm *FileManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileInfo returns information about a file
func (fm *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	return fm.validator.GetFileInfo(path)
}

// ReadFile reads a file with the given options
func (fm *FileManager) ReadFile(path string, opts FileReadOptions) ([]byte, error) {
	return fm.reader.ReadFile(path, opts)
}

// EnsureDirectory creates a directory and its parents if they don't exist
func (fm *FileManager) EnsureDirectory(path string, perm fs.FileMode) error {
	if fm.FileExists(path) {
		info, err := fm.GetFileInfo(path)
		if err != nil {
			return WrapError(err, "failed to check directory: "+path)
		}
		if !info.IsDir {
			return NewValidationError("path", path, "exists but is not a directory")
		}
		return nil
	}

	if err := os.MkdirAll(path, perm); err != nil {
		return WrapError(err, "failed to create directory: "+path)
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
			return WrapError(err, "failed to create parent directories for: "+path)
		}
	}

	return fm.writer.WriteFile(path, data, opts)
}
