# Differ Package

## Purpose
The `differ` package provides URL diffing capabilities for MonsterInc's analysis pipeline. It detects and analyzes changes in URL status and provides detailed reports for security monitoring and change tracking.

## Package Role in MonsterInc
As the change detection engine, this package:
- **Scanner Integration**: Compares current scan results with historical data stored in Datastore
- **Report Support**: Provides diff data for HTML report generation via Reporter
- **Historical Analysis**: Enables understanding of website evolution over time

## Main Components

### 1. URL Differ (`url_differ.go`)
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
```

### URL Comparer Configuration

```go
config := differ.URLComparerConfig{
    EnableURLNormalization: true,   // Normalize URLs before comparison
    CaseSensitive:         false,   // Case-sensitive URL comparison
}
```

## Features

### 1. URL Analysis
- **Status Tracking**: Identifies new, existing, and removed URLs
- **Historical Comparison**: Compares against previous scan results from Datastore
- **URL Normalization**: Consistent URL comparison across scans
- **Session Filtering**: Excludes current scan session from historical comparison

## Dependencies

- **github.com/aleister1102/monsterinc/internal/datastore** - Historical data access
- **github.com/aleister1102/monsterinc/internal/models** - Data structures
- **github.com/aleister1102/monsterinc/internal/config** - Configuration management
- **github.com/rs/zerolog** - Structured logging

## Best Practices

### Performance Optimization
- Use streaming operations for large datasets
- Configure reasonable lookback days for historical comparison

### Error Handling
- Always check for errors when comparing URLs
- Handle cases where historical data might not be available