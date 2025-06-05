package extractor

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/BishopFox/jsluice"
	"github.com/rs/zerolog"
)

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
		regexSet.CustomRegexes = common.CompileRegexes(extractorCfg.CustomRegexes, rc.logger)
		rc.logger.Debug().Int("compiled_count", len(regexSet.CustomRegexes)).Msg("Compiled custom regexes")
	}

	if len(extractorCfg.Allowlist) > 0 {
		regexSet.AllowlistRegexes = common.CompileRegexes(extractorCfg.Allowlist, rc.logger)
		rc.logger.Debug().Int("compiled_count", len(regexSet.AllowlistRegexes)).Msg("Compiled allowlist regexes")
	}

	if len(extractorCfg.Denylist) > 0 {
		regexSet.DenylistRegexes = common.CompileRegexes(extractorCfg.Denylist, rc.logger)
		rc.logger.Debug().Int("compiled_count", len(regexSet.DenylistRegexes)).Msg("Compiled denylist regexes")
	}

	return regexSet
}

// URLValidator handles URL validation and resolution
type URLValidator struct {
	logger zerolog.Logger
}

// NewURLValidator creates a new URL validator
func NewURLValidator(logger zerolog.Logger) *URLValidator {
	return &URLValidator{
		logger: logger.With().Str("component", "URLValidator").Logger(),
	}
}

// ValidationResult holds the result of URL validation
type ValidationResult struct {
	AbsoluteURL string
	IsValid     bool
	Error       error
}

// ValidateAndResolveURL validates and resolves a raw path to an absolute URL
func (uv *URLValidator) ValidateAndResolveURL(rawPath string, base *url.URL, sourceURL string) ValidationResult {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return ValidationResult{IsValid: false, Error: common.NewValidationError("raw_path", rawPath, "path cannot be empty")}
	}

	// Check if already absolute URL
	if result := uv.validateAbsoluteURL(rawPath); result.IsValid {
		return result
	}

	// Try to resolve relative URL
	if base != nil {
		if resolved, err := urlhandler.ResolveURL(rawPath, base); err == nil {
			return uv.validateResolvedURL(resolved)
		} else {
			uv.logger.Warn().Err(err).Str("raw_path", rawPath).Str("base_url", base.String()).Msg("Failed to resolve path")
		}
	}

	// Handle special cases for relative URLs without base
	if uv.isRelativeWithoutBase(rawPath, sourceURL) {
		return ValidationResult{IsValid: false, Error: common.NewError("cannot resolve relative path without valid base URL")}
	}

	// Final validation attempt
	return uv.validateResolvedURL(rawPath)
}

// validateAbsoluteURL checks if a URL is already absolute and valid
func (uv *URLValidator) validateAbsoluteURL(rawPath string) ValidationResult {
	parsedMatch, err := url.Parse(rawPath)
	if err != nil {
		return ValidationResult{IsValid: false, Error: common.WrapError(err, "failed to parse URL")}
	}

	if parsedMatch.Scheme != "" && parsedMatch.Host != "" {
		if !strings.Contains(parsedMatch.Host, ".") {
			uv.logger.Debug().Str("url", rawPath).Str("host", parsedMatch.Host).Msg("URL host seems invalid")
			return ValidationResult{IsValid: false, Error: common.NewValidationError("host", parsedMatch.Host, "host appears invalid")}
		}
		return ValidationResult{AbsoluteURL: rawPath, IsValid: true}
	}

	return ValidationResult{IsValid: false}
}

// validateResolvedURL performs final validation on a resolved URL
func (uv *URLValidator) validateResolvedURL(absoluteURL string) ValidationResult {
	finalParsed, err := url.Parse(absoluteURL)
	if err != nil {
		return ValidationResult{IsValid: false, Error: common.WrapError(err, "failed to parse resolved URL")}
	}

	if finalParsed.Scheme == "" || finalParsed.Host == "" || !strings.Contains(finalParsed.Host, ".") {
		return ValidationResult{IsValid: false, Error: common.NewValidationError("resolved_url", absoluteURL, "resolved URL is invalid")}
	}

	return ValidationResult{AbsoluteURL: absoluteURL, IsValid: true}
}

