package config

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

const (
	// Reporter Defaults
	DefaultReporterOutputDir    = "reports/scan"
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

	DefaultHTTPXExtractASN = true

	// Crawler Defaults
	DefaultCrawlerUserAgent             = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	DefaultCrawlerRequestTimeoutSecs    = 20
	DefaultCrawlerMaxConcurrentRequests = 10
	DefaultCrawlerMaxDepth              = 5
	DefaultCrawlerRespectRobotsTxt      = true
	DefaultCrawlerIncludeSubdomains     = false

	// Storage Defaults
	DefaultStorageParquetBasePath  = "database"
	DefaultStorageCompressionCodec = "zstd"

	// Log Defaults
	DefaultLogLevel      = "info"
	DefaultLogFormat     = "console"
	DefaultLogFile       = ""
	DefaultMaxLogSizeMB  = 100
	DefaultMaxLogBackups = 3

	// Diff Defaults
	DefaultDiffPreviousScanLookbackDays = 7

	// Monitor Defaults - using fast path file extensions
	DefaultMonitorJSFileExtensions   = ".js,.jsx,.ts,.tsx"
	DefaultMonitorHTMLFileExtensions = ".html,.htm"

	// Normalizer Defaults
	DefaultNormalizerDefaultScheme = "http" // Example for future use

	// Scheduler Defaults
	DefaultSchedulerScanIntervalMinutes = 10080 // 7 days
	DefaultSchedulerRetryAttempts       = 2
	DefaultSchedulerSQLiteDBPath        = "database/scheduler/scheduler_history.db"
)

type GlobalConfig struct {
	CrawlerConfig         CrawlerConfig         `json:"crawler_config,omitempty" yaml:"crawler_config,omitempty"`
	DiffConfig            DiffConfig            `json:"diff_config,omitempty" yaml:"diff_config,omitempty"`
	DiffReporterConfig    DiffReporterConfig    `json:"diff_reporter_config,omitempty" yaml:"diff_reporter_config,omitempty"`
	ExtractorConfig       ExtractorConfig       `json:"extractor_config,omitempty" yaml:"extractor_config,omitempty"`
	HttpxRunnerConfig     HttpxRunnerConfig     `json:"httpx_runner_config,omitempty" yaml:"httpx_runner_config,omitempty"`
	LogConfig             LogConfig             `json:"log_config,omitempty" yaml:"log_config,omitempty"`
	Mode                  string                `json:"mode,omitempty" yaml:"mode,omitempty" validate:"required,mode"`
	MonitorConfig         MonitorConfig         `json:"monitor_config,omitempty" yaml:"monitor_config,omitempty"`
	NotificationConfig    NotificationConfig    `json:"notification_config,omitempty" yaml:"notification_config,omitempty"`
	ReporterConfig        ReporterConfig        `json:"reporter_config,omitempty" yaml:"reporter_config,omitempty"`
	ResourceLimiterConfig ResourceLimiterConfig `json:"resource_limiter_config,omitempty" yaml:"resource_limiter_config,omitempty"`
	SchedulerConfig       SchedulerConfig       `json:"scheduler_config,omitempty" yaml:"scheduler_config,omitempty"`
	StorageConfig         StorageConfig         `json:"storage_config,omitempty" yaml:"storage_config,omitempty"`
	ScanBatchConfig       ScanBatchConfig       `json:"scan_batch_config,omitempty" yaml:"scan_batch_config,omitempty"`
	MonitorBatchConfig    MonitorBatchConfig    `json:"monitor_batch_config,omitempty" yaml:"monitor_batch_config,omitempty"`
}

func NewDefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		CrawlerConfig:         NewDefaultCrawlerConfig(),
		DiffConfig:            NewDefaultDiffConfig(),
		DiffReporterConfig:    NewDefaultDiffReporterConfig(),
		ExtractorConfig:       NewDefaultExtractorConfig(),
		HttpxRunnerConfig:     NewDefaultHTTPXRunnerConfig(),
		LogConfig:             NewDefaultLogConfig(),
		Mode:                  "onetime",
		MonitorConfig:         NewDefaultMonitorConfig(),
		NotificationConfig:    NewDefaultNotificationConfig(),
		ReporterConfig:        NewDefaultReporterConfig(),
		ResourceLimiterConfig: NewDefaultResourceLimiterConfig(),
		SchedulerConfig:       NewDefaultSchedulerConfig(),
		StorageConfig:         NewDefaultStorageConfig(),
		ScanBatchConfig:       NewDefaultScanBatchConfig(),
		MonitorBatchConfig:    NewDefaultMonitorBatchConfig(),
	}
}

// LoadGlobalConfig loads the configuration from a file or default locations.
// It determines the config file path using GetConfigPath, supports both JSON and YAML formats.
// YAML is preferred if the file extension is .yaml or .yml.
func LoadGlobalConfig(providedPath string, logger zerolog.Logger) (*GlobalConfig, error) {
	cfg := NewDefaultGlobalConfig()

	filePath := GetConfigPath(providedPath)
	if filePath == "" {

		return cfg, nil
	}

	fileManager := common.NewFileManager(logger)
	if !fileManager.FileExists(filePath) {
		return nil, common.NewValidationError("config_file", filePath, "config file does not exist")
	}

	data, err := loadConfigFileContent(fileManager, filePath)
	if err != nil {
		return nil, common.WrapError(err, "failed to load config file content")
	}

	if err := parseConfigContent(data, filePath, cfg); err != nil {
		return nil, common.WrapError(err, "failed to parse config content")
	}

	return cfg, nil
}

// loadConfigFileContent reads the config file using FileManager
func loadConfigFileContent(fileManager *common.FileManager, filePath string) ([]byte, error) {
	opts := common.DefaultFileReadOptions()
	opts.MaxSize = 10 * 1024 * 1024 // 10MB max config file size

	return fileManager.ReadFile(filePath, opts)
}

// parseConfigContent parses the config content based on file extension
func parseConfigContent(data []byte, filePath string, cfg *GlobalConfig) error {
	ext := filepath.Ext(filePath)
	if isYAMLFile(ext) {
		return parseYAMLConfig(data, filePath, cfg)
	}
	return parseJSONConfig(data, filePath, cfg)
}

// isYAMLFile checks if the file extension indicates a YAML file
func isYAMLFile(ext string) bool {
	return ext == ".yaml" || ext == ".yml"
}

// parseYAMLConfig parses YAML configuration
func parseYAMLConfig(data []byte, filePath string, cfg *GlobalConfig) error {
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return common.NewError("failed to unmarshal YAML from '%s': %w", filePath, err)
	}
	return nil
}

// parseJSONConfig parses JSON configuration
func parseJSONConfig(data []byte, filePath string, cfg *GlobalConfig) error {
	if err := json.Unmarshal(data, cfg); err != nil {
		return common.NewError("failed to unmarshal JSON from '%s': %w", filePath, err)
	}
	return nil
}

