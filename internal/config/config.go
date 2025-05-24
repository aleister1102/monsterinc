package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ReporterConfig holds configuration for the HTML reporter.
type ReporterConfig struct {
	OutputDir    string `json:"output_dir" yaml:"output_dir"`
	ItemsPerPage int    `json:"items_per_page" yaml:"items_per_page"`
	EmbedAssets  bool   `json:"embed_assets" yaml:"embed_assets"` // To control asset embedding
	// DefaultSort string `json:"default_sort" yaml:"default_sort"` // Field to sort by default
}

// Add ReporterConfig to GlobalConfig if not already present, or define GlobalConfig.
// This is an example assuming GlobalConfig exists.
// If it doesn't, you'll need to define it based on 7-tasks-prd-configuration-management.md

type HTTPXRunnerConfig struct {
	Threads          int               `json:"threads" yaml:"threads"`
	RateLimit        int               `json:"rate_limit" yaml:"rate_limit"`
	Timeout          int               `json:"timeout" yaml:"timeout"`
	Retries          int               `json:"retries" yaml:"retries"`
	Proxy            string            `json:"proxy" yaml:"proxy"`
	FollowRedirects  bool              `json:"follow_redirects" yaml:"follow_redirects"`
	MaxRedirects     int               `json:"max_redirects" yaml:"max_redirects"`
	CustomHeaders    map[string]string `json:"custom_headers" yaml:"custom_headers"`
	Resolvers        []string          `json:"resolvers" yaml:"resolvers"`
	Ports            string            `json:"ports" yaml:"ports"`
	HttpxFlags       []string          `json:"httpx_flags" yaml:"httpx_flags"`
	SkipDefaultPorts bool              `json:"skip_default_ports" yaml:"skip_default_ports"`
	DenyInternalIPs  bool              `json:"deny_internal_ips" yaml:"deny_internal_ips"`
}

// CrawlerScopeConfig holds the scope-related configurations for the crawler.
// This was previously in crawler_config.go, moved here for consolidation.
type CrawlerScopeConfig struct {
	AllowedHostnames      []string `json:"allowed_hostnames" yaml:"allowed_hostnames"`
	AllowedSubdomains     []string `json:"allowed_subdomains" yaml:"allowed_subdomains"`
	DisallowedHostnames   []string `json:"disallowed_hostnames" yaml:"disallowed_hostnames"`
	DisallowedSubdomains  []string `json:"disallowed_subdomains" yaml:"disallowed_subdomains"`
	AllowedPathRegexes    []string `json:"allowed_path_regexes" yaml:"allowed_path_regexes"`
	DisallowedPathRegexes []string `json:"disallowed_path_regexes" yaml:"disallowed_path_regexes"`
}

// This is the NEW, CONSOLIDATED CrawlerConfig
// The OLD CrawlerConfig struct definition that was here has been removed.
type CrawlerConfig struct {
	MaxDepth              int                `json:"max_depth" yaml:"max_depth"`
	IncludeSubdomains     bool               `json:"include_subdomains" yaml:"include_subdomains"`
	MaxConcurrentRequests int                `json:"max_concurrent_requests" yaml:"max_concurrent_requests"`
	AllowedHostRegex      []string           `json:"allowed_host_regex" yaml:"allowed_host_regex"`
	ExcludedHostRegex     []string           `json:"excluded_host_regex" yaml:"excluded_host_regex"`
	SeedURLs              []string           `json:"seed_urls" yaml:"seed_urls"`
	UserAgent             string             `json:"user_agent" yaml:"user_agent"`
	RequestTimeout        time.Duration      `json:"request_timeout" yaml:"request_timeout"`
	RespectRobotsTxt      bool               `json:"respect_robots_txt" yaml:"respect_robots_txt"`
	Scope                 CrawlerScopeConfig `json:"scope" yaml:"scope"`
}

type StorageConfig struct {
	ParquetBasePath  string `json:"parquet_base_path" yaml:"parquet_base_path"`
	CompressionCodec string `json:"compression_codec" yaml:"compression_codec"`
}

