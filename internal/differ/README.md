# Differ Package

## Purpose
The `differ` package provides comprehensive content comparison and URL diffing capabilities for MonsterInc's monitoring and analysis pipeline. It detects and analyzes changes in web content, URL status, and provides detailed diff reports for security monitoring and change tracking.

## Package Role in MonsterInc
As the change detection engine, this package:
- **Scanner Integration**: Compares current scan results with historical data stored in Datastore
- **Monitor Integration**: Analyzes content changes detected by the monitoring service
- **Report Support**: Provides diff data for HTML report generation via Reporter
- **Security Analysis**: Identifies potentially malicious content changes
- **Historical Analysis**: Enables understanding of website evolution over time

## Main Components

### 1. Content Differ (`content_differ.go`)
#### Purpose
- Generate detailed content diffs between different versions
- Support various content types (HTML, JavaScript, CSS, text)
- Provide statistical analysis of changes
- Handle large content with size validation

#### API Usage

```go
// Create content differ
differ, err := differ.NewContentDiffer(logger, diffReporterConfig)
if err != nil {
    return err
}

// Generate diff between content versions
previousContent := []byte("old content")
currentContent := []byte("new content")

diffResult, err := differ.GenerateDiff(
    previousContent,
    currentContent,
    "text/html",
    "oldHash123",
    "newHash456",
)

if err != nil {
    return err
}

// Access diff results
fmt.Printf("Lines added: %d\n", diffResult.LinesAdded)
fmt.Printf("Lines deleted: %d\n", diffResult.LinesDeleted)
fmt.Printf("Is identical: %t\n", diffResult.IsIdentical)
```

#### Advanced Usage

```go
// Create differ with custom configuration
builder := differ.NewContentDifferBuilder().
    WithDiffReporterConfig(&config.DiffReporterConfig{
        MaxDiffFileSizeMB: 50,
    }).
    WithDiffConfig(differ.DiffConfig{
        EnableSemanticCleanup: true,
        EnableLineBasedDiff:   true,
        ContextLines:          3,
    })

differ, err := builder.Build()
```

### 2. URL Differ (`url_differ.go`)
#### Purpose
- Compare current scan results with historical data from Datastore
- Identify new, existing, and removed URLs
- Track URL status changes over time
- Provide comprehensive URL analysis for Scanner workflow

#### API Usage

```go
// Create URL differ
urlDiffer, err := differ.NewUrlDiffer(parquetReader, logger)
if err != nil {
    return err
}

// Compare current scan with historical data
currentProbes := []*models.ProbeResult{
    // ... current scan results
}

diffResult, err := urlDiffer.Compare(currentProbes, "https://example.com", "scan-20240101-120000")
if err != nil {
    return err
}

// Access URL diff results
fmt.Printf("New URLs: %d\n", diffResult.New)
fmt.Printf("Existing URLs: %d\n", diffResult.Existing)
fmt.Printf("Old URLs: %d\n", diffResult.Old)

for _, diffedURL := range diffResult.Results {
    fmt.Printf("URL: %s, Status: %s\n", 
        diffedURL.ProbeResult.InputURL, 
        diffedURL.ProbeResult.URLStatus)
}
```

#### Builder Pattern

```go
// Create URL differ with builder
builder := differ.NewUrlDifferBuilder(logger).
    WithParquetReader(parquetReader).
    WithConfig(differ.URLComparerConfig{
        EnableURLNormalization: true,
        CaseSensitive:         false,
    })

urlDiffer, err := builder.Build()
```

### 3. Diff Processor (`diff_processor.go`)
#### Purpose
- Core diff processing using google/diff-match-patch
- Semantic cleanup of diffs
- Line-based diff analysis
- Statistics calculation

#### API Usage

```go
// Create diff processor
processor := differ.NewDiffProcessor(differ.DiffConfig{
    EnableSemanticCleanup: true,
    EnableLineBasedDiff:   true,
    ContextLines:          3,
})

// Process diff between texts
diffs := processor.ProcessDiff("old text", "new text")

// Calculate statistics
calculator := differ.NewDiffStatsCalculator()
stats := calculator.CalculateStats(diffs, "oldHash", "newHash")

fmt.Printf("Lines added: %d\n", stats.LinesAdded)
fmt.Printf("Lines deleted: %d\n", stats.LinesDeleted)
fmt.Printf("Is identical: %t\n", stats.IsIdentical)
```