type HttpxRunnerConfig struct {
	CustomHeaders        map[string]string `json:"custom_headers,omitempty" yaml:"custom_headers,omitempty"`
	ExtractASN           bool              `json:"extract_asn" yaml:"extract_asn"`
	ExtractBody          bool              `json:"extract_body" yaml:"extract_body"`
	ExtractContentLength bool              `json:"extract_content_length" yaml:"extract_content_length"`
	ExtractContentType   bool              `json:"extract_content_type" yaml:"extract_content_type"`
	ExtractHeaders       bool              `json:"extract_headers" yaml:"extract_headers"`
	ExtractIPs           bool              `json:"extract_ips" yaml:"extract_ips"`
	ExtractLocation      bool              `json:"extract_location" yaml:"extract_location"`
	ExtractServerHeader  bool              `json:"extract_server_header" yaml:"extract_server_header"`
	ExtractStatusCode    bool              `json:"extract_status_code" yaml:"extract_status_code"`
	ExtractTitle         bool              `json:"extract_title" yaml:"extract_title"`
	FollowRedirects      bool              `json:"follow_redirects" yaml:"follow_redirects"`
	MaxRedirects         int               `json:"max_redirects,omitempty" yaml:"max_redirects,omitempty" validate:"omitempty,min=0"`
	Method               string            `json:"method,omitempty" yaml:"method,omitempty"`
	RateLimit            int               `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty" validate:"omitempty,min=0"`
	RequestURIs          []string          `json:"request_uris,omitempty" yaml:"request_uris,omitempty" validate:"omitempty,dive,url"`
	Retries              int               `json:"retries,omitempty" yaml:"retries,omitempty" validate:"omitempty,min=0"`
	TechDetect           bool              `json:"tech_detect" yaml:"tech_detect"`
	Threads              int               `json:"threads,omitempty" yaml:"threads,omitempty" validate:"omitempty,min=1"`
	TimeoutSecs          int               `json:"timeout_secs,omitempty" yaml:"timeout_secs,omitempty" validate:"omitempty,min=1"`
	Verbose              bool              `json:"verbose" yaml:"verbose"`
}

func NewDefaultHTTPXRunnerConfig() HttpxRunnerConfig {
	return HttpxRunnerConfig{
		CustomHeaders:        make(map[string]string),
		ExtractASN:           DefaultHTTPXExtractASN,
		ExtractBody:          DefaultHTTPXExtractBody,
		ExtractContentLength: DefaultHTTPXExtractContentLength,
		ExtractContentType:   DefaultHTTPXExtractContentType,
		ExtractHeaders:       DefaultHTTPXExtractHeaders,
		ExtractIPs:           DefaultHTTPXExtractIPs,
		ExtractLocation:      DefaultHTTPXExtractLocation,
		ExtractServerHeader:  DefaultHTTPXExtractServerHeader,
		ExtractStatusCode:    DefaultHTTPXExtractStatusCode,
		ExtractTitle:         DefaultHTTPXExtractTitle,
		FollowRedirects:      DefaultHTTPXFollowRedirects,
		MaxRedirects:         DefaultHTTPXMaxRedirects,
		Method:               DefaultHTTPXMethod,
		RateLimit:            DefaultHTTPXRateLimit,
		RequestURIs:          []string{},
		Retries:              DefaultHTTPXRetries,
		TechDetect:           DefaultHTTPXTechDetect,
		Threads:              DefaultHTTPXThreads,
		TimeoutSecs:          DefaultHTTPXTimeoutSecs,
		Verbose:              DefaultHTTPXVerbose,
	}
}

type CrawlerScopeConfig struct {
	DisallowedHostnames      []string `json:"disallowed_hostnames,omitempty" yaml:"disallowed_hostnames,omitempty"`
	DisallowedSubdomains     []string `json:"disallowed_subdomains,omitempty" yaml:"disallowed_subdomains,omitempty"`
	DisallowedFileExtensions []string `json:"disallowed_file_extensions,omitempty" yaml:"disallowed_file_extensions,omitempty"`
}

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

func NewDefaultCrawlerScopeConfig() CrawlerScopeConfig {
	return CrawlerScopeConfig{
		DisallowedHostnames:      []string{},
		DisallowedSubdomains:     []string{},
		DisallowedFileExtensions: []string{".js", ".txt", ".css", ".xml"},
	}
}

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

