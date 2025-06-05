# URLHandler Package

Package `urlhandler` cung cấp các utility functions và managers cho việc xử lý URLs và targets trong MonsterInc application.

## Core Functions

### URL Normalization và Validation

```go
// Normalize URL (thêm scheme, lowercase domain)
normalizedURL, err := urlhandler.NormalizeURL("example.com")
// => "https://example.com"

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
// => "https://example.com/other"

// Works with absolute URLs too
resolved, err := urlhandler.ResolveURL("https://other.com", baseURL)
// => "https://other.com"
```

### Hostname Extraction và Management

```go
// Extract hostname with port
hostname, err := urlhandler.ExtractHostnameWithPort("https://example.com:8080/path")
// => "example.com:8080"

// Extract hostname without port
hostname, err := urlhandler.ExtractHostname("https://example.com:8080/path")
// => "example.com"

// Get base domain
baseDomain, err := urlhandler.GetBaseDomain("sub.example.com")
// => "example.com"

// Compare hostnames with case sensitivity options
same := urlhandler.CompareHostnames("Example.COM", "example.com", false) // true
same = urlhandler.CompareHostnames("Example.COM", "example.com", true)  // false
```

### Filename Sanitization

```go
// Sanitize for general filenames
safe := urlhandler.SanitizeFilename("https://example.com/path?param=value")
// => "example_com_path_param_value"

// Sanitize hostname:port for filenames
safe := urlhandler.SanitizeHostnamePort("example.com:8080")
// => "example_com_8080"

// Restore from sanitized
restored := urlhandler.RestoreHostnamePort("example_com_8080")
// => "example.com:8080"
```

### Batch Operations

```go
// Normalize multiple URLs
urls := []string{"example.com", "test.org", "invalid-url"}
normalized, err := urlhandler.NormalizeURLSlice(urls)
// Returns normalized valid URLs and error for invalid ones
```

## Target Management

### TargetManager

```go
// Create target manager
tm := urlhandler.NewTargetManager(logger)

// Load targets from various sources with priority
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
// Supports:
// - Comment lines (starting with #)
// - Empty lines (skipped)
// - Automatic URL validation
// - Detailed logging
```

## Integration Examples

### Scanner Integration

```go
// In scanner
tm := urlhandler.NewTargetManager(logger)
targets, source, err := tm.LoadAndSelectTargets(flags.ScanTargetsFile, cfg.InputConfig.InputURLs, cfg.InputConfig.InputFile)
if err != nil {
    return err
}

seedURLs := tm.GetTargetStrings(targets)
```

### Crawler Integration

```go
// In crawler discovery
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

### Extractor Integration

```go
// In path extractor
result := validator.ValidateAndResolveURL(rawPath, baseURL, sourceURL)
if !result.IsValid {
    logger.Debug().Err(result.Error).Msg("Invalid URL")
    continue
}
```

### Datastore Integration

```go
// In file path generation
hostnamePort, err := urlhandler.ExtractHostnameWithPort(recordURL)
if err != nil {
    return "", err
}

sanitizedPath := urlhandler.SanitizeHostnamePort(hostnamePort)
```

## Best Practices

1. **Always validate URLs** trước khi process:
   ```go
   if err := urlhandler.ValidateURLFormat(rawURL); err != nil {
       // Handle error
   }
   ```

2. **Use normalization** cho consistent comparison:
   ```go
   normalized, err := urlhandler.NormalizeURL(userInput)
   ```

3. **Leverage TargetManager** cho input handling:
   ```go
   tm := urlhandler.NewTargetManager(logger)
   targets, source, err := tm.LoadAndSelectTargets(...)
   ```

4. **Use resolution helpers** thay vì manual parsing:
   ```go
   // Good
   resolved, err := urlhandler.ResolveURL(href, baseURL)
   
   // Avoid manual url.Parse + ResolveReference
   ```

5. **Sanitize filenames** khi lưu files:
   ```go
   safeFilename := urlhandler.SanitizeFilename(urlString)
   ```

## Error Handling

Package sử dụng `internal/common` error types:

- `common.NewError()` - General errors
- `common.WrapError()` - Wrapped errors với context
- `common.NewValidationError()` - Validation errors

```go
normalizedURL, err := urlhandler.NormalizeURL("")
if err != nil {
    // err will be common.Error with descriptive message
    logger.Error().Err(err).Msg("Failed to normalize URL")
}
``` 