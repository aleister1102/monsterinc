package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// FileManager handles target file operations with proper locking
type FileManager struct {
	config PathsConfig
	logger zerolog.Logger
	mutex  sync.RWMutex
}

// NewFileManager creates a new file manager instance
func NewFileManager(config PathsConfig, logger zerolog.Logger) (*FileManager, error) {
	fm := &FileManager{
		config: config,
		logger: logger,
	}

	// Ensure targets directory exists
	if err := fm.ensureTargetsDirectory(); err != nil {
		return nil, fmt.Errorf("failed to create targets directory: %w", err)
	}

	// Ensure target files exist
	if err := fm.ensureTargetFiles(); err != nil {
		return nil, fmt.Errorf("failed to create target files: %w", err)
	}

	return fm, nil
}

// ensureTargetsDirectory creates the targets directory if it doesn't exist
func (fm *FileManager) ensureTargetsDirectory() error {
	if err := os.MkdirAll(fm.config.TargetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", fm.config.TargetsDir, err)
	}

	fm.logger.Debug().Str("dir", fm.config.TargetsDir).Msg("Targets directory ensured")
	return nil
}

// ensureTargetFiles creates target files if they don't exist
func (fm *FileManager) ensureTargetFiles() error {
	files := []string{
		filepath.Join(fm.config.TargetsDir, fm.config.URLsFile),
		filepath.Join(fm.config.TargetsDir, fm.config.JSHTMLFile),
	}

	for _, filePath := range files {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			file, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", filePath, err)
			}
			file.Close()
			fm.logger.Debug().Str("file", filePath).Msg("Target file created")
		}
	}

	return nil
}

// getFilePath returns the full path for a target file type
func (fm *FileManager) getFilePath(fileType string) string {
	switch fileType {
	case "urls":
		return filepath.Join(fm.config.TargetsDir, fm.config.URLsFile)
	case "js_html":
		return filepath.Join(fm.config.TargetsDir, fm.config.JSHTMLFile)
	default:
		return ""
	}
}

// ReadURLs reads all URLs from the specified file type
func (fm *FileManager) ReadURLs(fileType string) ([]string, error) {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	filePath := fm.getFilePath(fileType)
	if filePath == "" {
		return nil, fmt.Errorf("invalid file type: %s", fileType)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	fm.logger.Debug().
		Str("file_type", fileType).
		Int("count", len(urls)).
		Msg("URLs read from file")

	return urls, nil
}

// AddURL adds a new URL to the specified file type
func (fm *FileManager) AddURL(fileType, url string) error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	filePath := fm.getFilePath(fileType)
	if filePath == "" {
		return fmt.Errorf("invalid file type: %s", fileType)
	}

	// Check if URL already exists
	urls, err := fm.readURLsUnsafe(filePath)
	if err != nil {
		return fmt.Errorf("failed to read existing URLs: %w", err)
	}

	for _, existingURL := range urls {
		if existingURL == url {
			return fmt.Errorf("URL already exists: %s", url)
		}
	}

	// Append URL to file
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for writing %s: %w", filePath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(url + "\n"); err != nil {
		return fmt.Errorf("failed to write URL to file: %w", err)
	}

	fm.logger.Info().
		Str("file_type", fileType).
		Str("url", url).
		Msg("URL added to file")

	return nil
}

// RemoveURL removes a URL by line number from the specified file type
func (fm *FileManager) RemoveURL(fileType string, lineNumber int) (string, error) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	filePath := fm.getFilePath(fileType)
	if filePath == "" {
		return "", fmt.Errorf("invalid file type: %s", fileType)
	}

	urls, err := fm.readURLsUnsafe(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read URLs: %w", err)
	}

	if lineNumber < 1 || lineNumber > len(urls) {
		return "", fmt.Errorf("invalid line number: %d (valid range: 1-%d)", lineNumber, len(urls))
	}

	// Get the URL being removed
	removedURL := urls[lineNumber-1]

	// Remove the URL from slice
	urls = append(urls[:lineNumber-1], urls[lineNumber:]...)

	// Write back to file
	if err := fm.writeURLsUnsafe(filePath, urls); err != nil {
		return "", fmt.Errorf("failed to write updated URLs: %w", err)
	}

	fm.logger.Info().
		Str("file_type", fileType).
		Str("url", removedURL).
		Int("line_number", lineNumber).
		Msg("URL removed from file")

	return removedURL, nil
}

// UpdateURL updates a URL at the specified line number
func (fm *FileManager) UpdateURL(fileType string, lineNumber int, newURL string) (string, error) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	filePath := fm.getFilePath(fileType)
	if filePath == "" {
		return "", fmt.Errorf("invalid file type: %s", fileType)
	}

	urls, err := fm.readURLsUnsafe(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read URLs: %w", err)
	}

	if lineNumber < 1 || lineNumber > len(urls) {
		return "", fmt.Errorf("invalid line number: %d (valid range: 1-%d)", lineNumber, len(urls))
	}

	// Check if new URL already exists elsewhere
	for i, existingURL := range urls {
		if existingURL == newURL && i != lineNumber-1 {
			return "", fmt.Errorf("URL already exists at line %d: %s", i+1, newURL)
		}
	}

	// Get the old URL
	oldURL := urls[lineNumber-1]

	// Update the URL
	urls[lineNumber-1] = newURL

	// Write back to file
	if err := fm.writeURLsUnsafe(filePath, urls); err != nil {
		return "", fmt.Errorf("failed to write updated URLs: %w", err)
	}

	fm.logger.Info().
		Str("file_type", fileType).
		Str("old_url", oldURL).
		Str("new_url", newURL).
		Int("line_number", lineNumber).
		Msg("URL updated in file")

	return oldURL, nil
}

// GetURLCount returns the number of URLs in the specified file type
func (fm *FileManager) GetURLCount(fileType string) (int, error) {
	urls, err := fm.ReadURLs(fileType)
	if err != nil {
		return 0, err
	}
	return len(urls), nil
}

// readURLsUnsafe reads URLs without locking (internal use only)
func (fm *FileManager) readURLsUnsafe(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	return urls, scanner.Err()
}

// writeURLsUnsafe writes URLs without locking (internal use only)
func (fm *FileManager) writeURLsUnsafe(filePath string, urls []string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, url := range urls {
		if _, err := writer.WriteString(url + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// FormatURLList formats URLs for Discord display with line numbers
func (fm *FileManager) FormatURLList(fileType string, page, pageSize int) (string, int, error) {
	urls, err := fm.ReadURLs(fileType)
	if err != nil {
		return "", 0, err
	}

	if len(urls) == 0 {
		return "No URLs found.", 0, nil
	}

	totalPages := (len(urls) + pageSize - 1) / pageSize
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(urls) {
		end = len(urls)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("**%s URLs (Page %d/%d):**\n",
		strings.ToUpper(fileType), page, totalPages))
	result.WriteString("```\n")

	for i := start; i < end; i++ {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, urls[i]))
	}

	result.WriteString("```\n")
	result.WriteString(fmt.Sprintf("Total: %d URLs", len(urls)))

	return result.String(), totalPages, nil
}