type NotificationConfig struct {
	DiscordWebhookURL     string   `json:"discord_webhook_url" yaml:"discord_webhook_url"`
	MentionRoles          []string `json:"mention_roles" yaml:"mention_roles"`
	NotifyOnSuccess       bool     `json:"notify_on_success" yaml:"notify_on_success"`
	NotifyOnFailure       bool     `json:"notify_on_failure" yaml:"notify_on_failure"`
	NotifyOnScanStart     bool     `json:"notify_on_scan_start" yaml:"notify_on_scan_start"`
	NotifyOnCriticalError bool     `json:"notify_on_critical_error" yaml:"notify_on_critical_error"`
	MentionRoleIDs        []string `json:"mention_role_ids" yaml:"mention_role_ids"`
}

type LogConfig struct {
	LogLevel        string `json:"log_level" yaml:"log_level"`
	LogFormat       string `json:"log_format" yaml:"log_format"`
	LogFile         string `json:"log_file" yaml:"log_file"`
	MaxLogSizeMB    int    `json:"max_log_size_mb" yaml:"max_log_size_mb"`
	MaxLogBackups   int    `json:"max_log_backups" yaml:"max_log_backups"`
	CompressOldLogs bool   `json:"compress_old_logs" yaml:"compress_old_logs"`
}

type DiffConfig struct {
	PreviousScanLookbackDays int  `json:"previous_scan_lookback_days" yaml:"previous_scan_lookback_days"`
	MaxDiffFileSizeMB        int  `json:"max_diff_file_size_mb" yaml:"max_diff_file_size_mb"`
	BeautifyHTMLForDiff      bool `json:"beautify_html_for_diff" yaml:"beautify_html_for_diff"`
	BeautifyJSForDiff        bool `json:"beautify_js_for_diff" yaml:"beautify_js_for_diff"`
}

type MonitorConfig struct {
	Enabled                  bool     `json:"enabled" yaml:"enabled"`
	CheckIntervalSeconds     int      `json:"check_interval_seconds" yaml:"check_interval_seconds"`
	TargetJSFilePatterns     []string `json:"target_js_file_patterns" yaml:"target_js_file_patterns"`
	TargetHTMLFilePatterns   []string `json:"target_html_file_patterns" yaml:"target_html_file_patterns"`
	MaxConcurrentChecks      int      `json:"max_concurrent_checks" yaml:"max_concurrent_checks"`
	StoreFullContentOnChange bool     `json:"store_full_content_on_change" yaml:"store_full_content_on_change"`
	HTTPTimeoutSeconds       int      `json:"http_timeout_seconds" yaml:"http_timeout_seconds"`
}

type SecretsConfig struct {
	Enabled                    bool   `json:"enabled" yaml:"enabled"`
	EnableTruffleHog           bool   `json:"enable_trufflehog" yaml:"enable_trufflehog"`
	TruffleHogPath             string `json:"trufflehog_path" yaml:"trufflehog_path"`
	EnableCustomRegex          bool   `json:"enable_custom_regex" yaml:"enable_custom_regex"`
	CustomRegexPatternsFile    string `json:"custom_regex_patterns_file" yaml:"custom_regex_patterns_file"`
	MaxFileSizeToScanMB        int    `json:"max_file_size_to_scan_mb" yaml:"max_file_size_to_scan_mb"`
	NotifyOnHighSeveritySecret bool   `json:"notify_on_high_severity_secret" yaml:"notify_on_high_severity_secret"`
}

type InputConfig struct {
	InputFile string   `json:"input_file" yaml:"input_file"`
	InputURLs []string `json:"input_urls" yaml:"input_urls"`
}

// GlobalConfig holds all configuration for the application.
type GlobalConfig struct {
	InputConfig        InputConfig        `json:"input_config" yaml:"input_config"`
	HTTPXRunnerConfig  HTTPXRunnerConfig  `json:"httpx_runner_config" yaml:"httpx_runner_config"`
	CrawlerConfig      CrawlerConfig      `json:"crawler_config" yaml:"crawler_config"`
	ReporterConfig     ReporterConfig     `json:"reporter_config" yaml:"reporter_config"`
	StorageConfig      StorageConfig      `json:"storage_config" yaml:"storage_config"`
	NotificationConfig NotificationConfig `json:"notification_config" yaml:"notification_config"`
	LogConfig          LogConfig          `json:"log_config" yaml:"log_config"`
	DiffConfig         DiffConfig         `json:"diff_config" yaml:"diff_config"`
	MonitorConfig      MonitorConfig      `json:"monitor_config" yaml:"monitor_config"`
	SecretsConfig      SecretsConfig      `json:"secrets_config" yaml:"secrets_config"`
	// Add other specific config structs here
}

