# Datastore Package

High-performance data persistence layer using Apache Parquet format for scan results, file history, and monitoring data with compression and efficient querying.

## Overview

Provides comprehensive data persistence:
- **Parquet Storage**: Fast, compressed columnar storage format
- **Streaming Operations**: Memory-efficient reading/writing for large datasets
- **File History**: Track content changes and version history
- **Query Interface**: Efficient search and retrieval capabilities
- **Schema Management**: Structured data validation and transformation

## Core Components

### Parquet Writer

Write probe results and scan data with high-performance batch processing.

```go
// Create Parquet writer
writer, err := datastore.NewParquetWriter(storageConfig, logger)
if err != nil {
    return err
}

// Write probe results
probeResults := []models.ProbeResult{
    // ... your probe results
}

err = writer.Write(ctx, probeResults, "scan-20240101-120000", "https://example.com")
if err != nil {
    return err
}

// Builder pattern
builder := datastore.NewParquetWriterBuilder(logger)
writer, err = builder.
    WithStorageConfig(storageConfig).
    WithWriterConfig(datastore.ParquetWriterConfig{
        CompressionType:  "zstd",
        BatchSize:        1000,
        EnableValidation: true,
    }).
    Build()
```

### Parquet Reader

Read and query Parquet files efficiently with pagination support.

```go
// Create Parquet reader
reader := datastore.NewParquetReader(storageConfig, logger)

// Find all probe results for target
results, lastModified, err := reader.FindAllProbeResultsForTarget("https://example.com")
if err != nil {
    return err
}

// Search with query parameters
query := datastore.ProbeResultQuery{
    RootTargetURL: "https://example.com",
    Limit:         100,
    Offset:        0,
}

searchResult, err := reader.SearchProbeResults(query)
if err != nil {
    return err
}

fmt.Printf("Found %d results, total: %d\n", 
    len(searchResult.Results), searchResult.TotalCount)
```

### Streaming Operations

Memory-efficient processing for large datasets.

```go
// Create streaming reader
streamReader := datastore.NewStreamingParquetReader(storageConfig, logger)

// Stream probe results with callback
err := streamReader.StreamProbeResults(ctx, "https://example.com", 
    func(result models.ProbeResult) error {
        // Process each result individually
        fmt.Printf("Processing: %s\n", result.InputURL)
        return nil
    })

// Stream file history
err = streamReader.StreamFileHistory(ctx, "path/to/history.parquet",
    func(record models.FileHistoryRecord) error {
        // Process each history record
        return processHistoryRecord(record)
    })

// Count records without loading them
count, err := streamReader.CountRecords(ctx, "path/to/data.parquet")
```

### File History Store

Track file changes over time with thread-safe operations.

```go
// Create file history store
historyStore, err := datastore.NewParquetFileHistoryStore(storageConfig, logger)
if err != nil {
    return err
}

// Store file record
record := models.FileHistoryRecord{
    URL:         "https://example.com/api/data.json",
    Timestamp:   time.Now().UnixMilli(),
    Hash:        "sha256hash",
    ContentType: "application/json",
    Content:     jsonData,
    ETag:        "W/\"abc123\"",
    LastModified: "Wed, 21 Oct 2015 07:28:00 GMT",
}

err = historyStore.StoreFileRecord(record)

// Get last known record
lastRecord, err := historyStore.GetLastKnownRecord("https://example.com/api/data.json")
if lastRecord != nil {
    fmt.Printf("Last hash: %s\n", lastRecord.Hash)
}

// Get multiple records with limit
records, err := historyStore.GetRecordsForURL("https://example.com/api/data.json", 10)
```

### Batch Processing

Efficient batch processing for large datasets.

```go
// Batch processor
batchProcessor := datastore.NewBatchProcessor(100, 
    func(batch []models.ProbeResult) error {
        return processBatch(batch)
    })

// Add items to batch
for _, result := range results {
    err := batchProcessor.Add(result)
    if err != nil {
        return err
    }
}

// Flush remaining items
err = batchProcessor.Flush()
```

## Configuration

### Storage Configuration

```yaml
storage_config:
  parquet_base_path: "./data"           # Base directory for Parquet files
  compression_codec: "zstd"             # Compression: "zstd", "gzip", "snappy", "none"
```

### Writer Configuration

```go
writerConfig := datastore.ParquetWriterConfig{
    CompressionType:  "zstd",     // Compression algorithm
    BatchSize:        1000,       // Batch size for writing
    EnableValidation: true,       // Enable data validation
}
```

### Reader Configuration

```go
readerConfig := datastore.ParquetReaderConfig{
    BufferSize: 4096,    // Buffer size for reading
    ReadAll:    false,   // Whether to read all data at once
}
```

## File Organization

### Directory Structure

