package config

// MonitorConfig defines configuration for monitoring service
type MonitorConfig struct {
	CheckIntervalSeconds int      `json:"check_interval_seconds,omitempty" yaml:"check_interval_seconds,omitempty" validate:"omitempty,min=1"`
	Enabled              bool     `json:"enabled" yaml:"enabled"`
	HTMLFileExtensions   []string `json:"html_file_extensions,omitempty" yaml:"html_file_extensions,omitempty"`
	HTTPTimeoutSeconds   int      `json:"http_timeout_seconds,omitempty" yaml:"http_timeout_seconds,omitempty" validate:"omitempty,min=1"`
	InitialMonitorURLs   []string `json:"initial_monitor_urls,omitempty" yaml:"initial_monitor_urls,omitempty" validate:"omitempty,dive,url"`
	JSFileExtensions     []string `json:"js_file_extensions,omitempty" yaml:"js_file_extensions,omitempty"`

	MaxConcurrentChecks         int  `json:"max_concurrent_checks,omitempty" yaml:"max_concurrent_checks,omitempty" validate:"omitempty,min=1"`
	MaxContentSize              int  `json:"max_content_size,omitempty" yaml:"max_content_size,omitempty" validate:"omitempty,min=1"` // Max content size in bytes
	MonitorInsecureSkipVerify   bool `json:"monitor_insecure_skip_verify" yaml:"monitor_insecure_skip_verify"`
	StoreFullContentOnChange    bool `json:"store_full_content_on_change" yaml:"store_full_content_on_change"`
	MaxDiffResultsPerReportFile int  `json:"max_diff_results_per_report_file,omitempty" yaml:"max_diff_results_per_report_file,omitempty" validate:"omitempty,min=1"` // Maximum number of diff results per HTML report file
}

// NewDefaultMonitorConfig creates default monitor configuration
func NewDefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		CheckIntervalSeconds: 3600, // 1 hour
		Enabled:              false,
		HTMLFileExtensions:   []string{".html", ".htm"},
		HTTPTimeoutSeconds:   30,
		InitialMonitorURLs:   []string{},
		JSFileExtensions:     []string{".js", ".jsx", ".ts", ".tsx"},

		MaxConcurrentChecks:         5,
		MaxContentSize:              1048576, // Default 1MB
		MonitorInsecureSkipVerify:   true,    // Default to true to match previous hardcoded behavior
		StoreFullContentOnChange:    true,
		MaxDiffResultsPerReportFile: 500, // Default to 500 diff results per report file
	}
}
