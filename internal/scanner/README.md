# Scanner Package

The scanner package is the main orchestration service for MonsterInc's security analysis pipeline. It coordinates web crawling, HTTP probing, endpoint discovery, content diffing, and report generation into a unified workflow for comprehensive web security assessment and vulnerability discovery.

## Package Role in MonsterInc
As the central orchestrator, this package:
- **Workflow Coordination**: Manages the complete security scanning pipeline
- **Component Integration**: Seamlessly coordinates Crawler, HTTPx Runner, Extractor, and Differ
- **Security Analysis**: Provides comprehensive web application security assessment
- **Report Generation**: Produces detailed security reports via Reporter integration
- **Data Management**: Coordinates data flow between components and Datastore

## Overview

The scanner package provides:
- **Workflow Orchestration**: Manages the complete scanning pipeline
- **Component Integration**: Coordinates crawler, httpx, differ, and reporter
- **Configuration Management**: Centralized configuration for all components
- **Report Generation**: Produces HTML reports with scan results and diffs
- **Error Handling**: Comprehensive error recovery and reporting
- **Interrupt Management**: Immediate response to cancellation signals across all components

**Interrupt Handling Features:**
- **Context propagation** - cancellation signals are immediately passed to all active components
- **Graceful termination** - allows components to complete critical operations within timeout
- **Resource cleanup** - ensures proper cleanup of temporary files and connections
- **Progress preservation** - maintains scan state for partial results recovery

## File Structure

### Core Components

- **`scanner.go`** - Main scanner service and public API
- **`workflow_orchestrator.go`** - Pipeline coordination and execution
- **`workflow_input.go`** - Input data structures and validation
- **`config_builder.go`** - Component configuration assembly
- **`summary_builder.go`** - Scan statistics and summary generation

### Executors

- **`crawler_executor.go`** - Web crawling execution and integration
- **`httpx_executor.go`** - HTTP probing execution and integration
- **`diff_storage_processor.go`** - Content diffing and storage processing
- **`report_generator.go`** - HTML report generation coordination

## Features

### 1. Complete Scanning Pipeline

**Workflow Steps:**
1. **Target Loading**: Parse and normalize input URLs
2. **Web Crawling**: Discover additional URLs and assets
3. **HTTP Probing**: Test all discovered URLs for responses
4. **Content Diffing**: Compare with historical data
5. **Report Generation**: Create interactive HTML reports
6. **Notification**: Send results via Discord

### 2. Multi-Mode Operations

**Scan Modes:**
- **One-time Scan**: Single execution with immediate results
- **Automated Scan**: Scheduled recurring scans
- **Target Discovery**: Focus on URL discovery without probing
- **Diff Analysis**: Compare current results with historical data

### 3. Flexible Configuration

**Configuration Sources:**
- YAML/JSON configuration files
- Command-line parameters
- Environment variables
- Runtime configuration updates

## Usage Examples

### Basic Scanner Setup

```go
import (
    "github.com/aleister1102/monsterinc/internal/scanner"
    "github.com/aleister1102/monsterinc/internal/config"
    "context"
    "os"
    "os/signal"
    "syscall"
)

// Initialize scanner
scannerService := scanner.NewScanner(globalConfig, logger)

// Setup interrupt handling
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Handle interrupt signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigChan
    logger.Info().Msg("Interrupt signal received, cancelling scan...")
    cancel() // This immediately stops all scanner components
}()

// Prepare scan input
input := scanner.WorkflowInput{
    TargetsFile:   "targets.txt",
    ScanMode:      "onetime",
    SessionID:     "20240101-120000",
    RootTargets:   []string{"https://example.com"},
}

// Execute scan with context cancellation
summary, err := scannerService.ExecuteWorkflow(ctx, input)
if err != nil {
    if ctx.Err() == context.Canceled {
        logger.Info().Msg("Scan was interrupted by user")
        return nil // Graceful exit
    }
    return fmt.Errorf("scan failed: %w", err)
}

fmt.Printf("Scan completed: %d targets, %d results\n", 
    summary.TotalTargets, summary.ProbeStats.TotalProbed)
```

### Advanced Workflow Configuration

