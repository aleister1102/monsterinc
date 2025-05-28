# MonsterInc

MonsterInc is a command-line interface (CLI) tool written in Go, specialized for gathering information from websites, performing HTTP/HTTPS probing, and generating detailed reports.

## Key Features

### 🕷️ Web Crawling
- Collect URLs from websites starting from one or more seed URLs
- Control crawl scope (allowed/disallowed hostnames, subdomains, path regexes)
- Customize User-Agent, timeout, depth, number of threads
- Can respect or ignore `robots.txt`
- Check `Content-Length` before crawling to avoid downloading large files

### 🔍 HTTP/HTTPS Probing
- Uses ProjectDiscovery's `httpx` library
- Extract diverse information: status code, content type, content length, title, web server, headers, IPs, CNAMEs, ASN, TLS information, technologies used
- Customize HTTP method, request URIs, headers, proxy, timeout, retries

### 📊 HTML Reporting
- Generate interactive HTML reports from probing results
- Display results in table format with search, filter, and sort capabilities
- Embed custom CSS/JS for good user interface and experience
- Use Bootstrap (via CDN) for basic styling

### 💾 Parquet Storage
- Write probing results to Parquet files for later data analysis
- Support compression codecs: ZSTD (default), SNAPPY, GZIP, UNCOMPRESSED
- Save files in directory structure `ParquetBasePath/YYYYMMDD/scan_results_*.parquet`

### ⚙️ Flexible Configuration
- Manage configuration via YAML file (`config.yaml` preferred) or JSON (`config.json`)
- Support command-line parameters

### 🔄 Periodic Scanning (Automated Mode)
- Allow scheduling of recurring scans (e.g., daily)
- Reload target lists at the beginning of each scan cycle
- Maintain scan history in SQLite database
- Send notifications (e.g., via Discord) on scan start, success, and failure
- Include retry logic for failed scans

### 📁 File Monitoring
- Monitor JS/HTML file changes in real-time
- Detect content changes and generate diff reports
- Support aggregated notifications

### 🔐 Secret Detection
- Integrate TruffleHog for secret detection
- Support custom regex patterns
- Automatic notifications for high severity secrets

### 🔗 Path Extraction
- Extract paths/URLs from JS/HTML content
- Use jsluice library for JS analysis
- Support custom regex patterns

## Installation

### System Requirements
- Go version 1.23.0 or newer

### Install from Source

1. Clone repository:
```bash
git clone https://github.com/your-username/monsterinc.git
cd monsterinc
```

2. Build application:
```bash
# Windows
go build -o monsterinc.exe ./cmd/monsterinc

# Linux/macOS
go build -o monsterinc ./cmd/monsterinc
```

### Install from GitHub Releases

1. Download appropriate binary from [GitHub Releases](https://github.com/your-username/monsterinc/releases)
2. Extract and place in system PATH

### Install via Go install

```bash
go install github.com/your-username/monsterinc/cmd/monsterinc@latest
```

## Usage

### Basic Syntax

```bash
./monsterinc --mode <onetime|automated> [options]
```

### Main Command-Line Parameters

- `--mode <onetime|automated>`: (Required) Execution mode
  - `onetime`: Run once and exit
  - `automated`: Run continuously on schedule
- `-u, --urlfile <path>`: Path to file containing seed URLs list
- `--mtf, --monitor-target-file <path>`: File containing URLs to monitor (automated mode only)
- `--gc, --globalconfig <path>`: Path to configuration file

### Usage Examples

```bash
# Run once with URLs list from file
./monsterinc --mode onetime -u targets.txt

# Run automatically with monitoring
./monsterinc --mode automated --mtf monitor_targets.txt

# Use custom configuration file
./monsterinc --mode onetime --globalconfig custom_config.yaml -u targets.txt

# Run automated mode with both scan and monitor
./monsterinc --mode automated -u scan_targets.txt --mtf monitor_targets.txt
```

## Configuration

### Configuration File

Application searches for configuration files in order:
1. `config.yaml` (preferred)
2. `config.json` (fallback)

Copy `config.example.yaml` to `config.yaml` and edit as needed:

```bash
cp config.example.yaml config.yaml
```

### Main Configuration Sections

- **input_config**: Target URLs source configuration
- **httpx_runner_config**: Settings for httpx probing
- **crawler_config**: Web crawling configuration
- **reporter_config**: HTML report generation settings
- **storage_config**: Parquet storage configuration
- **notification_config**: Discord notification settings
- **monitor_config**: File monitoring configuration
- **secrets_config**: Secret detection settings
- **scheduler_config**: Automated mode configuration

## Directory Structure

```
monsterinc/
├── cmd/monsterinc/         # Application entry point
├── internal/               # Internal application logic
│   ├── config/            # Configuration management
│   ├── crawler/           # Web crawling module
│   ├── datastore/         # Data storage module
│   ├── differ/            # Difference comparison module
│   ├── extractor/         # Path extraction module
│   ├── httpxrunner/       # httpx wrapper
│   ├── logger/            # Logging module
│   ├── models/            # Data structure definitions
│   ├── monitor/           # File monitoring module
│   ├── notifier/          # Notification module
│   ├── orchestrator/      # Workflow orchestration
│   ├── reporter/          # HTML report generation
│   ├── scheduler/         # Automated scan scheduling
│   ├── secrets/           # Secret detection
│   └── urlhandler/        # URL handling and normalization
├── reports/               # HTML reports directory
├── database/              # Database and Parquet files directory
├── tasks/                 # PRD files and task lists
├── config.example.yaml    # Sample configuration file
└── README.md             # This file
```

## Workflow Operation

### Onetime Mode
1. **Initialization**: Load configuration, initialize logger and notification
2. **Target Acquisition**: Determine seed URLs from file or config
3. **Crawling**: Collect URLs from seed URLs
4. **Probing**: Perform HTTP/HTTPS probing with httpx
5. **Diffing**: Compare with historical data from Parquet
6. **Secret Detection**: Scan content for secrets (if enabled)
7. **Path Extraction**: Extract paths from JS/HTML content
8. **Storage**: Save results to Parquet files
9. **Reporting**: Generate HTML report
10. **Notification**: Send completion notification

### Automated Mode
1. **Scheduler**: Calculate next scan time
2. **Target Reloading**: Reload targets for each cycle
3. **Scan Execution**: Execute workflow like onetime mode
4. **History Management**: Save scan history to SQLite
5. **Retry Logic**: Retry if scan fails
6. **File Monitoring**: Monitor JS/HTML file changes (if enabled)

## Logging and Notifications

- Use `zerolog` for structured logging
- Support Discord notifications for:
  - Scan lifecycle events
  - File change notifications
  - Critical errors
  - High severity secrets

## Main Dependencies

- [colly](https://github.com/gocolly/colly) - Web crawling
- [httpx](https://github.com/projectdiscovery/httpx) - HTTP probing
- [parquet-go](https://github.com/parquet-go/parquet-go) - Parquet file handling
- [zerolog](https://github.com/rs/zerolog) - Structured logging
- [jsluice](https://github.com/BishopFox/jsluice) - JavaScript analysis

## Contributing

1. Fork repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'feat: add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Create Pull Request

## License

This project is distributed under the MIT License. See `LICENSE` file for more details.

## Support

- Create [GitHub Issue](https://github.com/your-username/monsterinc/issues) to report bugs or suggest features
- See [Wiki](./wiki.md) for more details about project structure and operation 