```
data/
├── probe_results/
│   ├── example.com_80/
│   │   ├── scan_20240101_120000.parquet
│   │   ├── scan_20240101_130000.parquet
│   │   └── ...
│   └── test.com_443/
│       ├── scan_20240101_120000.parquet
│       └── ...
└── file_history/
    ├── example.com_80/
    │   ├── a1b2c3d4.parquet  # URL hash-based files
    │   ├── e5f6g7h8.parquet
    │   └── ...
    └── test.com_443/
        └── ...
```

### File Naming Convention

- **Probe Results**: `{hostname}_{port}/scan_{timestamp}.parquet`
- **File History**: `{hostname}_{port}/{url_hash}.parquet`
- **Timestamps**: Format `YYYYMMDD_HHMMSS`
- **URL Hashes**: 8-character deterministic hashes

## Data Schemas

### Probe Result Schema

```go
type ParquetProbeResult struct {
    OriginalURL        string   `parquet:"original_url"`
    FinalURL           *string  `parquet:"final_url,optional"`
    StatusCode         *int32   `parquet:"status_code,optional"`
    ContentLength      *int64   `parquet:"content_length,optional"`
    ContentType        *string  `parquet:"content_type,optional"`
    Title              *string  `parquet:"title,optional"`
    WebServer          *string  `parquet:"web_server,optional"`
    Technologies       []string `parquet:"technologies,list"`
    IPAddress          []string `parquet:"ip_address,list"`
    RootTargetURL      *string  `parquet:"root_target_url,optional"`
    ProbeError         *string  `parquet:"probe_error,optional"`
    ScanTimestamp      int64    `parquet:"scan_timestamp"`
    HeadersJSON        *string  `parquet:"headers_json,optional"`
}
```

### File History Schema

```go
type FileHistoryRecord struct {
    URL                string  `parquet:"url,zstd"`
    Timestamp          int64   `parquet:"timestamp,zstd"`
    Hash               string  `parquet:"hash,zstd"`
    ContentType        string  `parquet:"content_type,zstd,optional"`
    Content            []byte  `parquet:"content,zstd,optional"`
    ETag               string  `parquet:"etag,zstd,optional"`
    LastModified       string  `parquet:"last_modified,zstd,optional"`
    DiffResultJSON     *string `parquet:"diff_result_json,zstd,optional"`
}
```

## Integration Examples

### With Scanner Service

```go
// Write scan results
err = parquetWriter.Write(ctx, probeResults, sessionID, rootTarget)
if err != nil {
    return fmt.Errorf("storage failed: %w", err)
}

// Read historical data for comparison
historicalResults, err := parquetReader.FindAllProbeResultsForTarget(rootTarget)
if err != nil {
    return fmt.Errorf("failed to load historical data: %w", err)
}
```

### With Monitor Service

```go
// Store file change record
record := models.FileHistoryRecord{
    URL:       monitoredURL,
    Hash:      contentHash,
    Content:   fileContent,
    Timestamp: time.Now().UnixMilli(),
}

err = fileHistoryStore.StoreFileRecord(record)
if err != nil {
    return fmt.Errorf("failed to store file record: %w", err)
}
```

## Performance Optimization

### Memory Management

```go
// Use object pools for high-frequency operations
var probeResultPool = sync.Pool{
    New: func() interface{} {
        return &models.ProbeResult{}
    },
}

func getProbeResult() *models.ProbeResult {
    return probeResultPool.Get().(*models.ProbeResult)
}

func putProbeResult(pr *models.ProbeResult) {
    *pr = models.ProbeResult{} // Reset fields
    probeResultPool.Put(pr)
}
```

### Compression Tuning

```go
// Compression level optimization based on data size
func getOptimalCompression(dataSize int64) parquet.WriterOption {
    if dataSize < 1024*1024 { // < 1MB
        return parquet.Compression(&parquet.Snappy{}) // Fast compression
    } else if dataSize < 10*1024*1024 { // < 10MB
        return parquet.Compression(&parquet.Gzip{Level: 6}) // Balanced
    } else {
        return parquet.Compression(&parquet.Zstd{Level: 3}) // High compression
    }
}
```

### Batch Operations

```go
// Optimize batch sizes based on available memory
func calculateOptimalBatchSize(availableMemoryMB int) int {
    // Rough calculation: assume 1KB per record on average
    return (availableMemoryMB * 1024) / 10 // Use 10% of available memory
}
```

## Best Practices

1. **Batch Writing**: Always batch multiple records for better performance
2. **Compression**: Use appropriate compression based on data size and usage patterns
3. **Error Handling**: Handle storage errors gracefully with retries
4. **Resource Management**: Properly close files and cleanup resources
5. **Monitoring**: Monitor disk space, I/O performance, and compression ratios
6. **Schema Evolution**: Plan for schema changes and backward compatibility

## Dependencies

- `github.com/parquet-go/parquet-go`: Parquet implementation
- `github.com/rs/zerolog`: Structured logging framework
