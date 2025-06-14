# Monitor Package

The monitor package provides continuous monitoring capabilities for MonsterInc's security analysis pipeline. It automatically tracks changes to web resources, detects content modifications, and triggers security analysis workflows when changes occur, enabling real-time threat detection and change tracking.

## Package Role in MonsterInc
As the continuous monitoring engine, this package:
- **Real-time Security**: Provides continuous monitoring for security threats
- **Change Detection**: Identifies modifications in web content and structure
- **Alert Generation**: Triggers notifications for security-relevant changes
- **Integration Hub**: Works with Differ, Reporter, and Notifier for complete workflows
- **Threat Intelligence**: Builds historical data for security trend analysis

## Overview

The monitor package enables:
- **Continuous Monitoring**: Automated checking of URLs at configurable intervals
- **Change Detection**: Content comparison using cryptographic hashing
- **Batched Processing**: Efficient processing of large URL sets using batch workflow
- **Content Diffing**: Detailed analysis of what changed between versions
- **Path Extraction**: URL and endpoint discovery from JavaScript files
- **Report Generation**: Visual HTML diff reports for change analysis
- **Immediate Interrupt Response**: Context-aware cancellation across all monitoring operations

**Interrupt Handling Features:**
- **Context propagation** - cancellation signals immediately stop all URL checking
- **Safe operation termination** - in-progress HTTP requests are cancelled within timeout
- **Batch processing shutdown** - graceful termination of batch operations
- **Resource cleanup** - proper cleanup of active connections and temporary data

## Architecture

```
┌─────────────────────┐    ┌──────────────────────┐
│ MonitoringService   │ ──► │ BatchURLManager      │
│  (Main Service)     │    │ (Batch Processing)   │
└─────────────────────┘    └──────────────────────┘
         │                          │
         ▼                          ▼
┌─────────────────────┐    ┌──────────────────────┐
│    URLChecker       │    │   CycleTracker       │
│ (Change Detection)  │    │ (Cycle Management)   │
└─────────────────────┘    └──────────────────────┘
         │                          │
         ▼                          ▼
┌─────────────────────┐    ┌──────────────────────┐
│ ContentProcessor    │    │  URLManager          │
│ (Content Analysis)  │    │ (URL Collection)     │
└─────────────────────┘    └──────────────────────┘
```

## File Structure

### Core Components

- **`service.go`** - Main monitoring service and orchestration
- **`batch_url_manager.go`** - Batched URL processing management
- **`url_checker.go`** - Individual URL change detection logic
- **`content_processor.go`** - Content processing and hashing
- **`cycle_tracker.go`** - Monitoring cycle state management

### Supporting Components

- **`url_manager.go`** - URL collection and validation
- **`url_mutex_manager.go`** - Thread-safe URL access coordination

## Features

### 1. Continuous URL Monitoring

**Capabilities:**
- Configurable check intervals (seconds to hours)
- Concurrent URL checking with limits
- HTTP timeout and retry handling
- Content size limits for efficiency
- TLS verification options

### 2. Change Detection

**Detection Methods:**
- SHA-256 content hashing for accuracy
- ETag and Last-Modified header support
- Content-type specific processing
- Path extraction from JavaScript files
- Historical comparison with previous versions

### 3. Batch Processing

**Features:**
- Intelligent batching for large URL sets
- Configurable batch sizes and concurrency
- Progress tracking across batches
- Memory-efficient processing
- Graceful shutdown handling

## Usage Examples

### Basic Monitoring Setup

```go
import (
    "github.com/aleister1102/monsterinc/internal/monitor"
    "github.com/aleister1102/monsterinc/internal/config"
    "context"
    "os"
    "os/signal"
    "syscall"
)

// Initialize monitoring service
monitoringService, err := monitor.NewMonitoringService(
    globalConfig,
    logger,
    notificationHelper,
)
if err != nil {
    return fmt.Errorf("monitoring initialization failed: %w", err)
}

// Setup context with cancellation for interrupt handling
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Handle interrupt signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigChan
    logger.Info().Msg("Interrupt signal received, stopping monitoring service...")
    cancel() // This immediately stops all monitoring operations
    monitoringService.Stop() // Additional cleanup
}()

// Set parent context for cancellation propagation
monitoringService.SetParentContext(ctx)

// Add URLs to monitor
monitoringService.AddMonitorUrl("https://example.com/app.js")
monitoringService.AddMonitorUrl("https://example.com/config.json")

// Monitor will respect context cancellation and stop immediately when interrupted
```

