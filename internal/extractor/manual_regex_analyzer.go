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
		return AnalysisResult{}
	}

	contentStr := string(content)

	var extractedPaths []models.ExtractedPath
	processedCount := 0

	for i, customRegex := range mra.customRegexes {
		matches := customRegex.FindAllString(contentStr, -1)
		if len(matches) == 0 {
			continue
		}

		for _, match := range matches {
			result := mra.validator.ValidateAndResolveURL(match, base, sourceURL)
			if !result.IsValid {
				continue
			}

			if _, exists := seenPaths[result.AbsoluteURL]; exists {
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
		}
	}

	return AnalysisResult{
		ExtractedPaths: extractedPaths,
		ProcessedCount: processedCount,
	}
}
