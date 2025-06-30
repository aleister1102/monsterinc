package urlhandler

import (
	"fmt"
	"net/url"
	"strings"
)

// URLNormalizationConfig configures URL normalization behavior
type URLNormalizationConfig struct {
	// Strip fragments from URLs to avoid duplicates (e.g., #section)
	StripFragments bool
	// Strip common tracking parameters
	StripTrackingParams bool
	// List of parameters to strip (in addition to common tracking params)
	CustomStripParams []string
}

// DefaultURLNormalizationConfig returns default configuration
func DefaultURLNormalizationConfig() URLNormalizationConfig {
	return URLNormalizationConfig{
		StripFragments:      true,
		StripTrackingParams: true,
		CustomStripParams:   []string{"utm_source", "utm_medium", "utm_campaign", "fbclid", "gclid"},
	}
}

// URLNormalizer handles URL normalization operations
type URLNormalizer struct {
	config            URLNormalizationConfig
	trackingParams    map[string]bool
	customStripParams map[string]bool
}

// NewURLNormalizer creates a new URL normalizer
func NewURLNormalizer(config URLNormalizationConfig) *URLNormalizer {
	// Common tracking parameters to strip
	commonTrackingParams := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"fbclid", "gclid", "msclkid", "_ga", "_gl", "mc_cid", "mc_eid",
		"ref", "referrer", "campaign", "source", "medium",
	}

	trackingParams := make(map[string]bool)
	if config.StripTrackingParams {
		for _, param := range commonTrackingParams {
			trackingParams[param] = true
		}
	}

	customStripParams := make(map[string]bool)
	for _, param := range config.CustomStripParams {
		customStripParams[param] = true
	}

	return &URLNormalizer{
		config:            config,
		trackingParams:    trackingParams,
		customStripParams: customStripParams,
	}
}

// NormalizeURL normalizes a URL according to configuration
func (un *URLNormalizer) NormalizeURL(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Strip fragment if configured
	if un.config.StripFragments {
		parsedURL.Fragment = ""
	}

	// Strip tracking and custom parameters
	if un.config.StripTrackingParams || len(un.config.CustomStripParams) > 0 {
		un.stripQueryParameters(parsedURL)
	}

	return parsedURL.String(), nil
}

// stripQueryParameters removes tracking and custom parameters from URL
func (un *URLNormalizer) stripQueryParameters(parsedURL *url.URL) {
	if parsedURL.RawQuery == "" {
		return
	}

	values := parsedURL.Query()
	modified := false

	for param := range values {
		paramLower := strings.ToLower(param)

		// Check if it's a tracking parameter
		if un.trackingParams[paramLower] {
			values.Del(param)
			modified = true
		}

		// Check if it's a custom strip parameter
		if un.customStripParams[paramLower] {
			values.Del(param)
			modified = true
		}
	}

	if modified {
		parsedURL.RawQuery = values.Encode()
	}
}