type CrawlerConfig struct {
	AutoAddSeedHostnames     bool                  `json:"auto_add_seed_hostnames" yaml:"auto_add_seed_hostnames"`
	EnableContentLengthCheck bool                  `json:"enable_content_length_check" yaml:"enable_content_length_check"`
	IncludeSubdomains        bool                  `json:"include_subdomains" yaml:"include_subdomains"`
	InsecureSkipTLSVerify    bool                  `json:"insecure_skip_tls_verify" yaml:"insecure_skip_tls_verify"`
	MaxConcurrentRequests    int                   `json:"max_concurrent_requests,omitempty" yaml:"max_concurrent_requests,omitempty" validate:"omitempty,min=1"`
	MaxContentLengthMB       int                   `json:"max_content_length_mb,omitempty" yaml:"max_content_length_mb,omitempty"`
	MaxDepth                 int                   `json:"max_depth,omitempty" yaml:"max_depth,omitempty" validate:"omitempty,min=0"`
	RequestTimeoutSecs       int                   `json:"request_timeout_secs,omitempty" yaml:"request_timeout_secs,omitempty" validate:"omitempty,min=1"`
	RespectRobotsTxt         bool                  `json:"respect_robots_txt" yaml:"respect_robots_txt"`
	Scope                    CrawlerScopeConfig    `json:"scope,omitempty" yaml:"scope,omitempty"`
	SeedURLs                 []string              `json:"seed_urls,omitempty" yaml:"seed_urls,omitempty" validate:"omitempty,dive,url"`
	UserAgent                string                `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	HeadlessBrowser          HeadlessBrowserConfig `json:"headless_browser,omitempty" yaml:"headless_browser,omitempty"`
	AutoCalibrate            AutoCalibrateConfig   `json:"auto_calibrate,omitempty" yaml:"auto_calibrate,omitempty"`
	// URL normalization configuration
	URLNormalization urlhandler.URLNormalizationConfig `json:"url_normalization,omitempty" yaml:"url_normalization,omitempty"`
	// Retry configuration for handling rate limits (429 errors)
	RetryConfig RetryConfig `json:"retry_config,omitempty" yaml:"retry_config,omitempty"`
}

func NewDefaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		AutoAddSeedHostnames:     true,
		EnableContentLengthCheck: false,
		IncludeSubdomains:        DefaultCrawlerIncludeSubdomains,
		InsecureSkipTLSVerify:    true,
		MaxConcurrentRequests:    DefaultCrawlerMaxConcurrentRequests,
		MaxContentLengthMB:       2,
		MaxDepth:                 DefaultCrawlerMaxDepth,
		RequestTimeoutSecs:       DefaultCrawlerRequestTimeoutSecs,
		RespectRobotsTxt:         DefaultCrawlerRespectRobotsTxt,
		Scope:                    NewDefaultCrawlerScopeConfig(),
		SeedURLs:                 []string{},
		UserAgent:                DefaultCrawlerUserAgent,
		HeadlessBrowser:          NewDefaultHeadlessBrowserConfig(),
		AutoCalibrate:            NewDefaultAutoCalibrateConfig(),
		URLNormalization:         urlhandler.DefaultURLNormalizationConfig(),
		RetryConfig:              NewDefaultRetryConfig(),
	}
}

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

type ReporterConfig struct {
	DefaultItemsPerPage          int    `json:"default_items_per_page,omitempty" yaml:"default_items_per_page,omitempty"`
	EmbedAssets                  bool   `json:"embed_assets" yaml:"embed_assets"`
	EnableDataTables             bool   `json:"enable_data_tables" yaml:"enable_data_tables"`
	GenerateEmptyReport          bool   `json:"generate_empty_report" yaml:"generate_empty_report"`
	ItemsPerPage                 int    `json:"items_per_page,omitempty" yaml:"items_per_page,omitempty" validate:"omitempty,min=1"`
	MaxProbeResultsPerReportFile int    `mapstructure:"max_probe_results_per_report_file" json:"max_probe_results_per_report_file,omitempty" yaml:"max_probe_results_per_report_file,omitempty"`
	OutputDir                    string `json:"output_dir,omitempty" yaml:"output_dir,omitempty" validate:"omitempty,dirpath"`
	ReportTitle                  string `json:"report_title,omitempty" yaml:"report_title,omitempty"`
	TemplatePath                 string `json:"template_path,omitempty" yaml:"template_path,omitempty"`
}

func NewDefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		EmbedAssets:                  DefaultReporterEmbedAssets,
		EnableDataTables:             true,
		GenerateEmptyReport:          false,
		ItemsPerPage:                 DefaultReporterItemsPerPage,
		MaxProbeResultsPerReportFile: 1000, // Default to 1000 results per file
		OutputDir:                    DefaultReporterOutputDir,
		ReportTitle:                  "MonsterInc Scan Report",
		TemplatePath:                 "",
	}
}

type StorageConfig struct {
	CompressionCodec string `json:"compression_codec,omitempty" yaml:"compression_codec,omitempty"`
	ParquetBasePath  string `json:"parquet_base_path,omitempty" yaml:"parquet_base_path,omitempty"`
}

func NewDefaultStorageConfig() StorageConfig {
	return StorageConfig{
		CompressionCodec: DefaultStorageCompressionCodec,
		ParquetBasePath:  DefaultStorageParquetBasePath,
	}
}

type NotificationConfig struct {
	AutoDeletePartialDiffReports    bool     `json:"auto_delete_partial_diff_reports" yaml:"auto_delete_partial_diff_reports"`
	MentionRoleIDs                  []string `json:"mention_role_ids,omitempty" yaml:"mention_role_ids,omitempty"`
	MonitorServiceDiscordWebhookURL string   `json:"monitor_service_discord_webhook_url,omitempty" yaml:"monitor_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
	NotifyOnCriticalError           bool     `json:"notify_on_critical_error" yaml:"notify_on_critical_error"`
	NotifyOnFailure                 bool     `json:"notify_on_failure" yaml:"notify_on_failure"`
	NotifyOnScanStart               bool     `json:"notify_on_scan_start" yaml:"notify_on_scan_start"`
	NotifyOnSuccess                 bool     `json:"notify_on_success" yaml:"notify_on_success"`
	ScanServiceDiscordWebhookURL    string   `json:"scan_service_discord_webhook_url,omitempty" yaml:"scan_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
}

func NewDefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		AutoDeletePartialDiffReports:    false,
		MentionRoleIDs:                  []string{},
		MonitorServiceDiscordWebhookURL: "",
		NotifyOnCriticalError:           true,
		NotifyOnFailure:                 true,
		NotifyOnScanStart:               false,
		NotifyOnSuccess:                 false,
		ScanServiceDiscordWebhookURL:    "",
	}
}

type LogConfig struct {
	LogFile       string `json:"log_file,omitempty" yaml:"log_file,omitempty" validate:"omitempty,filepath"`
	LogFormat     string `json:"log_format,omitempty" yaml:"log_format,omitempty" validate:"omitempty,logformat"`
	LogLevel      string `json:"log_level,omitempty" yaml:"log_level,omitempty" validate:"omitempty,loglevel"`
	MaxLogBackups int    `json:"max_log_backups,omitempty" yaml:"max_log_backups,omitempty"`
	MaxLogSizeMB  int    `json:"max_log_size_mb,omitempty" yaml:"max_log_size_mb,omitempty"`
}

func NewDefaultLogConfig() LogConfig {
	return LogConfig{
		LogFile:       "",        // Default to stderr, not a file
		LogFormat:     "console", // Default log format
		LogLevel:      "info",    // Default log level
		MaxLogBackups: 3,         // Example default if implementing rotation
		MaxLogSizeMB:  100,       // Example default if implementing rotation
	}
}

type DiffConfig struct {
	PreviousScanLookbackDays int `json:"previous_scan_lookback_days,omitempty" yaml:"previous_scan_lookback_days,omitempty"`
}

func NewDefaultDiffConfig() DiffConfig {
	return DiffConfig{
		PreviousScanLookbackDays: DefaultDiffPreviousScanLookbackDays,
	}
}

type MonitorConfig struct {
	AggregationIntervalSeconds  int      `json:"aggregation_interval_seconds,omitempty" yaml:"aggregation_interval_seconds,omitempty" validate:"omitempty,min=1"`
	CheckIntervalSeconds        int      `json:"check_interval_seconds,omitempty" yaml:"check_interval_seconds,omitempty" validate:"omitempty,min=1"`
	Enabled                     bool     `json:"enabled" yaml:"enabled"`
	HTMLFileExtensions          []string `json:"html_file_extensions,omitempty" yaml:"html_file_extensions,omitempty"`
	HTTPTimeoutSeconds          int      `json:"http_timeout_seconds,omitempty" yaml:"http_timeout_seconds,omitempty" validate:"omitempty,min=1"`
	InitialMonitorURLs          []string `json:"initial_monitor_urls,omitempty" yaml:"initial_monitor_urls,omitempty" validate:"omitempty,dive,url"`
	JSFileExtensions            []string `json:"js_file_extensions,omitempty" yaml:"js_file_extensions,omitempty"`
	MaxAggregatedEvents         int      `json:"max_aggregated_events,omitempty" yaml:"max_aggregated_events,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentChecks         int      `json:"max_concurrent_checks,omitempty" yaml:"max_concurrent_checks,omitempty" validate:"omitempty,min=1"`
	MaxContentSize              int      `json:"max_content_size,omitempty" yaml:"max_content_size,omitempty" validate:"omitempty,min=1"` // Max content size in bytes
	MonitorInsecureSkipVerify   bool     `json:"monitor_insecure_skip_verify" yaml:"monitor_insecure_skip_verify"`
	StoreFullContentOnChange    bool     `json:"store_full_content_on_change" yaml:"store_full_content_on_change"`
	MaxDiffResultsPerReportFile int      `json:"max_diff_results_per_report_file,omitempty" yaml:"max_diff_results_per_report_file,omitempty" validate:"omitempty,min=1"` // Maximum number of diff results per HTML report file
}

func NewDefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		AggregationIntervalSeconds:  600,  // Default to 10 minutes for aggregation
		CheckIntervalSeconds:        3600, // 1 hour
		Enabled:                     false,
		HTMLFileExtensions:          []string{".html", ".htm"},
		HTTPTimeoutSeconds:          30,
		InitialMonitorURLs:          []string{},
		JSFileExtensions:            []string{".js", ".jsx", ".ts", ".tsx"},
		MaxAggregatedEvents:         10, // Default to 10 events before sending aggregated notification
		MaxConcurrentChecks:         5,
		MaxContentSize:              1048576, // Default 1MB
		MonitorInsecureSkipVerify:   true,    // Default to true to match previous hardcoded behavior
		StoreFullContentOnChange:    true,
		MaxDiffResultsPerReportFile: 500, // Default to 500 diff results per report file
	}
}

type SchedulerConfig struct {
	CycleMinutes  int    `json:"cycle_minutes,omitempty" yaml:"cycle_minutes,omitempty" validate:"min=1"` // in minutes
	RetryAttempts int    `json:"retry_attempts,omitempty" yaml:"retry_attempts,omitempty" validate:"min=0"`
	SQLiteDBPath  string `json:"sqlite_db_path,omitempty" yaml:"sqlite_db_path,omitempty" validate:"required"`
}

func NewDefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		CycleMinutes:  DefaultSchedulerScanIntervalMinutes,
		RetryAttempts: DefaultSchedulerRetryAttempts,
		SQLiteDBPath:  DefaultSchedulerSQLiteDBPath,
	}
}

type ExtractorConfig struct {
	Allowlist     []string `json:"allowlist,omitempty" yaml:"allowlist,omitempty"`
	CustomRegexes []string `json:"custom_regexes,omitempty" yaml:"custom_regexes,omitempty"`
	Denylist      []string `json:"denylist,omitempty" yaml:"denylist,omitempty"`
}

func NewDefaultExtractorConfig() ExtractorConfig {
	return ExtractorConfig{
		CustomRegexes: []string{},
		Allowlist:     []string{},
		Denylist:      []string{},
	}
}

type DiffReporterConfig struct {
	MaxDiffFileSizeMB int `json:"max_diff_file_size_mb,omitempty" yaml:"max_diff_file_size_mb,omitempty"`
}

func NewDefaultDiffReporterConfig() DiffReporterConfig {
	return DiffReporterConfig{
		MaxDiffFileSizeMB: 10,
	}
}

// ResourceLimiterConfig holds configuration for resource monitoring
type ResourceLimiterConfig struct {
	MaxMemoryMB        int64   `json:"max_memory_mb,omitempty" yaml:"max_memory_mb,omitempty" validate:"omitempty,min=100"`
	MaxGoroutines      int     `json:"max_goroutines,omitempty" yaml:"max_goroutines,omitempty" validate:"omitempty,min=100"`
	CheckIntervalSecs  int     `json:"check_interval_secs,omitempty" yaml:"check_interval_secs,omitempty" validate:"omitempty,min=1"`
	MemoryThreshold    float64 `json:"memory_threshold,omitempty" yaml:"memory_threshold,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	GoroutineWarning   float64 `json:"goroutine_warning,omitempty" yaml:"goroutine_warning,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	SystemMemThreshold float64 `json:"system_mem_threshold,omitempty" yaml:"system_mem_threshold,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	CPUThreshold       float64 `json:"cpu_threshold,omitempty" yaml:"cpu_threshold,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	EnableAutoShutdown bool    `json:"enable_auto_shutdown" yaml:"enable_auto_shutdown"`
}

func NewDefaultResourceLimiterConfig() ResourceLimiterConfig {
	return ResourceLimiterConfig{
		MaxMemoryMB:        512,  // Giảm từ 1024 xuống 512MB để trigger sớm hơn
		MaxGoroutines:      5000, // Giảm từ 10000 xuống 5000
		CheckIntervalSecs:  15,   // Giảm từ 30 xuống 15 seconds để check thường xuyên hơn
		MemoryThreshold:    0.7,  // Giảm từ 0.8 xuống 0.7 (70%)
		GoroutineWarning:   0.6,  // Giảm từ 0.7 xuống 0.6 (60%)
		SystemMemThreshold: 0.4,  // Giảm từ 0.5 xuống 0.4 (40% system memory)
		CPUThreshold:       0.4,  // Giảm từ 0.5 xuống 0.4 (40% CPU usage)
		EnableAutoShutdown: true, // Enable auto-shutdown by default
	}
}

type ScanBatchConfig struct {
	BatchSize          int `json:"batch_size,omitempty" yaml:"batch_size,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentBatch int `json:"max_concurrent_batch,omitempty" yaml:"max_concurrent_batch,omitempty" validate:"omitempty,min=1"`
	BatchTimeoutMins   int `json:"batch_timeout_mins,omitempty" yaml:"batch_timeout_mins,omitempty" validate:"omitempty,min=1"`
	ThresholdSize      int `json:"threshold_size,omitempty" yaml:"threshold_size,omitempty" validate:"omitempty,min=1"`
}

