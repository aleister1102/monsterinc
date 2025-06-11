# Differ Package

## Purpose
The `differ` package provides comprehensive content comparison and URL diffing capabilities for MonsterInc's monitoring and analysis pipeline. It detects and analyzes changes in web content, URL status, and provides detailed diff reports for security monitoring and change tracking.

## Package Role in MonsterInc
As the change detection engine, this package:
- **Monitor Integration**: Analyzes content changes detected by the monitoring service
- **Historical Comparison**: Compares current scan results with historical data
- **Report Generation**: Provides diff data for HTML report generation
- **Security Analysis**: Identifies potentially malicious content changes
- **Trend Analysis**: Enables understanding of website evolution over time

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
- Compare current scan results with historical data
- Identify new, existing, and removed URLs
- Track URL status changes over time
- Provide comprehensive URL analysis

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

diffResult, err := urlDiffer.Compare(currentProbes, "https://example.com")
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
- **Historical Comparison**: Compares against previous scan results
- **URL Normalization**: Consistent URL comparison across scans
- **Flexible Configuration**: Customizable comparison parameters

### 3. Statistical Analysis
- **Change Metrics**: Lines added, deleted, and modified
- **Performance Tracking**: Processing time measurement
- **Hash Comparison**: Quick identical content detection
- **Result Aggregation**: Summary statistics across multiple diffs

## Data Models

### ContentDiffResult

```go
type ContentDiffResult struct {
    Timestamp        int64           `json:"timestamp"`
    ContentType      string          `json:"content_type"`
    Diffs            []ContentDiff   `json:"diffs"`
    LinesAdded       int             `json:"lines_added"`
    LinesDeleted     int             `json:"lines_deleted"`
    LinesChanged     int             `json:"lines_changed"`
    IsIdentical      bool            `json:"is_identical"`
    ErrorMessage     string          `json:"error_message,omitempty"`
    ProcessingTimeMs int64           `json:"processing_time_ms"`
    OldHash          string          `json:"old_hash,omitempty"`
    NewHash          string          `json:"new_hash,omitempty"`
    ExtractedPaths   []ExtractedPath `json:"extracted_paths,omitempty"`
}
```

### URLDiffResult

```go
type URLDiffResult struct {
    RootTargetURL string      `json:"root_target_url"`
    Results       []DiffedURL `json:"results,omitempty"`
    New           int         `json:"new"`
    Old           int         `json:"old"`
    Existing      int         `json:"existing"`
    Error         string      `json:"error,omitempty"`
}
```

## Integration Examples

### Scanner Integration

```go
// In scanner workflow - URL diffing
urlDiffer, err := differ.NewUrlDiffer(parquetReader, logger)
if err != nil {
    return err
}

urlDiffResult, err := urlDiffer.Compare(currentProbeResults, rootTarget)
if err != nil {
    logger.Error().Err(err).Msg("URL diff failed")
} else {
    logger.Info().
        Int("new", urlDiffResult.New).
        Int("existing", urlDiffResult.Existing).
        Int("old", urlDiffResult.Old).
        Msg("URL diff completed")
}
```

### Monitor Integration

```go
// In monitoring service - content diffing
contentDiffer, err := differ.NewContentDiffer(logger, diffReporterConfig)
if err != nil {
    return err
}

diffResult, err := contentDiffer.GenerateDiff(
    lastRecord.Content,
    currentContent,
    contentType,
    lastRecord.Hash,
    currentHash,
)

if err != nil {
    logger.Error().Err(err).Msg("Content diff failed")
} else if !diffResult.IsIdentical {
    logger.Info().
        Int("lines_added", diffResult.LinesAdded).
        Int("lines_deleted", diffResult.LinesDeleted).
        Msg("Content changes detected")
}
```

## Error Handling

### Common Error Types

- **Content Too Large**: When content exceeds size limits
- **Invalid Content Type**: For unsupported content types
- **Diff Processing Failure**: When diff generation fails
- **Historical Data Missing**: When no previous data exists

### Error Examples

```go
// Handle content size errors
diffResult, err := contentDiffer.GenerateDiff(prev, curr, contentType, oldHash, newHash)
if err != nil {
    if errors.Is(err, differ.ErrContentTooLarge) {
        logger.Warn().Msg("Content too large for diff processing")
        // Handle gracefully with summary
    } else {
        logger.Error().Err(err).Msg("Diff processing failed")
        return err
    }
}

// Handle URL diff errors
urlDiffResult, err := urlDiffer.Compare(currentProbes, rootTarget)
if err != nil {
    logger.Error().Err(err).Msg("URL comparison failed")
    // Use empty result or previous results
    urlDiffResult = &models.URLDiffResult{
        RootTargetURL: rootTarget,
        Error:         err.Error(),
    }
}
```

## Dependencies

- `github.com/sergi/go-diff/diffmatchpatch`: Core diff algorithms
- `github.com/rs/zerolog`: Structured logging
- `context`: Context handling for cancellation
- `time`: Time handling and measurement
- Internal packages:
  - `internal/datastore`: Historical data access
  - `internal/models`: Data models
  - `internal/config`: Configuration structures

## Performance Considerations

- **Memory Management**: Efficient handling of large content
- **Size Limits**: Configurable maximum file sizes
- **Context Cancellation**: Proper cleanup on cancellation
- **Batch Processing**: Optimized for multiple comparisons
- **Caching**: Efficient hash-based comparison

## Testing

The package includes comprehensive test coverage for:

- Content diff generation with various content types
- URL comparison with different scenarios
- Error handling for edge cases
- Performance with large content
- Statistical calculation accuracy
- Builder pattern functionality