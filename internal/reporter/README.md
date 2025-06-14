# Reporter Package

The reporter package provides comprehensive HTML report generation for MonsterInc's security analysis pipeline. It creates interactive, professional reports with embedded assets, data visualization, and responsive design for security scan results, content diff analysis, and monitoring findings.

## Package Role in MonsterInc
As the reporting engine, this package:
- **Scanner Integration**: Generates scan reports from Scanner workflow results
- **Monitor Integration**: Creates diff reports for content changes detected by Monitor
- **Data Visualization**: Provides interactive data visualization for security findings
- **Professional Output**: Delivers client-ready security assessment reports
- **Notification Support**: Integrates with Notifier for automated report sharing

## Overview

The reporter package generates comprehensive HTML reports:
- **URL Reports**: Interactive HTML reports for security scanning results with sortable tables and filters
- **Diff Reports**: Content difference reports for file monitoring with side-by-side comparison
- **Asset Management**: CSS/JS/Image embedding with efficient asset handling
- **Template Engine**: Go template-based report generation with custom functions

## Key Features

### Intelligent Template Selection

The reporter chooses optimal rendering approach based on content size and complexity:

#### Server-Side Template (Default)
- Full HTML rendering on server with embedded data
- Suitable for most reports under Discord's 10MB limit
- Better for offline viewing and compatibility
- Used when report size is manageable

#### Client-Side Template (For Large Reports)  
- Minimal HTML skeleton with JSON data for JavaScript rendering
- 30-50% smaller file sizes for large datasets
- Optimized for reports with extensive diff content
- Automatic fallback when size limits are exceeded

### Automatic Report Splitting

Smart file size management ensures compatibility with notification services:

- **Size Monitoring**: Continuously monitors report size during generation
- **Dynamic Splitting**: Automatically splits large reports into multiple parts
- **Discord Optimization**: Keeps individual files under 10MB for Discord sharing
- **Seamless Navigation**: Maintains navigation between report parts

### File Structure

#### Core Components

- **`url_report_generator.go`** - Main scan report generator
- **`diff_report_generator.go`** - Content diff report generator  
- **`asset_manager.go`** - CSS/JS/Image asset handling
- **`directory_manager.go`** - File and directory operations
- **`template_functions.go`** - Custom template functions

#### Templates

- **`templates/report_client_side.html.tmpl`** - Main scan report template
- **`templates/diff_report_client_side.html.tmpl`** - Content diff report template

#### Assets

- **`assets/css/report_client_side.css`** - Report styling with Bootstrap
- **`assets/css/diff_report_client_side.css`** - Diff report specific styles
- **`assets/js/report_client_side.js`** - Interactive functionality
- **`assets/js/diff_report_client_side.js`** - Diff report JavaScript
- **`assets/img/favicon.ico`** - Report favicon

## Integration with MonsterInc Components

### With Scanner Service

```go
// Scanner generates comprehensive scan reports
reportGenerator := scanner.GetReportGenerator()
reportData := &models.ReportData{
    ScanSummary:     scanSummary,
    ProbeResults:    allProbeResults,
    URLDiffResults:  urlDiffResults,
    ExtractedPaths:  extractedPaths,
    SecretsFindings: secretsFindings,
}

outputPath, err := reportGenerator.GenerateReport(reportData)
if err != nil {
    logger.Error().Err(err).Msg("Failed to generate scan report")
    return err
}

logger.Info().
    Str("report_path", outputPath).
    Int("total_urls", len(allProbeResults)).
    Msg("Scan report generated successfully")
```

### With Monitor Service

```go
// Monitor generates diff reports for content changes
diffReporter := monitor.GetDiffReporter()
reportPath, err := diffReporter.GenerateDiffReport(
    ctx,
    url, 
    diffResult,
    lastRecord,
    currentContent,
)

if err != nil {
    logger.Error().Err(err).Msg("Failed to generate diff report")
    return err
}

// Notify about changes with report attachment
notifier.SendChangeNotification(ctx, changeInfo, reportPath)
```

### With Notifier Integration

```go
// Reporter works with notifier for automated sharing
type ReportNotificationConfig struct {
    DiscordWebhook string
    MaxFileSize    int64  // 10MB for Discord
    SplitLargeReports bool
}

// Generate and share report
reportPath, err := reporter.GenerateReport(data)
if err != nil {
    return err
}

// Notifier handles report sharing
err = notifier.ShareReport(ctx, reportPath, discordWebhook)
if err != nil {
    logger.Error().Err(err).Msg("Failed to share report")
}
```

## Usage Examples

### URL Report Generation (Scanner Integration)

```go
import (
    "github.com/aleister1102/monsterinc/internal/reporter"
    "github.com/aleister1102/monsterinc/internal/models"
)

// Create URL report generator
urlReporter, err := reporter.NewUrlReportGenerator(
    cfg.ReporterConfig,
    logger,
)
if err != nil {
    return err
}

// Generate report from scanner results
reportData := &models.ReportData{
    ScanSummary: &models.ScanSummary{
        TotalURLs:    len(probeResults),
        SuccessfulScans: countSuccessful(probeResults),
        StartTime:    scanStartTime,
        EndTime:      time.Now(),
    },
    ProbeResults:   probeResults,
    URLDiffResults: urlDiffResults,
    ExtractedPaths: extractedPaths,
}

outputPath, err := urlReporter.GenerateReport(reportData)
if err != nil {
    return fmt.Errorf("URL report generation failed: %w", err)
}

logger.Info().
    Str("report_path", outputPath).
    Msg("URL report generated successfully")
```

