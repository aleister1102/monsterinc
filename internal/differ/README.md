# Differ Package

Content comparison and change detection engine for analyzing differences between current and historical scan results.

## Overview

Provides comprehensive change detection:
- **URL Comparison**: Detect new, existing, and removed URLs
- **Status Analysis**: Track URL status changes over time
- **Historical Integration**: Compare against stored scan data
- **Report Support**: Generate diff data for HTML reports
- **Performance**: Efficient comparison algorithms

## Core Components

### URL Differ

Compare current scan results with historical data to identify changes.

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
fmt.Printf("Removed URLs: %d\n", diffResult.Old)

for _, diffedURL := range diffResult.Results {
    fmt.Printf("URL: %s, Status: %s\n", 
        diffedURL.ProbeResult.InputURL, 
        diffedURL.ProbeResult.URLStatus)
}
```

### Builder Pattern

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

## Configuration

### Diff Configuration

```yaml
diff_config:
  enable_url_normalization: true    # Normalize URLs before comparison
  case_sensitive: false             # Case-sensitive URL comparison
  lookback_days: 30                 # Days to look back for historical data
```

### URL Comparer Configuration

```go
config := differ.URLComparerConfig{
    EnableURLNormalization: true,   // Normalize URLs before comparison
    CaseSensitive:         false,   // Case-sensitive URL comparison
}
```

## Features

### URL Analysis

- **Status Tracking**: Identifies new, existing, and removed URLs
- **Historical Comparison**: Compares against previous scan results from Datastore
- **URL Normalization**: Consistent URL comparison across scans  
- **Session Filtering**: Excludes current scan session from historical comparison

### Diff Result Structure

```go
type URLDiffResult struct {
    New      int                    // Count of new URLs
    Existing int                    // Count of existing URLs
    Old      int                    // Count of removed URLs
    Results  []DiffedProbeResult    // Detailed diff results
}

type DiffedProbeResult struct {
    ProbeResult models.ProbeResult
    Status      string             // "new", "existing", "old"
}
```

## Integration Examples

### With Scanner Service

```go
// Scanner uses differ to compare current vs historical results
diffProcessor := scanner.GetDiffProcessor()
urlDiffResult, err := diffProcessor.CompareWithHistorical(
    ctx, currentProbeResults, rootTargetURL, scanSessionID)

// Results used for generating diff reports
if urlDiffResult.New > 0 || urlDiffResult.Old > 0 {
    logger.Info().
        Int("new_urls", urlDiffResult.New).
        Int("old_urls", urlDiffResult.Old).
        Msg("URL changes detected")
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

### With Reporter

```go
// Generate diff report with comparison results
diffReportData := reporter.DiffReportData{
    CurrentResults:    currentProbes,
    HistoricalResults: historicalProbes,
    DiffSummary:      urlDiffResult,
    ComparisonDate:   time.Now(),
}

reportPath, err := reporter.GenerateDiffReport(diffReportData)
if err != nil {
    return fmt.Errorf("diff report generation failed: %w", err)
}
```

## Usage Examples

### Basic Comparison

```go
// Initialize components
parquetReader := datastore.NewParquetReader(storageConfig, logger)
urlDiffer, err := differ.NewUrlDiffer(parquetReader, logger)
if err != nil {
    return err
}

// Perform comparison
diffResult, err := urlDiffer.Compare(
    currentScanResults,
    "https://example.com",
    "scan-20240101-120000",
)

// Process results
for _, result := range diffResult.Results {
    switch result.Status {
    case "new":
        logger.Info().Str("url", result.ProbeResult.InputURL).Msg("New URL discovered")
    case "old":
        logger.Info().Str("url", result.ProbeResult.InputURL).Msg("URL no longer accessible")
    case "existing":
        logger.Debug().Str("url", result.ProbeResult.InputURL).Msg("URL still accessible")
    }
}
```

### Advanced Configuration

```go
// Custom differ configuration
config := differ.URLComparerConfig{
    EnableURLNormalization: true,
    CaseSensitive:         false,
    MaxHistoricalDays:     30,
    IgnoreQueryParams:     true,
}

urlDiffer, err := differ.NewUrlDifferBuilder(logger).
    WithParquetReader(parquetReader).
    WithConfig(config).
    Build()
```

### Error Handling

```go
// Handle comparison errors gracefully
diffResult, err := urlDiffer.Compare(currentResults, rootTarget, sessionID)
if err != nil {
    logger.Error().Err(err).Msg("URL comparison failed")
    // Continue with other operations, diff is not critical
    diffResult = &differ.URLDiffResult{
        New:      0,
        Existing: len(currentResults),
        Old:      0,
        Results:  []differ.DiffedProbeResult{},
    }
}
```

### Performance Optimization

```go
// Optimize for large datasets
if len(currentResults) > 10000 {
    // Use streaming comparison for large datasets
    err := urlDiffer.StreamingCompare(ctx, currentResults, rootTarget, 
        func(batchResult *differ.URLDiffResult) error {
            // Process batch results
            return processDiffBatch(batchResult)
        })
}
```

## Best Practices

### Performance Optimization

- Use streaming operations for large datasets
- Configure reasonable lookback days for historical comparison
- Filter out irrelevant historical data early

### Error Handling

- Always check for errors when comparing URLs
- Handle cases where historical data might not be available
- Gracefully degrade when diff operations fail

### Resource Management

- Properly cleanup resources after comparison
- Monitor memory usage during large comparisons
- Use context cancellation for long-running operations

## Dependencies

- `github.com/aleister1102/monsterinc/internal/datastore`: Historical data access
- `github.com/aleister1102/monsterinc/internal/models`: Data structures
- `github.com/aleister1102/monsterinc/internal/config`: Configuration management
- `github.com/rs/zerolog`: Structured logging