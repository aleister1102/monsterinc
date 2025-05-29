package urlhandler

import (
	"errors"
	"fmt"
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

// ReadURLsFromFile reads a file line by line, normalizes each line as a URL,
// and returns a slice of valid, normalized URLs.
func ReadURLsFromFile(filePath string, logger zerolog.Logger) ([]string, error) {
	fileLogger := logger.With().Str("filePath", filePath).Logger()

	// Use common file manager for file operations
	fileManager := common.NewFileManager(fileLogger)

	// Check if file exists and get info
	if !fileManager.FileExists(filePath) {
		fileLogger.Error().Msg("Input file not found")
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
	}

	info, err := fileManager.GetFileInfo(filePath)
	if err != nil {
		fileLogger.Error().Err(err).Msg("Error getting file info")
		return nil, fmt.Errorf("error checking file %s: %v", filePath, err)
	}

	if info.IsDir {
		fileLogger.Error().Msg("Input path is a directory, not a file")
		return nil, fmt.Errorf("input path is a directory, not a file: %s", filePath)
	}

	if info.Size == 0 {
		fileLogger.Warn().Msg("Input file is empty (0 bytes)")
		return nil, fmt.Errorf("%w: %s (size is 0)", ErrFileEmpty, filePath)
	}

	// Read file lines using common file utilities
	readOptions := common.DefaultFileReadOptions()
	readOptions.LineBased = true
	readOptions.TrimLines = true
	readOptions.SkipEmpty = true

	lines, err := fileManager.ReadLines(filePath, readOptions)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, common.ErrNotFound) {
			fileLogger.Error().Err(err).Msg("Input file not found")
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
		}

		// Check if it's a permission error by examining the error message
		if strings.Contains(err.Error(), "permission denied") {
			fileLogger.Error().Err(err).Msg("Permission denied reading input file")
			return nil, fmt.Errorf("%w: %s", ErrFilePermission, filePath)
		}

		fileLogger.Error().Err(err).Msg("Error reading input file")
		return nil, fmt.Errorf("%w: %s (cause: %v)", ErrReadingFile, filePath, err)
	}

	var normalizedURLs []string
	totalLinesRead := len(lines)
	successfullyNormalizedCount := 0
	skippedCount := 0
	hasValidURL := false

	fileLogger.Debug().Int("total_lines", totalLinesRead).Msg("Starting processing of file")

	for lineNumber, line := range lines {
		if line == "" {
			fileLogger.Debug().Int("lineNumber", lineNumber+1).Msg("Skipping empty line")
			continue
		}

		normalizedURL, normErr := NormalizeURL(line) // Assuming NormalizeURL doesn't need a logger or handles its own
		if normErr != nil {
			fileLogger.Warn().Err(normErr).Int("lineNumber", lineNumber+1).Str("originalURL", line).Msg("Error normalizing URL, skipping")
			skippedCount++
			continue
		}
		normalizedURLs = append(normalizedURLs, normalizedURL)
		successfullyNormalizedCount++
		hasValidURL = true
	}

	fileLogger.Debug().
		Int("totalLinesRead", totalLinesRead).
		Int("normalizedCount", successfullyNormalizedCount).
		Int("skippedCount", skippedCount).
		Msg("Finished processing file")

	if !hasValidURL && totalLinesRead > 0 {
		fileLogger.Warn().Int("totalLinesRead", totalLinesRead).Msg("Input file contained lines but no valid URLs were found")
		return nil, fmt.Errorf("%w: %s (no valid URLs found after processing %d lines)", ErrFileEmpty, filePath, totalLinesRead)
	}

	return normalizedURLs, nil
}
