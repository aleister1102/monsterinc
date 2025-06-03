package urlhandler

import (
	"fmt"
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// TargetManager handles loading, normalizing, and selecting targets based on configuration.
type TargetManager struct {
	logger zerolog.Logger
	// We can add configuration here if needed later, e.g., for concurrent processing
}

// NewTargetManager creates a new TargetManager.
func NewTargetManager(logger zerolog.Logger) *TargetManager {
	return &TargetManager{logger: logger}
}

// LoadAndSelectTargets loads target URLs from either a file or configuration and normalizes them.
// Priority: inputFileOption (from -urlfile flag) > cfgInputFile (from config) > inputConfigUrls (from config)
//
// Returns: (normalized Targets slice, source description, error)
func (tm *TargetManager) LoadAndSelectTargets(inputFileOption string, inputConfigUrls []string, cfgInputFile string) ([]models.Target, string, error) {
	var rawURLs []string
	var determinedSource string
	var err error

	// 1. Use inputFileOption (from command line flag) if provided
	if inputFileOption != "" {
		tm.logger.Debug().Str("file", inputFileOption).Msg("Using URL file from command line argument.")
		rawURLs, err = ReadURLsFromFile(inputFileOption, tm.logger)
		determinedSource = inputFileOption

		if err != nil {
			tm.logger.Error().Err(err).Str("file", inputFileOption).Msg("Failed to load URLs from file.")
			return nil, determinedSource, fmt.Errorf("failed to load URLs from file '%s': %w", determinedSource, err)
		}

		if len(rawURLs) == 0 {
			tm.logger.Warn().Str("file", inputFileOption).Msg("Provided URL file was empty. Will attempt to use URLs from config if available.")
		}
	}

	// 2. Fallback to inputConfigUrls if no URLs from file argument
	if len(rawURLs) == 0 && len(inputConfigUrls) > 0 {
		tm.logger.Debug().Int("count", len(inputConfigUrls)).Msg("Using input_urls from configuration.")
		rawURLs = inputConfigUrls
		determinedSource = "config.input_urls"
	}

	// 3. Fallback to cfgInputFile if still no URLs
	if len(rawURLs) == 0 && cfgInputFile != "" {
		tm.logger.Debug().Str("file", cfgInputFile).Msg("Using input_file from configuration.")
		rawURLs, err = ReadURLsFromFile(cfgInputFile, tm.logger)
		determinedSource = cfgInputFile

		if err != nil {
			tm.logger.Error().Err(err).Str("file", cfgInputFile).Msg("Failed to load URLs from config input_file.")
			return nil, determinedSource, fmt.Errorf("failed to load URLs from config input_file '%s': %w", determinedSource, err)
		}

		if len(rawURLs) == 0 {
			tm.logger.Warn().Str("file", cfgInputFile).Msg("Config input_file was empty.")
		}
	}

	// Final check - if still no URLs found
	if len(rawURLs) == 0 {
		tm.logger.Debug().Msg("No URL input source provided or all sources were empty. Returning empty target list.")
		return []models.Target{}, "no_input", nil
	}

	tm.logger.Debug().Int("raw_url_count", len(rawURLs)).Str("source", determinedSource).Msg("Loaded raw URLs.")

	// Filter out empty URLs and normalize
	var validTargets []models.Target
	for _, rawURL := range rawURLs {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		normalizedURL, err := NormalizeURL(trimmed)
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

// GetTargetStrings loads targets using LoadAndSelectTargets and returns a slice of normalized URL strings.
// This is a convenience method if only the string representation of targets is needed.
func (tm *TargetManager) GetTargetStrings(targets []models.Target) []string {
	var urlStrings []string
	for _, target := range targets {
		urlStrings = append(urlStrings, target.NormalizedURL)
	}
	return urlStrings
}
