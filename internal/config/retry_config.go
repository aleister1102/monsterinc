package config

// RetryConfig defines configuration for HTTP request retries
type RetryConfig struct {
	// Maximum number of retry attempts for 429 (Too Many Requests) errors
	MaxRetries int `json:"max_retries,omitempty" yaml:"max_retries,omitempty" validate:"omitempty,min=0,max=10"`
	// Base delay in seconds for exponential backoff
	BaseDelaySecs int `json:"base_delay_secs,omitempty" yaml:"base_delay_secs,omitempty" validate:"omitempty,min=1,max=300"`
	// Maximum delay in seconds for exponential backoff
	MaxDelaySecs int `json:"max_delay_secs,omitempty" yaml:"max_delay_secs,omitempty" validate:"omitempty,min=1,max=3600"`
	// Enable jitter to randomize delays slightly
	EnableJitter bool `json:"enable_jitter" yaml:"enable_jitter"`
	// HTTP status codes that should trigger retries (default: [429])
	RetryStatusCodes []int `json:"retry_status_codes,omitempty" yaml:"retry_status_codes,omitempty"`
	// Domain-level rate limiting configuration
	DomainLevelRateLimit DomainRateLimitConfig `json:"domain_level_rate_limit,omitempty" yaml:"domain_level_rate_limit,omitempty"`
}

// DomainRateLimitConfig configures domain-level rate limiting behavior
type DomainRateLimitConfig struct {
	// Enable domain-level rate limiting
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Maximum number of 429 errors per domain before blacklisting
	MaxRateLimitErrors int `json:"max_rate_limit_errors,omitempty" yaml:"max_rate_limit_errors,omitempty" validate:"omitempty,min=1,max=100"`
	// Duration to blacklist domain after hitting max errors (in minutes)
	BlacklistDurationMins int `json:"blacklist_duration_mins,omitempty" yaml:"blacklist_duration_mins,omitempty" validate:"omitempty,min=1,max=1440"`
	// Clear blacklist after this many hours
	BlacklistClearAfterHours int `json:"blacklist_clear_after_hours,omitempty" yaml:"blacklist_clear_after_hours,omitempty" validate:"omitempty,min=1,max=72"`
}

// NewDefaultRetryConfig creates default retry configuration
func NewDefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       3,
		BaseDelaySecs:    10,
		MaxDelaySecs:     60,
		EnableJitter:     true,
		RetryStatusCodes: []int{429},
		DomainLevelRateLimit: DomainRateLimitConfig{
			Enabled:                  true,
			MaxRateLimitErrors:       10,
			BlacklistDurationMins:    30,
			BlacklistClearAfterHours: 6,
		},
	}
}