### Diff Report Generation (Monitor Integration)

```go
// Create diff report generator
diffReporter, err := reporter.NewDiffReportGenerator(
    cfg.ReporterConfig,
    logger,
)
if err != nil {
    return err
}

// Generate diff report for content changes
reportPath, err := diffReporter.GenerateDiffReport(
    ctx,
    &models.DiffReportInput{
        URL:           "https://example.com/api/endpoint",
        DiffResult:    contentDiffResult,
        LastRecord:    historicalRecord,
        CurrentContent: newContent,
        ChangeTime:    time.Now(),
    },
)

if err != nil {
    return fmt.Errorf("diff report generation failed: %w", err)
}

logger.Info().
    Str("report_path", reportPath).
    Bool("has_changes", !contentDiffResult.IsIdentical).
    Msg("Diff report generated")
```

## Report Features

### 1. Interactive URL Reports

**Key Capabilities:**
- **Data Tables**: Sortable, searchable tables with pagination
- **Filtering**: Multi-column filtering with regex support
- **Technology Detection**: Visual display of detected technologies
- **Status Visualization**: Color-coded HTTP status codes
- **URL Diff Highlighting**: Clear indication of new/changed/removed URLs
- **Responsive Design**: Mobile-friendly layout
- **Export Options**: Copy to clipboard, CSV export

**Data Structure:**
```go
type ReportData struct {
    ScanSummary     *ScanSummary      `json:"scan_summary"`
    ProbeResults    []*ProbeResult    `json:"probe_results"`
    URLDiffResults  map[string]*URLDiffResult `json:"url_diff_results"`
    ExtractedPaths  []*ExtractedPath  `json:"extracted_paths"`
    SecretsFindings []*SecretFinding  `json:"secrets_findings"`
}
```

### 2. Content Diff Reports

**Features:**
- **Side-by-Side Comparison**: Visual diff with line-by-line comparison
- **Syntax Highlighting**: Language-aware syntax highlighting
- **Change Statistics**: Lines added/removed/modified counts
- **Context Lines**: Configurable context around changes
- **Path Extraction**: Display extracted URLs/paths from JavaScript
- **File Metadata**: Content type, size, hash information

**Diff Display:**
```go
type DiffReportPageData struct {
    ReportTitle   string              `json:"report_title"`
    GeneratedAt   string              `json:"generated_at"`
    URL           string              `json:"url"`
    DiffResult    *ContentDiffResult  `json:"diff_result"`
    LastRecord    *FileHistory        `json:"last_record"`
    NewContent    []byte              `json:"new_content"`
    ChangeTime    time.Time           `json:"change_time"`
}
```

### 3. Advanced Asset Management

**Capabilities:**
- **Embedded Assets**: CSS/JS directly embedded in HTML for portability
- **CDN Fallbacks**: External CDN links with local fallbacks
- **Asset Optimization**: Minification and compression
- **Cache Busting**: Version-based cache invalidation
- **Base64 Encoding**: Efficient encoding for small assets

## Configuration

### Reporter Configuration

```yaml
reporter_config:
  output_directory: "./reports"
  embed_assets: true
  enable_data_tables: true
  max_report_size_mb: 8
  enable_report_splitting: true
  template_dir: "./templates"
  assets_dir: "./assets"

  # URL report specific
  url_report:
    items_per_page: 100
    enable_filters: true
    enable_export: true

  # Diff report specific  
  diff_report:
    context_lines: 3
    max_content_size_mb: 5
    enable_syntax_highlighting: true
```

### Template Configuration

```go
type ReporterConfig struct {
    OutputDirectory     string `yaml:"output_directory"`
    EmbedAssets        bool   `yaml:"embed_assets"`
    EnableDataTables   bool   `yaml:"enable_data_tables"`
    MaxReportSizeMB    int64  `yaml:"max_report_size_mb"`
    EnableReportSplitting bool `yaml:"enable_report_splitting"`
    TemplateDir        string `yaml:"template_dir"`
    AssetsDir          string `yaml:"assets_dir"`
}
```

## Dependencies

- **github.com/aleister1102/monsterinc/internal/models** - Data structures
- **github.com/aleister1102/monsterinc/internal/config** - Configuration management
- **github.com/rs/zerolog** - Structured logging
- **html/template** - Go templating engine
- **encoding/json** - JSON data handling
- **path/filepath** - File path utilities

## Best Practices

### Performance Optimization
- Use client-side templates for large datasets
- Enable report splitting for Discord compatibility
- Embed assets for offline viewing
- Implement proper caching strategies

### File Size Management
- Monitor report sizes during generation
- Use automatic splitting for large reports
- Optimize asset sizes and formats
- Consider pagination for very large datasets

### Template Design
- Maintain responsive design principles
- Use semantic HTML for accessibility
- Implement proper error handling
- Test across different browsers and screen sizes