package urlhandler

import (
	"errors"
	"strings"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/rs/zerolog"
)

// Custom errors for file operations
var (
	ErrFileNotFound   = errors.New("input file not found")
	ErrFilePermission = errors.New("permission denied reading input file")
	ErrFileEmpty      = errors.New("input file is empty or contains no valid URLs")
	ErrReadingFile    = errors.New("error reading input file")
)

// ReadURLsFromFile reads URLs from a file and returns them as a slice of strings
func ReadURLsFromFile(filePath string, logger zerolog.Logger) ([]string, error) {
	logger.Debug().Str("file", filePath).Msg("Reading URLs from file")

	// Use FileManager for better file handling
	fileManager := common.NewFileManager(logger)

	// Check if file exists
	if !fileManager.FileExists(filePath) {
		return nil, WrapError(ErrNotFound, "file not found: "+filePath)
	}

	// Get file info for validation
	fileInfo, err := fileManager.GetFileInfo(filePath)
	if err != nil {
		return nil, WrapError(err, "failed to get file info for: "+filePath)
	}

	if fileInfo.IsDir {
		return nil, common.NewError("path %s is a directory, not a file", filePath)
	}

	if fileInfo.Size == 0 {
		return nil, common.NewError("file has size %d, it is empty", fileInfo.Size)
	}

	// Read file using FileManager
	opts := common.DefaultFileReadOptions()
	opts.LineBased = true
	opts.TrimLines = true
	opts.SkipEmpty = true

	content, err := fileManager.ReadFile(filePath, opts)
	if err != nil {
		return nil, WrapError(err, "failed to read file: "+filePath)
	}

	// Parse URLs from content
	lines := strings.Split(string(content), "\n")
	var urls []string

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue // Skip empty lines and comments
		}

		// Validate URL format
		if err := ValidateURLFormat(trimmed); err != nil {
			logger.Warn().
				Str("file", filePath).
				Int("line", lineNum+1).
				Str("url", trimmed).
				Err(err).
				Msg("Skipping invalid URL")
			continue
		}

		urls = append(urls, trimmed)
	}

	if len(urls) == 0 {
		return nil, common.NewError("no valid URLs found after processing %d lines", len(lines))
	}

	logger.Info().
		Str("file", filePath).
		Int("total_lines", len(lines)).
		Int("valid_urls", len(urls)).
		Msg("Successfully loaded URLs from file")

	return urls, nil
}
