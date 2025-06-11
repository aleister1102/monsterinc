# Reporter Package

The reporter package provides comprehensive HTML report generation for MonsterInc's security analysis pipeline. It creates interactive, professional reports with embedded assets, data visualization, and responsive design for security scan results, content diff analysis, and monitoring findings.

## Package Role in MonsterInc
As the reporting engine, this package:
- **Security Documentation**: Creates detailed reports for security findings
- **Visual Analysis**: Provides interactive data visualization for scan results
- **Diff Reporting**: Generates side-by-side content comparison reports
- **Professional Output**: Delivers client-ready security assessment reports
- **Integration Ready**: Seamlessly works with Scanner, Monitor, and Notifier

## Overview

The reporter package generates two main types of reports:
- **Scan Reports**: Interactive HTML reports for security scanning results
- **Diff Reports**: Content difference reports for file monitoring
- **Asset Management**: CSS/JS/Image embedding and external asset handling
- **Template Engine**: Go template-based report generation with custom functions

## Key Features

### Client-Side Rendering Optimization

The reporter now intelligently chooses between server-side and client-side rendering templates to optimize file sizes:

#### Client-Side Template (`diff_report_client_side.html.tmpl`) - Default
- Full HTML rendering on server
- Suitable for small to medium reports
- Better for offline viewing
- Used when:
  - ≤15 diff results
  - Total diff content <80KB
  - Individual diff <40KB
  - Single-part reports

#### Client-Side Template (`diff_report_client_side.html.tmpl`)
- Minimal HTML skeleton with JavaScript rendering
- JSON data embedded for client processing
- 30-50% smaller file sizes
- Used when:
  - >15 diff results
  - Total diff content ≥80KB
  - Individual diff ≥40KB
  - Multi-part reports

### File Size Management

Automatic splitting mechanism ensures reports stay under Discord's 10MB limit:

- **Server-side template**: 50% safety margin (5MB target)
- **Client-side template**: 70% safety margin (7MB target) 
- Iterative size checking with aggressive re-splitting if needed
- Dynamic chunk size adjustment based on actual file sizes

### Template Selection Logic

```go
func selectOptimalTemplate(pageData models.DiffReportPageData) string {
    // Factors considered:
    // 1. Number of diff results
    // 2. Total content size
    // 3. Individual diff sizes
    // 4. Multi-part report status
    
    if useClientSide {
        return "diff_report_client_side.html.tmpl"
    }
    return "diff_report_client_side.html.tmpl"
}
```

## File Structure

### Core Components

- **`html_reporter.go`** - Main scan report generator
- **`html_diff_reporter.go`** - Content diff report generator
- **`asset_manager.go`** - CSS/JS/Image asset management
- **`directory_manager.go`** - File and directory operations
- **`template_functions.go`** - Custom template functions
- **`diff_utils.go`** - Diff processing utilities

### Templates

- **`templates/report.html.tmpl`** - Main scan report template
- **`templates/diff_report_client_side.html.tmpl`** - Content diff report template (default)

- **`templates/diff_report_client_side.html.tmpl`** - Client-side template

### Assets

- **`assets/css/styles.css`** - Report styling
- **`assets/js/report.js`** - Interactive functionality
- **`assets/img/favicon.ico`** - Report favicon

## Features

### 1. Interactive Scan Reports

**Key Features:**
- Responsive design with Bootstrap
- Sortable data tables with search/filtering
- Technology detection display
- Status code highlighting
- URL diff visualization
- Multi-target support
- Dark/light theme support

**Components:**
```go
type ReportPageData struct {
    ReportTitle      string
    GeneratedAt      string
    ProbeResults     []ProbeResultDisplay
    URLDiffs         map[string]URLDiffResult
    EnableDataTables bool
    CustomCSS        template.CSS
    ReportJs         template.JS
}
```

### 2. Content Diff Reports

**Features:**
- Side-by-side diff visualization
- Syntax highlighting for different content types
- Change statistics (lines added/removed/modified)
- Full content display option
- Path extraction results for JavaScript
- Responsive design

**Data Structure:**
```go
type DiffReportPageData struct {
    ReportTitle      string
    GeneratedAt      string
    DiffResults      []DiffResultDisplay
    TotalDiffs       int
    EnableDataTables bool
}
```

### 3. Asset Management

**Capabilities:**
- Embed CSS/JS directly in HTML
- External asset linking for development
- Asset compression and minification
- Base64 favicon embedding
- Automatic asset copying

## Usage Examples

### Basic Scan Report Generation

