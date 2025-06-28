package urlhandler

import (
	"bufio"
	"os"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
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
func (tm *TargetManager) LoadAndSelectTargets(cliFile string) ([]Target, string, error) {
	var targets []Target
	var source string
	var err error

	// Only source: Command-line file option
	if cliFile != "" {
		// tm.logger.Info().Str("file", cliFile).Msg("Loading targets from command-line file option")
		targets, err = tm.getTargetsFromFile(cliFile)
		if err != nil {
			return nil, source, errorwrapper.WrapError(err, "failed to load URLs from file '"+cliFile+"'")
		}
		source = cliFile
		tm.logger.Info().Int("count", len(targets)).Str("source", source).Msg("Loaded targets from command-line file")
		return targets, source, nil
	}

	// No input source available
	tm.logger.Warn().Msg("No input source configured for targets")
	source = "no_input"

	// Validate that we have targets
	if len(targets) == 0 {
		return nil, source, errorwrapper.NewError("no valid URLs found in source: %s", source)
	}

	return targets, source, nil
}

func (tm *TargetManager) getTargetsFromFile(filePath string) ([]Target, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []Target
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := scanner.Text()
		normalizedURL, err := NormalizeURL(url)
		if err != nil {
			tm.logger.Warn().Str("url", url).Err(err).Msg("Failed to normalize URL, skipping")
			continue
		}
		targets = append(targets, Target{URL: normalizedURL})
	}
	return targets, scanner.Err()
}

// GetTargetStrings extracts URL strings from Target objects
func (tm *TargetManager) GetTargetStrings(targets []Target) []string {
	urls := make([]string, len(targets))
	for i, t := range targets {
		urls[i] = t.URL
	}
	return urls
}