```go
// Create workflow with custom configuration
workflowInput := scanner.WorkflowInput{
    TargetsFile:           "targets.txt",
    ScanMode:              "automated",
    SessionID:             generateSessionID(),
    EnableCrawling:        true,
    EnableHTTPProbing:     true,
    EnableDiffing:         true,
    EnableReportGeneration: true,
    
    // Component-specific options
    CrawlerOptions: scanner.CrawlerExecutorOptions{
        MaxDepth:       3,
        IncludeAssets:  true,
        RespectRobots:  true,
    },
    
    HTTPxOptions: scanner.HTTPxExecutorOptions{
        Threads:        50,
        Timeout:        30,
        FollowRedirects: true,
    },
    
    DiffOptions: scanner.DiffProcessorOptions{
        EnableContentDiff: true,
        LookbackDays:     30,
    },
}

summary, err := scannerService.ExecuteWorkflow(ctx, workflowInput)
```

### Component Integration

```go
// Access individual components
crawler := scannerService.GetCrawler()
httpxRunner := scannerService.GetHTTPxRunner()
differ := scannerService.GetDiffer()
reporter := scannerService.GetReporter()

// Execute individual phases
crawlResults := crawler.CrawlTargets(ctx, targets)
probeResults := httpxRunner.ProbeURLs(ctx, crawlResults.URLs)
diffResults := differ.CompareResults(ctx, probeResults, previousResults)
reportPath := reporter.GenerateReport(ctx, probeResults, diffResults)
```

## Configuration

### Scanner Configuration

```yaml
# Global scanning configuration
scanner_config:
  default_scan_mode: "onetime"
  enable_crawling: true
  enable_http_probing: true
  enable_diffing: true
  enable_report_generation: true
  max_concurrent_targets: 10
  scan_timeout_minutes: 60
  
# Component configurations
crawler_config:
  max_depth: 2
  max_concurrent_requests: 20
  request_timeout_secs: 30
  
httpx_runner_config:
  threads: 50
  timeout_secs: 30
  retries: 3
  
reporter_config:
  output_dir: "./reports"
  embed_assets: true
  enable_data_tables: true

# Logging Configuration
log_config:
  log_level: "info"
  log_format: "json"
  log_file: "./logs/monsterinc.log"
  max_log_size_mb: 100
  max_log_backups: 5

# Diff Reporter Configuration
diff_reporter_config:
  max_diff_file_size_mb: 10

# Resource Limiter Configuration
resource_limiter_config:
  max_memory_mb: 512        # Memory limit (reduced for earlier detection)
  memory_threshold: 0.7     # 70% threshold
  check_interval_secs: 15   # Check every 15 seconds
  enable_auto_shutdown: true
```

## Workflow Orchestration

### Memory-Efficient Batch Processing

To address memory issues with large target lists, the scanner implements automatic batch processing:

#### Automatic Batching Triggers
- **Threshold**: When input has more than 500 targets (configurable)
- **Batch Size**: 100 targets per batch (configurable)
- **Processing**: Sequential batch processing (no concurrent batches to avoid memory spikes)

#### Memory Optimizations
- **Auto-Tuning**: Automatically reduces concurrent threads when batching is active
  - Crawler concurrent requests: Limited to 10 max
  - HTTPx threads: Limited to 30 max
- **Garbage Collection**: Forced GC after each batch
- **Memory Monitoring**: Real-time memory usage logging

#### Configuration
```yaml
batch_processor_config:
  batch_size: 100           # Targets per batch
  max_concurrent_batch: 1   # Sequential processing
  batch_timeout_mins: 30    # Timeout per batch
  threshold_size: 500       # Minimum targets to trigger batching

resource_limiter_config:
  max_memory_mb: 512        # Memory limit (reduced for earlier detection)
  memory_threshold: 0.7     # 70% threshold
  check_interval_secs: 15   # Check every 15 seconds
  enable_auto_shutdown: true
```

#### Usage Example
```bash
# Large target file (>500 targets) will automatically use batch processing
./monsterinc -st large_targets.txt -cfg config.yaml
```

#### Batch Processing Features
- **Progress Tracking**: Real-time logging of batch progress
- **Interruption Handling**: Graceful handling of context cancellation
- **Result Aggregation**: Automatic aggregation of all batch results
- **Report Generation**: Consolidated reports from all batches
- **Memory Reporting**: Detailed memory usage before/after each batch

### Workflow Input Structure

