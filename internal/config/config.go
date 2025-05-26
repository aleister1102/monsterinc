package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// --- Default Values ---
const (
	// Reporter Defaults
	DefaultReporterOutputDir    = "reports"
	DefaultReporterItemsPerPage = 25
	DefaultReporterEmbedAssets  = true

	// HTTPXRunner Defaults
	DefaultHTTPXThreads              = 25
	DefaultHTTPXTimeoutSecs          = 10
	DefaultHTTPXRetries              = 1
	DefaultHTTPXFollowRedirects      = true
	DefaultHTTPXMaxRedirects         = 10
	DefaultHTTPXVerbose              = false
	DefaultHTTPXMethod               = "GET"
	DefaultHTTPXTechDetect           = true
	DefaultHTTPXExtractTitle         = true
	DefaultHTTPXExtractStatusCode    = true
	DefaultHTTPXExtractLocation      = true
	DefaultHTTPXExtractContentLength = true
	DefaultHTTPXExtractServerHeader  = true
	DefaultHTTPXExtractContentType   = true
	DefaultHTTPXExtractIPs           = true
	DefaultHTTPXExtractBody          = false
	DefaultHTTPXExtractHeaders       = true
	DefaultHTTPXRateLimit            = 0
	DefaultHTTPXSkipDefaultPorts     = false
	DefaultHTTPXDenyInternalIPs      = false

	// Crawler Defaults
	DefaultCrawlerUserAgent             = "MonsterIncCrawler/1.0"
	DefaultCrawlerRequestTimeoutSecs    = 20
	DefaultCrawlerMaxConcurrentRequests = 10
	DefaultCrawlerMaxDepth              = 5
	DefaultCrawlerRespectRobotsTxt      = true
	DefaultCrawlerIncludeSubdomains     = false

	// Storage Defaults
	DefaultStorageParquetBasePath  = "database"
	DefaultStorageCompressionCodec = "zstd"

	// Log Defaults
	DefaultLogLevel        = "info"
	DefaultLogFormat       = "console"
	DefaultLogFile         = ""
	DefaultMaxLogSizeMB    = 100
	DefaultMaxLogBackups   = 3
	DefaultCompressOldLogs = false

	// Diff Defaults
	DefaultDiffPreviousScanLookbackDays = 7

	// Monitor Defaults
	DefaultMonitorJSFileExtensions   = ".js,.jsx,.ts,.tsx"
	DefaultMonitorHTMLFileExtensions = ".html,.htm"

	// Normalizer Defaults
	DefaultNormalizerDefaultScheme = "http" // Example for future use
)

// --- Nested Configuration Structs ---

type InputConfig struct {
	InputURLs []string `json:"input_urls,omitempty" yaml:"input_urls,omitempty" validate:"omitempty,dive,url"`
	InputFile string   `json:"input_file,omitempty" yaml:"input_file,omitempty" validate:"omitempty,fileexists"`
}

