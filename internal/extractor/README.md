# Extractor Package

## Purpose
The `extractor` package provides comprehensive path and URL extraction capabilities from web content, particularly JavaScript and HTML files. It supports multiple extraction methods and provides intelligent filtering and validation of discovered paths.

## Main Components

### 1. Path Extractor (`path_extractor.go`)
#### Purpose
- Extract URLs and paths from web content (HTML, JavaScript, CSS)
- Support multiple extraction methods (JSluice, manual regex)
- Validate and resolve discovered URLs
- Apply allowlist/denylist filtering

#### API Usage

```go
// Create path extractor
extractor, err := extractor.NewPathExtractor(extractorConfig, logger)
if err != nil {
    return err
}

// Extract paths from JavaScript content
jsContent := []byte(`
    fetch('/api/users');
    window.location = 'https://example.com/dashboard';
    var endpoint = '/api/data?param=value';
`)

paths, err := extractor.ExtractPaths(
    "https://example.com/app.js",
    jsContent,
    "application/javascript",
)

if err != nil {
    return err
}

// Process extracted paths
for _, path := range paths {
    fmt.Printf("Source: %s\n", path.SourceURL)
    fmt.Printf("Extracted: %s\n", path.ExtractedAbsoluteURL)
    fmt.Printf("Type: %s\n", path.Type)
    fmt.Printf("Context: %s\n", path.Context)
}
```

#### Builder Pattern

```go
// Create extractor with builder
builder := extractor.NewPathExtractorBuilder(logger).
    WithExtractorConfig(config.ExtractorConfig{
        Allowlist:     []string{"api.*", "dashboard.*"},
        Denylist:      []string{"tracking.*", "analytics.*"},
        CustomRegexes: []string{`api/v\d+/[^"']+`},
    }).
    WithPathExtractorConfig(extractor.PathExtractorConfig{
        EnableJSluiceAnalysis: true,
        EnableManualRegex:     true,
        MaxContentSize:        1024 * 1024, // 1MB
        ContextSnippetSize:    50,
    })

pathExtractor, err := builder.Build()
```

### 2. JSluice Analyzer (`jsluice_analyzer.go`)
#### Purpose
- Advanced JavaScript analysis using JSluice library
- Extract URLs from JavaScript literals, fetch calls, and dynamic code
- Provide context information for discovered URLs
- Handle obfuscated and minified JavaScript

#### API Usage

```go
// Create JSluice analyzer
analyzer := extractor.NewJSluiceAnalyzer(validator, contextExtractor, logger)

// Analyze JavaScript content
baseURL, _ := url.Parse("https://example.com")
seenPaths := make(map[string]struct{})

result := analyzer.AnalyzeJavaScript(
    "https://example.com/app.js",
    jsContent,
    baseURL,
    seenPaths,
)

fmt.Printf("Extracted %d paths\n", result.ProcessedCount)
for _, path := range result.ExtractedPaths {
    fmt.Printf("Found: %s (Context: %s)\n", path.ExtractedAbsoluteURL, path.Context)
}
```

### 3. Manual Regex Analyzer (`manual_regex_analyzer.go`)
#### Purpose
- Pattern-based URL extraction using custom regular expressions
- Support for user-defined extraction patterns
- Fallback method when JSluice analysis fails
- Configurable regex patterns for specific use cases

#### API Usage

```go
// Compile custom regexes
regexCompiler := extractor.NewRegexCompiler(logger)
regexSet := regexCompiler.CompileRegexSets(extractorConfig)

// Create manual regex analyzer
analyzer := extractor.NewManualRegexAnalyzer(
    regexSet.CustomRegexes,
    validator,
    contextExtractor,
    logger,
)

// Analyze content with regex patterns
result := analyzer.AnalyzeWithRegex(
    "https://example.com/script.js",
    content,
    baseURL,
    seenPaths,
)
```

### 4. URL Validator (`url_validator.go`)
#### Purpose
- Validate and resolve discovered URLs
- Convert relative URLs to absolute URLs
- Validate URL format and accessibility
- Provide detailed validation results

#### API Usage

```go
// Create URL validator
validator := extractor.NewURLValidator(logger)