// isRelativeWithoutBase checks if path is relative and cannot be resolved without base
func (uv *URLValidator) isRelativeWithoutBase(rawPath, sourceURL string) bool {
	hasProtocol := strings.HasPrefix(rawPath, "http://") || strings.HasPrefix(rawPath, "https://") || strings.HasPrefix(rawPath, "//")
	if !hasProtocol {
		uv.logger.Warn().Str("raw_path", rawPath).Str("source_url", sourceURL).Msg("Relative path without valid base")
		return true
	}
	return false
}

// ContextExtractor handles extraction of context snippets
type ContextExtractor struct {
	logger      zerolog.Logger
	snippetSize int
}

// NewContextExtractor creates a new context extractor
func NewContextExtractor(snippetSize int, logger zerolog.Logger) *ContextExtractor {
	return &ContextExtractor{
		logger:      logger.With().Str("component", "ContextExtractor").Logger(),
		snippetSize: snippetSize,
	}
}

// ExtractContext extracts context around a match in the content
func (ce *ContextExtractor) ExtractContext(contentStr string, match string) string {
	matchStartIndex := strings.Index(contentStr, match)
	if matchStartIndex == -1 {
		ce.logger.Debug().Str("match", match).Msg("Match not found in content")
		return ""
	}

	start := matchStartIndex - ce.snippetSize
	if start < 0 {
		start = 0
	}

	end := matchStartIndex + len(match) + ce.snippetSize
	if end > len(contentStr) {
		end = len(contentStr)
	}

	context := contentStr[start:end]
	ce.logger.Debug().Str("match", match).Int("context_length", len(context)).Msg("Extracted context")

	return context
}

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

// AnalysisResult holds the result of jsluice analysis
type AnalysisResult struct {
	ExtractedPaths []models.ExtractedPath
	ProcessedCount int
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
	contentTypeAnalyzer := NewContentTypeAnalyzer(b.logger)

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

	b.logger.Info().Msg("PathExtractor initialized successfully")
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
		return common.NewValidationError("source_url", sourceURL, "source URL cannot be empty")
	}

	if content == nil {
		return common.NewValidationError("content", content, "content cannot be nil")
	}

	if pe.extractorConfig.MaxContentSize > 0 && int64(len(content)) > pe.extractorConfig.MaxContentSize {
		return common.NewValidationError("content", len(content),
			fmt.Sprintf("content too large (%d bytes > %d bytes limit)", len(content), pe.extractorConfig.MaxContentSize))
	}

	return nil
}

// parseBaseURL parses the source URL to get base URL for resolution
func (pe *PathExtractor) parseBaseURL(sourceURL string) (*url.URL, error) {
	base, err := url.Parse(sourceURL)
	if err != nil {
		pe.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Failed to parse source URL")
		return nil, common.WrapError(err, "failed to parse source URL: "+sourceURL)
	}
	return base, nil
}

// ExtractPaths uses jsluice for JavaScript AST-based analysis, then applies custom regexes
func (pe *PathExtractor) ExtractPaths(sourceURL string, content []byte, contentType string) ([]models.ExtractedPath, error) {
	// Validate inputs
	if err := pe.validateInputs(sourceURL, content, contentType); err != nil {
		return nil, common.WrapError(err, "failed to validate path extraction inputs")
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
		pe.logger.Debug().Int("jsluice_paths", len(jsluiceResult.ExtractedPaths)).Msg("JSluice analysis completed")
	} else {
		pe.logger.Debug().
			Str("source_url", sourceURL).
			Str("content_type", contentType).
			Bool("jsluice_enabled", pe.extractorConfig.EnableJSluiceAnalysis).
			Msg("Skipping jsluice analysis")
	}

	// Step 2: Manual regex-based scanning
	if pe.extractorConfig.EnableManualRegex {
		regexResult := pe.manualRegexAnalyzer.AnalyzeWithRegex(sourceURL, content, base, seenPaths)
		extractedPaths = append(extractedPaths, regexResult.ExtractedPaths...)
		pe.logger.Debug().Int("regex_paths", len(regexResult.ExtractedPaths)).Msg("Manual regex analysis completed")
	}

	pe.logger.Info().
		Str("source_url", sourceURL).
		Int("total_unique_extracted_count", len(extractedPaths)).
		Int("content_length", len(content)).
		Msg("Path extraction completed successfully")

	return extractedPaths, nil
}
