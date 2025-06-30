package filemanager

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/rs/zerolog"
)

// FileManager provides high-level file operations with standardized error handling and logging
type FileManager struct {
	logger zerolog.Logger
	reader *FileReader
	writer *FileWriter
}

// NewFileManager creates a new FileManager instance
func NewFileManager(logger zerolog.Logger) *FileManager {
	componentLogger := logger.With().Str("component", "FileManager").Logger()

	return &FileManager{
		logger: componentLogger,
		reader: NewFileReader(componentLogger),
		writer: NewFileWriter(componentLogger),
	}
}

// FileExists checks if a file or directory exists
func (fm *FileManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileInfo returns information about a file
// GetFileInfo returns information about a file
func (fv *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errorwrapper.WrapError(fmt.Errorf("file not found"), fmt.Sprintf("file not found: %s", path))
		}
		return nil, errorwrapper.WrapError(err, fmt.Sprintf("failed to get file info for: %s", path))
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

// ValidateFileForReading validates a file path and options before reading
func (fm *FileManager) ValidateFileForReading(path string, opts FileReadOptions) (*FileInfo, error) {
	// Check if file exists and validate
	info, err := fm.GetFileInfo(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir {
		return nil, errorwrapper.NewValidationError("path", path, "is a directory, not a file")
	}

	if opts.MaxSize > 0 && info.Size > opts.MaxSize {
		return nil, errorwrapper.NewValidationError("file_size", info.Size, fmt.Sprintf("exceeds maximum size of %d bytes", opts.MaxSize))
	}

	return info, nil
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
			return errorwrapper.WrapError(err, "failed to check directory: "+path)
		}
		if !info.IsDir {
			return errorwrapper.NewValidationError("path", path, "exists but is not a directory")
		}
		return nil
	}

	if err := os.MkdirAll(path, perm); err != nil {
		return errorwrapper.WrapError(err, "failed to create directory: "+path)
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
			return errorwrapper.WrapError(err, "failed to create parent directories for: "+path)
		}
	}

	return fm.writer.WriteFile(path, data, opts)
}
