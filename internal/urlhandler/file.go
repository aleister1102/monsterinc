package urlhandler

import (
	"bufio"
	"errors"
	"fmt"
	"log" // Using standard log package for now as per plan for task 2.3
	"os"
	"strings"
	// It's good practice to log errors or skipped URLs.
	// We'll add proper logging in later tasks (e.g., task 2.3 or 4.x related to logging).
	// For now, we can use a placeholder or basic printing if needed for debugging,
	// but the function signature will return errors for the caller to handle.
	// "log" // Placeholder for now, will be replaced by the project's logger.
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
func ReadURLsFromFile(filePath string) ([]string, error) {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, filePath)
	}
	if err != nil {
		return nil, fmt.Errorf("error checking file %s: %v", filePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("input path is a directory, not a file: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrFilePermission, filePath)
		}
		return nil, fmt.Errorf("%w: %s (cause: %v)", ErrReadingFile, filePath, err)
	}
	defer file.Close()

	if info.Size() == 0 {
		log.Printf("Input file is empty (0 bytes): %s", filePath) // Task 2.3: Log empty file
		return nil, fmt.Errorf("%w: %s (size is 0)", ErrFileEmpty, filePath)
	}

	var normalizedURLs []string
	scanner := bufio.NewScanner(file)

	totalLinesRead := 0
	successfullyNormalizedCount := 0
	skippedCount := 0
	hasValidURL := false

	log.Printf("Starting processing of file: %s", filePath) // Task 2.3: Log start of processing

	for scanner.Scan() {
		totalLinesRead++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			// PRD requirement: skip empty lines. Logging this is optional but good for transparency.
			log.Printf("Skipping empty line %d in %s", totalLinesRead, filePath) // Task 2.3: Log skipped empty line
			// skippedCount++ // Not explicitly counted as "skipped due to error" in PRD summary, but good to be aware.
			continue
		}

		normalizedURL, normErr := NormalizeURL(line)
		if normErr != nil {
			// PRD: "Log an error message indicating the original problematic URL and the reason for the failure."
			log.Printf("Error normalizing URL on line %d of '%s': \"%s\". Error: %v. Skipping URL.", totalLinesRead, filePath, line, normErr) // Task 2.3: Log skipped URL
			skippedCount++
			continue
		}
		normalizedURLs = append(normalizedURLs, normalizedURL)
		successfullyNormalizedCount++
		hasValidURL = true
	}

	if scanErr := scanner.Err(); scanErr != nil {
		log.Printf("Error during scanning of file '%s': %v", filePath, scanErr) // Task 2.3: Log scan error
		return nil, fmt.Errorf("%w: %s (scan error: %v)", ErrReadingFile, filePath, scanErr)
	}

	// PRD summary log
	log.Printf("Finished processing file: %s. Total lines read: %d, URLs successfully normalized: %d, URLs skipped due to errors: %d",
		filePath, totalLinesRead, successfullyNormalizedCount, skippedCount) // Task 2.3: Log summary

	if !hasValidURL && totalLinesRead > 0 { // File had lines, but none were valid URLs
		// This case is distinct from a 0-byte file. It had content, but nothing usable.
		log.Printf("Input file '%s' contained %d lines but no valid URLs were found.", filePath, totalLinesRead) // Task 2.3: Log specific empty outcome
		return nil, fmt.Errorf("%w: %s (no valid URLs found after processing %d lines)", ErrFileEmpty, filePath, totalLinesRead)
	}
	// If totalLinesRead is 0, it means the file was non-zero size but os.Open succeeded and scanner found nothing (highly unlikely for text files but covering edge cases).
	// The info.Size() == 0 check handles 0-byte files upfront.

	return normalizedURLs, nil
}
