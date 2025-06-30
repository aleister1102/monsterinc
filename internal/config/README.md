# Configuration Package

Comprehensive configuration management for MonsterInc with YAML/JSON support, validation, and environment variable integration.

## Overview

Provides centralized configuration management:
- **Structured Configuration**: Type-safe configuration with validation
- **Multiple Formats**: YAML and JSON support
- **Environment Integration**: Environment variable resolution
- **Validation**: Built-in and custom validation rules
- **Auto-Discovery**: Intelligent configuration file location

## Core Components

### Global Configuration

Main configuration structure containing all subsystem settings.

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
notificationCfg := cfg.NotificationConfig
```

### Configuration Loading

Intelligent file discovery with multiple search locations.

```go
// Get configuration path with fallback logic
configPath := config.GetConfigPath("custom-config.yaml")

// Create locator for configuration discovery
locator := config.NewConfigFileLocator(logger)
foundPath := locator.FindConfigFile("")

// Search order:
// 1. Provided path
// 2. MONSTERINC_CONFIG environment variable
// 3. Current working directory
// 4. Executable directory
// 5. configs/ subdirectory
```

### Configuration Validation

Comprehensive validation with custom rules.

```go
// Validate configuration
validator := config.NewConfigValidator(logger)
err := validator.Validate(cfg)
if err != nil {
    log.Printf("Configuration validation failed: %v", err)
}

// Simple validation
err = config.ValidateConfig(cfg)
```

## Essential Configuration

### Basic Example

```yaml
# Application mode
mode: "onetime"  # or "automated"

# HTTP probing configuration
httpx_runner_config:
  threads: 50
  timeout_secs: 30
  retries: 2
  tech_detect: true
  extract_status_code: true
  extract_headers: true
  custom_headers:
    User-Agent: "MonsterInc/1.0"

# Web crawling configuration
crawler_config:
  max_depth: 3
  max_concurrent_requests: 20
  request_timeout_secs: 30
  include_subdomains: false
  scope:
    disallowed_hostnames:
      - "ads.example.com"
    disallowed_file_extensions:
      - ".jpg"
      - ".css"
      - ".js"

# Data storage
storage_config:
  parquet_base_path: "./data"
  compression_codec: "zstd"

# Discord notifications
notification_config:
  scan_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  notify_on_success: true
  notify_on_failure: true

# HTML reports
reporter_config:
  output_dir: "./reports"
  embed_assets: true
  enable_data_tables: true

# Logging
log_config:
  log_level: "info"
  log_format: "json"
  log_file: "./logs/monsterinc.log"
  max_log_size_mb: 100
  max_log_backups: 5

# Automated scheduling
scheduler_config:
  cycle_minutes: 60
  retry_attempts: 3
  sqlite_db_path: "./scheduler.db"
```

### Environment Variables

```bash
# Configuration file location
export MONSTERINC_CONFIG="/path/to/config.yaml"

# Override specific values
export MONSTERINC_MODE="onetime"
export MONSTERINC_LOG_LEVEL="debug"
```

## Configuration Sections

### HTTPx Runner

```yaml
httpx_runner_config:
  method: "GET"
  threads: 50
  timeout_secs: 30
  retries: 2
  rate_limit: 100
  follow_redirects: true
  max_redirects: 5
  
  # Extraction options
  tech_detect: true
  extract_title: true
  extract_status_code: true
  extract_headers: true
  extract_ips: true
  extract_asn: true
  
  # Custom headers
  custom_headers:
    User-Agent: "MonsterInc/1.0"
    Accept: "application/json"
```

### Web Crawler

```yaml
crawler_config:
  max_depth: 3
  max_concurrent_requests: 20
  request_timeout_secs: 30
  max_content_length_mb: 50
  include_subdomains: true
  respect_robots_txt: false
  
  # Scope configuration
  scope:
    disallowed_hostnames:
      - "ads.example.com"
    disallowed_subdomains:
      - "cdn"
      - "static"
    disallowed_file_extensions:
      - ".jpg"
      - ".png"
      - ".css"
      - ".js"
```

### Storage & Reporting

```yaml
# Data storage
storage_config:
  parquet_base_path: "./data"
  compression_codec: "zstd"  # zstd, gzip, snappy

# HTML reports
reporter_config:
  output_dir: "./reports"
  report_title: "Security Scan Report"
  embed_assets: true
  enable_data_tables: true
  items_per_page: 50
  max_probe_results_per_report_file: 10000
```

### Notifications & Logging

```yaml
# Discord notifications
notification_config:
  scan_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  notify_on_scan_start: true
  notify_on_success: true
  notify_on_failure: true
  mention_role_ids:
    - "123456789012345678"

# Logging configuration
log_config:
  log_level: "info"           # trace, debug, info, warn, error
  log_format: "json"          # json, console, text
  log_file: "./logs/app.log"
  max_log_size_mb: 100
  max_log_backups: 5
  use_subdirs: true           # Organize logs by scan/cycle ID
```

## Validation Rules

### Built-in Validators

- **mode**: Application mode ("onetime" or "automated")
- **url**: Valid URL format
- **filepath**: File path existence
- **loglevel**: Log levels (trace, debug, info, warn, error)
- **logformat**: Log formats (json, text, console)

### Custom Validation

```go
// Add custom validator
func validateScanInterval(fl validator.FieldLevel) bool {
    interval := fl.Field().Int()
    return interval >= 60 && interval <= 86400 // 1 minute to 24 hours
}

// Register custom validator
validate.RegisterValidation("scan_interval", validateScanInterval)
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
scheduler_config:
  cycle_minutes: 60
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

## Integration Examples

### With Scanner Service

```go
// Load and validate configuration
cfg, err := config.LoadGlobalConfig("config.yaml", logger)
if err != nil {
    return fmt.Errorf("config load failed: %w", err)
}

// Initialize scanner with config
scanner := scanner.NewScanner(cfg, logger)
```

### Environment Override

```go
// Load configuration with environment overrides
func loadConfigWithEnv(cfg *config.GlobalConfig) {
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

## Best Practices

1. **Configuration Validation**: Always validate before using
2. **Default Values**: Provide sensible defaults for all options
3. **Environment Overrides**: Support environment variables for sensitive data
4. **Documentation**: Document all configuration options
5. **Security**: Never store secrets in config files, use environment variables
6. **Testing**: Test configuration loading with various scenarios

## Dependencies

- `github.com/go-playground/validator/v10`: Configuration validation
- `gopkg.in/yaml.v3`: YAML parsing
- `encoding/json`: JSON parsing
- `github.com/rs/zerolog`: Logging framework