# URL Handler Package

URL normalization, validation, and management utilities for the MonsterInc security scanning pipeline.

## Overview

Provides comprehensive URL handling capabilities:
- **URL Normalization**: Consistent URL formatting and validation
- **Target Management**: Loading and managing scan targets from files/config
- **URL Resolution**: Resolving relative URLs against base URLs
- **Hostname Operations**: Extracting and manipulating hostnames
- **Filename Sanitization**: Safe filename generation from URLs

## Core Functions

### URL Normalization and Validation

```go
// Normalize URL (add scheme, lowercase domain)
normalizedURL, err := urlhandler.NormalizeURL("example.com")
// Result: "https://example.com"

// Validate URL format
err := urlhandler.ValidateURLFormat("https://example.com")

// Check if URL is absolute
isAbs := urlhandler.IsAbsoluteURL("https://example.com") // true
isAbs = urlhandler.IsAbsoluteURL("/path/to/resource")    // false
```

### URL Resolution

```go
// Resolve relative URLs against base
baseURL, _ := url.Parse("https://example.com/page")
resolved, err := urlhandler.ResolveURL("../other", baseURL)
// Result: "https://example.com/other"

// Works with absolute URLs too
resolved, err := urlhandler.ResolveURL("https://other.com", baseURL)
// Result: "https://other.com"
```

### Hostname Operations

```go
// Extract hostname with port
hostname, err := urlhandler.ExtractHostnameWithPort("https://example.com:8080/path")
// Result: "example.com:8080"

// Extract hostname without port
hostname, err := urlhandler.ExtractHostname("https://example.com:8080/path")
// Result: "example.com"

// Get base domain
baseDomain, err := urlhandler.GetBaseDomain("sub.example.com")
// Result: "example.com"

// Compare hostnames
same := urlhandler.CompareHostnames("Example.COM", "example.com", false) // true (case insensitive)
same = urlhandler.CompareHostnames("Example.COM", "example.com", true)  // false (case sensitive)
```

### Filename Sanitization

```go
// Sanitize URL for filename use
safe := urlhandler.SanitizeFilename("https://example.com/path?param=value")
// Result: "example_com_path_param_value"

// Sanitize hostname:port for filenames
safe := urlhandler.SanitizeHostnamePort("example.com:8080")
// Result: "example_com_8080"

// Restore from sanitized
restored := urlhandler.RestoreHostnamePort("example_com_8080")
// Result: "example.com:8080"
```

### Batch Operations

```go
// Normalize multiple URLs
urls := []string{"example.com", "test.org", "invalid-url"}
normalized, err := urlhandler.NormalizeURLSlice(urls)
// Returns normalized valid URLs and errors for invalid ones
```

## Target Management

### TargetManager

Manages loading targets from various sources with priority handling.

```go
// Create target manager
tm := urlhandler.NewTargetManager(logger)

// Load targets with priority: CLI file > config URLs > config file
targets, source, err := tm.LoadAndSelectTargets(
    fileFlag,        // CLI file option (highest priority)
    configURLs,      // Config input_urls 
    configFile,      // Config input_file (lowest priority)
)

// Convert targets to URL strings
urls := tm.GetTargetStrings(targets)
```

### File Operations

```go
// Read URLs from file
urls, err := urlhandler.ReadURLsFromFile("targets.txt", logger)
// Features:
// - Skips comment lines (starting with #)
// - Ignores empty lines
// - Validates each URL
// - Detailed logging
```

## Integration Examples

### With Scanner

```go
// Load targets for scanning
tm := urlhandler.NewTargetManager(logger)
targets, source, err := tm.LoadAndSelectTargets(flags.ScanTargetsFile)
if err != nil {
    return err
}

seedURLs := tm.GetTargetStrings(targets)
```

### With Crawler

```go
// Resolve URLs during crawling
absURL, err := urlhandler.ResolveURL(rawURL, baseURL)
if err != nil {
    logger.Warn().Err(err).Msg("Could not resolve URL")
    return
}

// Validate scope
if err := urlhandler.ValidateURLFormat(absURL); err != nil {
    logger.Debug().Err(err).Msg("Invalid URL format")
    return
}
```

### With Datastore

```go
// Generate safe file paths
hostnamePort, err := urlhandler.ExtractHostnameWithPort(recordURL)
if err != nil {
    return "", err
}

sanitizedPath := urlhandler.SanitizeHostnamePort(hostnamePort)
```

## Error Handling

Common error types:
- **Invalid URL Format**: When URL cannot be parsed
- **Unsupported Scheme**: For non-HTTP/HTTPS schemes  
- **Resolution Errors**: When relative URL resolution fails
- **File Read Errors**: When target files cannot be read

```go
// Handle errors gracefully
normalized, err := urlhandler.NormalizeURL(input)
if err != nil {
    logger.Warn().Err(err).Str("url", input).Msg("Failed to normalize URL")
    continue
}
```

## Best Practices

1. **Always validate URLs** before processing
2. **Use normalization** for consistent comparison
3. **Leverage TargetManager** for input handling
4. **Use resolution helpers** instead of manual parsing
5. **Sanitize filenames** when creating files

## Dependencies

- `net/url`: Standard URL parsing
- `net`: Network address parsing  
- `strings`: String manipulation
- `path/filepath`: File path operations
- `github.com/rs/zerolog`: Structured logging

## Thread Safety

All functions are thread-safe and can be used concurrently.

## Testing

The package includes comprehensive test coverage for:

- URL normalization with various input formats
- Hostname extraction edge cases
- File reading with different content types
- Error conditions and edge cases
- Target loading from multiple sources

## Performance Considerations

- URL operations are generally fast (O(1) complexity)
- File reading scales linearly with file size
- Batch operations provide better performance than individual calls
- Hostname extraction uses optimized parsing methods 
