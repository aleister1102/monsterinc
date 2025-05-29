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

MonsterInc là một công cụ reconnaissance và monitoring web toàn diện được thiết kế cho security researchers và penetration testers. Nó kết hợp web crawling, HTTP probing, content monitoring, secret detection và path extraction thành một platform thống nhất.

### Core Capabilities

- **Web Discovery**: Automated crawling và URL discovery từ seed targets
- **HTTP Analysis**: Comprehensive HTTP/HTTPS probing với technology detection
- **Content Monitoring**: Theo dõi thay đổi trong web files (HTML, JS, CSS) với conditional requests
- **Secret Detection**: Phát hiện exposed secrets và sensitive information sử dụng TruffleHog và regex patterns
- **Path Extraction**: Trích xuất API endpoints và paths từ JS/HTML content sử dụng jsluice
- **Diff Analysis**: So sánh scan results theo thời gian để phát hiện thay đổi
- **Automated Reporting**: Tạo detailed HTML reports với visualizations và diff views
- **Discord Notifications**: Real-time alerts cho findings và changes
- **Parquet Storage**: Efficient columnar storage cho large datasets

### Execution Modes

#### Onetime Mode
- Single scan execution
- Immediate results và reporting
- Suitable cho ad-hoc reconnaissance

#### Automated Mode
- Continuous monitoring với scheduled scans
- Change detection và alerting
- Long-term target surveillance
- File monitoring với aggregated notifications

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
│               │    │   (Diff Reports)  │    │                 │
└───────┬───────┘    └─────────┬─────────┘    └────────┬────────┘
        │                      │                       │
