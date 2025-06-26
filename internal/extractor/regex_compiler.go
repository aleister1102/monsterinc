package extractor

import (
	"regexp"

	"github.com/aleister1102/monsterinc/internal/config"

	"github.com/rs/zerolog"
)

// RegexCompiler handles compilation of regular expressions
type RegexCompiler struct {
	logger zerolog.Logger
}

// NewRegexCompiler creates a new regex compiler
func NewRegexCompiler(logger zerolog.Logger) *RegexCompiler {
	return &RegexCompiler{
		logger: logger.With().Str("component", "RegexCompiler").Logger(),
	}
}

// CompiledRegexSet holds sets of compiled regular expressions
type CompiledRegexSet struct {
	CustomRegexes    []*regexp.Regexp
	AllowlistRegexes []*regexp.Regexp
	DenylistRegexes  []*regexp.Regexp
}

// CompileRegexSets compiles all regex sets from configuration
func (rc *RegexCompiler) CompileRegexSets(extractorCfg config.ExtractorConfig) CompiledRegexSet {
	regexSet := CompiledRegexSet{}

	if len(extractorCfg.CustomRegexes) > 0 {
		regexSet.CustomRegexes = CompileRegexes(extractorCfg.CustomRegexes, rc.logger)
		rc.logger.Debug().Int("compiled_count", len(regexSet.CustomRegexes)).Msg("Compiled custom regexes")
	}

	if len(extractorCfg.Allowlist) > 0 {
		regexSet.AllowlistRegexes = CompileRegexes(extractorCfg.Allowlist, rc.logger)
		rc.logger.Debug().Int("compiled_count", len(regexSet.AllowlistRegexes)).Msg("Compiled allowlist regexes")
	}

	if len(extractorCfg.Denylist) > 0 {
		regexSet.DenylistRegexes = CompileRegexes(extractorCfg.Denylist, rc.logger)
		rc.logger.Debug().Int("compiled_count", len(regexSet.DenylistRegexes)).Msg("Compiled denylist regexes")
	}

	return regexSet
}