### Loading URLs from File

```go
// Load URLs from file source
err := monitoringService.LoadAndMonitorFromSources("monitor-targets.txt")
if err != nil {
    logger.Error().Err(err).Msg("Failed to load monitor URLs")
}

// Preload initial URL set
initialURLs := []string{
    "https://api.example.com/endpoints.json",
    "https://example.com/static/main.js",
}
monitoringService.Preload(initialURLs)
```

### Manual URL Checking

```go
// Check specific URL immediately
monitoringService.CheckURL("https://example.com/critical-config.js")

// Get monitoring statistics
stats := monitoringService.GetMonitoringStats()
fmt.Printf("Total monitored: %v\n", stats["total_monitored"])
fmt.Printf("Changes detected: %v\n", stats["changes_detected"])

// Get currently monitored URLs
urls := monitoringService.GetCurrentlyMonitorUrls()
```

### Cycle Management

```go
// Generate new monitoring cycle
cycleID := monitoringService.GenerateNewCycleID()
monitoringService.SetCurrentCycleID(cycleID)

// Trigger cycle completion report
monitoringService.TriggerCycleEndReport()
```

## Configuration

### Monitor Configuration

```yaml
monitor_config:
  enabled: true                          # Enable monitoring service
  check_interval_seconds: 300            # URL check frequency (5 minutes)
  aggregation_interval_seconds: 1800     # Notification batching interval (30 minutes)
  max_concurrent_checks: 10              # Maximum parallel URL checks
  http_timeout_seconds: 30               # HTTP request timeout
  max_content_size: 10485760             # Maximum content size (10MB)
  monitor_insecure_skip_verify: false    # Skip TLS verification
  store_full_content_on_change: true     # Store complete content on changes
  max_aggregated_events: 100             # Maximum events before forced send
  initial_monitor_urls: []               # URLs to monitor at startup
  html_file_extensions:                  # HTML file extensions
    - ".html"
    - ".htm"
    - ".xhtml"
  js_file_extensions:                    # JavaScript file extensions
    - ".js"
    - ".mjs"
    - ".jsx"
```

### Configuration Options

- **`check_interval_seconds`**: How frequently to check URLs for changes
- **`aggregation_interval_seconds`**: Notification batching interval
- **`max_concurrent_checks`**: Concurrent URL checking limit
- **`max_content_size`**: Maximum file size to monitor (bytes)
- **`store_full_content_on_change`**: Whether to store complete content in history

## Change Detection Process

### 1. Content Fetching

```go
// Fetch URL content with proper context handling
fetchResult, err := urlChecker.fetchURLContentWithContext(ctx, url)
if err != nil {
    return urlChecker.createErrorResult(url, cycleID, "fetch", err)
}
```

### 2. Content Processing

```go
// Process and hash content
processedUpdate, err := urlChecker.processURLContent(url, fetchResult)
if err != nil {
    return urlChecker.createErrorResult(url, cycleID, "process", err)
}
```

### 3. Historical Comparison

```go
// Compare with last known state
lastRecord, err := urlChecker.getLastKnownRecord(url)
if urlChecker.hasContentChanged(lastRecord, processedUpdate) {
    // Generate detailed diff
    changeInfo, diffResult, err := urlChecker.detectURLChanges(
        url, processedUpdate, fetchResult)
}
```

### 4. Report Generation

```go
// Create individual diff report
reportPath := urlChecker.generateSingleDiffReport(
    url, diffResult, lastRecord, processedUpdate, fetchResult)

// Extract paths from JavaScript content
extractedPaths := urlChecker.extractPathsIfJavaScript(url, fetchResult)
```

## Event Types

### File Change Events

```go
type FileChangeInfo struct {
    URL            string
    OldHash        string
    NewHash        string
    ContentType    string
    ChangeTime     time.Time
    DiffReportPath *string
    ExtractedPaths []ExtractedPath
    CycleID        string
}
```

### Error Events

```go
type MonitorFetchErrorInfo struct {
    URL        string
    Error      string
    Source     string    // "fetch", "process", "store"
    OccurredAt time.Time
    CycleID    string
}
```

## Integration Examples

### With Scanner Service