## Integration with MonsterInc Components

### With Scanner Service
```go
// Scanner uses differ to compare current vs historical results
diffProcessor := scanner.GetDiffProcessor()
urlDiffResult, err := diffProcessor.CompareWithHistorical(
    ctx, currentProbeResults, rootTargetURL, scanSessionID)

// Results are used for generating diff reports
if urlDiffResult.New > 0 || urlDiffResult.Old > 0 {
    logger.Info().
        Int("new_urls", urlDiffResult.New).
        Int("old_urls", urlDiffResult.Old).
        Msg("URL changes detected")
}
```

### With Monitor Service
```go
// Monitor uses content differ for file change analysis
contentDiffer := monitor.GetContentDiffer()
if lastRecord != nil {
    diffResult, err := contentDiffer.GenerateDiff(
        lastRecord.Content,
        currentContent,
        contentType,
        lastRecord.Hash,
        currentHash,
    )
    
    if !diffResult.IsIdentical {
        // Generate diff report and notify
        reportPath := generateDiffReport(url, diffResult)
        notifier.SendChangeNotification(ctx, changeInfo, reportPath)
    }
}
```

### With Datastore
```go
// Differ relies on datastore for historical data
historicalProbes, _, err := parquetReader.FindAllProbeResultsForTarget(rootTarget)
if err != nil {
    return nil, fmt.Errorf("failed to load historical data: %w", err)
}

// Filter out current scan session to avoid self-comparison
filteredProbes := filterOutCurrentSession(historicalProbes, currentScanSessionID)
```

## Configuration

### Diff Configuration

```yaml
diff_config:
  previous_scan_lookback_days: 30  # How far back to look for historical data

diff_reporter_config:
  max_diff_file_size_mb: 10  # Maximum file size for diff processing
```

### URL Comparer Configuration

```go
config := differ.URLComparerConfig{
    EnableURLNormalization: true,   // Normalize URLs before comparison
    CaseSensitive:         false,   // Case-sensitive URL comparison
}
```

### Diff Processing Configuration

```go
config := differ.DiffConfig{
    EnableSemanticCleanup: true,    // Apply semantic cleanup to diffs
    EnableLineBasedDiff:   true,    // Enable line-based diff analysis
    ContextLines:          3,       // Number of context lines around changes
}
```

## Features

### 1. Content Diffing
- **Multiple Algorithms**: Supports character-level and line-based diffs
- **Semantic Cleanup**: Improves diff quality by removing noise
- **Size Validation**: Prevents memory issues with large files
- **Content Type Aware**: Optimizes processing based on content type
- **Error Handling**: Graceful handling of diff failures

### 2. URL Analysis
- **Status Tracking**: Identifies new, existing, and removed URLs
- **Historical Comparison**: Compares against previous scan results from Datastore
- **URL Normalization**: Consistent URL comparison across scans
- **Session Filtering**: Excludes current scan session from historical comparison

### 3. Statistical Analysis
- **Change Metrics**: Lines added, deleted, and modified
- **Performance Tracking**: Processing time measurement
- **Error Reporting**: Detailed error messages for diff failures

## Dependencies

- **github.com/aleister1102/monsterinc/internal/datastore** - Historical data access
- **github.com/aleister1102/monsterinc/internal/models** - Data structures
- **github.com/aleister1102/monsterinc/internal/config** - Configuration management
- **github.com/sergi/go-diff** - Core diff algorithms
- **github.com/rs/zerolog** - Structured logging

## Best Practices

### Performance Optimization
- Set appropriate file size limits to prevent memory issues
- Use streaming operations for large datasets
- Configure reasonable context lines for diff processing

### Error Handling
- Always check for diff processing errors
- Handle cases where historical data might not be available
- Implement fallbacks for large file processing

### Configuration Tuning
- Adjust lookback days based on scan frequency
- Set file size limits based on available memory
- Enable semantic cleanup for better diff quality