package extractor

import (
	"strings"

	"github.com/rs/zerolog"
)

// ContentTypeAnalyzer determines if content should be analyzed
type ContentTypeAnalyzer struct {
	logger zerolog.Logger
}

// NewContentTypeAnalyzer creates a new content type analyzer
func NewContentTypeAnalyzer(logger zerolog.Logger) *ContentTypeAnalyzer {
	return &ContentTypeAnalyzer{
		logger: logger.With().Str("component", "ContentTypeAnalyzer").Logger(),
	}
}

// ShouldAnalyzeWithJSluice determines if content should be analyzed with jsluice
func (cta *ContentTypeAnalyzer) ShouldAnalyzeWithJSluice(sourceURL, contentType string) bool {
	isJavaScript := strings.Contains(contentType, "javascript") || strings.HasSuffix(sourceURL, ".js")

	cta.logger.Debug().
		Str("source_url", sourceURL).
		Str("content_type", contentType).
		Bool("is_javascript", isJavaScript).
		Msg("Content type analysis for jsluice")

	return isJavaScript
}
