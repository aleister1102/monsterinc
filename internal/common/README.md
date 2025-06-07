# Common Package

## Purpose
The `common` package provides shared utilities used throughout the entire MonsterInc application, including HTTP client, file operations, error handling, memory management, and other utilities.

## Main Components

### 1. HTTP Client (`http_client.go`)
#### Purpose
- Provides high-performance HTTP client using net/http with HTTP/2 support
- Supports connection pooling, timeout, proxy, custom headers
- Thread-safe and reusable

#### API Usage

```go
// Create basic HTTP client
factory := common.NewHTTPClientFactory(logger)
client, err := factory.CreateBasicClient(30 * time.Second)

// Create client with custom configuration
config := common.HTTPClientConfig{
    Timeout:            30 * time.Second,
    FollowRedirects:    true,
    MaxRedirects:       5,
    InsecureSkipVerify: false,
    UserAgent:          "MonsterInc/1.0",
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

#### Related Configuration
```yaml
# No direct configuration, configured through code
```

#### Extensions
- Implement custom transport protocols
- Add middleware for request/response processing
- Create specialized clients for specific services

### 2. File Manager (`file_utils.go`)
#### Purpose
- Manage file operations safely with context support
- Validation, timeout, buffer management
- Cross-platform file handling

#### API Usage

```go
fm := common.NewFileManager(logger)

// Read file with options
opts := common.FileReadOptions{
    MaxSize:    10 * 1024 * 1024, // 10MB
    BufferSize: 4096,
    LineBased:  true,
    TrimLines:  true,
    SkipEmpty:  true,
    Timeout:    30 * time.Second,
    Context:    ctx,
}
content, err := fm.ReadFile("path/to/file.txt", opts)

// Write file
writeOpts := common.FileWriteOptions{
    CreateDirs:  true,
    Permissions: 0644,
    Timeout:     10 * time.Second,
    Context:     ctx,
}
err = fm.WriteFile("output/file.txt", data, writeOpts)

// Check file existence
exists := fm.FileExists("path/to/file.txt")

// Create directory
err = fm.EnsureDirectory("path/to/dir", 0755)
```

### 3. Error Handling (`errors.go`)
#### Purpose
- Standardized error types for the entire application
- Error wrapping and context preservation
- Typed errors for better error handling

#### API Usage

```go
// Wrap existing errors
err = common.WrapError(originalErr, "failed to process request")

// Create new errors
err = common.NewError("invalid input: %s", input)

// Validation errors
valErr := common.NewValidationError("email", userInput, "invalid email format")

// Network errors
netErr := common.NewNetworkError("https://api.com", "connection timeout", originalErr)

// HTTP errors
httpErr := common.NewHTTPErrorWithURL(404, "not found", "https://api.com/users/123")
```

### 4. Memory Pool (`memory_pool.go`)
#### Purpose
- Reduce GC pressure through object pooling
- Reuse buffers and slices
- Improve performance for high-throughput operations

#### API Usage

```go
// Buffer pool
bufferPool := common.NewBufferPool(1024) // initial capacity
buf := bufferPool.Get()
defer bufferPool.Put(buf)

// Use buffer
buf.WriteString("data")
data := buf.Bytes()

// Slice pool
slicePool := common.NewSlicePool(1024)
slice := slicePool.Get()
defer slicePool.Put(slice)

// String slice pool
stringPool := common.NewStringSlicePool(100)
strSlice := stringPool.Get()
defer stringPool.Put(strSlice)
```

### 5. Resource Limiter (`resource_limiter.go`)
#### Purpose
- Monitor and control application and system resource usage (memory, goroutines)
- Prevent OOM crashes and goroutine leaks
- Automatic garbage collection triggers for application memory
- **System memory monitoring with auto-shutdown capability**
- Graceful shutdown when system memory exceeds configurable threshold

#### Key Features
- **Application Memory Monitoring**: Track application memory usage and trigger GC
- **System Memory Monitoring**: Monitor total system memory usage using `gopsutil`
- **Auto-Shutdown**: Gracefully shutdown application when system memory > threshold
- **Configurable Thresholds**: Separate thresholds for app memory, system memory, and goroutines
- **Graceful Shutdown Callback**: Register custom shutdown logic for clean termination

#### API Usage

```go
// Create resource limiter with system monitoring
config := common.ResourceLimiterConfig{
    MaxMemoryMB:        1024, // 1GB app memory limit
    MaxGoroutines:      1000,
    CheckInterval:      30 * time.Second,
    MemoryThreshold:    0.8,  // 80% app memory warning
    GoroutineWarning:   0.7,  // 70% goroutine warning
    SystemMemThreshold: 0.5,  // 50% system memory shutdown trigger
    EnableAutoShutdown: true, // Enable auto-shutdown
}
limiter := common.NewResourceLimiter(config, logger)

