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
}

// NewDefaultRetryConfig creates default retry configuration
func NewDefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       3,
		BaseDelaySecs:    10,
		MaxDelaySecs:     60,
		EnableJitter:     true,
		RetryStatusCodes: []int{429},
	}
}
