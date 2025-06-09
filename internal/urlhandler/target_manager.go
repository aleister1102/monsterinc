package urlhandler

import (
	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// TargetManager handles loading and managing targets from various sources
type TargetManager struct {
	logger zerolog.Logger
	// We can add configuration here if needed later, e.g., for concurrent processing
}

// NewTargetManager creates a new TargetManager instance
func NewTargetManager(logger zerolog.Logger) *TargetManager {
	return &TargetManager{
		logger: logger.With().Str("component", "TargetManager").Logger(),
	}
}

// LoadAndSelectTargets loads targets from the command-line file option
func (tm *TargetManager) LoadAndSelectTargets(inputFileOption string) ([]models.Target, string, error) {
	var targets []models.Target
	var determinedSource string

	// Only source: Command-line file option
	if inputFileOption != "" {
		// tm.logger.Info().Str("file", inputFileOption).Msg("Loading targets from command-line file option")
		urls, err := ReadURLsFromFile(inputFileOption, tm.logger)
		if err != nil {
			return nil, determinedSource, common.WrapError(err, "failed to load URLs from file '"+inputFileOption+"'")
		}
		targets = tm.convertURLsToTargets(urls)
		determinedSource = inputFileOption
		tm.logger.Info().Int("count", len(targets)).Str("source", determinedSource).Msg("Loaded targets from command-line file")
		return targets, determinedSource, nil
	}

	// No input source available
	tm.logger.Warn().Msg("No input source configured for targets")
	determinedSource = "no_input"

	// Validate that we have targets
	if len(targets) == 0 {
		return nil, determinedSource, common.NewError("no valid URLs found in source: %s", determinedSource)
	}

	return targets, determinedSource, nil
}

// convertURLsToTargets converts a slice of URL strings to Target objects
func (tm *TargetManager) convertURLsToTargets(urls []string) []models.Target {
	targets := make([]models.Target, 0, len(urls))
	for _, url := range urls {
		normalizedURL, err := NormalizeURL(url)
		if err != nil {
			tm.logger.Warn().Str("url", url).Err(err).Msg("Failed to normalize URL, skipping")
			continue
		}
		targets = append(targets, models.Target{
			OriginalURL:   url,
			NormalizedURL: normalizedURL,
		})
	}
	return targets
}

// GetTargetStrings extracts URL strings from Target objects
func (tm *TargetManager) GetTargetStrings(targets []models.Target) []string {
	urls := make([]string, len(targets))
	for i, target := range targets {
		urls[i] = target.NormalizedURL
	}
	return urls
}