// Validate and resolve URL
baseURL, _ := url.Parse("https://example.com/page")
result := validator.ValidateAndResolveURL("/api/data", baseURL, "https://example.com/app.js")

if result.IsValid {
    fmt.Printf("Resolved URL: %s\n", result.AbsoluteURL)
} else {
    fmt.Printf("Invalid URL: %v\n", result.Error)
}

// Validate absolute URL
result = validator.ValidateAndResolveURL("https://api.example.com/data", nil, "")
```

## Configuration

### Extractor Configuration

```yaml
extractor_config:
  allowlist:
    - "api.*"
    - "dashboard.*"
    - "admin.*"
  denylist:
    - "tracking.*"
    - "analytics.*"
    - "cdn.*"
  custom_regexes:
    - 'api/v\d+/[^"'']+' 
    - 'endpoint:\s*["\']([^"'']+)'
    - 'fetch\(["\']([^"'']+)'
```

### Path Extractor Configuration

```go
config := extractor.PathExtractorConfig{
    EnableJSluiceAnalysis: true,        // Enable JSluice analysis
    EnableManualRegex:     true,        // Enable manual regex extraction
    MaxContentSize:        1024 * 1024, // Maximum content size (1MB)
    ContextSnippetSize:    50,          // Context snippet size around matches
}
```

## Features

### 1. Multi-Method Extraction
- **JSluice Analysis**: Advanced JavaScript parsing for complex patterns
- **Regex Patterns**: Custom regular expression matching
- **Content Type Detection**: Automatic method selection based on content type
- **Fallback Support**: Multiple methods ensure comprehensive extraction

### 2. URL Processing
- **Validation**: Comprehensive URL format validation
- **Resolution**: Convert relative URLs to absolute URLs
- **Normalization**: Consistent URL formatting
- **Deduplication**: Remove duplicate discoveries

### 3. Filtering and Validation
- **Allowlist/Denylist**: Pattern-based filtering of discovered URLs
- **Content Size Limits**: Prevent processing of oversized content
- **Context Extraction**: Provide surrounding context for discoveries
- **Error Handling**: Graceful handling of invalid or inaccessible URLs

### 4. Content Type Support
- **JavaScript**: Advanced analysis of JS files and inline scripts
- **HTML**: Extract URLs from HTML attributes and embedded scripts
- **CSS**: Extract URLs from CSS files and style attributes
- **Text**: Generic text-based URL extraction

## Data Models

### ExtractedPath

```go
type ExtractedPath struct {
    SourceURL            string    `json:"source_url"`
    ExtractedRawPath     string    `json:"extracted_raw_path"`
    ExtractedAbsoluteURL string    `json:"extracted_absolute_url"`
    Context              string    `json:"context"`
    Type                 string    `json:"type"`
    DiscoveryTimestamp   time.Time `json:"discovery_timestamp"`
}
```

### ValidationResult

```go
type ValidationResult struct {
    AbsoluteURL string
    IsValid     bool
    Error       error
}
```

### AnalysisResult

```go
type AnalysisResult struct {
    ExtractedPaths []models.ExtractedPath
    ProcessedCount int
}
```

## Integration Examples

### Monitor Integration

```go
// In monitoring service - extract paths from JavaScript files
pathExtractor, err := extractor.NewPathExtractor(extractorConfig, logger)
if err != nil {
    return err
}

