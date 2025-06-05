# Monitor Package

The monitor package provides a comprehensive file monitoring system for tracking changes in HTML and JavaScript files. It offers a modular architecture with separate components for different monitoring responsibilities.

## Overview

The monitoring service continuously watches specified URLs for content changes, generates diffs when changes are detected, extracts paths from JavaScript files, and sends notifications about file modifications.

## Architecture

The package follows a modular design with the following components:

### Core Components

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  MonitoringService  │ ──► │   URLManager     │ ──► │  CycleTracker   │
│  (Orchestrator)     │    │  (URL Management)│    │ (Cycle Tracking)│
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         ▼                        ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  EventAggregator│    │   URLChecker     │    │ URLMutexManager │
│  (Events/Notif) │    │ (Content Check)  │    │ (Concurrency)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │
         ▼                        ▼
┌─────────────────┐    ┌──────────────────┐
│ContentProcessor │    │  External Deps   │
│ (Hash/Process)  │    │ (Fetcher/Differ) │
└─────────────────┘    └──────────────────┘
```

## File Structure

### `service.go`
**Main monitoring service orchestrator**
- Coordinates all monitoring operations
- Manages service lifecycle
- Integrates all components
- Provides main API interface

### `url_manager.go`
**URL and cycle management**

#### Components:
- **`URLManager`**: Manages the list of monitored URLs
- **`CycleTracker`**: Tracks changes within monitoring cycles
- **`URLMutexManager`**: Prevents concurrent processing of same URL

### `url_checker.go`
**Individual URL checking logic**
- Fetches content from URLs
- Detects changes by comparing hashes
- Generates content diffs
- Extracts paths from JavaScript files
- Stores history records

### `content_processor.go`
**Content processing utilities**
- Calculates SHA256 hashes
- Processes fetched content
- Creates monitored file update records

### `event_aggregator.go`
**Event aggregation and notifications**
- Aggregates file change events
- Aggregates fetch error events
- Sends periodic notifications
- Handles immediate notifications for critical events

## Usage

### Basic Setup

```go
// Initialize monitoring service
monitoringService, err := monitor.NewMonitoringService(
    globalConfig,
    logger,
    notificationHelper,
)
if err != nil {
    log.Fatal("Failed to initialize monitoring service:", err)
}

// Preload URLs to monitor
monitoringService.Preload([]string{
    "https://example.com/app.js",
    "https://example.com/style.css",
})

// Add individual URLs
monitoringService.AddMonitorUrl("https://example.com/new-file.js")
```

### Checking URLs

```go
// Check a single URL for changes
monitoringService.CheckURL("https://example.com/app.js")

// Trigger end-of-cycle report
monitoringService.TriggerCycleEndReport()
```

### Service Lifecycle

```go
// Set parent context
monitoringService.SetParentContext(parentCtx)

// Generate new cycle ID
cycleID := monitoringService.GenerateNewCycleID()

// Stop the service
monitoringService.Stop()
```

## Configuration

The monitoring service is configured through the global configuration:

```yaml
monitor_config:
  enabled: true
  check_interval_seconds: 300
  aggregation_interval_seconds: 600
  max_aggregated_events: 50
  max_concurrent_checks: 10
  http_timeout_seconds: 30
  max_content_size: 10485760  # 10MB
  store_full_content_on_change: true
  monitor_insecure_skip_verify: false
```

### Key Configuration Options

- **`enabled`**: Enable/disable monitoring
- **`check_interval_seconds`**: How often to check URLs
- **`aggregation_interval_seconds`**: How often to send aggregated notifications
- **`max_aggregated_events`**: Maximum events before forcing immediate notification
- **`max_concurrent_checks`**: Maximum concurrent URL checks
- **`store_full_content_on_change`**: Whether to store full content when changes detected

## Features

### 1. Change Detection
- **Hash-based comparison**: Uses SHA256 hashes to detect content changes
- **New file detection**: Identifies when URLs are monitored for the first time
- **Content diffing**: Generates detailed diffs when changes are detected

### 2. Path Extraction
- **JavaScript analysis**: Extracts paths and URLs from JavaScript files
- **Content type detection**: Automatically identifies JavaScript content
- **Regex and JSluice support**: Multiple extraction methods

### 3. Reporting
- **Single diff reports**: Individual HTML reports for each change
- **Aggregated reports**: Combined reports for multiple changes in a cycle
- **Asset embedding**: Self-contained HTML reports

### 4. Notifications
- **Event aggregation**: Batches events to reduce notification spam
- **Discord integration**: Sends notifications to Discord webhooks
- **Error reporting**: Separate notifications for fetch/processing errors

### 5. Concurrency Control
- **Per-URL mutexes**: Prevents concurrent processing of same URL
- **Configurable limits**: Control maximum concurrent operations
- **Resource cleanup**: Automatic cleanup of unused resources

## Data Flow

```
1. URL Added to Monitor List
   │
   ▼