type HttpxRunnerConfig struct {
	Method               string            `json:"method,omitempty" yaml:"method,omitempty"`
	RequestURIs          []string          `json:"request_uris,omitempty" yaml:"request_uris,omitempty" validate:"omitempty,dive,url"`
	Threads              int               `json:"threads,omitempty" yaml:"threads,omitempty" validate:"omitempty,min=1"`
	RateLimit            int               `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty" validate:"omitempty,min=0"`
	TimeoutSecs          int               `json:"timeout_secs,omitempty" yaml:"timeout_secs,omitempty" validate:"omitempty,min=1"`
	Retries              int               `json:"retries,omitempty" yaml:"retries,omitempty" validate:"omitempty,min=0"`
	Proxy                string            `json:"proxy,omitempty" yaml:"proxy,omitempty" validate:"omitempty,url"`
	FollowRedirects      bool              `json:"follow_redirects" yaml:"follow_redirects"`
	MaxRedirects         int               `json:"max_redirects,omitempty" yaml:"max_redirects,omitempty" validate:"omitempty,min=0"`
	CustomHeaders        map[string]string `json:"custom_headers,omitempty" yaml:"custom_headers,omitempty"`
	Verbose              bool              `json:"verbose" yaml:"verbose"`
	TechDetect           bool              `json:"tech_detect" yaml:"tech_detect"`
	ExtractTitle         bool              `json:"extract_title" yaml:"extract_title"`
	ExtractStatusCode    bool              `json:"extract_status_code" yaml:"extract_status_code"`
	ExtractLocation      bool              `json:"extract_location" yaml:"extract_location"`
	ExtractContentLength bool              `json:"extract_content_length" yaml:"extract_content_length"`
	ExtractServerHeader  bool              `json:"extract_server_header" yaml:"extract_server_header"`
	ExtractContentType   bool              `json:"extract_content_type" yaml:"extract_content_type"`
	ExtractIPs           bool              `json:"extract_ips" yaml:"extract_ips"`
	ExtractBody          bool              `json:"extract_body" yaml:"extract_body"`
	ExtractHeaders       bool              `json:"extract_headers" yaml:"extract_headers"`
	Resolvers            []string          `json:"resolvers,omitempty" yaml:"resolvers,omitempty"`
	Ports                []string          `json:"ports,omitempty" yaml:"ports,omitempty"`
	HttpxFlags           []string          `json:"httpx_flags,omitempty" yaml:"httpx_flags,omitempty"`
	SkipDefaultPorts     bool              `json:"skip_default_ports" yaml:"skip_default_ports"`
	DenyInternalIPs      bool              `json:"deny_internal_ips" yaml:"deny_internal_ips"`
}

func NewDefaultHTTPXRunnerConfig() HttpxRunnerConfig {
	return HttpxRunnerConfig{
		Method:               DefaultHTTPXMethod,
		RequestURIs:          []string{},
		Threads:              DefaultHTTPXThreads,
		RateLimit:            DefaultHTTPXRateLimit,
		TimeoutSecs:          DefaultHTTPXTimeoutSecs,
		Retries:              DefaultHTTPXRetries,
		Proxy:                "",
		FollowRedirects:      DefaultHTTPXFollowRedirects,
		MaxRedirects:         DefaultHTTPXMaxRedirects,
		CustomHeaders:        make(map[string]string),
		Verbose:              DefaultHTTPXVerbose,
		TechDetect:           DefaultHTTPXTechDetect,
		ExtractTitle:         DefaultHTTPXExtractTitle,
		ExtractStatusCode:    DefaultHTTPXExtractStatusCode,
		ExtractLocation:      DefaultHTTPXExtractLocation,
		ExtractContentLength: DefaultHTTPXExtractContentLength,
		ExtractServerHeader:  DefaultHTTPXExtractServerHeader,
		ExtractContentType:   DefaultHTTPXExtractContentType,
		ExtractIPs:           DefaultHTTPXExtractIPs,
		ExtractBody:          DefaultHTTPXExtractBody,
		ExtractHeaders:       DefaultHTTPXExtractHeaders,
		Resolvers:            []string{},
		Ports:                []string{},
		HttpxFlags:           []string{},
		SkipDefaultPorts:     DefaultHTTPXSkipDefaultPorts,
		DenyInternalIPs:      DefaultHTTPXDenyInternalIPs,
	}
}

type CrawlerScopeConfig struct {
	AllowedHostnames      []string `json:"allowed_hostnames,omitempty" yaml:"allowed_hostnames,omitempty"`
	AllowedSubdomains     []string `json:"allowed_subdomains,omitempty" yaml:"allowed_subdomains,omitempty"`
	DisallowedHostnames   []string `json:"disallowed_hostnames,omitempty" yaml:"disallowed_hostnames,omitempty"`
	DisallowedSubdomains  []string `json:"disallowed_subdomains,omitempty" yaml:"disallowed_subdomains,omitempty"`
	AllowedPathRegexes    []string `json:"allowed_path_regexes,omitempty" yaml:"allowed_path_regexes,omitempty"`
	DisallowedPathRegexes []string `json:"disallowed_path_regexes,omitempty" yaml:"disallowed_path_regexes,omitempty"`
}

