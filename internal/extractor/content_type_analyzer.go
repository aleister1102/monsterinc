package extractor

import (
	"strings"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// ContentTypeAnalyzer determines if content should be analyzed
type ContentTypeAnalyzer struct {
	logger          zerolog.Logger
	extractorConfig config.ExtractorConfig
}

// NewContentTypeAnalyzer creates a new content type analyzer
func NewContentTypeAnalyzer(extractorConfig config.ExtractorConfig, logger zerolog.Logger) *ContentTypeAnalyzer {
	return &ContentTypeAnalyzer{
		logger:          logger.With().Str("component", "ContentTypeAnalyzer").Logger(),
		extractorConfig: extractorConfig,
	}
}

// ShouldAnalyzeWithJSluice determines if content should be analyzed with jsluice
func (cta *ContentTypeAnalyzer) ShouldAnalyzeWithJSluice(sourceURL, contentType string) bool {
	// Check content type first
	isJavaScript := strings.Contains(contentType, "javascript")

	// If not detected by content type, check URL extension
	if !isJavaScript {
		urlLower := strings.ToLower(sourceURL)
		// Use common JavaScript extensions as fallback since we don't have monitor config here
		jsExtensions := []string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}
		for _, ext := range jsExtensions {
			if strings.HasSuffix(urlLower, ext) {
				isJavaScript = true
				break
			}
		}
	}

	cta.logger.Debug().
		Str("source_url", sourceURL).
		Str("content_type", contentType).
		Bool("is_javascript", isJavaScript).
		Msg("Content type analysis for jsluice")

	return isJavaScript
}
