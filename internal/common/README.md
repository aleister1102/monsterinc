# Common Package

Shared utilities and foundational components used throughout MonsterInc for HTTP client operations, file management, error handling, memory management, and common operations.

## Overview

The common package provides:
- **HTTP Client**: High-performance HTTP client with connection pooling
- **File Operations**: Safe file management with context support
- **Error Handling**: Standardized error types and wrapping
- **Memory Management**: Object pools for performance optimization
- **Time Utilities**: Comprehensive time handling and formatting
- **Context Utils**: Cancellation detection and handling
- **URL Handling**: URL normalization and management utilities

## Core Components

### HTTP Client

High-performance HTTP client with HTTP/2 support, connection pooling, and configurable options.

```go
// Create HTTP client factory
factory := common.NewHTTPClientFactory(logger)

// Basic client
client, err := factory.CreateBasicClient(30 * time.Second)

// Custom client with configuration
config := common.HTTPClientConfig{
    Timeout:            30 * time.Second,
    FollowRedirects:    true,
    MaxRedirects:       5,
    InsecureSkipVerify: false,
    CustomHeaders:      map[string]string{"Accept": "application/json"},
}
client, err := common.NewHTTPClient(config, logger)

// Execute request
req := &common.HTTPRequest{
    URL:     "https://example.com",
    Method:  "GET",
    Headers: map[string]string{"Authorization": "Bearer token"},
    Context: ctx,
}
resp, err := client.Do(req)
```

### File Manager

Safe file operations with context support, validation, and cross-platform compatibility.

```go
fm := common.NewFileManager(logger)

// Read file with options
opts := common.FileReadOptions{
    MaxSize:    10 * 1024 * 1024, // 10MB limit
    BufferSize: 4096,
    LineBased:  true,
    TrimLines:  true,
    SkipEmpty:  true,
    Timeout:    30 * time.Second,
    Context:    ctx,
}
content, err := fm.ReadFile("path/to/file.txt", opts)

// Write file with options
writeOpts := common.FileWriteOptions{
    CreateDirs:  true,
    Permissions: 0644,
    Timeout:     10 * time.Second,
    Context:     ctx,
}
err = fm.WriteFile("output/file.txt", data, writeOpts)

// Utility functions
exists := fm.FileExists("path/to/file.txt")
err = fm.EnsureDirectory("path/to/dir", 0755)
```

### Error Handling

Standardized error types with context preservation and error wrapping.

```go
// Wrap existing errors
err = common.WrapError(originalErr, "failed to process request")

// Create new errors
err = common.NewError("invalid input: %s", input)

// Specific error types
valErr := common.NewValidationError("email", userInput, "invalid email format")
netErr := common.NewNetworkError("https://api.com", "connection timeout", originalErr)
httpErr := common.NewHTTPErrorWithURL(404, "not found", "https://api.com/users/123")
```

### Memory Pools

Object pooling for performance optimization and reduced GC pressure.

```go
// Buffer pool for byte operations
bufferPool := common.NewBufferPool(1024) // initial capacity
buf := bufferPool.Get()
defer bufferPool.Put(buf)

buf.WriteString("data")
data := buf.Bytes()

// Slice pool for reusable slices
slicePool := common.NewSlicePool(1024)
slice := slicePool.Get()
defer slicePool.Put(slice)

// String slice pool
stringPool := common.NewStringSlicePool(100)
strSlice := stringPool.Get()
defer stringPool.Put(strSlice)
```

### Time Utilities

Comprehensive time handling with multiple formatters and validation.

