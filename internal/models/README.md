# Models Package

The models package defines the core data structures and interfaces for MonsterInc's security scanning and monitoring system. It provides strongly-typed models that ensure data consistency across web crawling, HTTP probing, content monitoring, and reporting components.

## Package Role in MonsterInc
As the data contract foundation, this package:
- **Type Safety**: Ensures consistent data structures across all components
- **API Contracts**: Defines interfaces for component communication
- **Data Validation**: Provides validation patterns for security data
- **Serialization**: Supports JSON/Parquet serialization for persistence
- **Integration Bridge**: Enables seamless data flow between packages

## Overview

This package defines the core data models that flow between different components of the MonsterInc system:
- **Probe Results**: HTTP probing and scanning data
- **File History**: Content change tracking
- **Notifications**: Discord messaging structures
- **Content Diffs**: Change detection and reporting
- **Assets & Targets**: Web crawling discoveries
- **Reports**: HTML report generation data

## File Structure

### Core Data Models

- **`probe_result.go`** - HTTP probing results and technology detection
- **`file_history.go`** - File change tracking and history storage
- **`content_diff.go`** - Content comparison and diff results
- **`extracted_path.go`** - Path extraction from JavaScript/HTML
- **`asset.go`** - Web assets discovered during crawling
- **`target.go`** - URL targets for scanning
- **`monitored_file.go`** - File monitoring structures

### Notification Models

- **`notification_models.go`** - Discord webhook and embed structures
- **`scan_summary_builder.go`** - Scan summary construction

### Report Models

- **`report_data.go`** - HTML report generation data structures
- **`url_diff.go`** - URL comparison and diffing results

### Persistence Models

- **`parquet_schema.go`** - Apache Parquet serialization schemas
- **`interfaces.go`** - Common interfaces and contracts

## Core Data Models

### 1. ProbeResult

HTTP probing and security scanning results:

```go
type ProbeResult struct {
    InputURL            string            `json:"input_url"`
    FinalURL            string            `json:"final_url,omitempty"`
    StatusCode          int               `json:"status_code,omitempty"`
    ContentType         string            `json:"content_type,omitempty"`
    Headers             map[string]string `json:"headers,omitempty"`
    Technologies        []Technology      `json:"technologies,omitempty"`
    Duration            float64           `json:"duration,omitempty"`
    Method              string            `json:"method"`
    RootTargetURL       string            `json:"root_target_url,omitempty"`
    URLStatus           string            `json:"url_status,omitempty"` // "new", "old", "existing"
    Timestamp           time.Time         `json:"timestamp"`
    // Network information
    IPs                 []string          `json:"ips,omitempty"`
    ASN                 int               `json:"asn,omitempty"`
    ASNOrg              string            `json:"asn_org,omitempty"`
}
```

**Usage:**
```go
// Create probe result
result := models.ProbeResult{
    InputURL:    "https://example.com",
    StatusCode:  200,
    ContentType: "text/html",
    Timestamp:   time.Now(),
}

// Check for technologies
if result.HasTechnologies() {
    for _, tech := range result.Technologies {
        fmt.Printf("Found: %s %s\n", tech.Name, tech.Version)
    }
}
```

### 2. FileHistoryRecord

File change tracking and content storage:

```go
type FileHistoryRecord struct {
    URL            string  `parquet:"url,zstd"`
    Timestamp      int64   `parquet:"timestamp,zstd"`
    Hash           string  `parquet:"hash,zstd"`
    ContentType    string  `parquet:"content_type,zstd,optional"`
    Content        []byte  `parquet:"content,zstd,optional"`
    ETag           string  `parquet:"etag,zstd,optional"`
    LastModified   string  `parquet:"last_modified,zstd,optional"`
    DiffResultJSON *string `parquet:"diff_result_json,zstd,optional"`
    ExtractedPathsJSON *string `parquet:"extracted_paths_json,zstd,optional"`
}
```

**Interface:**
```go
type FileHistoryStore interface {
    GetLastKnownRecord(url string) (*FileHistoryRecord, error)
    GetLastKnownHash(url string) (string, error)
    StoreFileRecord(record FileHistoryRecord) error
    GetRecordsForURL(url string, limit int) ([]*FileHistoryRecord, error)
    GetHostnamesWithHistory() ([]string, error)
}
```

### 3. ContentDiffResult

Content change detection and analysis:

```go
type ContentDiffResult struct {
    Timestamp        int64           `json:"timestamp"`
    ContentType      string          `json:"content_type"`
    Diffs            []ContentDiff   `json:"diffs"`
    LinesAdded       int             `json:"lines_added"`
    LinesDeleted     int             `json:"lines_deleted"`
    LinesChanged     int             `json:"lines_changed"`
    IsIdentical      bool            `json:"is_identical"`
    ProcessingTimeMs int64           `json:"processing_time_ms"`
    OldHash          string          `json:"old_hash,omitempty"`
    NewHash          string          `json:"new_hash,omitempty"`
    ExtractedPaths   []ExtractedPath `json:"extracted_paths,omitempty"`
}

type ContentDiff struct {
    Operation DiffOperation `json:"operation"` // Insert, Delete, Equal
    Text      string        `json:"text"`
}
```

### 4. ExtractedPath

Path extraction from JavaScript and HTML:

```go
type ExtractedPath struct {
    SourceURL            string    `json:"source_url"`
    ExtractedRawPath     string    `json:"extracted_raw_path"`
    ExtractedAbsoluteURL string    `json:"extracted_absolute_url"`
    Context              string    `json:"context"` // e.g., "script[src]", "JS:fetch"
    Type                 string    `json:"type"`    // e.g., "html_attr_link", "js_string"
    DiscoveryTimestamp   time.Time `json:"discovery_timestamp"`
}
```

## Notification Models

### Discord Integration

```go
type DiscordMessagePayload struct {
    Content         string           `json:"content,omitempty"`
    Username        string           `json:"username,omitempty"`
    Embeds          []DiscordEmbed   `json:"embeds,omitempty"`
    AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"`
}

type DiscordEmbed struct {
    Title       string              `json:"title,omitempty"`
    Description string              `json:"description,omitempty"`
    Color       int                 `json:"color,omitempty"`
    Fields      []DiscordEmbedField `json:"fields,omitempty"`
    Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
    Timestamp   string              `json:"timestamp,omitempty"`
}
```

**Builder Pattern:**
```go
// Build Discord embed
embed := models.NewDiscordEmbedBuilder().
    WithTitle("🔍 Scan Complete").
    WithDescription("Security scan finished successfully").
    WithSuccessColor().
    AddField("Total URLs", "150", true).
    AddField("New URLs", "25", true).
    WithCurrentTimestamp().
    Build()
```

### Scan Summary

```go
type ScanSummaryData struct {
    ScanSessionID    string        `json:"scan_session_id"`
    TargetSource     string        `json:"target_source"`
    ScanMode         string        `json:"scan_mode"`
    Targets          []string      `json:"targets"`
    TotalTargets     int           `json:"total_targets"`
    ProbeStats       ProbeStats    `json:"probe_stats"`
    DiffStats        DiffStats     `json:"diff_stats"`
    ScanDuration     time.Duration `json:"scan_duration"`
    Status           string        `json:"status"`
    ReportPath       string        `json:"report_path"`
}
```

**Builder:**
```go
summary := models.NewScanSummaryDataBuilder().
    WithScanSessionID("20240101-120000").
    WithScanMode("onetime").
    WithTotalTargets(100).
    WithProbeStatsBuilder(
        models.NewProbeStatsBuilder().
            WithTotalProbed(95).
            WithSuccessfulProbes(90).
            WithFailedProbes(5),
    ).
    WithStatus(models.ScanStatusCompleted).
    Build()
```

## Report Models

### Report Page Data

```go
type ReportPageData struct {
    ReportTitle      string                 `json:"report_title"`
    GeneratedAt      string                 `json:"generated_at"`
    ProbeResults     []ProbeResultDisplay   `json:"probe_results"`
    TotalResults     int                    `json:"total_results"`
    SuccessResults   int                    `json:"success_results"`
    FailedResults    int                    `json:"failed_results"`
    URLDiffs         map[string]URLDiffResult `json:"url_diffs,omitempty"`
    DiffSummaryData  map[string]DiffSummaryEntry `json:"diff_summary_data"`
    EnableDataTables bool                   `json:"enable_data_tables"`
    CustomCSS        template.CSS           `json:"-"`
    ReportJs         template.JS            `json:"-"`
}
```

### Diff Report Data

```go
type DiffReportPageData struct {
    ReportTitle      string              `json:"report_title"`
    GeneratedAt      string              `json:"generated_at"`
    DiffResults      []DiffResultDisplay `json:"diff_results"`
    TotalDiffs       int                 `json:"total_diffs"`
    EnableDataTables bool                `json:"enable_data_tables"`
}
```

## Interfaces

### Core Interfaces

```go
// Validation interface
type Validator interface {
    Validate() error
}

