package config

import "github.com/aleister1102/monsterinc/internal/urlhandler"

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

// HeadlessBrowserConfig defines configuration for headless browser
type HeadlessBrowserConfig struct {
	Enabled             bool     `json:"enabled" yaml:"enabled"`
	ChromePath          string   `json:"chrome_path,omitempty" yaml:"chrome_path,omitempty"`
	UserDataDir         string   `json:"user_data_dir,omitempty" yaml:"user_data_dir,omitempty"`
	WindowWidth         int      `json:"window_width,omitempty" yaml:"window_width,omitempty" validate:"omitempty,min=100"`
	WindowHeight        int      `json:"window_height,omitempty" yaml:"window_height,omitempty" validate:"omitempty,min=100"`
	PageLoadTimeoutSecs int      `json:"page_load_timeout_secs,omitempty" yaml:"page_load_timeout_secs,omitempty" validate:"omitempty,min=1"`
	WaitAfterLoadMs     int      `json:"wait_after_load_ms,omitempty" yaml:"wait_after_load_ms,omitempty" validate:"omitempty,min=0"`
	DisableImages       bool     `json:"disable_images" yaml:"disable_images"`
	DisableCSS          bool     `json:"disable_css" yaml:"disable_css"`
	DisableJavaScript   bool     `json:"disable_javascript" yaml:"disable_javascript"`
	IgnoreHTTPSErrors   bool     `json:"ignore_https_errors" yaml:"ignore_https_errors"`
	PoolSize            int      `json:"pool_size,omitempty" yaml:"pool_size,omitempty" validate:"omitempty,min=1"`
	BrowserArgs         []string `json:"browser_args,omitempty" yaml:"browser_args,omitempty"`
}

// NewDefaultHeadlessBrowserConfig creates default headless browser configuration
func NewDefaultHeadlessBrowserConfig() HeadlessBrowserConfig {
	return HeadlessBrowserConfig{
		Enabled:             false,
		ChromePath:          "",
		UserDataDir:         "",
		WindowWidth:         1920,
		WindowHeight:        1080,
		PageLoadTimeoutSecs: 30,
		WaitAfterLoadMs:     1000,
		DisableImages:       true,
		DisableCSS:          false,
		DisableJavaScript:   false,
		IgnoreHTTPSErrors:   true,
		PoolSize:            3,
		BrowserArgs:         []string{"--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"},
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

	MaxConcurrentRequests int                   `json:"max_concurrent_requests,omitempty" yaml:"max_concurrent_requests,omitempty" validate:"omitempty,min=1"`
	MaxContentLengthMB    int                   `json:"max_content_length_mb,omitempty" yaml:"max_content_length_mb,omitempty"`
	MaxDepth              int                   `json:"max_depth,omitempty" yaml:"max_depth,omitempty" validate:"omitempty,min=0"`
	RequestTimeoutSecs    int                   `json:"request_timeout_secs,omitempty" yaml:"request_timeout_secs,omitempty" validate:"omitempty,min=1"`
	Scope                 CrawlerScopeConfig    `json:"scope,omitempty" yaml:"scope,omitempty"`
	SeedURLs              []string              `json:"seed_urls,omitempty" yaml:"seed_urls,omitempty" validate:"omitempty,dive,url"`
	HeadlessBrowser       HeadlessBrowserConfig `json:"headless_browser,omitempty" yaml:"headless_browser,omitempty"`
	AutoCalibrate         AutoCalibrateConfig   `json:"auto_calibrate,omitempty" yaml:"auto_calibrate,omitempty"`
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
		HeadlessBrowser:       NewDefaultHeadlessBrowserConfig(),
		AutoCalibrate:         NewDefaultAutoCalibrateConfig(),
		URLNormalization:      urlhandler.DefaultURLNormalizationConfig(),
		RetryConfig:           NewDefaultRetryConfig(),
	}
}