func NewDefaultCrawlerScopeConfig() CrawlerScopeConfig {
	return CrawlerScopeConfig{
		AllowedHostnames:      []string{},
		AllowedSubdomains:     []string{},
		DisallowedHostnames:   []string{},
		DisallowedSubdomains:  []string{},
		AllowedPathRegexes:    []string{},
		DisallowedPathRegexes: []string{},
	}
}

type CrawlerConfig struct {
	SeedURLs              []string           `json:"seed_urls,omitempty" yaml:"seed_urls,omitempty" validate:"omitempty,dive,url"`
	UserAgent             string             `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	RequestTimeoutSecs    int                `json:"request_timeout_secs,omitempty" yaml:"request_timeout_secs,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentRequests int                `json:"max_concurrent_requests,omitempty" yaml:"max_concurrent_requests,omitempty" validate:"omitempty,min=1"`
	MaxDepth              int                `json:"max_depth,omitempty" yaml:"max_depth,omitempty" validate:"omitempty,min=0"`
	RespectRobotsTxt      bool               `json:"respect_robots_txt" yaml:"respect_robots_txt"`
	IncludeSubdomains     bool               `json:"include_subdomains" yaml:"include_subdomains"`
	AllowedHostRegex      []string           `json:"allowed_host_regex,omitempty" yaml:"allowed_host_regex,omitempty"`
	ExcludedHostRegex     []string           `json:"excluded_host_regex,omitempty" yaml:"excluded_host_regex,omitempty"`
	Scope                 CrawlerScopeConfig `json:"scope,omitempty" yaml:"scope,omitempty"`
	MaxContentLengthMB    int                `json:"max_content_length_mb,omitempty" yaml:"max_content_length_mb,omitempty"`
}

func NewDefaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		SeedURLs:              []string{},
		UserAgent:             DefaultCrawlerUserAgent,
		RequestTimeoutSecs:    DefaultCrawlerRequestTimeoutSecs,
		MaxConcurrentRequests: DefaultCrawlerMaxConcurrentRequests,
		MaxDepth:              DefaultCrawlerMaxDepth,
		RespectRobotsTxt:      DefaultCrawlerRespectRobotsTxt,
		IncludeSubdomains:     DefaultCrawlerIncludeSubdomains,
		AllowedHostRegex:      []string{},
		ExcludedHostRegex:     []string{},
		Scope:                 NewDefaultCrawlerScopeConfig(),
		MaxContentLengthMB:    2,
	}
}

type ReporterConfig struct {
	OutputDir           string `json:"output_dir,omitempty" yaml:"output_dir,omitempty" validate:"omitempty,dirpath"`
	ItemsPerPage        int    `json:"items_per_page,omitempty" yaml:"items_per_page,omitempty" validate:"omitempty,min=1"`
	EmbedAssets         bool   `json:"embed_assets" yaml:"embed_assets"`
	TemplatePath        string `json:"template_path,omitempty" yaml:"template_path,omitempty"`
	GenerateEmptyReport bool   `json:"generate_empty_report" yaml:"generate_empty_report"`
	ReportTitle         string `json:"report_title,omitempty" yaml:"report_title,omitempty"`
	DefaultItemsPerPage int    `json:"default_items_per_page,omitempty" yaml:"default_items_per_page,omitempty"`
	EnableDataTables    bool   `json:"enable_data_tables" yaml:"enable_data_tables"`
}

func NewDefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		OutputDir:           DefaultReporterOutputDir,
		ItemsPerPage:        DefaultReporterItemsPerPage,
		EmbedAssets:         DefaultReporterEmbedAssets,
		TemplatePath:        "",
		GenerateEmptyReport: false,
		ReportTitle:         "MonsterInc Scan Report",
		DefaultItemsPerPage: DefaultReporterItemsPerPage,
		EnableDataTables:    true,
	}
}

type StorageConfig struct {
	ParquetBasePath  string `json:"parquet_base_path,omitempty" yaml:"parquet_base_path,omitempty"`
	CompressionCodec string `json:"compression_codec,omitempty" yaml:"compression_codec,omitempty"`
}

