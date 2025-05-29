# MonsterInc Wiki

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Module Details](#module-details)
4. [Data Flow](#data-flow)
5. [Configuration System](#configuration-system)
6. [Storage and Persistence](#storage-and-persistence)
7. [Notification System](#notification-system)
8. [Development Guidelines](#development-guidelines)
9. [Troubleshooting](#troubleshooting)
10. [Package Documentation](#package-documentation)

## Project Overview

MonsterInc is a comprehensive web reconnaissance and monitoring tool designed for security researchers and penetration testers. It combines web crawling, HTTP probing, content monitoring, and secret detection into a unified platform.

### Core Capabilities

- **Web Discovery**: Automated crawling and URL discovery from seed targets
- **HTTP Analysis**: Comprehensive HTTP/HTTPS probing with technology detection
- **Content Monitoring**: Track changes in web files (HTML, JS, CSS)
- **Secret Detection**: Identify exposed secrets and sensitive information
- **Diff Analysis**: Compare scan results over time to identify changes
- **Automated Reporting**: Generate detailed HTML reports with visualizations
- **Discord Notifications**: Real-time alerts for findings and changes

### Execution Modes

#### Onetime Mode
- Single scan execution
- Immediate results and reporting
- Suitable for ad-hoc reconnaissance

#### Automated Mode
- Continuous monitoring with scheduled scans
- Change detection and alerting
- Long-term target surveillance

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI Interface │    │  Configuration  │    │     Logging     │
│   (cmd/main.go) │    │    Management   │    │    Framework    │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │     Orchestrator        │
                    │  (Workflow Management)  │
                    └────────────┬────────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                       │                        │
┌───────▼───────┐    ┌──────────▼──────────┐    ┌────────▼────────┐
│    Crawler    │    │   HTTPx Runner      │    │   Monitor       │
│  (URL Discovery)│   │  (HTTP Probing)     │    │  (File Tracking)│
└───────┬───────┘    └──────────┬──────────┘    └────────┬────────┘
        │                       │                        │
        └───────────────────────┼────────────────────────┘
                                │
                    ┌───────────▼───────────┐
                    │      Datastore        │
                    │   (Parquet Storage)   │
                    └───────────┬───────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                      │                       │
┌───────▼───────┐    ┌─────────▼─────────┐    ┌────────▼────────┐
│    Differ     │    │     Reporter      │    │    Notifier     │
│ (Change Det.) │    │  (HTML Reports)   │    │ (Discord Alerts)│
└───────────────┘    └───────────────────┘    └─────────────────┘
```

### Component Interaction

1. **CLI Interface** parses arguments and loads configuration
2. **Orchestrator** coordinates the entire workflow
3. **Crawler** discovers URLs from seed targets
4. **HTTPx Runner** probes discovered URLs for detailed information
5. **Datastore** persists results in Parquet format
6. **Differ** compares current results with historical data
7. **Monitor** tracks specific files for changes
8. **Reporter** generates HTML reports
9. **Notifier** sends Discord notifications

## Module Details

### cmd/monsterinc
**Purpose**: Main application entry point
- Command-line argument parsing
- Configuration loading and validation
- Mode selection (onetime vs automated)
- Graceful shutdown handling

**main.go**: The main application file that handles:
- Command-line argument parsing
- Configuration loading and validation
- Mode selection and execution
- Graceful shutdown handling
- Global initialization of services

#### Command-Line Arguments

**Required Arguments**
- `--mode <onetime|automated>`: Execution mode

**Optional Arguments**
- `--scan-targets, -st <path>`: Path to file containing seed URLs
- `--monitor-targets, -mt <path>`: File containing URLs to monitor (automated mode)
- `--globalconfig, -gc <path>`: Path to configuration file (default: config.yaml)

#### Execution Modes

**Onetime Mode**: Executes a single scan cycle and exits:
1. Load targets from file or configuration
2. Execute complete scan workflow
3. Generate report and send notifications
4. Exit

**Automated Mode**: Runs continuously with scheduled scans:
1. Initialize scheduler and monitoring services
2. Execute scan cycles at configured intervals
3. Handle retries for failed scans
4. Maintain scan history in SQLite database
5. Optional file monitoring for real-time change detection

### internal/config
**Purpose**: Configuration management
- YAML/JSON configuration loading
- Validation with custom rules
- Default value management
- Environment variable support

### internal/crawler
**Purpose**: Web crawling and URL discovery
- Colly-based web crawling
- Scope management (hostnames, paths, regexes)
- Asset extraction (JS, CSS, images)
- Robots.txt compliance
- Rate limiting and concurrency control

### internal/httpxrunner
**Purpose**: HTTP/HTTPS probing
- ProjectDiscovery httpx integration
- Technology detection
- TLS information extraction
- Response analysis
- Concurrent probing with rate limiting

### internal/datastore
**Purpose**: Data persistence
- Parquet file storage
- Compression (ZSTD, GZIP, SNAPPY)
- Schema management
- Efficient querying
- File history tracking

### internal/differ
**Purpose**: Change detection
- URL diff analysis
- Content change detection
- Status comparison
- Historical data analysis

### internal/monitor
**Purpose**: File monitoring
- Periodic file checking
- Content hash comparison
- Change notification
- Aggregated reporting

### internal/notifier
**Purpose**: Notification system
- Discord webhook integration
- Message formatting
- Role mentions
- Notification aggregation

### internal/reporter
**Purpose**: Report generation
- HTML report creation
- DataTables integration
- Asset embedding
- Diff visualization

### internal/secrets
**Purpose**: Secret detection
- Regex pattern matching
- TruffleHog integration
- Custom pattern support
- Confidence scoring

## Data Flow

### Onetime Scan Flow

```
1. Parse CLI arguments
2. Load and validate configuration
3. Initialize components (crawler, httpx, etc.)
4. Crawl seed URLs to discover endpoints
5. Probe discovered URLs with httpx
6. Store results in Parquet files
7. Compare with historical data (if exists)
8. Generate HTML reports
9. Send Discord notifications
10. Cleanup and exit
```

### Automated Mode Flow

```
1. Parse CLI arguments
2. Load and validate configuration
3. Initialize scheduler and monitor services
4. Start periodic scan cycles:
   a. Execute onetime scan workflow
   b. Monitor specific files for changes
   c. Aggregate notifications
   d. Wait for next cycle
5. Continue until interrupted
```

### Data Processing Pipeline

```
Seed URLs → Crawler → Discovered URLs → HTTPx → Probe Results
                                                      ↓
Historical Data ← Parquet Storage ← Result Processing
                                                      ↓
Diff Analysis → Change Detection → Notifications + Reports
```

## Configuration System

### Configuration Hierarchy

1. **Command-line flags** (highest priority)
2. **Configuration file** (YAML/JSON)
3. **Default values** (lowest priority)

### Configuration Sections

- **Input**: Target URLs and input files
- **Crawler**: Web crawling parameters
- **HTTPx**: HTTP probing configuration
- **Storage**: Parquet storage settings
- **Monitor**: File monitoring configuration
- **Secrets**: Secret detection settings
- **Notifications**: Discord webhook settings
- **Logging**: Log level and format
- **Scheduler**: Automated mode settings

### Configuration Validation

- Required field validation
- Type checking
- Range validation
- Custom validators for specific fields
- Detailed error reporting

## Storage and Persistence

### Parquet Storage Architecture

```
database/
├── <target1>/
│   ├── data.parquet          # Consolidated scan results
│   └── file_history.parquet  # File change history
├── <target2>/
│   ├── data.parquet
│   └── file_history.parquet
└── secrets/
    └── findings.parquet      # Secret detection results
```

### Data Models

#### ProbeResult
- URL and HTTP response information
- Technology detection results
- TLS/SSL information
- DNS resolution data
- Timing and error information

#### MonitoredFile
- File URL and content hash
- Change timestamps
- Content snapshots
- Size and type information

#### SecretFinding
- Secret type and value
- Confidence score
- Location information
- Context and metadata

### Storage Benefits

- **Compression**: Efficient storage with ZSTD compression
- **Querying**: Fast columnar queries
- **Schema Evolution**: Support for schema changes
- **Interoperability**: Standard Parquet format

## Notification System

### Discord Integration

- **Webhook Support**: Send messages to Discord channels
- **Rich Formatting**: Embedded messages with colors and fields
- **Role Mentions**: Notify specific roles for critical findings
- **Aggregation**: Batch notifications to reduce spam

### Notification Types

- **Scan Start/Complete**: Workflow status updates
- **New Findings**: Newly discovered URLs or technologies
- **Changes Detected**: Content or status changes
- **Secrets Found**: Detected sensitive information
- **Errors**: Critical errors and failures

### Message Formatting

- **Color Coding**: Different colors for different message types
- **Structured Fields**: Organized information display
- **Timestamps**: All messages include timestamps
- **Links**: Direct links to reports and findings

## Development Guidelines

### Code Organization

- **Package Structure**: Logical separation of concerns
- **Interface Design**: Clear interfaces between components
- **Error Handling**: Comprehensive error handling and logging
- **Testing**: Unit tests for critical functionality

### Coding Standards

- **Go Conventions**: Follow standard Go conventions
- **Documentation**: Comprehensive code documentation
- **Logging**: Structured logging with zerolog
- **Configuration**: Externalized configuration

### Adding New Features

1. **Design**: Plan the feature and its integration points
2. **Implementation**: Implement with proper error handling
3. **Testing**: Add unit and integration tests
4. **Documentation**: Update README and wiki
5. **Configuration**: Add configuration options if needed

### Performance Considerations

- **Concurrency**: Use goroutines for parallel processing
- **Memory Management**: Efficient memory usage
- **Rate Limiting**: Respect target server limits
- **Caching**: Cache frequently accessed data

## Troubleshooting

### Common Issues

#### High Memory Usage
- **Cause**: Large result sets or inefficient processing
- **Solution**: Reduce concurrency, implement batching

#### Network Timeouts
- **Cause**: Slow targets or network issues
- **Solution**: Increase timeouts, reduce concurrency

#### Storage Issues
- **Cause**: Disk space or permission problems
- **Solution**: Check disk space, verify permissions

#### Configuration Errors
- **Cause**: Invalid configuration values
- **Solution**: Validate configuration, check examples

### Debugging Tips

1. **Enable Debug Logging**: Set log level to debug
2. **Check Configuration**: Validate all configuration values
3. **Monitor Resources**: Watch CPU, memory, and network usage
4. **Test Components**: Test individual components in isolation
5. **Review Logs**: Examine logs for error patterns

### Performance Optimization

1. **Tune Concurrency**: Adjust thread counts for optimal performance
2. **Optimize Timeouts**: Balance speed vs completeness
3. **Use Appropriate Compression**: Choose compression based on use case
4. **Monitor Metrics**: Track performance metrics over time

### Error Recovery

1. **Graceful Degradation**: Continue operation when possible
2. **Retry Logic**: Implement retries for transient errors
3. **State Recovery**: Recover from partial failures
4. **Backup Strategies**: Maintain data backups

## Security Considerations

### Target Interaction
- **Rate Limiting**: Avoid overwhelming target servers
- **User-Agent**: Use appropriate user-agent strings
- **Robots.txt**: Respect robots.txt when appropriate

### Data Handling
- **Sensitive Data**: Handle secrets and sensitive information carefully
- **Storage Security**: Secure storage of collected data
- **Access Control**: Implement appropriate access controls

### Network Security
- **Proxy Support**: Route traffic through proxies when needed
- **TLS Verification**: Proper TLS certificate handling
- **DNS Security**: Secure DNS resolution

## Future Enhancements

### Planned Features
- **API Interface**: REST API for programmatic access
- **Web UI**: Web-based user interface
- **Plugin System**: Extensible plugin architecture
- **Advanced Analytics**: Enhanced data analysis capabilities

### Scalability Improvements
- **Distributed Processing**: Support for distributed scanning
- **Database Backend**: Optional database storage
- **Cloud Integration**: Cloud platform integration
- **Container Support**: Docker and Kubernetes support

### Integration Enhancements
- **SIEM Integration**: Security Information and Event Management
- **Threat Intelligence**: Integration with threat intelligence feeds
- **Vulnerability Scanning**: Integration with vulnerability scanners
- **Compliance Reporting**: Compliance and audit reporting

## Package Documentation

### cmd/monsterinc Package

This package contains the main entry point for the MonsterInc application.

#### Overview

The `cmd/monsterinc` package provides the command-line interface and orchestrates the execution of different operational modes (onetime and automated).

#### Files

**main.go**: The main application file that handles:
- Command-line argument parsing
- Configuration loading and validation
- Mode selection and execution
- Graceful shutdown handling
- Global initialization of services

#### Command-Line Arguments

**Required Arguments**
- `--mode <onetime|automated>`: Execution mode

**Optional Arguments**
- `--scan-targets, -st <path>`: Path to file containing seed URLs
- `--monitor-targets, -mt <path>`: File containing URLs to monitor (automated mode)
- `--globalconfig, -gc <path>`: Path to configuration file (default: config.yaml)

#### Execution Modes

**Onetime Mode**: Executes a single scan cycle and exits:
1. Load targets from file or configuration
2. Execute complete scan workflow
3. Generate report and send notifications
4. Exit

**Automated Mode**: Runs continuously with scheduled scans:
1. Initialize scheduler and monitoring services
2. Execute scan cycles at configured intervals
3. Handle retries for failed scans
4. Maintain scan history in SQLite database
5. Optional file monitoring for real-time change detection

### internal/config Package

This package handles configuration management for the MonsterInc application, including loading, validation, and default value management.

#### Overview

The config package provides a centralized configuration system that supports YAML and JSON formats with comprehensive validation and default values.

#### Files

**config.go**: Contains all configuration structures and their default constructors
**loader.go**: Handles configuration file discovery and loading
**validator.go**: Provides configuration validation

#### Configuration Structure

The configuration is organized into logical sections:
- **InputConfig**: Target URLs and input files
- **CrawlerConfig**: Web crawling parameters
- **HttpxRunnerConfig**: HTTP probing settings
- **ReporterConfig**: HTML report generation
- **StorageConfig**: Parquet storage configuration
- **NotificationConfig**: Discord notifications
- **MonitorConfig**: File monitoring settings
- **SecretsConfig**: Secret detection configuration
- **SchedulerConfig**: Automated mode settings
- **LogConfig**: Logging configuration

#### Configuration Loading

The configuration loader searches for files in this order:
1. Path specified by `--globalconfig` flag
2. `config.yaml` in current directory
3. `config.json` in current directory

### internal/crawler Package

This package provides web crawling functionality for discovering URLs from seed targets using the Colly framework.

#### Overview

The crawler package implements intelligent web crawling with configurable scope, rate limiting, and content filtering. It discovers URLs, extracts assets, and respects crawling boundaries.

#### Files

**crawler.go**: Main crawler implementation
**scope.go**: Crawling scope management
**asset.go**: Asset extraction from HTML content

#### Key Features

**Intelligent Crawling**:
- Depth control with configurable maximum crawl depth
- Concurrent processing with multi-threaded crawling
- Content filtering to skip large files
- Optional robots.txt compliance
- Configurable request timeouts

**Scope Management**:
- Hostname filtering (allow/disallow specific hostnames)
- Subdomain control (include/exclude subdomains)
- Path regex pattern-based filtering
- Comprehensive URL validation

**Asset Discovery**:
- JavaScript file extraction (.js files)
- CSS stylesheet extraction (.css files)
- Image URL extraction including srcset
- Link extraction from anchor tags
- Relative URL resolution to absolute URLs

### internal/httpxrunner Package

This package provides a Go wrapper around ProjectDiscovery's httpx tool for HTTP/HTTPS probing and information extraction.

#### Overview

The httpxrunner package integrates the powerful httpx library to perform comprehensive HTTP analysis, including technology detection, TLS information extraction, and response analysis.

#### Files

**runner.go**: Main httpx wrapper implementation
**result.go**: Result processing utilities

#### Key Features

**HTTP/HTTPS Probing**:
- Multiple HTTP methods support (GET, POST, HEAD, etc.)
- Custom headers and proxy support
- Configurable redirect handling
- Rate limiting to avoid overwhelming targets

**Information Extraction**:
- Technology detection (identify web technologies and frameworks)
- TLS analysis (extract TLS version, cipher, certificate information)
- Response headers capture and analysis
- Status codes and redirect tracking
- Content analysis (type, length, body)

**Network Information**:
- IP resolution and capture
- CNAME record extraction
- ASN information identification
- Comprehensive DNS information gathering

### internal/datastore Package

This package provides data storage and persistence functionality using Apache Parquet format for efficient storage and querying of scan results.

#### Overview

The datastore package implements a comprehensive data storage solution using Parquet files for storing probe results, file history, and secrets. It provides efficient compression, fast querying, and schema evolution capabilities.

#### Files

**parquet_writer.go**: Parquet file writing functionality
**parquet_reader.go**: Parquet file reading functionality
**parquet_file_history_store.go**: File history storage implementation
**secrets_store.go**: Secrets storage implementation

#### Key Features

**Parquet Storage**:
- Columnar format for efficient storage and querying
- Multiple compression algorithms (ZSTD, GZIP, SNAPPY)
- Schema evolution support for changes over time
- Strong typing with Go struct mapping
- Batch processing for efficient reads and writes

**Data Management**:
- Data partitioning by date, target, or other dimensions
- Fast lookups using Parquet metadata
- Deduplication to prevent duplicate records
- Data versioning to track changes over time

**Performance Optimization**:
- Lazy loading (load data only when needed)
- Predicate pushdown (filter data at storage level)
- Parallel processing for concurrent reads and writes
- Efficient memory usage for large datasets

### internal/differ Package

This package is responsible for comparing current scan results against historical data to identify new, existing, and old URLs.

#### Core Component

**url_differ.go**: Defines the `UrlDiffer` struct and its methods:
- `NewUrlDiffer()`: Constructor that takes a `datastore.ParquetReader` and logger
- `Compare()`: Main method for performing URL comparison

#### Logic Overview

1. **Fetch Historical Data**: Uses `ParquetReader` to load all probe results from the target's `data.parquet` file
2. **Map Creation**: Creates maps for efficient lookup of current and historical probe results
3. **Comparison & Status Assignment**:
   - **Existing URLs**: URLs found in both current and historical data
   - **New URLs**: URLs found only in current scan
   - **Old URLs**: URLs found only in historical data
4. **Result Aggregation**: Populates `URLDiffResult` structure with all diffed URLs and summary counts

### internal/logger Package

The `logger` package is responsible for initializing and configuring the application-wide logger using the `zerolog` library.

#### Features

- **Structured Logging**: Utilizes `zerolog` for fast, structured JSON logging
- **Configurable Log Levels**: Supports standard log levels (debug, info, warn, error, fatal, panic)
- **Configurable Output Formats**:
  - `console`: Human-readable, colorized output for development
  - `json`: Machine-readable JSON output for log aggregation
  - `text`: Plain text output without color codes
- **File Logging**: Optionally logs to a specified file
- **Log Rotation**: Implements log rotation for file-based logging
- **Multi-Writer Support**: Can log to both console and file simultaneously
- **Standard Log Redirection**: Redirects Go's standard `log` package output to `zerolog`

### internal/models Package

This package defines various data structures used throughout the MonsterInc application, particularly for representing scan results, configuration, and reporting data.

#### Core Data Structures

**ProbeResult**: Represents detailed findings for a single probed URL including:
- Basic info: `InputURL`, `Method`, `Timestamp`, `Duration`, `Error`, `RootTargetURL`
- HTTP response: `StatusCode`, `ContentLength`, `ContentType`, `Headers`, `Body`, `Title`, `WebServer`
- Redirect info: `FinalURL`
- DNS info: `IPs`, `CNAMEs`, `ASN`, `ASNOrg`
- Technology detection: `Technologies` slice
- TLS info: `TLSVersion`, `TLSCipher`, `TLSCertIssuer`, `TLSCertExpiry`

**ParquetProbeResult**: Defines the schema for data stored in Parquet files with optional pointers to handle missing data gracefully

**ReportPageData**: Holds all data necessary for rendering HTML reports including probe results, statistics, and configuration

**URLDiffResult**, **DiffedURL**, **URLStatus**: Central structures for URL diffing feature

### internal/notifier Package

This package is responsible for sending notifications to various services based on scan events and application status.

#### Core Components

**discord_notifier.go**: Contains the `DiscordNotifier` struct for direct Discord webhook API communication
**discord_formatter.go**: Provides functions to format scan data into Discord message payloads
**notification_helper.go**: Higher-level service that simplifies sending specific notification types

#### Key Features

- Send notifications for scan start, completion, and critical errors
- Support for Discord webhooks with rich formatting
- Configurable notification settings
- HTML report attachment to Discord messages
- Graceful handling of missing webhook URLs
- Structured logging for notification attempts

### internal/orchestrator Package

The `orchestrator` package is responsible for managing the overall scan workflow and coordinating various modules.

#### Key Responsibilities

- **Workflow Execution**: Manages the sequence of operations for a scan
- **Module Initialization**: Initializes and uses other internal modules
- **Data Flow Management**: Passes data between modules
- **Configuration Handling**: Uses `config.GlobalConfig` to configure modules
- **Logging**: Integrates with the application's `zerolog` logger

#### Main Workflow

The primary method `ExecuteScanWorkflow()` performs these steps:
1. **Crawler Phase**: Discovers URLs from seed targets (optional)
2. **HTTPX Runner Phase**: Executes HTTP/S probes on discovered URLs
3. **URL Differ Phase**: Compares current results against historical data
4. **Parquet Writer Phase**: Writes collected results to Parquet files
5. **Return Results**: Returns probe results and diff results

### internal/reporter Package

This package is responsible for generating reports from scan results, currently focusing on HTML reports.

#### Components

**HtmlReporter**: Main component for HTML report generation
- `NewHtmlReporter()`: Constructor for `HtmlReporter`
- `GenerateReport()`: Main method for generating HTML reports

#### Features

- **HTML Report Generation**: Creates self-contained HTML files
- **Interactive UI**: Includes global search, filtering, sorting, and pagination
- **Customizable**: Configurable report title and items per page
- **Asset Embedding**: Custom CSS and JavaScript embedded in HTML
- **Multi-target Support**: Navigation sidebar for multiple root targets
- **Modal Views**: Detailed information display for each probe result

### internal/scheduler Package

The `scheduler` package is responsible for managing and orchestrating periodic (automated) scan operations within MonsterInc.

#### Key Responsibilities

1. **Task Scheduling & Main Loop**: Manages the main application loop in automated mode
2. **Scan Cycle Management**: Initiates and manages scan cycle lifecycle
3. **State and History Persistence**: Uses SQLite database for scan history
4. **Error Handling and Retries**: Implements retry mechanism for failed scans
5. **Notifications**: Integrates with notification system for scan events

#### Core Components

**scheduler.go**: Defines the `Scheduler` struct and main scheduling logic
**db.go**: Manages SQLite database connection and schema
**target_manager.go**: Handles loading and selection of target URLs

#### Database Schema

The `scan_history` table stores:
- `scan_session_id`: Unique ID for scan session
- `target_source`: Source of targets
- `num_targets`: Number of targets
- `scan_start_time` / `scan_end_time`: Timing information
- `status`: Scan status (STARTED, COMPLETED, FAILED, etc.)
- `report_file_path`: Path to generated HTML report
- `diff_new` / `diff_old` / `diff_existing`: Diff statistics

### internal/urlhandler Package

Package `urlhandler` provides utilities for processing, normalizing, and validating URLs, as well as reading URLs from files.

#### Core Functions

**URL Validation and Normalization**:
- `NormalizeURL()`: Takes a raw URL string and applies normalization rules
- `ValidateURL()`: Validates a single URL string
- `IsValidURL()`: Convenience function to quickly check URL validity
- `NormalizeURLs()`: Normalizes a slice of URL strings
- `ValidateURLs()`: Validates a slice of URL strings

**File Operations**:
- `ReadURLsFromFile()`: Reads URLs from a specified file, one URL per line

**Utility Functions**:
- `GetBaseURL()`: Extracts the base URL from a given URL string
- `IsDomainOrSubdomain()`: Checks domain/subdomain relationships
- `ResolveURL()`: Resolves relative URLs against a base URL

#### Features

- Comprehensive URL normalization (scheme addition, case conversion, fragment removal)
- File-based URL loading with error handling
- Domain and subdomain validation
- Relative URL resolution
- Detailed error reporting with custom error types 