package extractor

import (
	"net/url"
	"strings"

	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

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

// ValidateAndResolveURL validates and resolves a raw path to an absolute URL
func (uv *URLValidator) ValidateAndResolveURL(rawPath string, base *url.URL, sourceURL string) ValidationResult {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return ValidationResult{IsValid: false, Error: NewValidationError("raw_path", rawPath, "path cannot be empty")}
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
		return ValidationResult{IsValid: false, Error: NewError("cannot resolve relative path without valid base URL")}
	}

	// Final validation attempt
	return uv.validateResolvedURL(rawPath)
}

// validateAbsoluteURL checks if a URL is already absolute and valid
func (uv *URLValidator) validateAbsoluteURL(rawPath string) ValidationResult {
	// First check if URL is absolute using urlhandler
	if !urlhandler.IsAbsoluteURL(rawPath) {
		return ValidationResult{IsValid: false}
	}

	// Validate URL format
	if err := urlhandler.ValidateURLFormat(rawPath); err != nil {
		return ValidationResult{IsValid: false, Error: WrapError(err, "failed to validate URL format")}
	}

	parsedMatch, err := url.Parse(rawPath)
	if err != nil {
		return ValidationResult{IsValid: false, Error: WrapError(err, "failed to parse URL")}
	}

	if !strings.Contains(parsedMatch.Host, ".") {
		uv.logger.Debug().Str("url", rawPath).Str("host", parsedMatch.Host).Msg("URL host seems invalid")
		return ValidationResult{IsValid: false, Error: NewValidationError("host", parsedMatch.Host, "host appears invalid")}
	}

	return ValidationResult{AbsoluteURL: rawPath, IsValid: true}
}

// validateResolvedURL performs final validation on a resolved URL
func (uv *URLValidator) validateResolvedURL(absoluteURL string) ValidationResult {
	// Check if URL is absolute using urlhandler
	if !urlhandler.IsAbsoluteURL(absoluteURL) {
		return ValidationResult{IsValid: false, Error: NewValidationError("resolved_url", absoluteURL, "resolved URL is not absolute")}
	}

	// Validate URL format using urlhandler
	if err := urlhandler.ValidateURLFormat(absoluteURL); err != nil {
		return ValidationResult{IsValid: false, Error: WrapError(err, "failed to validate resolved URL format")}
	}

	finalParsed, err := url.Parse(absoluteURL)
	if err != nil {
		return ValidationResult{IsValid: false, Error: WrapError(err, "failed to parse resolved URL")}
	}

	if !strings.Contains(finalParsed.Host, ".") {
		return ValidationResult{IsValid: false, Error: NewValidationError("resolved_url", absoluteURL, "resolved URL host appears invalid")}
	}

	return ValidationResult{AbsoluteURL: absoluteURL, IsValid: true}
}

// isRelativeWithoutBase checks if path is relative and cannot be resolved without base
func (uv *URLValidator) isRelativeWithoutBase(rawPath, sourceURL string) bool {
	// Use urlhandler to check if URL is absolute
	if urlhandler.IsAbsoluteURL(rawPath) {
		return false
	}

	// If not absolute and we don't have protocol prefix, it's relative
	hasProtocol := strings.HasPrefix(rawPath, "//")
	if !hasProtocol {
		uv.logger.Warn().Str("raw_path", rawPath).Str("source_url", sourceURL).Msg("Relative path without valid base")
		return true
	}

	return false
}