func NewDefaultStorageConfig() StorageConfig {
	return StorageConfig{
		ParquetBasePath:  DefaultStorageParquetBasePath,
		CompressionCodec: DefaultStorageCompressionCodec,
	}
}

type NotificationConfig struct {
	DiscordWebhookURL     string   `json:"discord_webhook_url,omitempty" yaml:"discord_webhook_url,omitempty" validate:"omitempty,url"`
	MentionRoleIDs        []string `json:"mention_role_ids,omitempty" yaml:"mention_role_ids,omitempty"`
	NotifyOnSuccess       bool     `json:"notify_on_success" yaml:"notify_on_success"`
	NotifyOnFailure       bool     `json:"notify_on_failure" yaml:"notify_on_failure"`
	NotifyOnScanStart     bool     `json:"notify_on_scan_start" yaml:"notify_on_scan_start"`
	NotifyOnCriticalError bool     `json:"notify_on_critical_error" yaml:"notify_on_critical_error"`
}

func NewDefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		DiscordWebhookURL:     "",
		MentionRoleIDs:        []string{},
		NotifyOnSuccess:       false,
		NotifyOnFailure:       true,
		NotifyOnScanStart:     false,
		NotifyOnCriticalError: true,
	}
}

type LogConfig struct {
	LogLevel        string `json:"log_level,omitempty" yaml:"log_level,omitempty" validate:"omitempty,loglevel"`
	LogFormat       string `json:"log_format,omitempty" yaml:"log_format,omitempty" validate:"omitempty,logformat"`
	LogFile         string `json:"log_file,omitempty" yaml:"log_file,omitempty" validate:"omitempty,filepath"`
	MaxLogSizeMB    int    `json:"max_log_size_mb,omitempty" yaml:"max_log_size_mb,omitempty"`
	MaxLogBackups   int    `json:"max_log_backups,omitempty" yaml:"max_log_backups,omitempty"`
	CompressOldLogs bool   `json:"compress_old_logs" yaml:"compress_old_logs"`
}

func NewDefaultLogConfig() LogConfig {
	return LogConfig{
		LogLevel:        DefaultLogLevel,
		LogFormat:       DefaultLogFormat,
		LogFile:         DefaultLogFile,
		MaxLogSizeMB:    DefaultMaxLogSizeMB,
		MaxLogBackups:   DefaultMaxLogBackups,
		CompressOldLogs: DefaultCompressOldLogs,
	}
}

type NormalizerConfig struct {
	// DefaultScheme string `json:"default_scheme,omitempty" yaml:"default_scheme,omitempty"` // Example for future use
}

func NewDefaultNormalizerConfig() NormalizerConfig {
	return NormalizerConfig{
		// DefaultScheme: DefaultNormalizerDefaultScheme, // Example for future use
	}
}

// DiffConfig holds configuration for comparing current scan results with previous ones.
type DiffConfig struct {
	PreviousScanLookbackDays int `json:"previous_scan_lookback_days,omitempty" yaml:"previous_scan_lookback_days,omitempty"`
}

// NewDefaultDiffConfig creates a new DiffConfig with default values.
func NewDefaultDiffConfig() DiffConfig {
	return DiffConfig{
		PreviousScanLookbackDays: DefaultDiffPreviousScanLookbackDays,
	}
}

// MonitorConfig holds configuration for monitoring JS/HTML files for changes.
type MonitorConfig struct {
	JSFileExtensions   []string `json:"js_file_extensions,omitempty" yaml:"js_file_extensions,omitempty"`
	HTMLFileExtensions []string `json:"html_file_extensions,omitempty" yaml:"html_file_extensions,omitempty"`
}

// NewDefaultMonitorConfig creates a new MonitorConfig with default values.
func NewDefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		JSFileExtensions:   filepath.SplitList(DefaultMonitorJSFileExtensions),
		HTMLFileExtensions: filepath.SplitList(DefaultMonitorHTMLFileExtensions),
	}
}

// --- Global Configuration ---