// Set shutdown callback for graceful termination
limiter.SetShutdownCallback(func() {
    log.Info().Msg("Graceful shutdown triggered by memory limit")
    // Custom cleanup logic here
    cancel() // Cancel application context
})

// Start monitoring (includes system memory monitoring)
limiter.Start()
defer limiter.Stop()

// Check limits manually
err := limiter.CheckMemoryLimit()        // Application memory
err = limiter.CheckGoroutineLimit()      // Goroutine count
exceeded, err := limiter.CheckSystemMemoryLimit() // System memory

// Force GC
limiter.ForceGC()

// Get comprehensive resource usage
usage := limiter.GetResourceUsage()
fmt.Printf("App Memory: %d MB, System Memory: %d/%d MB (%.1f%%), Goroutines: %d\n", 
    usage.AllocMB, usage.SystemMemUsedMB, usage.SystemMemTotalMB, 
    usage.SystemMemUsedPercent, usage.Goroutines)

// Global instance (singleton pattern)
common.StartGlobalResourceLimiter(logger)
common.SetGlobalShutdownCallback(shutdownFunc) // Set global shutdown callback
defer common.StopGlobalResourceLimiter()
```

#### Related Configuration
```yaml
resource_limiter_config:
  max_memory_mb: 1024           # Application memory limit (1GB)
  max_goroutines: 10000         # Max goroutines allowed
  check_interval_secs: 30       # Monitoring interval
  memory_threshold: 0.8         # App memory warning threshold (80%)
  goroutine_warning: 0.7        # Goroutine warning threshold (70%)
  system_mem_threshold: 0.5     # System memory shutdown threshold (50%)
  enable_auto_shutdown: true    # Enable auto-shutdown feature
```

#### Safety Features
- **Graceful Shutdown**: Uses callback pattern to ensure clean application termination
- **Context Cancellation**: Integrates with application context for coordinated shutdown
- **Detailed Logging**: Comprehensive resource usage logging with system memory stats
- **Error Handling**: Graceful fallback if system memory monitoring fails

### 6. Time Utilities (`time_utils.go`)
#### Purpose
- Centralized time handling with multiple formatters
- Timezone-aware operations
- Validation and conversion utilities

#### API Usage

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

// Pointer utilities
timePtr := common.TimePtr(time.Now())
duration := common.DurationPtr(30 * time.Second)
```

### 7. Context Utilities (`context_utils.go`)
#### Purpose
- Context cancellation checking with logging
- Centralized context error handling
- Cancellation detection utilities

#### API Usage

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

// Check if error messages contain cancellation keywords
messages := []string{"context canceled", "operation timeout"}
hasCancellation := common.ContainsCancellationError(messages)
```

### 8. Regex Utilities (`regex_utils.go`)
#### Purpose
- Compile multiple regexes with error handling
- Centralized regex compilation for performance

#### API Usage

```go
regexStrings := []string{
    `\d+`,
    `[a-zA-Z]+`,
    `\b\w+@\w+\.\w+\b`,
}
regexes := common.CompileRegexes(regexStrings, logger)

// Use compiled regexes
for _, regex := range regexes {
    if regex.MatchString(input) {
        matches := regex.FindAllString(input, -1)
    }
}
```

## Overall Configuration
The common package doesn't have a separate configuration file but is configured through:
- Constructor parameters
- Configuration structs passed to builders
- Environment variables for global settings

## Extension Patterns

### 1. Builder Pattern
```go
type CustomClientBuilder struct {
    config CustomConfig
    logger zerolog.Logger
}

func (b *CustomClientBuilder) WithCustomFeature(feature Feature) *CustomClientBuilder {
    b.config.Feature = feature
    return b
}

func (b *CustomClientBuilder) Build() (*CustomClient, error) {
    // Implementation
}
```

### 2. Factory Pattern
```go
type CustomFactory struct {
    logger zerolog.Logger
}

func (f *CustomFactory) CreateSpecializedClient(purpose string) (*Client, error) {
    // Create client for specific purpose
}
```

### 3. Strategy Pattern
```go
type ProcessingStrategy interface {
    Process(data []byte) ([]byte, error)
}

type Processor struct {
    strategy ProcessingStrategy
}
```

## Best Practices

1. **Always use context**: All operations should support context cancellation
2. **Resource cleanup**: Use defer statements and proper cleanup
3. **Error wrapping**: Always wrap errors with context information
4. **Pool reuse**: Use memory pools for high-frequency operations
5. **Configuration validation**: Validate all config parameters
6. **Logging**: Log all significant events and errors

## Dependencies
- `golang.org/x/net/http2`: HTTP/2 client implementation
- `github.com/rs/zerolog`: Logging framework
- Standard library packages: `context`, `sync`, `time`, `io`, `os` 