```go
type WorkflowInput struct {
    // Basic scan parameters
    TargetsFile            string
    ScanMode               string
    SessionID              string
    RootTargets            []string
    
    // Component toggles
    EnableCrawling         bool
    EnableHTTPProbing      bool
    EnableDiffing          bool
    EnableReportGeneration bool
    
    // Advanced options
    CrawlerOptions         CrawlerExecutorOptions
    HTTPxOptions          HTTPxExecutorOptions
    DiffOptions           DiffProcessorOptions
    ReportOptions         ReportGeneratorOptions
    
    // Context and timing
    Context               context.Context
    Timeout               time.Duration
    MaxConcurrentTargets  int
}
```

### Workflow Execution Phases

#### Phase 1: Initialization and Validation
```go
// Input validation and configuration assembly
func (w *WorkflowOrchestrator) validateInput(input WorkflowInput) error {
    if input.SessionID == "" {
        return errors.New("session ID is required")
    }
    if len(input.RootTargets) == 0 && input.TargetsFile == "" {
        return errors.New("targets must be provided")
    }
    return nil
}
```

#### Phase 2: Target Discovery
```go
// Crawling phase (if enabled)
if input.EnableCrawling {
    crawlResults, err := w.crawlerExecutor.Execute(ctx, crawlInput)
    if err != nil {
        return nil, fmt.Errorf("crawling failed: %w", err)
    }
    allTargets = append(allTargets, crawlResults.DiscoveredURLs...)
}
```

#### Phase 3: HTTP Probing
```go
// HTTP probing phase
if input.EnableHTTPProbing {
    probeResults, err := w.httpxExecutor.Execute(ctx, httpxInput)
    if err != nil {
        return nil, fmt.Errorf("probing failed: %w", err)
    }
}
```

#### Phase 4: Content Analysis
```go
// Diffing and storage phase
if input.EnableDiffing {
    diffResults, err := w.diffProcessor.Process(ctx, diffInput)
    if err != nil {
        logger.Warn().Err(err).Msg("Diffing failed, continuing")
    }
}
```

#### Phase 5: Report Generation
```go
// Report generation phase
if input.EnableReportGeneration {
    reportPath, err := w.reportGenerator.Generate(ctx, reportInput)
    if err != nil {
        logger.Warn().Err(err).Msg("Report generation failed")
    }
}
```

## Component Executors

### 1. CrawlerExecutor

Manages web crawling operations:

```go
type CrawlerExecutor struct {
    config   config.CrawlerConfig
    logger   zerolog.Logger
    crawler  *crawler.Crawler
}

type CrawlerExecutorOptions struct {
    MaxDepth         int
    IncludeAssets    bool
    RespectRobots    bool
    MaxConcurrent    int
    TimeoutSeconds   int
    IncludeSubdomains bool
}

func (ce *CrawlerExecutor) Execute(ctx context.Context, input CrawlerInput) (*CrawlerResult, error) {
    // Configure and execute crawling
    crawler.SetTargets(input.Targets)
    crawler.Start(ctx)
    
    return &CrawlerResult{
        DiscoveredURLs: crawler.GetDiscoveredURLs(),
        Assets:         crawler.GetDiscoveredAssets(),
        Statistics:     crawler.GetStatistics(),
    }, nil
}
```

### 2. HTTPxExecutor

Handles HTTP probing execution:

```go
type HTTPxExecutor struct {
    config config.HttpxRunnerConfig
    logger zerolog.Logger
}

type HTTPxExecutorOptions struct {
    Threads         int
    Timeout         int
    Retries         int
    FollowRedirects bool
    CustomHeaders   map[string]string
}

func (he *HTTPxExecutor) Execute(ctx context.Context, input HTTPxInput) (*HTTPxResult, error) {
    // Configure and execute HTTP probing
    runner := httpxrunner.NewRunner(he.config, input.RootTarget, he.logger)
    
    err := runner.Run(ctx)
    if err != nil {
        return nil, fmt.Errorf("httpx execution failed: %w", err)
    }
    
    return &HTTPxResult{
        ProbeResults: runner.GetResults(),
        Statistics:   calculateHTTPxStats(runner.GetResults()),
    }, nil
}
```

### 3. DiffStorageProcessor

Manages content diffing and storage:

