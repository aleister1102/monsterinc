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
	jsa.logger.Debug().Str("source_url", sourceURL).Msg("Starting jsluice analysis")

	analyzer := jsluice.NewAnalyzer(content)
	jsluiceResults := analyzer.GetURLs()

	jsa.logger.Debug().Int("jsluice_url_count", len(jsluiceResults)).Msg("Jsluice analysis completed")

	var extractedPaths []models.ExtractedPath
	processedCount := 0

	for _, jsluiceRes := range jsluiceResults {
		result := jsa.validator.ValidateAndResolveURL(jsluiceRes.URL, base, sourceURL)
		if !result.IsValid {
			jsa.logger.Debug().Str("url", jsluiceRes.URL).Err(result.Error).Msg("Invalid URL from jsluice")
			continue
		}

		if _, exists := seenPaths[result.AbsoluteURL]; exists {
			jsa.logger.Debug().Str("absolute_url", result.AbsoluteURL).Msg("Duplicate URL from jsluice")
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

		jsa.logger.Debug().
			Str("source_url", sourceURL).
			Str("absolute_url", result.AbsoluteURL).
			Str("type", pathType).
			Msg("Added path from jsluice")
	}

	return AnalysisResult{
		ExtractedPaths: extractedPaths,
		ProcessedCount: processedCount,
	}
}