┌───────▼───────┐    ┌─────────▼─────────┐    ┌────────▼────────┐
│   Secrets     │    │    Extractor      │    │    Scheduler    │
│  (Detection)  │    │ (Path Extraction) │    │  (Automation)   │
└───────────────┘    └───────────────────┘    └─────────────────┘
```

### Component Interaction

1. **CLI Interface** parses arguments và loads configuration
2. **Orchestrator** coordinates toàn bộ workflow
3. **Crawler** discovers URLs từ seed targets
4. **HTTPx Runner** probes discovered URLs cho detailed information
5. **Datastore** persists results trong Parquet format
6. **Differ** compares current results với historical data
7. **Monitor** tracks specific files cho changes với conditional requests
8. **Reporter** generates HTML reports và diff reports
9. **Notifier** sends Discord notifications
10. **Secrets** detects sensitive information sử dụng TruffleHog và regex
11. **Extractor** extracts paths và API endpoints từ content
12. **Scheduler** manages automated scans và retry logic

## Module Details

### cmd/monsterinc
**Purpose**: Main application entry point
- Command-line argument parsing với flag support
- Configuration loading và validation
- Mode selection (onetime vs automated)
- Graceful shutdown handling với context cancellation
- Global initialization của services

### internal/common
**Purpose**: Shared utilities và patterns
- **constructor_patterns.go**: Standardized service construction patterns
- **context_utils.go**: Context management utilities với cancellation support
- **errors.go**: Custom error types và error handling patterns
- **file_utils.go**: File operations với safety checks
- **http_client.go**: HTTP client factory và configuration
- **interfaces.go**: Common interfaces cho dependency injection
- **notification_utils.go**: Notification building và formatting utilities
- **service_lifecycle.go**: Service lifecycle management patterns
- **workflow_patterns.go**: Workflow execution patterns

### internal/config
**Purpose**: Configuration management
- **config.go**: Configuration structures với default values
- **loader.go**: Configuration file discovery và loading
- **validator.go**: Configuration validation rules
- **manager.go**: Hot-reload configuration management với file watching
- YAML/JSON configuration loading
- Environment variable support
- Validation với custom rules

### internal/crawler
**Purpose**: Web crawling và URL discovery
- **crawler.go**: Main crawler implementation sử dụng Colly
- **scope.go**: Crawling scope management với hostname/path filtering
- **asset.go**: Asset extraction từ HTML content
- Colly-based web crawling với rate limiting
- Scope management (hostnames, paths, regexes)
- Robots.txt compliance
- Content length checks

### internal/httpxrunner
**Purpose**: HTTP/HTTPS probing
- **runner.go**: ProjectDiscovery httpx integration
- **result.go**: Result processing utilities
- Technology detection với comprehensive fingerprinting
- TLS information extraction
- Response analysis với detailed metadata
- Concurrent probing với rate limiting
- Proxy support

### internal/datastore
**Purpose**: Data persistence sử dụng Parquet
- **parquet_writer.go**: Efficient Parquet file writing với compression
- **parquet_reader.go**: Parquet file reading với query optimization
- **parquet_file_history_store.go**: File history storage cho monitoring
- **secrets_store.go**: Secrets storage trong Parquet format
- Columnar format cho efficient storage và querying
- Multiple compression algorithms (ZSTD, GZIP, SNAPPY)
- Schema evolution support
- Batch processing

### internal/differ
**Purpose**: Change detection và comparison
- **url_differ.go**: URL comparison logic với status tracking
- **content_differ.go**: Content diff generation với beautification
- URL diff analysis (New, Existing, Old)
- Content change detection với detailed diffs
- Historical data analysis
- Diff report generation

### internal/monitor
**Purpose**: File monitoring trong real-time
- **service.go**: Main monitoring service với lifecycle management
- **fetcher.go**: HTTP fetching với conditional requests (ETag, Last-Modified)
- **processor.go**: Content processing và change detection
- **scheduler.go**: Periodic checking scheduler
- Conditional GET requests sử dụng ETag và Last-Modified
- Content hash comparison
- Change notification với aggregation
- File type filtering

### internal/notifier
**Purpose**: Notification system
- **discord_notifier.go**: Discord webhook integration
- **discord_formatter.go**: Message formatting với rich embeds
- **notification_helper.go**: High-level notification service
- Discord webhook integration với rich formatting
- Role mentions cho critical findings
- Notification aggregation để reduce spam
- HTML report attachment

### internal/reporter
**Purpose**: Report generation
- **html_reporter.go**: Main HTML report generation
- **html_diff_reporter.go**: Diff report generation với syntax highlighting
- **templates/**: HTML templates cho reports
- **assets/**: CSS, JS, và images cho reports
- HTML report creation với DataTables integration
- Asset embedding cho self-contained reports
- Diff visualization với syntax highlighting
- Multi-target navigation

### internal/secrets
**Purpose**: Secret detection
- **detector_service.go**: Main secret detection service
- **regex_scanner.go**: Custom regex pattern scanning
- **trufflehog_adapter.go**: TruffleHog integration
- **patterns.go**: Default regex patterns
- **patterns.yaml**: Mantra-style pattern definitions
- TruffleHog integration cho comprehensive detection
- Custom regex patterns từ Mantra project
- Confidence scoring và verification
- Parquet storage cho findings

### internal/extractor
**Purpose**: Path extraction từ content
- **path_extractor.go**: Main path extraction logic
- jsluice library integration cho JS analysis
- Custom regex patterns cho additional detection
- API endpoint discovery
- Relative URL resolution

### internal/scheduler
**Purpose**: Automated scan scheduling
- **scheduler.go**: Main scheduling logic với retry mechanisms
- **db.go**: SQLite database management
- **target_manager.go**: Target loading và selection
- SQLite database cho scan history
- Retry logic cho failed scans
- Target reloading cho each cycle
- Notification integration

### internal/logger
**Purpose**: Structured logging
- **logger.go**: Logger initialization và configuration
- zerolog integration với structured JSON logging
- Configurable log levels và formats
- File rotation support
- Multi-writer support

### internal/models
**Purpose**: Data structure definitions
- **probe_result.go**: HTTP probing results
- **file_history.go**: File monitoring history
- **secret_finding.go**: Secret detection findings
- **extracted_path.go**: Path extraction results
- **content_diff.go**: Content diff structures
- **report_data.go**: Report rendering data
- **parquet_schema.go**: Parquet schema definitions
- **notification_models.go**: Discord notification structures

### internal/urlhandler
**Purpose**: URL processing và normalization
- **urlhandler.go**: URL validation và normalization
- **file.go**: File-based URL loading
- Comprehensive URL normalization
- Domain và subdomain validation
- Relative URL resolution
- File-based URL loading với error handling

## Data Flow

### Onetime Scan Flow

```
1. Parse CLI arguments với flag validation
2. Load và validate configuration từ YAML/JSON
3. Initialize components (crawler, httpx, monitor, secrets, extractor)
4. Load target URLs từ file hoặc config
5. Crawl seed URLs để discover endpoints
6. Probe discovered URLs với httpx
7. Extract paths từ JS/HTML content
8. Scan content cho secrets
9. Store results trong Parquet files
10. Compare với historical data (nếu exists)
11. Generate HTML reports và diff reports
12. Send Discord notifications với attachments
13. Cleanup và exit
```

### Automated Mode Flow

```
1. Parse CLI arguments và load configuration
2. Initialize scheduler và monitor services
3. Setup SQLite database cho scan history
4. Start periodic scan cycles:
   a. Load targets từ files
   b. Execute onetime scan workflow
   c. Store scan history trong SQLite
   d. Monitor specific files cho changes
   e. Generate aggregated notifications
   f. Wait cho next cycle với configurable interval
