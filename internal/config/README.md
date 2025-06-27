# Config Package

## Purpose
The `config` package provides comprehensive configuration management for MonsterInc - a security tool for website crawling, HTTP/HTTPS probing, and content monitoring. It handles loading, validation, and centralized management of all subsystem configurations with support for YAML/JSON formats and environment-based resolution.

## Package Role in MonsterInc
As the configuration backbone, this package enables:
- **Centralized Configuration**: Single source of truth for all component settings
- **Type-Safe Configuration**: Structured configuration with validation
- **Environment Flexibility**: Development, staging, and production configurations
- **Hot-Reload Support**: Dynamic configuration updates without restart
- **Component Integration**: Seamless configuration distribution to all packages

## Main Components

### 1. Global Configuration (`config.go`)
#### Purpose
- Centralized configuration structure for all application components
- Type-safe configuration with struct tags
- JSON and YAML format support
- Default value management

#### API Usage

```go
// Load configuration from file
cfg, err := config.LoadGlobalConfig("config.yaml", logger)
if err != nil {
    return err
}

// Create default configuration
defaultCfg := config.NewDefaultGlobalConfig()

// Access subsystem configurations
crawlerCfg := cfg.CrawlerConfig
httpxCfg := cfg.HttpxRunnerConfig
monitorCfg := cfg.MonitorConfig
```

#### Configuration Structure
```go
type GlobalConfig struct {
    CrawlerConfig         CrawlerConfig         `yaml:"crawler_config"`
    DiffConfig            DiffConfig            `yaml:"diff_config"`
    DiffReporterConfig    DiffReporterConfig    `yaml:"diff_reporter_config"`
    HttpxRunnerConfig     HttpxRunnerConfig     `yaml:"httpx_runner_config"`
    LogConfig             LogConfig             `yaml:"log_config"`
    Mode                  string                `yaml:"mode" validate:"required,mode"`
    MonitorConfig         MonitorConfig         `yaml:"monitor_config"`
    NotificationConfig    NotificationConfig    `yaml:"notification_config"`
    ReporterConfig        ReporterConfig        `yaml:"reporter_config"`
    ResourceLimiterConfig ResourceLimiterConfig `yaml:"resource_limiter_config"`
    SchedulerConfig       SchedulerConfig       `yaml:"scheduler_config"`
    StorageConfig         StorageConfig         `yaml:"storage_config"`
}
```

### 2. Configuration Loading (`loader.go`)
#### Purpose
- Intelligent configuration file discovery
- Multiple search locations
- Environment variable support
- File validation

#### API Usage

```go
// Get configuration path with fallback logic
configPath := config.GetConfigPath("custom-config.yaml")

// Create locator for configuration discovery
locator := config.NewConfigFileLocator(logger)
foundPath := locator.FindConfigFile("")

// The locator checks in this order:
// 1. Provided path
// 2. MONSTERINC_CONFIG environment variable
// 3. Current working directory
// 4. Executable directory
// 5. configs/ subdirectory
```

### 3. Configuration Validation (`validator.go`)
#### Purpose
- Comprehensive configuration validation
- Custom validation rules
- Field-level validation with detailed error messages
- Built-in and custom validators

#### API Usage

```go
// Validate configuration
validator := config.NewConfigValidator(logger)
err := validator.Validate(cfg)
if err != nil {
    log.Printf("Configuration validation failed: %v", err)
}

// Simple validation function
err = config.ValidateConfig(cfg)
```

## Configuration Examples