2. URL Queued for Checking
   │
   ▼
3. Content Fetched from URL
   │
   ▼
4. Content Processed (Hashed)
   │
   ▼
5. Changes Detected (Compare with History)
   │
   ▼
6. Diff Generated (if changed)
   │
   ▼
7. Paths Extracted (if JavaScript)
   │
   ▼
8. Record Stored in History
   │
   ▼
9. Events Aggregated
   │
   ▼
10. Notifications Sent
```

## Error Handling

The monitoring service handles various error scenarios:

- **Network errors**: Timeout, connection refused, DNS failures
- **Content errors**: Invalid content, size limits exceeded
- **Processing errors**: Hash calculation, diff generation failures
- **Storage errors**: History store failures

All errors are:
1. Logged with appropriate detail
2. Converted to `MonitorFetchErrorInfo` objects
3. Aggregated and reported via notifications
4. Do not stop the monitoring of other URLs

## Extension Points

### Custom Content Processors
```go
// Implement custom processing logic
type CustomProcessor struct {
    logger zerolog.Logger
}

func (cp *CustomProcessor) ProcessContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error) {
    // Custom processing logic
    return &models.MonitoredFileUpdate{...}, nil
}
```

### Custom Path Extractors
```go
// Add custom path extraction logic
pathExtractor, err := extractor.NewPathExtractor(config.ExtractorConfig{
    CustomRegexes: []string{
        `custom-pattern-here`,
    },
}, logger)
```

## Dependencies

### Internal Dependencies
- `internal/common`: HTTP client and utilities
- `internal/config`: Configuration management
- `internal/datastore`: History storage (Parquet files)
- `internal/differ`: Content diffing
- `internal/extractor`: Path extraction
- `internal/models`: Data models
- `internal/notifier`: Notification system
- `internal/reporter`: Report generation

### External Dependencies
- `github.com/rs/zerolog`: Structured logging
- Content diffing libraries
- HTTP client libraries

## Performance Considerations

### Memory Usage
- Content is processed in memory
- Full content storage is optional
- Diff results can be large for big files

### Disk Usage
- History stored in compressed Parquet files
- Report files generated on disk
- Configurable cleanup policies

### Network Usage
- Periodic fetching of monitored URLs
- Configurable intervals and timeouts
- Respect for HTTP caching headers (ETag, Last-Modified)

## Best Practices

1. **URL Selection**: Monitor only essential files to reduce resource usage
2. **Interval Configuration**: Balance between responsiveness and resource usage
3. **Content Size Limits**: Set appropriate limits to prevent memory issues
4. **Error Monitoring**: Monitor aggregated error notifications for issues
5. **Regular Cleanup**: Implement periodic cleanup of old reports and history

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Reduce `max_content_size`
   - Disable `store_full_content_on_change`
   - Reduce `max_concurrent_checks`

2. **Network Timeouts**
   - Increase `http_timeout_seconds`
   - Check network connectivity
   - Verify URLs are accessible

3. **Missing Notifications**
   - Check Discord webhook configuration
   - Verify `aggregation_interval_seconds` setting
   - Check notification helper setup

4. **Storage Issues**
   - Verify write permissions for storage path
   - Check available disk space
   - Review Parquet file configuration

### Debug Mode

Enable debug logging to see detailed monitoring operations:

```go
logger := logger.New(config.LogConfig{
    LogLevel: "debug",
})
```

## Migration from Legacy Code

If migrating from the old monolithic service:

1. Replace `NewProcessor` with `NewContentProcessor`
2. Use new modular service constructor
3. Update configuration references
4. Review error handling patterns
5. Test notification integrations

The new architecture maintains API compatibility while providing better modularity and testability. 