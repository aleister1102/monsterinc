package extractor

import (
	"fmt"
	"net/url"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// PathExtractor is responsible for extracting paths from content
type PathExtractor struct {
	config              config.ExtractorConfig
	extractorConfig     PathExtractorConfig
	logger              zerolog.Logger
	regexSet            CompiledRegexSet
	urlValidator        *URLValidator
	contextExtractor    *ContextExtractor
	jsluiceAnalyzer     *JSluiceAnalyzer
	manualRegexAnalyzer *ManualRegexAnalyzer
	contentTypeAnalyzer *ContentTypeAnalyzer
}

// PathExtractorBuilder provides a fluent interface for creating PathExtractor
type PathExtractorBuilder struct {
	config          config.ExtractorConfig
	extractorConfig PathExtractorConfig
	logger          zerolog.Logger
}

// NewPathExtractorBuilder creates a new builder
func NewPathExtractorBuilder(logger zerolog.Logger) *PathExtractorBuilder {
	return &PathExtractorBuilder{
		logger:          logger.With().Str("component", "PathExtractor").Logger(),
		extractorConfig: DefaultPathExtractorConfig(),
	}
}

// WithExtractorConfig sets the extractor configuration
func (b *PathExtractorBuilder) WithExtractorConfig(cfg config.ExtractorConfig) *PathExtractorBuilder {
	b.config = cfg
	return b
}

// WithPathExtractorConfig sets the path extractor specific configuration
func (b *PathExtractorBuilder) WithPathExtractorConfig(cfg PathExtractorConfig) *PathExtractorBuilder {
	b.extractorConfig = cfg
	return b
}

// Build creates a new PathExtractor instance
func (b *PathExtractorBuilder) Build() (*PathExtractor, error) {
	// Compile regex sets
	regexCompiler := NewRegexCompiler(b.logger)
	regexSet := regexCompiler.CompileRegexSets(b.config)

	// Create components
	urlValidator := NewURLValidator(b.logger)
	contextExtractor := NewContextExtractor(b.extractorConfig.ContextSnippetSize, b.logger)
	jsluiceAnalyzer := NewJSluiceAnalyzer(urlValidator, contextExtractor, b.logger)
	manualRegexAnalyzer := NewManualRegexAnalyzer(regexSet.CustomRegexes, urlValidator, contextExtractor, b.logger)
	contentTypeAnalyzer := NewContentTypeAnalyzer(b.config, b.logger)

	pathExtractor := &PathExtractor{
		config:              b.config,
		extractorConfig:     b.extractorConfig,
		logger:              b.logger,
		regexSet:            regexSet,
		urlValidator:        urlValidator,
		contextExtractor:    contextExtractor,
		jsluiceAnalyzer:     jsluiceAnalyzer,
		manualRegexAnalyzer: manualRegexAnalyzer,
		contentTypeAnalyzer: contentTypeAnalyzer,
	}

	return pathExtractor, nil
}

// NewPathExtractor creates a new PathExtractor using builder pattern
func NewPathExtractor(extractorCfg config.ExtractorConfig, logger zerolog.Logger) (*PathExtractor, error) {
	return NewPathExtractorBuilder(logger).
		WithExtractorConfig(extractorCfg).
		Build()
}

// validateInputs validates the input parameters for path extraction
func (pe *PathExtractor) validateInputs(sourceURL string, content []byte, contentType string) error {
	if sourceURL == "" {
		return NewValidationError("source_url", sourceURL, "source URL cannot be empty")
	}

	if content == nil {
		return NewValidationError("content", content, "content cannot be nil")
	}

	if pe.extractorConfig.MaxContentSize > 0 && int64(len(content)) > pe.extractorConfig.MaxContentSize {
		return NewValidationError("content", len(content),
			fmt.Sprintf("content too large (%d bytes > %d bytes limit)", len(content), pe.extractorConfig.MaxContentSize))
	}

	return nil
}

// parseBaseURL parses the source URL to get base URL for resolution
func (pe *PathExtractor) parseBaseURL(sourceURL string) (*url.URL, error) {
	base, err := url.Parse(sourceURL)
	if err != nil {
		pe.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Failed to parse source URL")
		return nil, WrapError(err, "failed to parse source URL: "+sourceURL)
	}
	return base, nil
}

// ExtractPaths uses jsluice for JavaScript AST-based analysis, then applies custom regexes
func (pe *PathExtractor) ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error) {
	// Validate inputs
	if err := pe.validateInputs(sourceURL, content, contentType); err != nil {
		return nil, WrapError(err, "failed to validate path extraction inputs")
	}

	var extractedPaths []models.ExtractedPath
	seenPaths := make(map[string]struct{})

	// Parse base URL for relative path resolution
	base, err := pe.parseBaseURL(sourceURL)
	if err != nil {
		pe.logger.Warn().Err(err).Msg("Failed to parse base URL, relative path resolution may be affected")
		// Continue with base as nil
	}

	// Step 1: JSluice AST-based analysis (primarily for JavaScript)
	if pe.extractorConfig.EnableJSluiceAnalysis && pe.contentTypeAnalyzer.ShouldAnalyzeWithJSluice(sourceURL, contentType) {
		jsluiceResult := pe.jsluiceAnalyzer.AnalyzeJavaScript(sourceURL, content, base, seenPaths)
		extractedPaths = append(extractedPaths, jsluiceResult.ExtractedPaths...)
	}

	// Step 2: Manual regex-based scanning
	if pe.extractorConfig.EnableManualRegex {
		regexResult := pe.manualRegexAnalyzer.AnalyzeWithRegex(sourceURL, content, base, seenPaths)
		extractedPaths = append(extractedPaths, regexResult.ExtractedPaths...)
	}

	// TODO: Add other analyzers here

	return extractedPaths, nil
}