### Complete YAML Configuration
```yaml
# Application mode: "onetime" or "automated"
mode: "automated"

# HTTPx Runner Configuration
httpx_runner_config:
  extract_asn: true
  extract_body: false
  extract_content_length: true
  extract_content_type: true
  extract_headers: true
  extract_ips: true
  extract_location: true
  extract_server_header: true
  extract_status_code: true
  extract_title: true
  follow_redirects: true
  max_redirects: 5
  method: "GET"
  rate_limit: 100
  retries: 2
  tech_detect: true
  threads: 50
  timeout_secs: 30
  verbose: false
  custom_headers:
    User-Agent: "MonsterInc-Scanner/1.0"
    Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

# Crawler Configuration
crawler_config:
  auto_add_seed_hostnames: true
  enable_content_length_check: true
  include_subdomains: false
  insecure_skip_tls_verify: false
  max_concurrent_requests: 10
  max_content_length_mb: 50
  max_depth: 3
  request_timeout_secs: 30
  respect_robots_txt: false
  user_agent: "MonsterInc-Crawler/1.0"
  
  # Scope configuration
  scope:
    disallowed_hostnames:
      - "ads.example.com"
      - "analytics.example.com"
    disallowed_subdomains:
      - "cdn"
      - "static"
    disallowed_file_extensions:
      - ".jpg"
      - ".png"
      - ".gif"
      - ".css"
      - ".ico"
  
# Monitor Configuration
monitor_config:
  enabled: true
  check_interval_seconds: 300    # 5 minutes
  aggregation_interval_seconds: 600  # 10 minutes
  http_timeout_seconds: 30
  max_concurrent_checks: 5
  max_content_size: 10485760    # 10MB
  max_aggregated_events: 100
  monitor_insecure_skip_verify: false
  store_full_content_on_change: true
  html_file_extensions:
    - ".html"
    - ".htm"
    - ".php"
    - ".asp"
    - ".jsp"
  js_file_extensions:
    - ".js"
    - ".mjs"
  initial_monitor_urls:
    - "https://example.com/api/config.json"
    - "https://example.com/static/app.js"

# Storage Configuration
storage_config:
  parquet_base_path: "./data"
  compression_codec: "zstd"

# Notification Configuration
notification_config:
  scan_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  monitor_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  notify_on_scan_start: true
  notify_on_success: true
  notify_on_failure: true
  mention_role_ids:
    - "123456789012345678"
    - "987654321098765432"

# Reporter Configuration
reporter_config:
  output_dir: "./reports"
  report_title: "MonsterInc Security Scan Report"
  embed_assets: true
  enable_data_tables: true
  items_per_page: 50
  default_items_per_page: 25
  max_probe_results_per_report_file: 10000

# Scheduler Configuration
scheduler_config:
  sqlite_db_path: "./scheduler.db"
  cycle_minutes: 60
  retry_attempts: 3

# Logging Configuration
log_config:
  log_level: "info"
  log_format: "json"
  log_file: "./logs/monsterinc.log"
  max_log_size_mb: 100
  max_log_backups: 5

# Diff Configuration
diff_config:
  previous_scan_lookback_days: 7

# Diff Reporter Configuration
diff_reporter_config:
  max_diff_file_size_mb: 10

# Resource Limiter Configuration
resource_limiter_config:
  max_memory_mb: 1024           # Application memory limit (1GB)
  max_goroutines: 10000         # Maximum goroutines allowed
  check_interval_secs: 30       # Resource monitoring interval (seconds)
  memory_threshold: 0.8         # App memory warning threshold (80%)
  goroutine_warning: 0.7        # Goroutine warning threshold (70%)
  system_mem_threshold: 0.5     # System memory shutdown threshold (50%)
  enable_auto_shutdown: true    # Enable auto-shutdown on system memory limit
```

### Environment Variable Configuration
```bash
# Configuration file location
export MONSTERINC_CONFIG="/path/to/config.yaml"

# Override specific values
export MONSTERINC_MODE="onetime"
export MONSTERINC_LOG_LEVEL="debug"
```

## Validation Rules

### Built-in Validators
- **mode**: Validates application mode ("onetime" or "automated")
- **url**: Validates URL format
- **filepath**: Validates file path existence
- **dirpath**: Validates directory path
- **loglevel**: Validates log levels (trace, debug, info, warn, error, fatal, panic)
- **logformat**: Validates log formats (json, text, console)

