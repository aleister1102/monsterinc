package extractor

import "github.com/aleister1102/monsterinc/internal/models"

// PathExtractorConfig holds additional configuration for PathExtractor
type PathExtractorConfig struct {
	EnableJSluiceAnalysis bool
	EnableManualRegex     bool
	MaxContentSize        int64
	ContextSnippetSize    int
}

// DefaultPathExtractorConfig returns default configuration
func DefaultPathExtractorConfig() PathExtractorConfig {
	return PathExtractorConfig{
		EnableJSluiceAnalysis: true,
		EnableManualRegex:     true,
		MaxContentSize:        10 * 1024 * 1024, // 10MB
		ContextSnippetSize:    100,
	}
}

// ValidationResult holds the result of URL validation
type ValidationResult struct {
	AbsoluteURL string
	IsValid     bool
	Error       error
}

// AnalysisResult holds the result of jsluice analysis
type AnalysisResult struct {
	ExtractedPaths []models.ExtractedPath
	ProcessedCount int
}