5. Handle graceful shutdown với context cancellation
```

### File Monitoring Flow

```
1. Load monitored URLs từ target files
2. Fetch content sử dụng conditional requests
3. Compare content hashes với stored history
4. Detect changes và generate diffs
5. Extract paths từ changed content
6. Scan cho secrets trong changed content
7. Store change history trong Parquet
8. Generate diff reports với syntax highlighting
9. Send aggregated notifications
```

### Data Processing Pipeline

```
Seed URLs → Crawler → Discovered URLs → HTTPx → Probe Results
                                                      ↓
Path Extractor ← Content Analysis ← Historical Data ← Parquet Storage
       ↓                                                     ↓
Extracted Paths → Secret Scanner → Secret Findings → Notifications
                         ↓
Diff Analysis → Change Detection → Diff Reports → Discord Alerts
```

## Configuration System

### Configuration Hierarchy

1. **Command-line flags** (highest priority)
2. **Configuration file** (YAML/JSON)
3. **Default values** (lowest priority)

### Configuration Sections

- **Input**: Target URLs và input files
- **Crawler**: Web crawling parameters với scope settings
- **HTTPx**: HTTP probing configuration với proxy support
- **Storage**: Parquet storage settings với compression options
- **Monitor**: File monitoring configuration với conditional requests
- **Secrets**: Secret detection settings với TruffleHog và regex config
- **Extractor**: Path extraction configuration với custom regexes
- **Notifications**: Discord webhook settings với role mentions
- **Logging**: Log level và format configuration
- **Scheduler**: Automated mode settings với retry logic
- **Reporter**: HTML report generation settings
- **Diff**: Content diff settings với beautification options

### Hot-Reload Configuration

- File watching với fsnotify
- Automatic configuration reload
- Validation trước khi applying changes
- Graceful handling của configuration errors

## Storage and Persistence

### Parquet Storage Architecture

```
database/
├── scan/
│   └── <hostname>/
│       └── data.parquet              # Consolidated scan results
├── monitor/
│   └── <hostname>/
│       └── file_history.parquet     # File change history
├── secrets/
│   └── findings.parquet             # Secret detection results
└── scheduler/
    └── scheduler_history.db         # SQLite for scan history