func NewDefaultScanBatchConfig() ScanBatchConfig {
	return ScanBatchConfig{
		BatchSize:          200,  // Larger batch size for scan service
		MaxConcurrentBatch: 2,    // Higher concurrency for scan service
		BatchTimeoutMins:   45,   // Longer timeout for scan service
		ThresholdSize:      1000, // Higher threshold for scan service
	}
}

type MonitorBatchConfig struct {
	BatchSize          int `json:"batch_size,omitempty" yaml:"batch_size,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentBatch int `json:"max_concurrent_batch,omitempty" yaml:"max_concurrent_batch,omitempty" validate:"omitempty,min=1"`
	BatchTimeoutMins   int `json:"batch_timeout_mins,omitempty" yaml:"batch_timeout_mins,omitempty" validate:"omitempty,min=1"`
	ThresholdSize      int `json:"threshold_size,omitempty" yaml:"threshold_size,omitempty" validate:"omitempty,min=1"`
}

func NewDefaultMonitorBatchConfig() MonitorBatchConfig {
	return MonitorBatchConfig{
		BatchSize:          50,  // Smaller batch size for monitor service
		MaxConcurrentBatch: 1,   // Sequential processing for monitor service
		BatchTimeoutMins:   20,  // Shorter timeout for monitor service
		ThresholdSize:      200, // Lower threshold for monitor service
	}
}

// SaveGlobalConfig saves the configuration to a file in the configs directory
// Supports both JSON and YAML formats based on file extension
func SaveGlobalConfig(cfg *GlobalConfig, fileName string, logger zerolog.Logger) error {
	if cfg == nil {
		return common.NewValidationError("config", cfg, "config cannot be nil")
	}

	if fileName == "" {
		fileName = "config.yaml" // Default to YAML
	}

	// Ensure configs directory exists
	configsDir := "configs"
	fileManager := common.NewFileManager(logger)
	if err := fileManager.EnsureDirectory(configsDir, 0755); err != nil {
		return common.WrapError(err, "failed to create configs directory")
	}

	filePath := filepath.Join(configsDir, fileName)
	return saveConfigToFile(cfg, filePath, fileManager, logger)
}

// saveConfigToFile saves the config to a specific file path
func saveConfigToFile(cfg *GlobalConfig, filePath string, fileManager *common.FileManager, logger zerolog.Logger) error {
	var data []byte
	var err error

	ext := filepath.Ext(filePath)
	if isYAMLFile(ext) {
		data, err = yaml.Marshal(cfg)
		if err != nil {
			return common.NewError("failed to marshal config to YAML: %w", err)
		}
	} else {
		data, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return common.NewError("failed to marshal config to JSON: %w", err)
		}
	}

	opts := common.DefaultFileWriteOptions()
	if err := fileManager.WriteFile(filePath, data, opts); err != nil {
		return common.WrapError(err, "failed to write config file")
	}

	logger.Info().
		Str("path", filePath).
		Str("format", ext).
		Msg("Successfully saved config file")

	return nil
}

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

// BatchConfig interface for converting to common.BatchProcessorConfig
type BatchConfig interface {
	ToBatchProcessorConfig() common.BatchProcessorConfig
}

// ToBatchProcessorConfig converts ScanBatchConfig to common.BatchProcessorConfig
func (sbc ScanBatchConfig) ToBatchProcessorConfig() common.BatchProcessorConfig {
	return common.BatchProcessorConfig{
		BatchSize:          sbc.BatchSize,
		MaxConcurrentBatch: sbc.MaxConcurrentBatch,
		BatchTimeout:       time.Duration(sbc.BatchTimeoutMins) * time.Minute,
		ThresholdSize:      sbc.ThresholdSize,
	}
}

// ToBatchProcessorConfig converts MonitorBatchConfig to common.BatchProcessorConfig
func (mbc MonitorBatchConfig) ToBatchProcessorConfig() common.BatchProcessorConfig {
	return common.BatchProcessorConfig{
		BatchSize:          mbc.BatchSize,
		MaxConcurrentBatch: mbc.MaxConcurrentBatch,
		BatchTimeout:       time.Duration(mbc.BatchTimeoutMins) * time.Minute,
		ThresholdSize:      mbc.ThresholdSize,
	}
}
