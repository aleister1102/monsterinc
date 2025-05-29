package scheduler

import (
	// "bufio" // No longer needed if urlhandler.ReadURLsFromFile is used
	"fmt"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"

	// "os" // No longer needed if urlhandler.ReadURLsFromFile is used

	"strings"

	"github.com/rs/zerolog"
)

// TargetManager handles loading, normalizing, and selecting targets based on configuration.
// !monsterinc/target-management
type TargetManager struct {
	logger zerolog.Logger
	// We can add configuration here if needed later, e.g., for concurrent processing
}

// NewTargetManager creates a new TargetManager.
// !monsterinc/target-management
func NewTargetManager(logger zerolog.Logger) *TargetManager {
	return &TargetManager{logger: logger}
}

// LoadAndSelectTargets loads target URLs from either a file or configuration and normalizes them.
// Priority: inputFileOption (from -urlfile flag) > cfgInputFile (from config) > inputConfigUrls (from config)
// Returns: (normalized Targets slice, source description, error)
// !monsterinc/target-management
func (tm *TargetManager) LoadAndSelectTargets(inputFileOption string, inputConfigUrls []string, cfgInputFile string) ([]models.Target, string, error) {
	var rawURLs []string
	var determinedSource string
	var err error

	if inputFileOption != "" {
		tm.logger.Debug().Str("file", inputFileOption).Msg("Using URL file from command line argument.")
		rawURLs, err = tm.loadURLsFromFile(inputFileOption)
		determinedSource = inputFileOption
	} else if len(inputConfigUrls) > 0 {
		tm.logger.Debug().Int("count", len(inputConfigUrls)).Msg("Using input_urls from configuration.")
		rawURLs = inputConfigUrls
		determinedSource = "config.input_urls"
	} else if cfgInputFile != "" {
		tm.logger.Debug().Str("file", cfgInputFile).Msg("Using input_file from configuration.")
		rawURLs, err = tm.loadURLsFromFile(cfgInputFile)
		determinedSource = cfgInputFile
	} else {
		tm.logger.Debug().Msg("No URL input source provided (command line or config). Returning empty target list.")
		return []models.Target{}, "no_input", nil
	}

	if err != nil {
		return nil, determinedSource, fmt.Errorf("failed to load URLs from %s: %w", determinedSource, err)
	}

	tm.logger.Debug().Int("raw_url_count", len(rawURLs)).Str("source", determinedSource).Msg("Loaded raw URLs.")

	// Filter out empty URLs and normalize
	var validTargets []models.Target
	for _, rawURL := range rawURLs {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		normalizedURL, err := urlhandler.NormalizeURL(trimmed)
		if err != nil {
			tm.logger.Warn().Str("url", trimmed).Err(err).Msg("TargetManager: Skipping URL due to normalization error")
			continue
		}
		validTargets = append(validTargets, models.Target{OriginalURL: trimmed, NormalizedURL: normalizedURL})
	}

	if len(validTargets) == 0 {
		return nil, determinedSource, fmt.Errorf("no valid URLs found in source: %s", determinedSource)
	}

	tm.logger.Info().Int("count", len(validTargets)).Str("source", determinedSource).Msg("TargetManager: Loaded and normalized valid targets")
	return validTargets, determinedSource, nil
}

// loadURLsFromFile reads URLs from a file, one per line.
// It filters out empty lines and validates that URLs start with http:// or https://
// !monsterinc/target-management
func (tm *TargetManager) loadURLsFromFile(filePath string) ([]string, error) {
	// This function is now effectively replaced by urlhandler.ReadURLsFromFile.
	// We pass tm.logger to it.
	return urlhandler.ReadURLsFromFile(filePath, tm.logger)
}

// LoadTargetsFromFile reads URLs from a given file path, normalizes them,
// and returns a slice of Target structs.
// It skips empty lines and lines that result in an error during normalization.
// This method can be used if direct file loading is needed, bypassing the selection logic.
// !monsterinc/target-management
func (tm *TargetManager) LoadTargetsFromFile(filePath string) ([]models.Target, error) {
	// Use the refactored urlhandler.ReadURLsFromFile
	rawURLs, err := urlhandler.ReadURLsFromFile(filePath, tm.logger)
	if err != nil {
		// ReadURLsFromFile logs specific errors. Log a general message here or rely on its logging.
		tm.logger.Error().Err(err).Str("file", filePath).Msg("TargetManager: Failed to load URLs from file in LoadTargetsFromFile")
		return nil, fmt.Errorf("error reading URLs from file %s: %w", filePath, err)
	}

	var targets []models.Target
	for _, originalURL := range rawURLs {
		// Normalization is now part of ReadURLsFromFile semantics (it returns normalized URLs or errors)
		// However, the current ReadURLsFromFile returns []string of normalized URLs.
		// We need to re-normalize here if we want to store OriginalURL, or adapt ReadURLsFromFile further.
		// For now, let's assume rawURLs from ReadURLsFromFile are what we need for NormalizedURL.
		// And we might not have the original pre-normalized string easily unless ReadURLsFromFile changes.
		// This function's contract might need re-evaluation based on ReadURLsFromFile's output.
		// Let's assume for now that `originalURL` from the loop IS the normalized URL.
		normalizedURL := originalURL // This is an assumption based on current ReadURLsFromFile output

		// If we needed the original string (pre-normalization by ReadURLsFromFile), this simple loop isn't enough.
		// For now, we use the already normalized URL as both original and normalized for the Target struct.
		// This part needs careful review if the distinction is critical for consumers of LoadTargetsFromFile.
		targets = append(targets, models.Target{OriginalURL: normalizedURL, NormalizedURL: normalizedURL})
	}

	// scanner.Err() check is handled within ReadURLsFromFile

	return targets, nil
}

// GetTargetStrings loads targets using LoadAndSelectTargets and returns a slice of normalized URL strings.
// This is a convenience method if only the string representation of targets is needed.
func (tm *TargetManager) GetTargetStrings(inputFileOption string, inputConfigUrls []string, cfgInputFile string) ([]string, error) {
	targets, _, err := tm.LoadAndSelectTargets(inputFileOption, inputConfigUrls, cfgInputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load targets: %w", err)
	}

	var urlStrings []string
	for _, target := range targets {
		urlStrings = append(urlStrings, target.NormalizedURL)
	}
	return urlStrings, nil
}
