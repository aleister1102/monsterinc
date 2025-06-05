package extractor

import (
	"net/url"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/BishopFox/jsluice"
	"github.com/rs/zerolog"
)

// JSluiceAnalyzer handles JavaScript analysis using jsluice
type JSluiceAnalyzer struct {
	logger     zerolog.Logger
	validator  *URLValidator
	contextExt *ContextExtractor
}

// NewJSluiceAnalyzer creates a new jsluice analyzer
func NewJSluiceAnalyzer(validator *URLValidator, contextExt *ContextExtractor, logger zerolog.Logger) *JSluiceAnalyzer {
	return &JSluiceAnalyzer{
		logger:     logger.With().Str("component", "JSluiceAnalyzer").Logger(),
		validator:  validator,
		contextExt: contextExt,
	}
}

// AnalyzeJavaScript processes JavaScript content using jsluice
func (jsa *JSluiceAnalyzer) AnalyzeJavaScript(sourceURL string, content []byte, base *url.URL, seenPaths map[string]struct{}) AnalysisResult {
	analyzer := jsluice.NewAnalyzer(content)
	jsluiceResults := analyzer.GetURLs()

	var extractedPaths []models.ExtractedPath
	processedCount := 0

	for _, jsluiceRes := range jsluiceResults {
		result := jsa.validator.ValidateAndResolveURL(jsluiceRes.URL, base, sourceURL)
		if !result.IsValid {
			continue
		}

		if _, exists := seenPaths[result.AbsoluteURL]; exists {
			continue
		}

		pathType := jsluiceRes.Type
		if pathType == "" {
			pathType = "jsluice_default_unknown_type"
		}

		extractedPath := models.ExtractedPath{
			SourceURL:            sourceURL,
			ExtractedRawPath:     jsluiceRes.URL,
			ExtractedAbsoluteURL: result.AbsoluteURL,
			Context:              jsluiceRes.Source,
			Type:                 pathType,
			DiscoveryTimestamp:   time.Now(),
		}

		extractedPaths = append(extractedPaths, extractedPath)
		seenPaths[result.AbsoluteURL] = struct{}{}
		processedCount++
	}

	return AnalysisResult{
		ExtractedPaths: extractedPaths,
		ProcessedCount: processedCount,
	}
}
