package config

import "github.com/aleister1102/monsterinc/internal/common/urlhandler"

// CrawlerScopeConfig defines scope restrictions for the crawler
type CrawlerScopeConfig struct {
	DisallowedHostnames      []string `json:"disallowed_hostnames,omitempty" yaml:"disallowed_hostnames,omitempty"`
	DisallowedSubdomains     []string `json:"disallowed_subdomains,omitempty" yaml:"disallowed_subdomains,omitempty"`
	DisallowedFileExtensions []string `json:"disallowed_file_extensions,omitempty" yaml:"disallowed_file_extensions,omitempty"`
}

// NewDefaultCrawlerScopeConfig creates default crawler scope configuration
func NewDefaultCrawlerScopeConfig() CrawlerScopeConfig {
	return CrawlerScopeConfig{
		DisallowedHostnames:      []string{},
		DisallowedSubdomains:     []string{},
		DisallowedFileExtensions: []string{".js", ".txt", ".css", ".xml"},
	}
}

// AutoCalibrateConfig defines configuration for auto-calibrate feature
type AutoCalibrateConfig struct {
	// Whether auto-calibrate feature is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Maximum number of similar URLs to allow per pattern before skipping
	MaxSimilarURLs int `json:"max_similar_urls,omitempty" yaml:"max_similar_urls,omitempty" validate:"omitempty,min=1"`
	// Parameters to ignore when detecting similar URL patterns
	IgnoreParameters []string `json:"ignore_parameters,omitempty" yaml:"ignore_parameters,omitempty"`
	// Automatically detect and ignore locale codes in path segments
	AutoDetectLocales bool `json:"auto_detect_locales" yaml:"auto_detect_locales"`
	// Custom locale codes to recognize (in addition to built-in ones)
	CustomLocaleCodes []string `json:"custom_locale_codes,omitempty" yaml:"custom_locale_codes,omitempty"`
	// Enable logging when URLs are skipped due to pattern similarity
	EnableSkipLogging bool `json:"enable_skip_logging" yaml:"enable_skip_logging"`
}

// NewDefaultAutoCalibrateConfig creates default auto-calibrate configuration
func NewDefaultAutoCalibrateConfig() AutoCalibrateConfig {
	return AutoCalibrateConfig{
		Enabled:           true,
		MaxSimilarURLs:    1,
		IgnoreParameters:  []string{"tid", "fid", "page", "id", "p", "offset", "limit"},
		AutoDetectLocales: true, // Enable automatic locale detection
		CustomLocaleCodes: []string{},
		EnableSkipLogging: true,
	}
}

// CrawlerConfig defines the main crawler configuration
type CrawlerConfig struct {
	AutoAddSeedHostnames bool `json:"auto_add_seed_hostnames" yaml:"auto_add_seed_hostnames"`

	MaxConcurrentRequests int                 `json:"max_concurrent_requests,omitempty" yaml:"max_concurrent_requests,omitempty" validate:"omitempty,min=1"`
	MaxContentLengthMB    int                 `json:"max_content_length_mb,omitempty" yaml:"max_content_length_mb,omitempty"`
	MaxDepth              int                 `json:"max_depth,omitempty" yaml:"max_depth,omitempty" validate:"omitempty,min=0"`
	RequestTimeoutSecs    int                 `json:"request_timeout_secs,omitempty" yaml:"request_timeout_secs,omitempty" validate:"omitempty,min=1"`
	Scope                 CrawlerScopeConfig  `json:"scope,omitempty" yaml:"scope,omitempty"`
	SeedURLs              []string            `json:"seed_urls,omitempty" yaml:"seed_urls,omitempty" validate:"omitempty,dive,url"`
	AutoCalibrate         AutoCalibrateConfig `json:"auto_calibrate,omitempty" yaml:"auto_calibrate,omitempty"`
	// URL normalization configuration
	URLNormalization urlhandler.URLNormalizationConfig `json:"url_normalization,omitempty" yaml:"url_normalization,omitempty"`
	// Retry configuration for handling rate limits (429 errors)
	RetryConfig RetryConfig `json:"retry_config,omitempty" yaml:"retry_config,omitempty"`
}

// NewDefaultCrawlerConfig creates default crawler configuration
func NewDefaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		AutoAddSeedHostnames: true,

		MaxConcurrentRequests: DefaultCrawlerMaxConcurrentRequests,
		MaxContentLengthMB:    2,
		MaxDepth:              DefaultCrawlerMaxDepth,
		RequestTimeoutSecs:    DefaultCrawlerRequestTimeoutSecs,
		Scope:                 NewDefaultCrawlerScopeConfig(),
		SeedURLs:              []string{},
		AutoCalibrate:         NewDefaultAutoCalibrateConfig(),
		URLNormalization:      urlhandler.DefaultURLNormalizationConfig(),
		RetryConfig:           NewDefaultRetryConfig(),
	}
}
