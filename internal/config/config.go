package config

import (
	"encoding/json"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/common"
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
	DefaultLogLevel      = "info"
	DefaultLogFormat     = "console"
	DefaultLogFile       = ""
	DefaultMaxLogSizeMB  = 100
	DefaultMaxLogBackups = 3

	// Diff Defaults
	DefaultDiffPreviousScanLookbackDays = 7

	// Monitor Defaults
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
	CrawlerConfig        CrawlerConfig      `json:"crawler_config,omitempty" yaml:"crawler_config,omitempty"`
	DiffConfig           DiffConfig         `json:"diff_config,omitempty" yaml:"diff_config,omitempty"`
	DiffReporterConfig   DiffReporterConfig `json:"diff_reporter_config,omitempty" yaml:"diff_reporter_config,omitempty"`
	ExtractorConfig      ExtractorConfig    `json:"extractor_config,omitempty" yaml:"extractor_config,omitempty"`
	HttpxRunnerConfig    HttpxRunnerConfig  `json:"httpx_runner_config,omitempty" yaml:"httpx_runner_config,omitempty"`
	InputConfig          InputConfig        `json:"input_config,omitempty" yaml:"input_config,omitempty"`
	LogConfig            LogConfig          `json:"log_config,omitempty" yaml:"log_config,omitempty"`
	Mode                 string             `json:"mode,omitempty" yaml:"mode,omitempty" validate:"required,mode"`
	MonitorConfig        MonitorConfig      `json:"monitor_config,omitempty" yaml:"monitor_config,omitempty"`
	NotificationConfig   NotificationConfig `json:"notification_config,omitempty" yaml:"notification_config,omitempty"`
	PathExtractorDomains []string           `json:"path_extractor_domains,omitempty" yaml:"path_extractor_domains,omitempty" validate:"omitempty,dive,hostname_rfc1123"`
	ReporterConfig       ReporterConfig     `json:"reporter_config,omitempty" yaml:"reporter_config,omitempty"`
	SchedulerConfig      SchedulerConfig    `json:"scheduler_config,omitempty" yaml:"scheduler_config,omitempty"`
	StorageConfig        StorageConfig      `json:"storage_config,omitempty" yaml:"storage_config,omitempty"`
}

func NewDefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		CrawlerConfig:        NewDefaultCrawlerConfig(),
		DiffConfig:           NewDefaultDiffConfig(),
		DiffReporterConfig:   NewDefaultDiffReporterConfig(),
		ExtractorConfig:      NewDefaultExtractorConfig(),
		HttpxRunnerConfig:    NewDefaultHTTPXRunnerConfig(),
		InputConfig:          InputConfig{InputURLs: []string{}, InputFile: ""},
		LogConfig:            NewDefaultLogConfig(),
		Mode:                 "",
		MonitorConfig:        NewDefaultMonitorConfig(),
		NotificationConfig:   NewDefaultNotificationConfig(),
		PathExtractorDomains: []string{},
		ReporterConfig:       NewDefaultReporterConfig(),
		SchedulerConfig:      NewDefaultSchedulerConfig(),
		StorageConfig:        NewDefaultStorageConfig(),
	}
}