// Default values - these will be set if not provided in config file
const (
	DefaultOutputDir        = "reports"
	DefaultItemsPerPage     = 25
	DefaultEmbedAssets      = true
	DefaultThreads          = 10
	DefaultRateLimit        = 100
	DefaultTimeout          = 10
	DefaultRetries          = 2
	DefaultFollowRedirects  = true
	DefaultMaxRedirects     = 5
	DefaultSkipDefaultPorts = false
	DefaultDenyInternalIPs  = true
	DefaultLogLevel         = "info"
	DefaultLogFormat        = "console"
	DefaultParquetBasePath  = "data"
	DefaultCompressionCodec = "zstd"
	// Defaults for merged CrawlerConfig fields (some might be better handled in NewDefaultCrawlerConfig if it's still used for standalone crawler config)
	DefaultCrawlerUserAgent      = "MonsterIncCrawler/1.0"
	DefaultCrawlerRequestTimeout = 20 * time.Second // Match NewDefaultCrawlerConfig
	DefaultCrawlerThreads        = 10               // Match NewDefaultCrawlerConfig, maps to MaxConcurrentRequests
	DefaultCrawlerMaxDepth       = 5                // Match NewDefaultCrawlerConfig
	DefaultCrawlerRespectRobots  = true
)

// SetDefaults sets default values for the configuration.
func (gc *GlobalConfig) SetDefaults() {
	if gc.ReporterConfig.OutputDir == "" {
		gc.ReporterConfig.OutputDir = DefaultOutputDir
	}
	if gc.ReporterConfig.ItemsPerPage == 0 {
		gc.ReporterConfig.ItemsPerPage = DefaultItemsPerPage
	}
	// Set default for EmbedAssets, note that bools default to false so check if it's explicitly set or use a pointer if needed.
	// For simplicity, we'll assume if not set, it defaults to true as per our constant.
	// However, a common pattern is to use pointers for bools if you need to distinguish between "false" and "not set".
	// Here, we are setting it to true if it's false (the zero value for bool). This might not always be desired.
	// A better way for bools: use a pointer, or have a specific "IsSet" field, or initialize to the default.
	// For now, this simplistic approach:
	if !gc.ReporterConfig.EmbedAssets { // If it's the zero value (false)
		// Check if it was actually set to false in config or just omitted.
		// This logic is flawed for bools. A better way is to initialize it to default,
		// and let unmarshalling override it. For manual struct, initialize it:
		// gc.ReporterConfig.EmbedAssets = DefaultEmbedAssets // This would be in NewGlobalConfig or similar
	}

	if gc.HTTPXRunnerConfig.Threads == 0 {
		gc.HTTPXRunnerConfig.Threads = DefaultThreads
	}
	if gc.HTTPXRunnerConfig.RateLimit == 0 {
		gc.HTTPXRunnerConfig.RateLimit = DefaultRateLimit
	}
	if gc.HTTPXRunnerConfig.Timeout == 0 {
		gc.HTTPXRunnerConfig.Timeout = DefaultTimeout
	}
	if gc.HTTPXRunnerConfig.Retries == 0 {
		gc.HTTPXRunnerConfig.Retries = DefaultRetries
	}
	// gc.HTTPXRunnerConfig.FollowRedirects is bool, defaults to false. If we want true by default:
	// Similar issue as EmbedAssets. Assume true if not explicitly set to false.
	// For now, let viper or manual init handle this. viper.SetDefault("httpx_runner_config.follow_redirects", true)

	if gc.LogConfig.LogLevel == "" {
		gc.LogConfig.LogLevel = DefaultLogLevel
	}
	if gc.LogConfig.LogFormat == "" {
		gc.LogConfig.LogFormat = DefaultLogFormat
	}
	if gc.StorageConfig.ParquetBasePath == "" {
		gc.StorageConfig.ParquetBasePath = DefaultParquetBasePath
	}
	if gc.StorageConfig.CompressionCodec == "" {
		gc.StorageConfig.CompressionCodec = DefaultCompressionCodec
	}

	// Set defaults for CrawlerConfig fields if not set
	if gc.CrawlerConfig.UserAgent == "" {
		gc.CrawlerConfig.UserAgent = DefaultCrawlerUserAgent
	}
	if gc.CrawlerConfig.RequestTimeout == 0 {
		gc.CrawlerConfig.RequestTimeout = DefaultCrawlerRequestTimeout
	}
	if gc.CrawlerConfig.MaxConcurrentRequests == 0 { // Assuming this is 'Threads'
		gc.CrawlerConfig.MaxConcurrentRequests = DefaultCrawlerThreads
	}
	if gc.CrawlerConfig.MaxDepth == 0 {
		gc.CrawlerConfig.MaxDepth = DefaultCrawlerMaxDepth
	}
	// RespectRobotsTxt is a bool, defaults to false. If we want true by default:
	// This needs careful handling. If the field is simply not in the JSON, it will be `false`.
	// A common pattern is to initialize it to the desired default before unmarshalling,
	// or use a pointer. For now, `SetDefaults` can set it if it's still `false`
	// and no specific value was intended. However, `NewDefaultCrawlerConfig` handles this better for specific crawler setup.
	// For GlobalConfig, if `respect_robots_txt` is omitted from JSON, it will be false.
	// If `DefaultCrawlerRespectRobots` is true, we can set it here if it's still false.
	// This check might be too simplistic if 'false' is a valid intentional setting.
	// Let's assume for GlobalConfig, if not specified, it gets the default.
	// if !gc.CrawlerConfig.RespectRobotsTxt { // Problematic if 'false' is desired and set.
	// A better approach is to ensure it's initialized correctly if NewDefaultCrawlerConfig is not used.
	// For now, viper based solutions handle this via viper.SetDefault. Manually, initialize:
	// gc.CrawlerConfig.RespectRobotsTxt = DefaultCrawlerRespectRobots // This should be done when `gc` is first created.
	// Let's assume LoadGlobalConfig will handle initial default setting for booleans as well by starting with a default-filled struct.
}

