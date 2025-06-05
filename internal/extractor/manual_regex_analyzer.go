package extractor

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// ManualRegexAnalyzer handles manual regex-based analysis
type ManualRegexAnalyzer struct {
	logger        zerolog.Logger
	validator     *URLValidator
	contextExt    *ContextExtractor
	customRegexes []*regexp.Regexp
}

// NewManualRegexAnalyzer creates a new manual regex analyzer
func NewManualRegexAnalyzer(customRegexes []*regexp.Regexp, validator *URLValidator, contextExt *ContextExtractor, logger zerolog.Logger) *ManualRegexAnalyzer {
	return &ManualRegexAnalyzer{
		logger:        logger.With().Str("component", "ManualRegexAnalyzer").Logger(),
		validator:     validator,
		contextExt:    contextExt,
		customRegexes: customRegexes,
	}
}

// AnalyzeWithRegex processes content using manual regex patterns
func (mra *ManualRegexAnalyzer) AnalyzeWithRegex(sourceURL string, content []byte, base *url.URL, seenPaths map[string]struct{}) AnalysisResult {
	if len(mra.customRegexes) == 0 || len(content) == 0 {
		mra.logger.Debug().Msg("Skipping manual regex analysis - no regexes or empty content")
		return AnalysisResult{}
	}

	contentStr := string(content)
	mra.logger.Debug().
		Int("custom_regex_count", len(mra.customRegexes)).
		Int("content_length", len(content)).
		Msg("Starting manual regex scan")

	var extractedPaths []models.ExtractedPath
	processedCount := 0

	for i, customRegex := range mra.customRegexes {
		matches := customRegex.FindAllString(contentStr, -1)
		if len(matches) == 0 {
			continue
		}

		mra.logger.Debug().
			Str("regex", customRegex.String()).
			Int("match_count", len(matches)).
			Msg("Regex found matches")

		for _, match := range matches {
			result := mra.validator.ValidateAndResolveURL(match, base, sourceURL)
			if !result.IsValid {
				mra.logger.Debug().Str("match", match).Err(result.Error).Msg("Invalid match from regex")
				continue
			}

			if _, exists := seenPaths[result.AbsoluteURL]; exists {
				mra.logger.Debug().Str("absolute_url", result.AbsoluteURL).Msg("Duplicate URL from regex")
				continue
			}

			contextSnippet := mra.contextExt.ExtractContext(contentStr, match)
			pathType := fmt.Sprintf("manual_config_regex_%d", i)

			extractedPath := models.ExtractedPath{
				SourceURL:            sourceURL,
				ExtractedRawPath:     strings.TrimSpace(match),
				ExtractedAbsoluteURL: result.AbsoluteURL,
				Context:              contextSnippet,
				Type:                 pathType,
				DiscoveryTimestamp:   time.Now(),
			}

			extractedPaths = append(extractedPaths, extractedPath)
			seenPaths[result.AbsoluteURL] = struct{}{}
			processedCount++

			mra.logger.Debug().
				Str("source_url", sourceURL).
				Str("absolute_url", result.AbsoluteURL).
				Str("type", pathType).
				Msg("Added path from manual regex")
		}
	}

	return AnalysisResult{
		ExtractedPaths: extractedPaths,
		ProcessedCount: processedCount,
	}
}