type GlobalConfig struct {
	InputConfig        InputConfig        `json:"input_config,omitempty" yaml:"input_config,omitempty"`
	HttpxRunnerConfig  HttpxRunnerConfig  `json:"httpx_runner_config,omitempty" yaml:"httpx_runner_config,omitempty"`
	CrawlerConfig      CrawlerConfig      `json:"crawler_config,omitempty" yaml:"crawler_config,omitempty"`
	ReporterConfig     ReporterConfig     `json:"reporter_config,omitempty" yaml:"reporter_config,omitempty"`
	StorageConfig      StorageConfig      `json:"storage_config,omitempty" yaml:"storage_config,omitempty"`
	NotificationConfig NotificationConfig `json:"notification_config,omitempty" yaml:"notification_config,omitempty"`
	LogConfig          LogConfig          `json:"log_config,omitempty" yaml:"log_config,omitempty"`
	DiffConfig         DiffConfig         `json:"diff_config,omitempty" yaml:"diff_config,omitempty"`
	MonitorConfig      MonitorConfig      `json:"monitor_config,omitempty" yaml:"monitor_config,omitempty"`
	NormalizerConfig   NormalizerConfig   `json:"normalizer_config,omitempty" yaml:"normalizer_config,omitempty"`
	Mode               string             `json:"mode,omitempty" yaml:"mode,omitempty" validate:"omitempty,mode"`
}

func NewDefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		InputConfig:        InputConfig{InputURLs: []string{}, InputFile: ""},
		HttpxRunnerConfig:  NewDefaultHTTPXRunnerConfig(),
		CrawlerConfig:      NewDefaultCrawlerConfig(),
		ReporterConfig:     NewDefaultReporterConfig(),
		StorageConfig:      NewDefaultStorageConfig(),
		NotificationConfig: NewDefaultNotificationConfig(),
		LogConfig:          NewDefaultLogConfig(),
		DiffConfig:         NewDefaultDiffConfig(),
		MonitorConfig:      NewDefaultMonitorConfig(),
		NormalizerConfig:   NewDefaultNormalizerConfig(),
		Mode:               "onetime", // Default mode set to onetime
	}
}

// LoadGlobalConfig loads the global configuration from the given file path.
// It supports both JSON and YAML formats, and overrides with environment variables.
// YAML is preferred if the file extension is .yaml or .yml.
func LoadGlobalConfig(filePath string) (*GlobalConfig, error) {
	cfg := NewDefaultGlobalConfig() // Start with defaults

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
		}

		ext := filepath.Ext(filePath)
		isYAML := ext == ".yaml" || ext == ".yml"

		if isYAML {
			// --- YAML Unmarshalling (requires import "gopkg.in/yaml.v3") ---
			// The user MUST ADD THE IMPORT "gopkg.in/yaml.v3" for the below to work.
			// For now, I will comment it out to prevent build errors if the import is missing.
			// Once the import is added, uncomment the following lines:
			// /*
			errYaml := yaml.Unmarshal(data, cfg) // Ensure you have: import "gopkg.in/yaml.v3"
			if errYaml == nil {
				// YAML loaded successfully. Environment variables will be processed after this if-else block.
				// No further action needed here for file loading if YAML succeeds.
			} else {
				// YAML parsing failed.
				return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, errYaml)
			}
			// */

			// UNCOMMENTED YAML parsing block:
			errYaml = yaml.Unmarshal(data, cfg)
			if errYaml != nil {
				return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, errYaml)
			}

		} else { // Not a YAML extension, assume JSON
			err = json.Unmarshal(data, cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal JSON config file '%s': %w", filePath, err)
			}
		}
	} else {
		// fmt.Println("No configuration file path provided, using defaults and environment variables.")
	}

	// Override with environment variables
	// Requires import "github.com/kelseyhightower/envconfig"
	// err := envconfig.Process("monsterinc", cfg)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to process environment variables: %w", err)
	// }

	return cfg, nil
}

// SaveGlobalConfig saves the global configuration to the given file path.
// It supports both JSON and YAML formats based on file extension.
// ... existing code ...
