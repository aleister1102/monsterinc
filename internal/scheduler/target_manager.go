package scheduler

import (
	"bufio"
	"fmt"
	"log"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"os"
	"strings"
)

// TargetManager handles loading, normalizing, and selecting targets based on configuration.
// !monsterinc/target-management
type TargetManager struct {
	logger *log.Logger
	// We can add configuration here if needed later, e.g., for concurrent processing
}

// NewTargetManager creates a new TargetManager.
// !monsterinc/target-management
func NewTargetManager(logger *log.Logger) *TargetManager {
	return &TargetManager{logger: logger}
}

// LoadAndSelectTargets loads target URLs from either a file or configuration and normalizes them.
// Priority: inputFileOption (from -urlfile flag) > cfgInputFile (from config) > inputConfigUrls (from config)
// Returns: (normalized Targets slice, source description, error)
// !monsterinc/target-management
func (tm *TargetManager) LoadAndSelectTargets(inputFileOption string, inputConfigUrls []string, cfgInputFile string) ([]models.Target, string, error) {
	var rawURLs []string
	var source string

	// Priority 1: Command-line file option (-urlfile)
	if inputFileOption != "" {
		tm.logger.Printf("[INFO] TargetManager: Loading targets from command-line file: %s", inputFileOption)
		loadedURLs, err := tm.loadURLsFromFile(inputFileOption) // Call as method
		if err != nil {
			return nil, "", fmt.Errorf("failed to load URLs from file '%s': %w", inputFileOption, err)
		}
		rawURLs = loadedURLs
		source = inputFileOption
	} else if cfgInputFile != "" {
		// Priority 2: Config file input_file
		tm.logger.Printf("[INFO] TargetManager: Loading targets from config file: %s", cfgInputFile)
		loadedURLs, err := tm.loadURLsFromFile(cfgInputFile) // Call as method
		if err != nil {
			return nil, "", fmt.Errorf("failed to load URLs from config file '%s': %w", cfgInputFile, err)
		}
		rawURLs = loadedURLs
		source = cfgInputFile
	} else if len(inputConfigUrls) > 0 {
		// Priority 3: Config input_urls
		tm.logger.Printf("[INFO] TargetManager: Using %d URLs from config input_urls", len(inputConfigUrls))
		rawURLs = inputConfigUrls
		source = "config_input_urls"
	} else {
		return nil, "", fmt.Errorf("no target URLs provided: specify -urlfile, input_file in config, or input_urls in config")
	}

	// Filter out empty URLs and normalize
	var validTargets []models.Target
	for _, rawURL := range rawURLs {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		normalizedURL, err := urlhandler.NormalizeURL(trimmed)
		if err != nil {
			tm.logger.Printf("[WARN] TargetManager: Skipping URL %s due to normalization error: %v", trimmed, err)
			continue
		}
		validTargets = append(validTargets, models.Target{OriginalURL: trimmed, NormalizedURL: normalizedURL})
	}

	if len(validTargets) == 0 {
		return nil, source, fmt.Errorf("no valid URLs found in source: %s", source)
	}

	tm.logger.Printf("[INFO] TargetManager: Loaded and normalized %d valid targets from %s", len(validTargets), source)
	return validTargets, source, nil
}

// loadURLsFromFile reads URLs from a file, one per line.
// It filters out empty lines and validates that URLs start with http:// or https://
// !monsterinc/target-management
func (tm *TargetManager) loadURLsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		url := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if url == "" {
			continue
		}

		// Skip comment lines (starting with # or //)
		if strings.HasPrefix(url, "#") || strings.HasPrefix(url, "//") {
			continue
		}

		// Validate URL format (basic check)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			tm.logger.Printf("[WARN] TargetManager: Line %d in file %s: URL does not start with http:// or https:// - skipping: %s", lineNum, filePath, url)
			continue
		}

		urls = append(urls, url)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return urls, nil
}

// LoadTargetsFromFile reads URLs from a given file path, normalizes them,
// and returns a slice of Target structs.
// It skips empty lines and lines that result in an error during normalization.
// This method can be used if direct file loading is needed, bypassing the selection logic.
// !monsterinc/target-management
func (tm *TargetManager) LoadTargetsFromFile(filePath string) ([]models.Target, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []models.Target
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		originalURL := scanner.Text()
		if originalURL == "" {
			continue // Skip empty lines
		}

		normalizedURL, err := urlhandler.NormalizeURL(originalURL)
		if err != nil {
			tm.logger.Printf("[WARN] TargetManager: Skipping URL %s from file %s due to normalization error: %v", originalURL, filePath, err)
			continue
		}
		targets = append(targets, models.Target{OriginalURL: originalURL, NormalizedURL: normalizedURL})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return targets, nil
}