```go
type DiffStorageProcessor struct {
    urlDiffer     *differ.UrlDiffer
    parquetWriter *datastore.ParquetWriter
    logger        zerolog.Logger
}

func (dsp *DiffStorageProcessor) Process(ctx context.Context, input DiffInput) (*DiffResult, error) {
    // Store current results
    err := dsp.parquetWriter.Write(ctx, input.CurrentResults, input.SessionID, input.RootTarget)
    if err != nil {
        return nil, fmt.Errorf("storage failed: %w", err)
    }
    
    // Generate diffs
    diffResult, err := dsp.urlDiffer.Compare(input.CurrentResults, input.RootTarget)
    if err != nil {
        return nil, fmt.Errorf("diffing failed: %w", err)
    }
    
    return &DiffResult{
        URLDiff:    diffResult,
        StoragePath: dsp.getStoragePath(input.RootTarget),
    }, nil
}
```

## Error Handling and Recovery

### Graceful Degradation

```go
// Continue execution even if non-critical components fail
func (w *WorkflowOrchestrator) executeWithRecovery(ctx context.Context, input WorkflowInput) (*models.ScanSummaryData, error) {
    var criticalErrors []string
    var warnings []string
    
    // Critical: Target loading
    targets, err := w.loadTargets(input)
    if err != nil {
        return nil, fmt.Errorf("critical: target loading failed: %w", err)
    }
    
    // Non-critical: Crawling
    var crawlResults *CrawlerResult
    if input.EnableCrawling {
        crawlResults, err = w.crawlerExecutor.Execute(ctx, crawlInput)
        if err != nil {
            warnings = append(warnings, fmt.Sprintf("crawling failed: %v", err))
        }
    }
    
    // Critical: HTTP probing
    probeResults, err := w.httpxExecutor.Execute(ctx, httpxInput)
    if err != nil {
        criticalErrors = append(criticalErrors, fmt.Sprintf("probing failed: %v", err))
    }
    
    // Non-critical: Diffing
    if input.EnableDiffing {
        _, err = w.diffProcessor.Process(ctx, diffInput)
        if err != nil {
            warnings = append(warnings, fmt.Sprintf("diffing failed: %v", err))
        }
    }
    
    // Build summary with error information
    summary := w.buildSummary(probeResults, criticalErrors, warnings)
    return summary, nil
}
```

### Timeout and Cancellation

```go
// Handle context cancellation and timeouts
func (w *WorkflowOrchestrator) executeWithTimeout(ctx context.Context, input WorkflowInput) error {
    // Set timeout if not provided
    if input.Timeout == 0 {
        input.Timeout = 60 * time.Minute
    }
    
    // Create timeout context
    timeoutCtx, cancel := context.WithTimeout(ctx, input.Timeout)
    defer cancel()
    
    // Monitor for cancellation
    go func() {
        <-timeoutCtx.Done()
        if timeoutCtx.Err() == context.DeadlineExceeded {
            w.logger.Warn().Msg("Scan timeout exceeded, initiating graceful shutdown")
        }
    }()
    
    return w.execute(timeoutCtx, input)
}
```

## Performance Optimization

### Concurrent Target Processing

```go
// Process multiple targets concurrently
func (w *WorkflowOrchestrator) processTargetsConcurrently(ctx context.Context, targets []string, maxConcurrent int) {
    semaphore := make(chan struct{}, maxConcurrent)
    var wg sync.WaitGroup
    
    for _, target := range targets {
        semaphore <- struct{}{} // Acquire
        wg.Add(1)
        
        go func(target string) {
            defer func() {
                <-semaphore // Release
                wg.Done()
            }()
            
            w.processTarget(ctx, target)
        }(target)
    }
    
    wg.Wait()
}
```

### Memory Management

```go
// Efficient memory usage for large scans
func (w *WorkflowOrchestrator) processLargeResultSet(results []models.ProbeResult) {
    const batchSize = 1000
    
    // Process in batches to control memory usage
    for i := 0; i < len(results); i += batchSize {
        end := i + batchSize
        if end > len(results) {
            end = len(results)
        }
        
        batch := results[i:end]
        w.processBatch(batch)
        
        // Force garbage collection between batches
        runtime.GC()
    }
}
```

## Integration Examples

### With Scheduler

