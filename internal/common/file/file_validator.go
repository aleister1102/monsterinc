package file

import (
	"fmt"
	"os"

	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/rs/zerolog"
)

// FileValidator handles file validation operations
type FileValidator struct {
	logger zerolog.Logger
}

// NewFileValidator creates a new FileValidator instance
func NewFileValidator(logger zerolog.Logger) *FileValidator {
	return &FileValidator{
		logger: logger.With().Str("component", "FileValidator").Logger(),
	}
}

// GetFileInfo returns information about a file
func (fv *FileValidator) GetFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.WrapError(fmt.Errorf("file not found"), fmt.Sprintf("file not found: %s", path))
		}
		return nil, errors.WrapError(err, fmt.Sprintf("failed to get file info for: %s", path))
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
func (fv *FileValidator) ValidateFileForReading(path string, opts FileReadOptions) (*FileInfo, error) {
	// Check if file exists and validate
	info, err := fv.GetFileInfo(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir {
		return nil, errors.NewValidationError("path", path, "is a directory, not a file")
	}

	if opts.MaxSize > 0 && info.Size > opts.MaxSize {
		return nil, errors.NewValidationError("file_size", info.Size, fmt.Sprintf("exceeds maximum size of %d bytes", opts.MaxSize))
	}

	return info, nil
}