```

### Data Models

#### ProbeResult
- URL và HTTP response information
- Technology detection results với detailed fingerprinting
- TLS/SSL information với certificate details
- DNS resolution data với ASN information
- Timing và error information
- Diff status (New, Existing, Old)

#### FileHistoryRecord
- File URL và content hash
- Change timestamps với millisecond precision
- Content snapshots với optional full content storage
- ETag và Last-Modified headers
- Diff results trong JSON format
- Extracted paths trong JSON format

#### SecretFinding
- Secret type và value với truncation
- Confidence score từ detection tools
- Location information với line numbers
- Context và metadata
- Verification state
- Tool name (TruffleHog, RegexScanner)

#### ExtractedPath
- Source URL và extracted raw path
- Resolved absolute URL
- Context information (HTML attribute, JS string literal)
- Discovery timestamp
- Path type classification

### Storage Benefits

- **Compression**: Efficient storage với ZSTD compression
- **Querying**: Fast columnar queries với predicate pushdown
- **Schema Evolution**: Support cho schema changes over time
- **Interoperability**: Standard Parquet format cho external tools
- **Partitioning**: Data organized by hostname cho efficient access

## Notification System

### Discord Integration

- **Webhook Support**: Send messages đến Discord channels
- **Rich Formatting**: Embedded messages với colors, fields, và timestamps
- **Role Mentions**: Notify specific roles cho critical findings
- **Aggregation**: Batch notifications để reduce spam
- **File Attachments**: HTML reports attached đến notifications

### Notification Types

- **Scan Start/Complete**: Workflow status updates với timing information
- **New Findings**: Newly discovered URLs hoặc technologies
- **Changes Detected**: Content hoặc status changes với diff summaries
- **Secrets Found**: Detected sensitive information với severity levels
- **Monitor Alerts**: File changes với aggregated summaries
- **Errors**: Critical errors và failures với context

### Message Formatting

- **Color Coding**: Different colors cho different message types
- **Structured Fields**: Organized information display
- **Timestamps**: All messages include ISO8601 timestamps
- **Links**: Direct links đến reports và findings
- **Embed Limits**: Respect Discord's embed limits với pagination

## Development Guidelines

### Code Organization

- **Package Structure**: Logical separation của concerns
- **Interface Design**: Clear interfaces giữa components
- **Error Handling**: Comprehensive error handling với custom types
- **Testing**: Unit tests cho critical functionality
- **Documentation**: Comprehensive code documentation với examples

### Coding Standards

- **Go Conventions**: Follow standard Go conventions
- **Constructor Patterns**: Standardized New* functions với validation
- **Service Lifecycle**: Consistent service startup/shutdown patterns
- **Configuration**: Externalized configuration với validation
- **Context Usage**: Proper context propagation cho cancellation

### Adding New Features

1. **Design**: Plan feature và integration points
2. **Implementation**: Implement với proper error handling
3. **Testing**: Add unit và integration tests
4. **Documentation**: Update README và wiki
5. **Configuration**: Add configuration options nếu needed
6. **Package Documentation**: Update package-specific documentation

### Performance Considerations

- **Concurrency**: Use goroutines cho parallel processing
- **Memory Management**: Efficient memory usage với pooling
- **Rate Limiting**: Respect target server limits
- **Caching**: Cache frequently accessed data
- **Batch Processing**: Process data trong batches

## Troubleshooting

### Common Issues

#### High Memory Usage
- **Cause**: Large result sets hoặc inefficient processing
- **Solution**: Reduce concurrency, implement batching, use streaming

#### Network Timeouts
- **Cause**: Slow targets hoặc network issues
- **Solution**: Increase timeouts, reduce concurrency, use retries

#### Storage Issues
- **Cause**: Disk space hoặc permission problems
- **Solution**: Check disk space, verify permissions, implement cleanup

#### Configuration Errors
- **Cause**: Invalid configuration values
- **Solution**: Validate configuration, check examples, review logs

#### Discord Notification Failures
- **Cause**: Invalid webhook URLs hoặc rate limiting
- **Solution**: Verify webhook URLs, implement backoff, use aggregation

### Debugging Tips

1. **Enable Debug Logging**: Set log level đến debug
2. **Check Configuration**: Validate all configuration values
3. **Monitor Resources**: Watch CPU, memory, và network usage
4. **Test Components**: Test individual components trong isolation
5. **Review Logs**: Examine logs cho error patterns
6. **Database Inspection**: Check Parquet files và SQLite database

### Performance Optimization

1. **Tune Concurrency**: Adjust thread counts cho optimal performance
2. **Optimize Timeouts**: Balance speed vs completeness
3. **Use Appropriate Compression**: Choose compression based on use case
4. **Monitor Metrics**: Track performance metrics over time
5. **Implement Caching**: Cache expensive operations
6. **Batch Operations**: Group operations cho efficiency

### Error Recovery

1. **Graceful Degradation**: Continue operation khi possible
2. **Retry Logic**: Implement retries cho transient errors
3. **State Recovery**: Recover từ partial failures
4. **Backup Strategies**: Maintain data backups
5. **Circuit Breakers**: Implement circuit breakers cho external services

## Security Considerations

### Target Interaction
- **Rate Limiting**: Avoid overwhelming target servers
- **User-Agent**: Use appropriate user-agent strings
- **Robots.txt**: Respect robots.txt khi appropriate
- **Conditional Requests**: Use ETag và Last-Modified cho efficiency

### Data Handling
- **Sensitive Data**: Handle secrets và sensitive information carefully
- **Storage Security**: Secure storage của collected data
- **Access Control**: Implement appropriate access controls
- **Data Retention**: Implement data retention policies

### Network Security
- **Proxy Support**: Route traffic through proxies khi needed
- **TLS Verification**: Proper TLS certificate handling
- **DNS Security**: Secure DNS resolution
- **Input Validation**: Validate all inputs cho security

## Future Enhancements

### Planned Features
- **API Interface**: REST API cho programmatic access
- **Web UI**: Web-based user interface
- **Plugin System**: Extensible plugin architecture
- **Advanced Analytics**: Enhanced data analysis capabilities
- **ML Integration**: Machine learning cho pattern detection

### Scalability Improvements
- **Distributed Processing**: Support cho distributed scanning
- **Database Backend**: Optional database storage
- **Cloud Integration**: Cloud platform integration
- **Container Support**: Docker và Kubernetes support
- **Horizontal Scaling**: Scale across multiple instances

### Integration Enhancements
- **SIEM Integration**: Security Information và Event Management
- **Threat Intelligence**: Integration với threat intelligence feeds
- **Vulnerability Scanning**: Integration với vulnerability scanners
- **Compliance Reporting**: Compliance và audit reporting
- **Automation Tools**: Integration với automation platforms

## Package Documentation

### cmd/monsterinc Package

Package chứa main entry point cho MonsterInc application.

#### Overview

`cmd/monsterinc` package provides command-line interface và orchestrates execution của different operational modes.

#### Files

**main.go**: Main application file handling:
- Command-line argument parsing với comprehensive flag support
- Configuration loading và validation với error handling
- Mode selection và execution với proper lifecycle management
- Graceful shutdown handling với context cancellation
- Global initialization của services với dependency injection

#### Command-Line Arguments

**Required Arguments**
- `--mode <onetime|automated>`: Execution mode

**Optional Arguments**
- `--scan-targets, -st <path>`: Path đến file containing seed URLs
- `--monitor-targets, -mt <path>`: File containing URLs để monitor
- `--globalconfig, -gc <path>`: Path đến configuration file

#### Execution Modes

**Onetime Mode**: Single scan cycle:
1. Load targets từ file hoặc configuration
2. Execute complete scan workflow
3. Generate reports và send notifications
4. Exit gracefully

**Automated Mode**: Continuous operation:
1. Initialize scheduler và monitoring services
2. Execute scan cycles at configured intervals
3. Handle retries cho failed scans
4. Maintain scan history trong SQLite database
5. Monitor files cho real-time changes

### internal/common Package

Package chứa shared utilities và patterns được sử dụng throughout application.

#### Key Modules

**constructor_patterns.go**: Standardized service construction
**context_utils.go**: Context management với cancellation
**errors.go**: Custom error types và handling
**file_utils.go**: Safe file operations
**http_client.go**: HTTP client factory
**service_lifecycle.go**: Service lifecycle management
**workflow_patterns.go**: Workflow execution patterns

### internal/config Package

Handles configuration management cho MonsterInc application.

#### Features

- YAML và JSON format support
- Hot-reload với file watching
- Comprehensive validation
- Default value management
- Environment variable override support

### internal/datastore Package

Provides data storage và persistence functionality sử dụng Apache Parquet.

#### Key Features

- **Parquet Storage**: Columnar format với compression
- **Schema Evolution**: Support cho changes over time
- **Efficient Querying**: Fast lookups với metadata
- **Data Partitioning**: Organized by hostname
- **Deduplication**: Prevent duplicate records

### internal/monitor Package

Implements file monitoring functionality với real-time change detection.

#### Features

- **Conditional Requests**: ETag và Last-Modified support
- **Content Hashing**: Efficient change detection
- **Aggregated Notifications**: Reduce notification spam
- **File Type Filtering**: Monitor specific file types
- **Diff Generation**: Detailed change reports

### internal/secrets Package

Implements secret detection functionality.

#### Features

- **TruffleHog Integration**: Comprehensive secret detection
- **Custom Regex Patterns**: Mantra-style patterns
- **Confidence Scoring**: Reliability assessment
- **Verification**: Attempt to verify findings
- **Parquet Storage**: Efficient findings storage

### internal/extractor Package

Implements path extraction từ web content.

#### Features

- **jsluice Integration**: JavaScript analysis
- **Custom Regex Patterns**: Additional detection methods
- **API Endpoint Discovery**: Identify potential endpoints
- **URL Resolution**: Convert relative đến absolute URLs
- **Context Preservation**: Maintain discovery context 