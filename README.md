# MonsterInc

A comprehensive security tool for website crawling, HTTP/HTTPS probing, content monitoring, and vulnerability discovery with real-time change detection and automated reporting.

## Features

- **🕷️ Web Crawling**: Discover URLs and assets with scope control and headless browser support
- **🔍 HTTP Probing**: Test endpoints with httpx integration and metadata extraction  
- **📊 Content Monitoring**: Track changes with diff detection and history storage
- **📈 Reporting**: Interactive HTML reports with DataTables and visualizations
- **🔔 Notifications**: Real-time Discord alerts with file attachments
- **⚡ Performance**: Batch processing, memory optimization, and interrupt handling

## Quick Start

### Installation

```bash
git clone https://github.com/your-org/monsterinc.git
cd monsterinc
go build -o bin/monsterinc cmd/monsterinc/main.go
```

### Configuration

Copy example config:
```bash
cp configs/config.example.yaml config.yaml
```

Edit `config.yaml` with your settings (see [Configuration](#configuration)).

### Basic Usage

**One-time scan:**
```bash
./bin/monsterinc -config config.yaml -st targets.txt
```

**Automated scanning:**
```bash
./bin/monsterinc -config config.yaml -st targets.txt -mode automated
```

**Custom configuration:**
```bash
./bin/monsterinc -config /path/to/config.yaml -st targets.txt
```

## Configuration

### Essential Settings

```yaml
# Application mode
mode: "onetime"  # or "automated"

# HTTP probing
httpx_runner_config:
  threads: 50
  timeout_secs: 30
  tech_detect: true

# Web crawling
crawler_config:
  max_depth: 3
  max_concurrent_requests: 20
  request_timeout_secs: 30

# Data storage
storage_config:
  parquet_base_path: "./data"
  compression_codec: "zstd"

# Discord notifications
notification_config:
  scan_service_discord_webhook_url: "https://discord.com/api/webhooks/..."

# HTML reports
reporter_config:
  output_dir: "./reports"
  embed_assets: true
```

## Architecture

```
┌─────────────────────┐
│   CLI Entry Point   │ 
│  (cmd/monsterinc)   │
└─────────┬───────────┘
          │
┌─────────▼───────────┐
│      Scanner        │ ← Main orchestrator
│  (internal/scanner) │
└─────────┬───────────┘
          │
    ┌─────▼─────┐
    │ Components │
    └─────┬─────┘
          │
┌─────────▼───────────┐
│    Web Crawler      │ → Discover URLs/assets
│ (internal/crawler)  │
└─────────────────────┘
┌─────────────────────┐
│   HTTPx Runner      │ → Probe endpoints  
│(internal/httpxrunner)│
└─────────────────────┘
┌─────────────────────┐
│     Datastore       │ → Store results
│(internal/datastore) │
└─────────────────────┘
┌─────────────────────┐
│      Reporter       │ → Generate reports
│ (internal/reporter) │
└─────────────────────┘
┌─────────────────────┐
│     Notifier        │ → Send alerts
│ (internal/notifier) │
└─────────────────────┘
```

## Core Packages

| Package | Purpose | Key Features |
|---------|---------|--------------|
| `scanner` | Main orchestrator | Workflow coordination, batch processing |
| `crawler` | Web crawling | URL discovery, asset extraction, scope control |
| `httpxrunner` | HTTP probing | Endpoint testing, technology detection |
| `datastore` | Data persistence | Parquet storage, query interface |
| `reporter` | Report generation | Interactive HTML reports with charts |
| `notifier` | Notifications | Discord integration with file attachments |
| `differ` | Content comparison | Change detection and diff analysis |
| `scheduler` | Automation | Periodic scans with SQLite persistence |
| `config` | Configuration | YAML/JSON config management |
| `logger` | Logging | Structured logging with file rotation |
| `common` | Utilities | HTTP client, file operations, memory pools |

## Usage Examples

### Simple Security Scan

```bash
# Create targets file
echo "https://example.com" > targets.txt

# Run scan
./bin/monsterinc -config config.yaml -st targets.txt

# Check results
ls reports/     # HTML reports
ls data/        # Raw scan data
```

### Automated Monitoring

```yaml
# config.yaml
mode: "automated"
scheduler_config:
  cycle_minutes: 60  # Scan every hour
  retry_attempts: 3

notification_config:
  scan_service_discord_webhook_url: "your-webhook-url"
  notify_on_success: true
```

```bash
./bin/monsterinc -config config.yaml -st targets.txt -mode automated
```

### Custom Crawling Scope

```yaml
crawler_config:
  max_depth: 2
  include_subdomains: true
  scope:
    disallowed_hostnames:
      - "ads.example.com"
    disallowed_file_extensions:
      - ".jpg"
      - ".css"
      - ".js"
```

## Development

### Project Structure
```
monsterinc/
├── cmd/monsterinc/          # CLI entry point
├── internal/                # Core packages
│   ├── scanner/            # Main orchestrator
│   ├── crawler/            # Web crawling
│   ├── httpxrunner/        # HTTP probing
│   ├── datastore/          # Data persistence
│   ├── reporter/           # Report generation
│   ├── notifier/           # Notifications
│   ├── differ/             # Content comparison
│   ├── scheduler/          # Automation
│   ├── config/             # Configuration
│   ├── logger/             # Logging
│   └── common/             # Utilities
├── configs/                 # Example configurations
└── tasks/                  # Development tasks
```

### Testing
```bash
# Run all tests
go test ./...

# Test specific package
go test ./internal/scanner/
```

### Building
```bash
# Build for current platform
go build -o bin/monsterinc cmd/monsterinc/main.go

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o bin/monsterinc.exe cmd/monsterinc/main.go
```

## Package Documentation

Each internal package has detailed documentation:
- [Common Utilities](internal/common/README.md)
- [Configuration Management](internal/config/README.md)
- [Web Crawler](internal/crawler/README.md)
- [Data Storage](internal/datastore/README.md)
- [Content Differ](internal/differ/README.md)
- [HTTP Runner](internal/httpxrunner/README.md)
- [Logging Framework](internal/logger/README.md)
- [Notifications](internal/notifier/README.md)
- [Report Generator](internal/reporter/README.md)
- [Main Scanner](internal/scanner/README.md)
- [Task Scheduler](internal/scheduler/README.md)
- [URL Handler](internal/common/urlhandler/README.md)

## License

This project is licensed under the MIT License.
