package urlhandler

import (
	"bufio"
	"errors"
	"fmt"

	// "log" // Standard log replaced by zerolog
	"os"
	"strings"

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
// Task 2.1: Implement file reading logic.
// Task 2.2: Implement error handling for file operations.
// Task 2.3: Implement logging for file processing.
func ReadURLsFromFile(filePath string, logger zerolog.Logger) ([]string, error) {
	fileLogger := logger.With().Str("filePath", filePath).Logger()

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		fileLogger.Error().Err(err).Msg("Input file not found")
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
	}
	if err != nil {
		fileLogger.Error().Err(err).Msg("Error checking file stat")
		return nil, fmt.Errorf("error checking file %s: %v", filePath, err)
	}
	if info.IsDir() {
		fileLogger.Error().Msg("Input path is a directory, not a file")
		return nil, fmt.Errorf("input path is a directory, not a file: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsPermission(err) {
			fileLogger.Error().Err(err).Msg("Permission denied reading input file")
			return nil, fmt.Errorf("%w: %s", ErrFilePermission, filePath)
		}
		fileLogger.Error().Err(err).Msg("Error opening input file")
		return nil, fmt.Errorf("%w: %s (cause: %v)", ErrReadingFile, filePath, err)
	}
	defer file.Close()

	if info.Size() == 0 {
		fileLogger.Warn().Msg("Input file is empty (0 bytes)")
		return nil, fmt.Errorf("%w: %s (size is 0)", ErrFileEmpty, filePath)
	}

	var normalizedURLs []string
	scanner := bufio.NewScanner(file)

	totalLinesRead := 0
	successfullyNormalizedCount := 0
	skippedCount := 0
	hasValidURL := false

	fileLogger.Info().Msg("Starting processing of file")

	for scanner.Scan() {
		totalLinesRead++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			fileLogger.Debug().Int("lineNumber", totalLinesRead).Msg("Skipping empty line")
			continue
		}

		normalizedURL, normErr := NormalizeURL(line) // Assuming NormalizeURL doesn't need a logger or handles its own
		if normErr != nil {
			fileLogger.Warn().Err(normErr).Int("lineNumber", totalLinesRead).Str("originalURL", line).Msg("Error normalizing URL, skipping")
			skippedCount++
			continue
		}
		normalizedURLs = append(normalizedURLs, normalizedURL)
		successfullyNormalizedCount++
		hasValidURL = true
	}

	if scanErr := scanner.Err(); scanErr != nil {
		fileLogger.Error().Err(scanErr).Msg("Error during scanning of file")
		return nil, fmt.Errorf("%w: %s (scan error: %v)", ErrReadingFile, filePath, scanErr)
	}

	fileLogger.Info().
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