### Custom Validation Examples
```go
// Custom validator for scan intervals
func validateScanInterval(fl validator.FieldLevel) bool {
    interval := fl.Field().Int()
    return interval >= 60 && interval <= 86400 // 1 minute to 24 hours
}

// Custom validator for file extensions
func validateFileExtensions(fl validator.FieldLevel) bool {
    extensions := fl.Field().Interface().([]string)
    for _, ext := range extensions {
        if !strings.HasPrefix(ext, ".") {
            return false
        }
    }
    return true
}
```

## Extension Methods

### 1. Adding New Configuration Sections
```go
// Define new configuration struct
type NewServiceConfig struct {
    Enabled     bool   `yaml:"enabled"`
    Endpoint    string `yaml:"endpoint" validate:"required,url"`
    Timeout     int    `yaml:"timeout" validate:"min=1,max=300"`
    Retries     int    `yaml:"retries" validate:"min=0,max=10"`
}

// Add to GlobalConfig
type GlobalConfig struct {
    // ... existing fields
    NewServiceConfig NewServiceConfig `yaml:"new_service_config"`
}

// Provide default values
func NewDefaultNewServiceConfig() NewServiceConfig {
    return NewServiceConfig{
        Enabled:  false,
        Endpoint: "https://api.example.com",
        Timeout:  30,
        Retries:  3,
    }
}
```

### 2. Environment Variable Integration
```go
// Custom environment variable handling
func loadConfigFromEnv(cfg *GlobalConfig) {
    if mode := os.Getenv("MONSTERINC_MODE"); mode != "" {
        cfg.Mode = mode
    }
    
    if logLevel := os.Getenv("MONSTERINC_LOG_LEVEL"); logLevel != "" {
        cfg.LogConfig.LogLevel = logLevel
    }
    
    if webhookURL := os.Getenv("DISCORD_WEBHOOK_URL"); webhookURL != "" {
        cfg.NotificationConfig.ScanServiceDiscordWebhookURL = webhookURL
    }
}
```

### 3. Dynamic Configuration Reloading
```go
// Configuration watcher
type ConfigWatcher struct {
    configPath string
    onChange   func(*GlobalConfig)
    logger     zerolog.Logger
}

func (cw *ConfigWatcher) Watch() {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        cw.logger.Error().Err(err).Msg("Failed to create config watcher")
        return
    }
    defer watcher.Close()
    
    err = watcher.Add(cw.configPath)
    if err != nil {
        cw.logger.Error().Err(err).Msg("Failed to watch config file")
        return
    }
    
    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                cw.reloadConfig()
            }
        case err := <-watcher.Errors:
            cw.logger.Error().Err(err).Msg("Config watcher error")
        }
    }
}
```

## Configuration Profiles

### Development Profile
```yaml
mode: "onetime"
log_config:
  log_level: "debug"
  log_format: "console"
crawler_config:
  max_depth: 1
  max_concurrent_requests: 5
httpx_runner_config:
  threads: 10
  verbose: true
```

### Production Profile
```yaml
mode: "automated"
log_config:
  log_level: "info"
  log_format: "json"
  log_file: "/var/log/monsterinc/app.log"
crawler_config:
  max_depth: 3
  max_concurrent_requests: 50
httpx_runner_config:
  threads: 100
  verbose: false
monitor_config:
  enabled: true
  check_interval_seconds: 300
```

### High-Performance Profile
```yaml
crawler_config:
  max_concurrent_requests: 100
  request_timeout_secs: 10
httpx_runner_config:
  threads: 200
  timeout_secs: 15
  rate_limit: 500
storage_config:
  compression_codec: "snappy"  # Faster compression
```

## Best Practices

1. **Configuration Validation**: Always validate configuration before using
2. **Default Values**: Provide sensible defaults for all configuration options
3. **Environment Overrides**: Support environment variables for sensitive data
4. **Documentation**: Document all configuration options with examples
5. **Backward Compatibility**: Maintain compatibility when adding new options
6. **Security**: Never store secrets in configuration files, use environment variables
7. **Testing**: Test configuration loading with various scenarios

## Dependencies
- `github.com/go-playground/validator/v10`: Configuration validation
- `gopkg.in/yaml.v3`: YAML parsing
- `encoding/json`: JSON parsing
- `github.com/rs/zerolog`: Logging framework