// Check if content should be analyzed
if shouldExtractPaths(url, contentType) {
    paths, err := pathExtractor.ExtractPaths(url, content, contentType)
    if err != nil {
        logger.Error().Err(err).Msg("Path extraction failed")
    } else {
        logger.Info().
            Int("extracted_count", len(paths)).
            Str("source_url", url).
            Msg("Paths extracted from content")
        
        // Store extracted paths
        for _, path := range paths {
            storeExtractedPath(path)
        }
    }
}
```

### Crawler Integration

```go
// In crawler - discover additional URLs from JavaScript
if isJavaScriptContent(contentType) {
    paths, err := pathExtractor.ExtractPaths(pageURL, content, contentType)
    if err != nil {
        logger.Warn().Err(err).Msg("Failed to extract paths")
    } else {
        // Queue discovered URLs for crawling
        for _, path := range paths {
            if shouldCrawlURL(path.ExtractedAbsoluteURL) {
                crawler.DiscoverURL(path.ExtractedAbsoluteURL, baseURL)
            }
        }
    }
}
```

## Advanced Usage

### Custom Regex Patterns

```go
// Define custom extraction patterns
customRegexes := []string{
    `api/v\d+/[^"'\s]+`,                    // API endpoints
    `endpoint:\s*["']([^"']+)["']`,         // Configuration endpoints
    `fetch\(["']([^"']+)["']\)`,           // Fetch calls
    `xhr\.open\(["'][^"']*["'],\s*["']([^"']+)["']\)`, // XHR requests
}

config := config.ExtractorConfig{
    CustomRegexes: customRegexes,
    Allowlist:     []string{"api.*", "graphql.*"},
    Denylist:      []string{"tracking.*", "analytics.*"},
}
```

### Content Type Analysis

```go
// Create content type analyzer
analyzer := extractor.NewContentTypeAnalyzer(extractorConfig, logger)

// Check if content should be analyzed with JSluice
shouldAnalyze := analyzer.ShouldAnalyzeWithJSluice(sourceURL, contentType)

if shouldAnalyze {
    // Perform advanced JavaScript analysis
    paths, err := pathExtractor.ExtractPaths(sourceURL, content, contentType)
}
```

### Context Extraction

```go
// Create context extractor
contextExtractor := extractor.NewContextExtractor(50, logger) // 50 character snippets

// Extract context around discovered URLs
context := contextExtractor.ExtractContext(contentString, matchedURL)
fmt.Printf("Context: %s\n", context)
```

## Error Handling

### Common Error Types

- **Content Too Large**: When content exceeds size limits
- **Invalid URL Format**: For malformed URLs
- **Regex Compilation Error**: When custom regex patterns are invalid
- **Context Cancellation**: When operations are cancelled

### Error Examples

```go
// Handle content size limits
paths, err := pathExtractor.ExtractPaths(sourceURL, content, contentType)
if err != nil {
    if errors.Is(err, extractor.ErrContentTooLarge) {
        logger.Warn().
            Int("content_size", len(content)).
            Msg("Content too large for path extraction")
        // Skip or use alternative method
    } else {
        logger.Error().Err(err).Msg("Path extraction failed")
        return err
    }
}

// Handle validation errors
result := validator.ValidateAndResolveURL(rawURL, baseURL, sourceURL)
if !result.IsValid {
    logger.Debug().
        Err(result.Error).
        Str("raw_url", rawURL).
        Str("source", sourceURL).
        Msg("URL validation failed")
    continue // Skip invalid URL
}
```

## Dependencies

- `github.com/BishopFox/jsluice`: JavaScript analysis
- `github.com/rs/zerolog`: Structured logging
- `regexp`: Regular expression processing
- `net/url`: URL parsing and resolution
- `context`: Context handling for cancellation
- Internal packages:
  - `internal/models`: Data models
  - `internal/config`: Configuration structures

## Performance Considerations

- **Content Size Limits**: Configurable maximum content size
- **Regex Optimization**: Compiled regex patterns for performance
- **Memory Management**: Efficient handling of large content
- **Context Cancellation**: Proper cleanup on cancellation
- **Deduplication**: Efficient tracking of seen URLs

## Testing

The package includes comprehensive test coverage for:

- Path extraction from various JavaScript patterns
- URL validation and resolution scenarios
- Regex pattern matching accuracy
- Content type detection
- Error handling for edge cases
- Performance with large content files
- Integration with different content types 