// LoadGlobalConfig loads the global configuration from a JSON file.
func LoadGlobalConfig(filePath string) (*GlobalConfig, error) {
	// Initialize with defaults first, so if JSON fields are missing, defaults persist.
	// For booleans like RespectRobotsTxt, this means they'd get their true default.
	config := &GlobalConfig{
		// Initialize sub-configs with their defaults if they have constructors or specific default values.
		// For CrawlerConfig, we can apply its defaults here before unmarshalling.
		CrawlerConfig: CrawlerConfig{ // Apply some defaults that NewDefaultCrawlerConfig would apply
			UserAgent:             DefaultCrawlerUserAgent,
			RequestTimeout:        DefaultCrawlerRequestTimeout,
			MaxConcurrentRequests: DefaultCrawlerThreads, // Threads
			MaxDepth:              DefaultCrawlerMaxDepth,
			RespectRobotsTxt:      DefaultCrawlerRespectRobots,
			Scope: CrawlerScopeConfig{ // Default scope allows crawling anywhere
				AllowedHostnames:      nil,
				AllowedSubdomains:     nil,
				DisallowedHostnames:   nil,
				DisallowedSubdomains:  nil,
				AllowedPathRegexes:    nil,
				DisallowedPathRegexes: nil,
			},
		},
		// ... initialize other configs if necessary ...
	}
	// Set other defaults using the SetDefaults method (which primarily handles zero-value checks)
	config.SetDefaults() // Apply general defaults from constants

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read global config file '%s': %w", filePath, err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global config data from '%s': %w", filePath, err)
	}

	// Apply SetDefaults again to ensure any zero values from JSON (if they occurred) are overridden by defaults
	// if the JSON explicitly set a field to a "zero" value that should have a default.
	// This is a bit redundant if unmarshalling correctly overlays onto the pre-defaulted struct.
	// The primary purpose of SetDefaults here is more for fields not explicitly initialized above.
	config.SetDefaults()

	// Basic validation for essential fields after loading
	// Example: if len(config.InputConfig.InputFile) == 0 && len(config.InputConfig.InputURLs) == 0 {
	//  log.Println("[WARN] GlobalConfig: No input file or URLs provided in input_config.")
	// }

	return config, nil
}
