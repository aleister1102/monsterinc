package extractor

import (
	"regexp"

	"github.com/rs/zerolog"
)

// CompileRegexes compiles a slice of regex patterns into a slice of *regexp.Regexp
func CompileRegexes(patterns []string, logger zerolog.Logger) []*regexp.Regexp {
	var compiledRegexes []*regexp.Regexp
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiledRegexes = append(compiledRegexes, re)
		} else {
			logger.Warn().
				Str("pattern", pattern).
				Err(err).
				Msg("Failed to compile regex, skipping")
		}
	}
	return compiledRegexes
} 