```go
// Integration with automated scheduling
scheduler.OnScanTrigger(func(targets []string) {
    input := scanner.WorkflowInput{
        RootTargets:   targets,
        ScanMode:      "automated",
        SessionID:     generateTimestampID(),
        EnableCrawling: true,
        EnableHTTPProbing: true,
        EnableDiffing: true,
    }
    
    summary, err := scannerService.ExecuteWorkflow(ctx, input)
    if err != nil {
        notifier.SendErrorNotification(ctx, err)
        return
    }
    
    notifier.SendScanCompleteNotification(ctx, summary)
})
```

### With Monitor Service

```go
// Triggered scans from monitoring
monitor.OnFileChange(func(changes []models.FileChangeInfo) {
    // Extract domains from changed files
    domains := extractDomainsFromChanges(changes)
    
    input := scanner.WorkflowInput{
        RootTargets:   domains,
        ScanMode:      "change-triggered",
        SessionID:     generateChangeTriggeredID(),
        EnableDiffing: true, // Focus on diffing for change analysis
    }
    
    scannerService.ExecuteWorkflow(ctx, input)
})
```

## Dependencies

- **github.com/aleister1102/monsterinc/internal/crawler** - Web crawling
- **github.com/aleister1102/monsterinc/internal/httpxrunner** - HTTP probing
- **github.com/aleister1102/monsterinc/internal/differ** - Content diffing
- **github.com/aleister1102/monsterinc/internal/reporter** - Report generation
- **github.com/aleister1102/monsterinc/internal/datastore** - Data persistence
- **github.com/aleister1102/monsterinc/internal/models** - Data models
- **github.com/aleister1102/monsterinc/internal/config** - Configuration

## Thread Safety

- All scanner operations are thread-safe
- Component executors support concurrent execution
- Shared resources are properly synchronized
- Context cancellation propagates to all components

## Best Practices

### Scanner Usage
- Always provide proper context with timeout
- Use meaningful session IDs for tracking
- Enable appropriate components based on scan goals
- Handle errors gracefully with fallback strategies

### Performance
- Limit concurrent targets based on system resources
- Use streaming for large result sets
- Monitor memory usage during large scans
- Implement proper cleanup for temporary resources

## URL Preprocessor

### URL Preprocessor Features
- **URL Normalization**: Normalize URLs theo cấu hình (strip fragments, tracking parameters)
- **Auto-Calibrate**: Detect và filter similar URL patterns để tránh quá tải
- **Deduplication**: Loại bỏ URLs trùng lặp
- **Batch Processing**: Xử lý URLs theo batch để tối ưu memory

### URL Preprocessor Configuration
```yaml
crawler_config:
  url_normalization:
    strip_fragments: true
    strip_tracking_params: true
    custom_strip_params: ["utm_source", "fbclid"]
  
  auto_calibrate:
    enabled: true
    max_similar_urls: 50
    ignore_parameters: ["page", "offset"]
    auto_detect_locales: true
    custom_locale_codes: ["vn", "sg"]
    enable_skip_logging: true
```

### URL Preprocessor Statistics
- `total_processed`: Tổng số URLs được xử lý
- `normalized`: Số URLs được normalize
- `skipped_by_pattern`: Số URLs bị skip bởi auto-calibrate
- `skipped_duplicate`: Số URLs trùng lặp
- `final_count`: Số URLs cuối cùng sau preprocessing

### URL Preprocessor Usage
```go
// Create scanner
scanner := NewScanner(globalConfig, logger, parquetReader, parquetWriter)

// Execute scan với URL preprocessing
results, diffs, err := scanner.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
```

### URL Preprocessor Configuration Options
```yaml
# Enable/disable các features
crawler_config:
  auto_calibrate:
    enabled: true
    max_similar_urls: 100
    
  url_normalization:
    strip_fragments: true
    strip_tracking_params: true
```

### URL Preprocessor Monitoring & Logging
- Progress tracking qua 5 steps (thêm preprocessing step)
- Detailed statistics về URL processing
- Pattern detection logging
- Error handling và graceful degradation

### URL Preprocessor Thread Safety
- Tất cả operations đều thread-safe
- Mutexes protect shared state
- Safe for concurrent access

## Integration

URL Preprocessor được tích hợp hoàn toàn vào Scanner workflow và sử dụng configuration từ CrawlerConfig. Không cần thay đổi gì ở caller code, chỉ cần cập nhật config nếu muốn customize behavior.