```go
import (
    "github.com/aleister1102/monsterinc/internal/reporter"
    "github.com/aleister1102/monsterinc/internal/models"
)

// Create reporter
htmlReporter := reporter.NewHtmlReporter(cfg.ReporterConfig, logger)

// Prepare report data
pageData := models.ReportPageData{
    ReportTitle:    "Security Scan Report",
    GeneratedAt:    time.Now().Format(time.RFC3339),
    ProbeResults:   probeResultsDisplay,
    TotalResults:   len(probeResults),
    SuccessResults: countSuccessful(probeResults),
    URLDiffs:       urlDiffResults,
}

// Generate report
outputPath, err := htmlReporter.GenerateReport(pageData, "scan-report.html")
if err != nil {
    return fmt.Errorf("report generation failed: %w", err)
}
```

### Content Diff Report Generation

```go
// Create diff reporter
diffReporter := reporter.NewHtmlDiffReporter(
    historyStore,
    logger,
    notificationHelper,
)

// Generate individual diff report
reportPath := diffReporter.GenerateSingleDiffReport(
    url,
    diffResult,
    lastRecord,
    processedUpdate,
    fetchResult,
)

// Generate aggregated diff report
aggregatedPath := diffReporter.GenerateAggregatedDiffReport(changes)
```

### Asset Management

```go
// Create asset manager
assetManager := reporter.NewAssetManager(logger)

// Embed assets in page data
assetManager.EmbedAssetsIntoPageData(
    &pageData,
    cssFS,   // Embedded CSS filesystem
    jsFS,    // Embedded JS filesystem
    true,    // Enable asset embedding
)

// Copy external assets
err := assetManager.CopyEmbedDir(
    assetsFS,
    "assets",
    "/path/to/output/assets",
)
```

## Configuration

### Reporter Configuration

```yaml
reporter_config:
  output_dir: "./reports"
  template_path: ""                    # Use embedded templates
  embed_assets: true                   # Embed CSS/JS in HTML
  enable_data_tables: true             # Enable DataTables.js
  generate_empty_report: false         # Generate reports even with no data
  items_per_page: 25                   # Items per page for pagination
  max_probe_results_per_report_file: 5000  # Split large reports
  report_title: "MonsterInc Security Scan"
```

### Configuration Options

- **`output_dir`**: Directory for generated reports
- **`embed_assets`**: Whether to embed CSS/JS in HTML (vs external files)
- **`enable_data_tables`**: Enable interactive data tables
- **`items_per_page`**: Number of items per page in reports
- **`max_probe_results_per_report_file`**: Split large reports into multiple files

## Templates

### Report Template Structure

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{.ReportTitle}}</title>
    {{if .EnableDataTables}}
        <!-- DataTables CSS/JS includes -->
    {{end}}
    <style>{{.CustomCSS}}</style>
</head>
<body>
    <!-- Navigation and filtering -->
    <nav class="navbar">
        <div class="container-fluid">
            <h1>{{.ReportTitle}}</h1>
            <span>Generated: {{.GeneratedAt}}</span>
        </div>
    </nav>

    <!-- Statistics cards -->
    <div class="container-fluid">
        <div class="row">
            <div class="col-md-3">
                <div class="card">
                    <h5>Total Results</h5>
                    <h2>{{.TotalResults}}</h2>
                </div>
            </div>
            <!-- More stats cards -->
        </div>
    </div>

    <!-- Results table -->
    <table id="probeResultsTable" class="table">
        <thead>
            <tr>
                <th>URL</th>
                <th>Status</th>
                <th>Content Type</th>
                <th>Technologies</th>
                <!-- More columns -->
            </tr>
        </thead>
        <tbody>
            {{range .ProbeResults}}
            <tr class="{{statusRowClass .StatusCode}}">
                <td>{{truncateURL .InputURL 50}}</td>
                <td><span class="badge {{statusBadgeClass .StatusCode}}">{{.StatusCode}}</span></td>
                <!-- More cells -->
            </tr>
            {{end}}
        </tbody>
    </table>

    <script>{{.ReportJs}}</script>