// Timestamp interface
type Timestamped interface {
    GetTimestamp() time.Time
}

// Content provider interface
type ContentProvider interface {
    GetContent() []byte
    GetContentType() string
}

// Error provider interface
type ErrorProvider interface {
    GetError() string
    HasError() bool
}

// Technology detector interface
type TechnologyDetector interface {
    HasTechnologies() bool
    GetTechnologies() []Technology
}
```

## Usage Examples

### Creating Probe Results

```go
// Basic probe result
result := models.ProbeResult{
    InputURL:    "https://example.com/api",
    FinalURL:    "https://example.com/api/v1",
    StatusCode:  200,
    ContentType: "application/json",
    Method:      "GET",
    Timestamp:   time.Now(),
}

// With technologies
result.Technologies = []models.Technology{
    {Name: "nginx", Version: "1.18.0", Category: "web server"},
    {Name: "Express", Version: "4.17.1", Category: "web framework"},
}
```

### File History Tracking

```go
// Create history record
record := models.FileHistoryRecord{
    URL:         "https://example.com/app.js",
    Timestamp:   time.Now().UnixMilli(),
    Hash:        "sha256:abc123...",
    ContentType: "application/javascript",
    Content:     jsContent,
}

// Store in history
err := historyStore.StoreFileRecord(record)
```

### Building Notifications

```go
// Create file change notification
changes := []models.FileChangeInfo{
    {
        URL:         "https://example.com/app.js",
        OldHash:     "abc123",
        NewHash:     "def456",
        ContentType: "application/javascript",
        ChangeTime:  time.Now(),
    },
}

// Build Discord payload
payload := notifier.FormatAggregatedFileChangesMessage(changes, cfg)
```

### Report Generation

```go
// Convert probe results for display
displayResults := make([]models.ProbeResultDisplay, len(probeResults))
for i, result := range probeResults {
    displayResults[i] = models.ToProbeResultDisplay(result)
}

// Create report page data
pageData := models.ReportPageData{
    ReportTitle:    "Security Scan Report",
    GeneratedAt:    time.Now().Format(time.RFC3339),
    ProbeResults:   displayResults,
    TotalResults:   len(displayResults),
    SuccessResults: countSuccessful(displayResults),
}
```

## Validation Patterns

### Built-in Validators

```go
// Scan summary validation
validator := models.NewScanSummaryValidator()
if err := validator.ValidateSummary(summary); err != nil {
    return fmt.Errorf("invalid summary: %w", err)
}

// Discord embed validation
embedValidator := models.NewDiscordEmbedValidator()
if err := embedValidator.ValidateEmbed(embed); err != nil {
    return fmt.Errorf("invalid embed: %w", err)
}
```

### Custom Validation

```go
// Implement Validator interface
func (pr *ProbeResult) Validate() error {
    if pr.InputURL == "" {
        return errors.New("input URL is required")
    }
    if pr.StatusCode < 100 || pr.StatusCode > 599 {
        return errors.New("invalid HTTP status code")
    }
    return nil
}
```

## Parquet Serialization

### Schema Definition

```go
type ParquetProbeResult struct {
    OriginalURL   string   `parquet:"original_url"`
    FinalURL      *string  `parquet:"final_url,optional"`
    StatusCode    *int32   `parquet:"status_code,optional"`
    ContentLength *int64   `parquet:"content_length,optional"`
    Technologies  []string `parquet:"technologies,list"`
    ScanTimestamp int64    `parquet:"scan_timestamp"`
    DiffStatus    *string  `parquet:"diff_status,optional"`
}
```

### Conversion Utilities

```go
// Convert to Parquet format
parquetResult := transformer.TransformToParquetResult(probeResult, scanTime)

// Convert from Parquet format
probeResult := parquetResult.ToProbeResult()
```

## Thread Safety

- All model types are designed for concurrent read access
- Builders use method chaining for fluent construction
- Immutable data structures where possible
- Proper synchronization in mutable collections

## Dependencies

- **time** - Timestamp handling
- **html/template** - Template data for reports
- **encoding/json** - JSON serialization
- **github.com/parquet-go/parquet-go** - Parquet serialization tags

## Best Practices

### Model Creation
- Use builders for complex objects
- Validate data at construction time
- Use interfaces for polymorphism
- Keep models simple and focused

### JSON Handling
- Use `omitempty` for optional fields
- Provide clear field names in JSON tags
- Handle nil pointers properly
- Validate deserialized data

### Error Handling
- Implement meaningful error messages
- Use typed errors where appropriate
- Validate inputs at boundaries
- Provide context in error messages