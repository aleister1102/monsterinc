# HTTPx Runner Package

## Description

The `httpxrunner` package is a wrapper for ProjectDiscovery's httpx library, designed to integrate seamlessly into MonsterInc with features like configuration management, result mapping, and collection.

## File Structure

The package has been refactored into separate files following the Single Responsibility Principle:

### `config.go`
- `Config` struct: Contains all configuration for HTTPX runner
- `DefaultConfig()`: Returns default configuration

### `options_configurator.go`
- `HTTPXOptionsConfigurator`: Converts MonsterInc config to httpx options
- Contains methods to apply each type of configuration separately

### `result_mapper.go`
- `ProbeResultMapper`: Converts httpx result to MonsterInc ProbeResult
- Handles mapping of fields like headers, technologies, network info, ASN

### `result_collector.go`
- `ResultCollector`: Collects and manages probe results
- Thread-safe with mutex to support concurrent access

### `builder.go`
- `RunnerBuilder`: Builder pattern to create Runner instance
- Fluent interface for configuring runner

### `runner.go`
- `Runner`: Main struct wrapping httpx runner
- Methods to execute and manage lifecycle

## Usage

```go
// Create runner with builder pattern
runner, err := httpxrunner.NewRunnerBuilder(logger).
    WithConfig(config).
    WithRootTargetURL("https://example.com").
    Build()

if err != nil {
    return err
}

// Run the runner
err = runner.Run(ctx)
if err != nil {
    return err
}

// Get results
results := runner.GetResults()
```

## Benefits of Refactor

1. **Single Responsibility**: Each file has a clear purpose
2. **Easy Maintenance**: Code is logically organized, easy to find and fix
3. **Testability**: Can test each component separately
4. **Reusability**: Components can be reused in other contexts
5. **Clean Architecture**: Follows clean code principles

## Configuration

The HTTPx Runner is configured through the global configuration:

```yaml
httpx_runner_config:
  threads: 50
  timeout_secs: 10
  retries: 2
  rate_limit: 150
  method: "GET"
  follow_redirects: true
  max_redirects: 5
  verbose: false
  
  # Extraction options
  extract_status_code: true
  extract_content_length: true
  extract_content_type: true
  extract_title: true
  extract_server_header: true
  extract_headers: true
  extract_body: false
  extract_ips: true
  extract_location: true
  extract_asn: true
  tech_detect: true
  
  # Custom headers
  custom_headers:
    User-Agent: "MonsterInc/1.0"
    Accept: "text/html,application/xhtml+xml"
```

## Components

### 1. HTTPXOptionsConfigurator
Converts MonsterInc configuration to httpx runner options.

```go
configurator := httpxrunner.NewHTTPXOptionsConfigurator(logger)
options := configurator.ConfigureOptions(config)
```

#### Features:
- **Target Configuration**: Sets input targets and request URIs
- **Performance Settings**: Configures threads, timeouts, rate limits
- **Extraction Settings**: Controls what data to extract from responses
- **Custom Headers**: Applies user-defined HTTP headers
- **Redirect Handling**: Configures redirect following behavior

### 2. ProbeResultMapper
Maps httpx runner results to MonsterInc ProbeResult format.

```go
mapper := httpxrunner.NewProbeResultMapper(logger)
probeResult := mapper.MapResult(httpxResult, rootURL)
```

#### Mapping Features:
- **HTTP Response Data**: Status code, headers, content length
- **Network Information**: IP addresses, ASN data
- **Technology Detection**: Identifies web technologies
- **Performance Metrics**: Response times and durations
- **Error Handling**: Maps errors and failures

### 3. ResultCollector
Thread-safe collection and management of probe results.

```go
collector := httpxrunner.NewResultCollector(logger)
collector.AddResult(probeResult)
results := collector.GetResults()
```

#### Features:
- **Thread Safety**: Uses mutex for concurrent access
- **Memory Efficient**: Optimized for large result sets
- **Statistics**: Provides count and summary information

### 4. RunnerBuilder
Builder pattern for creating configured Runner instances.

```go
builder := httpxrunner.NewRunnerBuilder(logger).
    WithConfig(config).
    WithRootTargetURL(rootURL)

runner, err := builder.Build()
```

#### Configuration Options:
- **Config**: HTTPx runner configuration
- **Root Target URL**: Primary target for the scan
- **Logger**: Structured logging instance

## Advanced Usage

### Custom Configuration

```go
config := &httpxrunner.Config{
    Threads:              100,
    Timeout:              30,
    Retries:              3,
    RateLimit:            200,
    Method:               "GET",
    FollowRedirects:      true,
    MaxRedirects:         10,
    ExtractStatusCode:    true,
    ExtractContentType:   true,
    ExtractHeaders:       true,
    TechDetect:          true,
    CustomHeaders: map[string]string{
        "User-Agent": "Custom-Scanner/1.0",
        "Accept":     "*/*",
    },
}
```

### Error Handling

```go
runner, err := httpxrunner.NewRunnerBuilder(logger).
    WithConfig(config).
    WithRootTargetURL(rootURL).
    Build()

if err != nil {
    logger.Error().Err(err).Msg("Failed to build runner")
    return err
}

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

if err := runner.Run(ctx); err != nil {
    logger.Error().Err(err).Msg("Runner execution failed")
    return err
}
```

### Result Processing

```go
results := runner.GetResults()
for _, result := range results {
    logger.Info().
        Str("url", result.InputURL).
        Int("status", result.StatusCode).
        Str("title", result.Title).
        Msg("Probe result")
}
```

## Integration with MonsterInc

### Scanner Integration

```go
// In scanner workflow
httpxRunner, err := httpxrunner.NewRunnerBuilder(logger).
    WithConfig(globalConfig.HttpxRunnerConfig).
    WithRootTargetURL(target).
    Build()

if err != nil {
    return nil, fmt.Errorf("failed to create httpx runner: %w", err)
}

if err := httpxRunner.Run(ctx); err != nil {
    return nil, fmt.Errorf("httpx execution failed: %w", err)
}

probeResults := httpxRunner.GetResults()
```

### Configuration Loading

```go
// Convert global config to httpx config
httpxConfig := &httpxrunner.Config{
    Threads:              globalConfig.HttpxRunnerConfig.Threads,
    Timeout:              globalConfig.HttpxRunnerConfig.TimeoutSecs,
    ExtractStatusCode:    globalConfig.HttpxRunnerConfig.ExtractStatusCode,
    ExtractContentType:   globalConfig.HttpxRunnerConfig.ExtractContentType,
    TechDetect:          globalConfig.HttpxRunnerConfig.TechDetect,
    CustomHeaders:        globalConfig.HttpxRunnerConfig.CustomHeaders,
}
```

## Dependencies

- `github.com/projectdiscovery/httpx/runner`: Core httpx functionality
- `github.com/rs/zerolog`: Structured logging
- `context`: Context handling for cancellation
- `sync`: Thread safety with mutexes

## Performance Considerations

- **Concurrency**: Configurable thread count for parallel requests
- **Rate Limiting**: Built-in rate limiting to avoid overwhelming targets
- **Memory Management**: Efficient result collection and storage
- **Context Cancellation**: Proper cleanup on cancellation
- **Resource Cleanup**: Automatic cleanup of resources after execution

## Testing

The package includes comprehensive test coverage for:

- Configuration conversion and validation
- Result mapping from various httpx outputs
- Concurrent result collection
- Builder pattern functionality
- Error handling scenarios
- Integration with httpx runner 