</body>
</html>
```

### Custom Template Functions

```go
// Template function registration
funcMap := template.FuncMap{
    "statusRowClass":    getStatusRowClass,
    "statusBadgeClass":  getStatusBadgeClass,
    "truncateURL":       truncateURL,
    "formatDuration":    formatDuration,
    "formatTimestamp":   formatTimestamp,
    "containsText":      strings.Contains,
    "joinStrings":       strings.Join,
    "minInt":            MinInt,
}
```

**Available Functions:**
- `statusRowClass`: CSS class based on HTTP status code
- `statusBadgeClass`: Badge style for status codes
- `truncateURL`: Truncate long URLs with ellipsis
- `formatDuration`: Format duration in human-readable format
- `formatTimestamp`: Format timestamps for display
- `minInt`: Return minimum of two integers

## Report Types

### 1. Scan Result Report

**Structure:**
- Header with scan statistics
- Filter and search functionality
- Sortable results table
- Technology detection display
- Status code highlighting
- Responsive design

**Data Flow:**
```
ProbeResults → ProbeResultDisplay → Template → HTML Report
```

### 2. Diff Result Report

**Structure:**
- Summary of changes
- Individual diff sections
- Side-by-side comparison
- Change statistics
- Syntax highlighting

**Data Flow:**
```
ContentDiffResult → DiffResultDisplay → Template → HTML Diff Report
```

### 3. Multi-Part Reports

For large datasets, reports are automatically split:

```go
// Automatic splitting based on configuration
maxResultsPerFile := cfg.MaxProbeResultsPerReportFile
if len(probeResults) > maxResultsPerFile {
    // Generate multiple report files
    parts := splitIntoChunks(probeResults, maxResultsPerFile)
    for i, part := range parts {
        partFilename := fmt.Sprintf("report-part-%d.html", i+1)
        generateReportPart(part, partFilename)
    }
}
```

## Advanced Features

### 1. Asset Embedding

Choose between embedded and external assets:

```go
// Embedded assets (self-contained HTML)
pageData.CustomCSS = template.CSS(embeddedCSS)
pageData.ReportJs = template.JS(embeddedJS)

// External assets (separate files)
assetManager.CopyEmbedDir(assetsFS, "assets", outputDir)
```

### 2. Interactive Data Tables

When enabled, reports include:
- Client-side sorting and filtering
- Search functionality
- Pagination
- Column visibility controls
- Export capabilities

```javascript
// DataTables initialization
$('#probeResultsTable').DataTable({
    "order": [[ 2, "desc" ]],  // Sort by status code
    "pageLength": 25,
    "responsive": true,
    "search": {
        "regex": true
    }
});
```

### 3. Responsive Design

Reports adapt to different screen sizes:
- Mobile-friendly navigation
- Collapsible columns
- Touch-friendly controls
- Optimized for tablets and phones

### 4. Dark Theme Support

Toggle between light and dark themes:
```css
.dark-theme body {
    background-color: #1a1a1a;
    color: #ffffff;
}
```

## Integration Examples

### With Scanner Service

```go
// Generate scan report
scanner.OnScanComplete(func(results []models.ProbeResult) {
    pageData := buildReportPageData(results)
    reportPath, err := htmlReporter.GenerateReport(pageData, "scan-results.html")
    if err != nil {
        logger.Error().Err(err).Msg("Report generation failed")
        return
    }
    logger.Info().Str("path", reportPath).Msg("Report generated")
})
```

### With Monitor Service

```go
// Generate diff report for file changes
monitor.OnFileChanged(func(changes []models.FileChangeInfo) {
    reportPath := diffReporter.GenerateAggregatedDiffReport(changes)
    notificationHelper.SendFileChangesNotification(ctx, changes, reportPath)
})
```

### With Notification System

```go
// Send report as Discord attachment
reportPaths := []string{scanReportPath, diffReportPath}
helper.SendScanCompletionNotification(ctx, summary, 
    notifier.ScanServiceNotification, reportPaths)
```

## Performance Considerations

### Template Optimization
- Templates parsed once at startup
- Template caching for repeated generations
- Efficient data structures for large datasets

### Asset Management
- Asset compression and minification
- Base64 encoding for small assets
- Lazy loading for large reports

### Memory Usage
- Streaming template execution for large datasets
- Chunked report generation
- Automatic cleanup of temporary files

## Error Handling

### Template Errors
```go
if err := tmpl.Execute(writer, pageData); err != nil {
    return fmt.Errorf("template execution failed: %w", err)
}
```

### File Operations
- Directory creation with proper permissions
- Atomic file writes to prevent corruption
- Cleanup of incomplete files on error

### Asset Loading
- Graceful fallback for missing assets
- Validation of embedded resources
- Error recovery for asset embedding failures

## Dependencies

- **html/template** - Go template engine
- **embed** - Asset embedding
- **Bootstrap** - CSS framework (via CDN)
- **DataTables** - Interactive tables (via CDN)
- **github.com/aleister1102/monsterinc/internal/models** - Data models

## Thread Safety

- Template execution is thread-safe
- Asset management supports concurrent access
- File operations use proper locking
- Multiple report generation can run concurrently

## Best Practices

### Report Generation
- Use builders for complex page data
- Validate data before template execution
- Handle large datasets with pagination
- Implement proper error handling

### Asset Management
- Choose appropriate embedding strategy
- Optimize assets for file size
- Use CDN links for common libraries
- Implement fallbacks for external dependencies

## Monitoring

Log messages indicate template selection reasoning:
- `"Using client-side template due to number of diffs"`
- `"Using client-side template due to large diff content size"`
- `"Using client-side template for multi-part report"`

## Backward Compatibility

Existing workflows remain unchanged. All optimizations are transparent to end users.