// LoadGlobalConfig loads the configuration from a file or default locations.
// It determines the config file path using GetConfigPath, supports both JSON and YAML formats.
// YAML is preferred if the file extension is .yaml or .yml.
func LoadGlobalConfig(providedPath string, logger zerolog.Logger) (*GlobalConfig, error) {
	cfg := NewDefaultGlobalConfig()

	filePath := GetConfigPath(providedPath)
	if filePath == "" {
		logger.Debug().Msg("No config file found, using defaults")
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

type InputConfig struct {
	InputURLs []string `json:"input_urls,omitempty" yaml:"input_urls,omitempty" validate:"omitempty,dive,url"`
	InputFile string   `json:"input_file,omitempty" yaml:"input_file,omitempty" validate:"omitempty,fileexists"`
}

type HttpxRunnerConfig struct {
	CustomHeaders        map[string]string `json:"custom_headers,omitempty" yaml:"custom_headers,omitempty"`
	DenyInternalIPs      bool              `json:"deny_internal_ips" yaml:"deny_internal_ips"`
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
	HttpxFlags           []string          `json:"httpx_flags,omitempty" yaml:"httpx_flags,omitempty"`
	MaxRedirects         int               `json:"max_redirects,omitempty" yaml:"max_redirects,omitempty" validate:"omitempty,min=0"`
	Method               string            `json:"method,omitempty" yaml:"method,omitempty"`
	Ports                []string          `json:"ports,omitempty" yaml:"ports,omitempty"`
	RateLimit            int               `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty" validate:"omitempty,min=0"`
	RequestURIs          []string          `json:"request_uris,omitempty" yaml:"request_uris,omitempty" validate:"omitempty,dive,url"`
	Resolvers            []string          `json:"resolvers,omitempty" yaml:"resolvers,omitempty"`
	Retries              int               `json:"retries,omitempty" yaml:"retries,omitempty" validate:"omitempty,min=0"`
	SkipDefaultPorts     bool              `json:"skip_default_ports" yaml:"skip_default_ports"`
	TechDetect           bool              `json:"tech_detect" yaml:"tech_detect"`
	Threads              int               `json:"threads,omitempty" yaml:"threads,omitempty" validate:"omitempty,min=1"`
	TimeoutSecs          int               `json:"timeout_secs,omitempty" yaml:"timeout_secs,omitempty" validate:"omitempty,min=1"`
	Verbose              bool              `json:"verbose" yaml:"verbose"`
}

func NewDefaultHTTPXRunnerConfig() HttpxRunnerConfig {
	return HttpxRunnerConfig{
		CustomHeaders:        make(map[string]string),
		DenyInternalIPs:      DefaultHTTPXDenyInternalIPs,
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
		HttpxFlags:           []string{},
		MaxRedirects:         DefaultHTTPXMaxRedirects,
		Method:               DefaultHTTPXMethod,
		Ports:                []string{},
		RateLimit:            DefaultHTTPXRateLimit,
		RequestURIs:          []string{},
		Resolvers:            []string{},
		Retries:              DefaultHTTPXRetries,
		SkipDefaultPorts:     DefaultHTTPXSkipDefaultPorts,
		TechDetect:           DefaultHTTPXTechDetect,
		Threads:              DefaultHTTPXThreads,
		TimeoutSecs:          DefaultHTTPXTimeoutSecs,
		Verbose:              DefaultHTTPXVerbose,
	}
}

type CrawlerScopeConfig struct {
	AllowedHostnames      []string `json:"allowed_hostnames,omitempty" yaml:"allowed_hostnames,omitempty"`
	AllowedPathRegexes    []string `json:"allowed_path_regexes,omitempty" yaml:"allowed_path_regexes,omitempty"`
	AllowedSubdomains     []string `json:"allowed_subdomains,omitempty" yaml:"allowed_subdomains,omitempty"`
	DisallowedHostnames   []string `json:"disallowed_hostnames,omitempty" yaml:"disallowed_hostnames,omitempty"`
	DisallowedPathRegexes []string `json:"disallowed_path_regexes,omitempty" yaml:"disallowed_path_regexes,omitempty"`
	DisallowedSubdomains  []string `json:"disallowed_subdomains,omitempty" yaml:"disallowed_subdomains,omitempty"`
}

func NewDefaultCrawlerScopeConfig() CrawlerScopeConfig {
	return CrawlerScopeConfig{
		AllowedHostnames:      []string{},
		AllowedPathRegexes:    []string{},
		AllowedSubdomains:     []string{},
		DisallowedHostnames:   []string{},
		DisallowedPathRegexes: []string{},
		DisallowedSubdomains:  []string{},
	}
}

type CrawlerConfig struct {
	AllowedHostRegex      []string           `json:"allowed_host_regex,omitempty" yaml:"allowed_host_regex,omitempty"`
	AutoAddSeedHostnames  bool               `json:"auto_add_seed_hostnames" yaml:"auto_add_seed_hostnames"`
	ExcludedHostRegex     []string           `json:"excluded_host_regex,omitempty" yaml:"excluded_host_regex,omitempty"`
	IncludeSubdomains     bool               `json:"include_subdomains" yaml:"include_subdomains"`
	MaxConcurrentRequests int                `json:"max_concurrent_requests,omitempty" yaml:"max_concurrent_requests,omitempty" validate:"omitempty,min=1"`
	MaxContentLengthMB    int                `json:"max_content_length_mb,omitempty" yaml:"max_content_length_mb,omitempty"`
	MaxDepth              int                `json:"max_depth,omitempty" yaml:"max_depth,omitempty" validate:"omitempty,min=0"`
	RequestTimeoutSecs    int                `json:"request_timeout_secs,omitempty" yaml:"request_timeout_secs,omitempty" validate:"omitempty,min=1"`
	RespectRobotsTxt      bool               `json:"respect_robots_txt" yaml:"respect_robots_txt"`
	Scope                 CrawlerScopeConfig `json:"scope,omitempty" yaml:"scope,omitempty"`
	SeedURLs              []string           `json:"seed_urls,omitempty" yaml:"seed_urls,omitempty" validate:"omitempty,dive,url"`
	UserAgent             string             `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
}

func NewDefaultCrawlerConfig() CrawlerConfig {
	return CrawlerConfig{
		AutoAddSeedHostnames:  false,
		IncludeSubdomains:     DefaultCrawlerIncludeSubdomains,
		MaxConcurrentRequests: DefaultCrawlerMaxConcurrentRequests,
		MaxContentLengthMB:    2,
		MaxDepth:              DefaultCrawlerMaxDepth,
		RequestTimeoutSecs:    DefaultCrawlerRequestTimeoutSecs,
		RespectRobotsTxt:      DefaultCrawlerRespectRobotsTxt,
		Scope:                 NewDefaultCrawlerScopeConfig(),
		SeedURLs:              []string{},
		UserAgent:             DefaultCrawlerUserAgent,
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
	AutoDeleteSingleDiffReportsAfterDiscordNotification bool     `json:"auto_delete_single_diff_reports_after_discord_notification" yaml:"auto_delete_single_diff_reports_after_discord_notification"`
	MentionRoleIDs                                      []string `json:"mention_role_ids,omitempty" yaml:"mention_role_ids,omitempty"`
	MonitorServiceDiscordWebhookURL                     string   `json:"monitor_service_discord_webhook_url,omitempty" yaml:"monitor_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
	NotifyOnCriticalError                               bool     `json:"notify_on_critical_error" yaml:"notify_on_critical_error"`
	NotifyOnFailure                                     bool     `json:"notify_on_failure" yaml:"notify_on_failure"`
	NotifyOnScanStart                                   bool     `json:"notify_on_scan_start" yaml:"notify_on_scan_start"`
	NotifyOnSuccess                                     bool     `json:"notify_on_success" yaml:"notify_on_success"`
	ScanServiceDiscordWebhookURL                        string   `json:"scan_service_discord_webhook_url,omitempty" yaml:"scan_service_discord_webhook_url,omitempty" validate:"omitempty,url"`
}

func NewDefaultNotificationConfig() NotificationConfig {
	return NotificationConfig{
		AutoDeleteSingleDiffReportsAfterDiscordNotification: false,
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
	AggregationIntervalSeconds int      `json:"aggregation_interval_seconds,omitempty" yaml:"aggregation_interval_seconds,omitempty" validate:"omitempty,min=1"`
	CheckIntervalSeconds       int      `json:"check_interval_seconds,omitempty" yaml:"check_interval_seconds,omitempty" validate:"omitempty,min=1"`
	Enabled                    bool     `json:"enabled" yaml:"enabled"`
	HTMLFileExtensions         []string `json:"html_file_extensions,omitempty" yaml:"html_file_extensions,omitempty"`
	HTTPTimeoutSeconds         int      `json:"http_timeout_seconds,omitempty" yaml:"http_timeout_seconds,omitempty" validate:"omitempty,min=1"`
	InitialMonitorURLs         []string `json:"initial_monitor_urls,omitempty" yaml:"initial_monitor_urls,omitempty" validate:"omitempty,dive,url"`
	InputFile                  string   `json:"input_file,omitempty" yaml:"input_file,omitempty" validate:"omitempty,fileexists"` // For compatibility with urlhandler.LoadAndSelectTargets
	InputURLs                  []string `json:"input_urls,omitempty" yaml:"input_urls,omitempty" validate:"omitempty,dive,url"`   // For compatibility with urlhandler.LoadAndSelectTargets
	JSFileExtensions           []string `json:"js_file_extensions,omitempty" yaml:"js_file_extensions,omitempty"`
	MaxAggregatedEvents        int      `json:"max_aggregated_events,omitempty" yaml:"max_aggregated_events,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentChecks        int      `json:"max_concurrent_checks,omitempty" yaml:"max_concurrent_checks,omitempty" validate:"omitempty,min=1"`
	MaxContentSize             int      `json:"max_content_size,omitempty" yaml:"max_content_size,omitempty" validate:"omitempty,min=1"` // Max content size in bytes
	MonitorInsecureSkipVerify  bool     `json:"monitor_insecure_skip_verify" yaml:"monitor_insecure_skip_verify"`
	StoreFullContentOnChange   bool     `json:"store_full_content_on_change" yaml:"store_full_content_on_change"`
	TargetHTMLFilePatterns     []string `json:"target_html_file_patterns,omitempty" yaml:"target_html_file_patterns,omitempty"`
	TargetJSFilePatterns       []string `json:"target_js_file_patterns,omitempty" yaml:"target_js_file_patterns,omitempty"`
}

func NewDefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		AggregationIntervalSeconds: 600,  // Default to 10 minutes for aggregation
		CheckIntervalSeconds:       3600, // 1 hour
		Enabled:                    false,
		HTMLFileExtensions:         []string{"\\.html", "\\.htm"},
		HTTPTimeoutSeconds:         30,
		InitialMonitorURLs:         []string{},
		JSFileExtensions:           []string{"\\.js", "\\.jsx", "\\.ts", "\\.tsx"},
		MaxAggregatedEvents:        10, // Default to 10 events before sending aggregated notification
		MaxConcurrentChecks:        5,
		MaxContentSize:             1048576, // Default 1MB
		MonitorInsecureSkipVerify:  true,    // Default to true to match previous hardcoded behavior
		StoreFullContentOnChange:   true,
		TargetHTMLFilePatterns:     []string{},
		TargetJSFilePatterns:       []string{},
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
	BeautifyHTMLForDiff bool `json:"beautify_html_for_diff,omitempty" yaml:"beautify_html_for_diff,omitempty"`
	BeautifyJSForDiff   bool `json:"beautify_js_for_diff,omitempty" yaml:"beautify_js_for_diff,omitempty"`
	MaxDiffFileSizeMB   int  `json:"max_diff_file_size_mb,omitempty" yaml:"max_diff_file_size_mb,omitempty"`
}

func NewDefaultDiffReporterConfig() DiffReporterConfig {
	return DiffReporterConfig{
		BeautifyHTMLForDiff: true, // TODO: implement
		BeautifyJSForDiff:   true, // TODO: implement
		MaxDiffFileSizeMB:   5,    // Default 5MB
	}
}