```go
// Monitor URLs discovered during scans
scanner.OnScanComplete(func(results []models.ProbeResult) {
    for _, result := range results {
        if shouldMonitor(result) {
            monitoringService.AddMonitorUrl(result.InputURL)
        }
    }
})
```

### With Notification System

```go
// Automatic change notifications
monitor.OnChangesDetected(func(changes []models.FileChangeInfo) {
    // Generate aggregated report
    reportPath := diffReporter.GenerateAggregatedDiffReport(changes)
    
    // Send notification with attachments
    notifier.SendFileChangesNotification(ctx, changes, reportPath)
})
```

### With Scheduler

```go
// Scheduled monitoring cycles
scheduler.ScheduleMonitorTask(scheduler.MonitorTaskDefinition{
    Name:     "continuous-monitoring",
    Interval: 4 * time.Hour,
    Config: scheduler.MonitorTaskConfig{
        CheckInterval: 300,
        MaxChecks:     200,
    },
})
```

## Performance Optimization

### Concurrency Control

```go
// Configurable concurrency limits
func (s *MonitoringService) CheckURL(url string) {
    if !s.acquireURLMutex() {
        return // Skip if at capacity
    }
    defer s.releaseURLMutex(url)
    
    result := s.performURLCheck(url)
    s.handleCheckResult(url, result)
}
```

### Memory Management

```go
// Efficient content processing
func (cp *ContentProcessor) ProcessContent(url string, content []byte, contentType string) {
    // Stream processing for large content
    if len(content) > maxSizeForMemory {
        return cp.processLargeContent(url, content, contentType)
    }
    return cp.processSmallContent(url, content, contentType)
}
```

### Network Optimization

```go
// HTTP optimization with conditional requests
type FetchFileContentInput struct {
    URL                  string
    PreviousETag         string    // For conditional requests
    PreviousLastModified string    // For If-Modified-Since
    Context              context.Context
}
```

## Error Handling

### Retry Logic

```go
// Automatic retry for transient errors
func (uc *URLChecker) fetchWithRetry(ctx context.Context, url string) (*common.FetchFileContentResult, error) {
    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        result, err := uc.fetcher.FetchFileContent(fetchInput)
        if err == nil {
            return result, nil
        }
        
        if !isRetryableError(err) {
            return nil, err
        }
        
        lastErr = err
        time.Sleep(retryDelay * time.Duration(attempt+1))
    }
    return nil, lastErr
}
```

### Error Aggregation

```go
// Batch error notifications
func (ea *EventAggregator) AddFetchErrorEvent(errorInfo models.MonitorFetchErrorInfo) {
    if !ea.shouldAcceptEvent() {
        return
    }
    
    ea.fetchErrorsMutex.Lock()
    ea.fetchErrors = append(ea.fetchErrors, errorInfo)
    ea.fetchErrorsMutex.Unlock()
}
```

## Thread Safety

- All public methods are thread-safe
- URL-level mutexes prevent concurrent checks of the same resource
- Event aggregation uses proper synchronization
- Context-based cancellation for graceful shutdown
- Atomic operations for counters and state

## Dependencies

- **github.com/aleister1102/monsterinc/internal/common** - HTTP client and utilities
- **github.com/aleister1102/monsterinc/internal/datastore** - File history persistence
- **github.com/aleister1102/monsterinc/internal/differ** - Content diffing capabilities
- **github.com/aleister1102/monsterinc/internal/extractor** - Path extraction from content
- **github.com/aleister1102/monsterinc/internal/reporter** - HTML diff report generation
- **github.com/aleister1102/monsterinc/internal/notifier** - Discord notifications
- **github.com/aleister1102/monsterinc/internal/urlhandler** - URL management

## Best Practices

### Monitoring Strategy
- Monitor critical configuration files and API endpoints
- Include JavaScript files for security analysis
- Set appropriate check intervals based on change frequency
- Use content size limits to avoid performance issues

### Error Management
- Monitor error rates and adjust timeouts accordingly
- Review aggregated error reports for patterns
- Implement proper alerting for persistent failures
- Handle network connectivity issues gracefully

### Performance Tuning
- Adjust concurrent check limits based on system resources
- Use appropriate aggregation intervals to balance responsiveness and efficiency
- Monitor memory usage during large file processing
- Implement proper cleanup for old monitoring data