```go
timeUtils := common.NewTimeUtils()

// Time conversion
unixMilli := timeUtils.Convert().TimeToUnixMilli(time.Now())
t := timeUtils.Convert().UnixMilliToTime(unixMilli)

// Optional time handling
optionalTime := common.UnixMilliToTimeOptional(&unixMilli)
formatted := common.FormatTimeOptional(t, time.RFC3339)

// Validation
isValid := timeUtils.Validate().IsValid(t)
isFuture := timeUtils.Validate().IsInFuture(t)
isPast := timeUtils.Validate().IsInPast(t)

// Custom formatters
timeUtils.AddFormatter("custom", &common.DisplayFormatter{})
formatted := timeUtils.FormatWith("rfc3339", time.Now())

// Utility functions
timePtr := common.TimePtr(time.Now())
duration := common.DurationPtr(30 * time.Second)
```

### Context Utilities

Context cancellation detection with logging support.

```go
// Check cancellation with logging
result := common.CheckCancellationWithLog(ctx, logger, "database query")
if result.Cancelled {
    return result.Error
}

// Simple cancellation check
result = common.CheckCancellation(ctx)
if result.Cancelled {
    // Handle cancellation
}

// Check for cancellation keywords in error messages
messages := []string{"context canceled", "operation timeout"}
hasCancellation := common.ContainsCancellationError(messages)
```

### Batch Processor

Efficient batch processing for large datasets.

```go
// Create batch processor
processor := common.NewBatchProcessor(100, func(batch []interface{}) error {
    // Process batch of items
    return processBatchItems(batch)
})

// Add items to batch
for _, item := range items {
    err := processor.Add(item)
    if err != nil {
        return err
    }
}

// Flush remaining items
err = processor.Flush()
```

## Subpackages

### URL Handler

URL normalization, validation, and management utilities.

```go
// URL normalization
normalizedURL, err := urlhandler.NormalizeURL("example.com")
// Result: "https://example.com"

// URL validation
err := urlhandler.ValidateURLFormat("https://example.com")

// URL resolution
baseURL, _ := url.Parse("https://example.com/page")
resolved, err := urlhandler.ResolveURL("../other", baseURL)
// Result: "https://example.com/other"

// Hostname extraction
hostname, err := urlhandler.ExtractHostname("https://example.com:8080/path")
// Result: "example.com"

// Filename sanitization
safe := urlhandler.SanitizeFilename("https://example.com/path?param=value")
// Result: "example_com_path_param_value"
```

### Summary Builder

Scan statistics and summary generation.

```go
// Create summary builder
builder := summary.NewScanSummaryBuilder()

// Build scan summary
summaryData := builder.
    WithScanSessionID("scan-20240101-120000").
    WithTargets(targets).
    WithProbeStats(probeStats).
    WithScanDuration(duration).
    Build()

// Batch information
batchInfo := summary.NewBatchInfo(batchSize, totalBatches)
batchInfo.SetProcessedBatches(processedBatches)
```

## Integration Examples

### With Scanner Service

```go
// HTTP client for probing
httpClient, err := common.NewHTTPClientFactory(logger).CreateBasicClient(30*time.Second)

// File operations for target loading
fm := common.NewFileManager(logger)
content, err := fm.ReadFile(targetsFile, common.FileReadOptions{})

// Error handling
if err != nil {
    return common.WrapError(err, "failed to load targets")
}
```

### With Datastore Operations

```go
// Memory pools for efficient processing
bufferPool := common.NewBufferPool(1024)
buf := bufferPool.Get()
defer bufferPool.Put(buf)

// Batch processing for large datasets
processor := common.NewBatchProcessor(100, func(batch []interface{}) error {
    return datastore.WriteBatch(batch)
})
```

## Best Practices

1. **Always use context**: All operations support context cancellation
2. **Resource cleanup**: Use defer statements and proper cleanup  
3. **Error wrapping**: Always wrap errors with context information
4. **Pool reuse**: Use memory pools for high-frequency operations
5. **Configuration validation**: Validate all config parameters
6. **Logging**: Log all significant events and errors

## Dependencies

- `golang.org/x/net/http2`: HTTP/2 client implementation
- `github.com/rs/zerolog`: Structured logging framework
- Standard library: `context`, `sync`, `time`, `io`, `os`, `net/http`