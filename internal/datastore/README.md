# Datastore Package

## Purpose
The `datastore` package provides a high-performance persistence layer using Apache Parquet format for storing probe results and file history. It supports compression, streaming operations, and efficient querying.

## Main Components

### 1. Parquet Writer (`parquet_writer.go`)
#### Purpose
- Write probe results and scan data to Parquet files
- High-performance batch writing with compression
- Context-aware operations with cancellation support
- Memory-efficient transformations

#### API Usage

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

### 2. Parquet Reader (`parquet_reader.go`)
#### Purpose
- Read and query Parquet files efficiently
- Support pagination and filtering
- Memory-optimized streaming operations
- Metadata extraction

#### API Usage

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

searchResult, err := reader.searchProbeResults(query)
if err != nil {
    return err
}

fmt.Printf("Found %d results, total: %d\n", 
    len(searchResult.Results), searchResult.TotalCount)
```

### 3. Streaming Reader (`memory_optimized_reader.go`)
#### Purpose
- Memory-efficient streaming of large datasets
- Callback-based processing to avoid loading all data into memory
- Batch processing support
- Real-time processing capabilities

#### API Usage

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

// Batch processing
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

### 4. File History Store (`parquet_file_history.go`)
#### Purpose
- Track file changes over time
- Store content diffs and extracted paths
- Support monitoring workflows
- Thread-safe operations with URL-based mutexes

#### API Usage

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

// Get all hostnames with history
hostnames, err := historyStore.GetHostnamesWithHistory()

// Get diff results
diffResults, err := historyStore.GetAllLatestDiffResultsForURLs(urls)
```

### 5. Record Transformation (`record_transformer.go`)
#### Purpose
- Transform between different data formats
- Convert models.ProbeResult to Parquet-compatible format
- Handle optional fields and null values
- Optimize storage efficiency

#### API Usage

```go
// Create transformer
transformer := datastore.NewRecordTransformer(logger)

// Transform probe result to Parquet format
parquetResult := transformer.TransformToParquetResult(probeResult, scanTime)

// The transformer handles:
// - Technology names extraction
// - Headers JSON marshaling
// - Timestamp calculations
// - Optional field conversions
```

### 6. URL Management (`url_hash_generator.go`, `url_mutex_manager.go`)
#### Purpose
- Generate consistent URL hashes for file naming
- Thread-safe URL-based locking
- Cleanup unused resources

#### API Usage

```go
// URL hash generation
hashGen := datastore.NewURLHashGenerator(8) // 8 character hash
hash := hashGen.GenerateHash("https://example.com/very/long/path")
fmt.Printf("Short hash: %s\n", hash) // e.g., "a1b2c3d4"

// Mutex management
mutexManager := datastore.NewURLMutexManager(true, logger)
mutex := mutexManager.GetMutex("https://example.com")

mutex.Lock()
// Critical section for URL
mutex.Unlock()

// Cleanup unused mutexes
activeURLs := []string{"https://example.com", "https://test.com"}
mutexManager.CleanupUnusedMutexes(activeURLs)
```

## Configuration

### Storage Config
```yaml
storage_config:
  parquet_base_path: "./data"           # Base directory for Parquet files
  compression_codec: "zstd"             # Compression: "zstd", "gzip", "snappy", "none"
```

### Writer Config
```go
writerConfig := datastore.ParquetWriterConfig{
    CompressionType:  "zstd",     // Compression algorithm
    BatchSize:        1000,       // Batch size for writing
    EnableValidation: true,       // Enable data validation
}
```

### Reader Config
```go
readerConfig := datastore.ParquetReaderConfig{
    BufferSize: 4096,    // Buffer size for reading
    ReadAll:    false,   // Whether to read all data at once
}
```

### File History Config
```go
historyConfig := datastore.ParquetFileHistoryStoreConfig{
    MaxFileSize:       100 * 1024 * 1024, // 100MB max file size
    EnableCompression: true,                // Enable compression
    CompressionCodec:  "zstd",             // Compression codec
    EnableURLMutexes:  true,               // Enable URL-based locking
    CleanupInterval:   3600,               // Cleanup interval in seconds
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
    Method             *string  `parquet:"method,optional"`
    HeadersJSON        *string  `parquet:"headers_json,optional"`
    DiffStatus         *string  `parquet:"diff_status,optional"`
    ScanTimestamp      int64    `parquet:"scan_timestamp"`
    FirstSeenTimestamp *int64   `parquet:"first_seen_timestamp,optional"`
    LastSeenTimestamp  *int64   `parquet:"last_seen_timestamp,optional"`
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
    ExtractedPathsJSON *string `parquet:"extracted_paths_json,zstd,optional"`
}
```

## Performance Optimization

### 1. Memory Management
```go
// Use object pools
var probeResultPool = sync.Pool{
    New: func() interface{} {
        return &models.ProbeResult{}
    },
}

func getProbeResult() *models.ProbeResult {
    return probeResultPool.Get().(*models.ProbeResult)
}

func putProbeResult(pr *models.ProbeResult) {
    // Reset fields
    *pr = models.ProbeResult{}
    probeResultPool.Put(pr)
}
```

### 2. Compression Tuning
```go
// Compression level optimization
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

## Extending Datastore

### 1. Custom Storage Backends
```go
// Storage interface
type StorageBackend interface {
    Write(ctx context.Context, data interface{}) error
    Read(ctx context.Context, query interface{}) (interface{}, error)
    Delete(ctx context.Context, key string) error
}

// S3 backend implementation
type S3Backend struct {
    client *s3.Client
    bucket string
}

func (s3b *S3Backend) Write(ctx context.Context, data interface{}) error {
    // Implementation for S3 storage
}
```

### 2. Custom Data Formats
```go
// Format interface
type DataFormat interface {
    Marshal(data interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
    FileExtension() string
}

// JSON format implementation
type JSONFormat struct{}

func (jf *JSONFormat) Marshal(data interface{}) ([]byte, error) {
    return json.Marshal(data)
}
```

## Best Practices

1. **Batch Writing**: Always batch multiple records to improve performance
2. **Compression**: Use appropriate compression based on data size and usage patterns
3. **Error Handling**: Handle storage errors gracefully with retries
4. **Resource Management**: Properly close files and cleanup resources
5. **Monitoring**: Monitor disk space, I/O performance, and compression ratios
6. **Schema Evolution**: Plan for schema changes and backward compatibility
7. **Partitioning**: Partition data by time or other relevant dimensions

## Dependencies
- `github.com/parquet-go/parquet-go`: Parquet implementation
- `github.com/rs/zerolog`: Logging framework 