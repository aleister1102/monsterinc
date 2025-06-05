package common

import (
	"regexp"

	"github.com/rs/zerolog"
)

// CompileRegexes compiles a slice of regex strings into a slice of *regexp.Regexp.
// It logs an error for any regex that fails to compile but continues with others.
func CompileRegexes(
	regexStrings []string,
	logger zerolog.Logger,
) []*regexp.Regexp {
	var compiledRegexes []*regexp.Regexp
	if len(regexStrings) == 0 {
		return compiledRegexes
	}

	for _, regexStr := range regexStrings {
		re, err := regexp.Compile(regexStr)
		if err != nil {
			logger.Error().Err(err).Str("regex", regexStr).Msg("Failed to compile regex, skipping.")
		} else {
			compiledRegexes = append(compiledRegexes, re)
		}
	}
	return compiledRegexes
}
