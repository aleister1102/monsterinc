# Notifier Package

The notifier package provides comprehensive Discord notification capabilities for MonsterInc's security operations. It delivers real-time alerts for scan results, monitoring events, security findings, and system status with rich formatting, file attachments, and intelligent message routing.

## Package Role in MonsterInc
As the communication hub, this package:
- **Scanner Integration**: Sends scan result notifications with HTML reports
- **Monitor Integration**: Delivers real-time alerts for content changes with diff reports
- **Reporter Integration**: Uploads and shares generated HTML reports via Discord
- **Team Communication**: Enables team collaboration on security findings
- **System Notifications**: Provides status updates and error alerts

## Overview

The notifier package enables:
- **Discord Integration**: Webhook-based notification delivery to Discord channels
- **Rich Formatting**: Structured embeds with colors, fields, and metadata
- **File Attachments**: Automatic report upload with size optimization and compression
- **Message Building**: Fluent API for constructing Discord messages
- **Service Routing**: Different webhooks for scanner and monitor services
- **Error Handling**: Robust delivery with retry mechanisms and fallback strategies

## Architecture

```
┌─────────────────────┐    ┌──────────────────────┐
│ NotificationHelper  │ ──► │   DiscordNotifier    │
│ (High-level API)    │    │ (HTTP Client)        │
└─────────────────────┘    └──────────────────────┘
         │                          │
         ▼                          ▼
┌─────────────────────┐    ┌──────────────────────┐
│  Message Formatters │    │   File Operations    │
│ (Content Creation)  │    │ (Upload, Compress)   │
└─────────────────────┘    └──────────────────────┘
         │                          │
         ▼                          ▼
┌─────────────────────┐    ┌──────────────────────┐
│  Message Builders   │    │     Utilities        │
│ (Discord Objects)   │    │ (Validation, Retry)  │
└─────────────────────┘    └──────────────────────┘
```

## Integration with MonsterInc Components

### With Scanner Service

```go
// Scanner sends scan completion notifications with reports
helper := scanner.GetNotificationHelper()
reportPaths := []string{"./reports/scan-report.html"}

helper.SendScanCompletionNotification(ctx, scanSummary, 
    notifier.ScanServiceNotification, reportPaths)

// Error notifications for failed scans
if scanError != nil {
    helper.SendScanFailureNotification(ctx, scanSummary, scanError)
}
```

### With Monitor Service

```go
// Monitor sends file change notifications with diff reports
helper := monitor.GetNotificationHelper()

// Single file change notification
helper.SendFileChangeNotification(ctx, changeInfo, diffReportPath)

// Aggregated changes notification
helper.SendAggregatedFileChangesNotification(ctx, changes, aggregatedReportPath)

// Monitor cycle completion
helper.SendMonitorCycleCompleteNotification(ctx, cycleData)
```

### With Reporter Integration

```go
// Notifier automatically handles report uploads
type NotificationConfig struct {
    MaxFileSize    int64 // Discord's 10MB limit
    CompressLarge  bool  // Auto-compress oversized files
    CleanupAfter   bool  // Remove files after upload
}

// Reporter generates report, notifier handles delivery
reportPath, err := reporter.GenerateReport(data)
if err != nil {
    return err
}

// Notifier handles file size checks and compression
err = notifier.SendReportNotification(ctx, reportPath, webhook)
if err != nil {
    logger.Error().Err(err).Msg("Failed to send report")
}
```

## File Structure

### Core Components

- **`notifier.go`** - Main notification service and Discord HTTP client
- **`helper.go`** - High-level API and service integrations
- **`scan_formatters.go`** - Scanner service message formatting
- **`monitor_formatters.go`** - Monitor service message formatting
- **`builders.go`** - Discord embed and payload builders

### Supporting Components

- **`utils.go`** - Utility functions and validation
- **`constants.go`** - Color constants and configuration values

## Features

### 1. Discord Integration

**Capabilities:**
- **Webhook Delivery**: Reliable webhook-based message delivery
- **Rich Embeds**: Structured embeds with colors, fields, and metadata
- **File Attachments**: Upload HTML reports and other files up to Discord's limits
- **Message Threading**: Group related notifications together
- **Role Mentions**: Notify specific roles for critical alerts

### 2. Message Types

**Scanner Service Notifications:**
- **Scan Start**: Notifications with target information and expected duration
- **Scan Completion**: Success notifications with statistics and HTML reports
- **Scan Failure**: Error notifications with detailed failure information
- **Critical Alerts**: High-priority security finding notifications

**Monitor Service Notifications:**
- **File Changes**: Individual file change notifications with diff reports
- **Batch Changes**: Aggregated change notifications for multiple files
- **Cycle Completion**: Monitor cycle summary with statistics
- **Error Aggregation**: Bundled error reports for fetch failures

### 3. File Management

**Advanced Features:**
- **Size Detection**: Automatic file size validation before upload
- **Compression**: ZIP compression for files exceeding Discord's 10MB limit
- **Multi-part Handling**: Smart handling of split reports from Reporter
- **Auto-cleanup**: Configurable cleanup of temporary files after delivery
- **Fallback Strategies**: Graceful degradation when file upload fails

## Usage Examples

### Basic Setup and Configuration

```go
import (
    "github.com/aleister1102/monsterinc/internal/notifier"
    "github.com/aleister1102/monsterinc/internal/config"
    "github.com/aleister1102/monsterinc/internal/common"
)

// Create HTTP client optimized for Discord
httpClientFactory := common.NewHTTPClientFactory(logger)
discordClient, err := httpClientFactory.CreateDiscordClient(30 * time.Second)
if err != nil {
    return fmt.Errorf("failed to create Discord client: %w", err)
}

// Create Discord notifier with retry logic
discordNotifier, err := notifier.NewDiscordNotifier(logger, discordClient)
if err != nil {
    return fmt.Errorf("failed to create Discord notifier: %w", err)
}

// Create notification helper with configuration
helper := notifier.NewNotificationHelper(
    discordNotifier,
    cfg.NotificationConfig,
    logger,
)
```

### Scanner Integration Examples

```go
// Scan workflow notifications
ctx := context.Background()

// 1. Scan start notification
summary := &models.ScanSummaryData{
    ScanSessionID: "scan-20240101-120000",
    ScanMode:      "onetime",
    TargetSource:  "file:targets.txt",
    TotalTargets:  150,
    StartTime:     time.Now(),
}

helper.SendScanStartNotification(ctx, summary)

// 2. Progress updates (optional)
helper.SendScanProgressNotification(ctx, summary, 75) // 75% complete

// 3. Scan completion with results
summary.ProbeStats = models.ProbeStats{
    TotalProbed:      150,
    SuccessfulProbes: 142,
    FailedProbes:     8,
}
summary.ScanDuration = 5 * time.Minute  
summary.Status = "COMPLETED"

// Include generated reports
reportPaths := []string{
    "./reports/scan-report.html",
    "./reports/extracted-paths.html",
}

helper.SendScanCompletionNotification(ctx, summary, 
    notifier.ScanServiceNotification, reportPaths)

// 4. Error handling
if scanError != nil {
    helper.SendScanFailureNotification(ctx, summary, scanError)
}
```

### Monitor Integration Examples

```go
// Monitor workflow notifications
ctx := context.Background()

// 1. Individual file change notification
changeInfo := models.FileChangeInfo{
    URL:            "https://example.com/critical-config.js",
    OldHash:        "abc123def456",
    NewHash:        "789ghi012jkl",
    ContentType:    "application/javascript",
    ChangeTime:     time.Now(),
    DiffReportPath: stringPtr("./reports/config-js-diff.html"),
    CycleID:        "cycle-20240101-001",
}

helper.SendFileChangeNotification(ctx, changeInfo, 
    *changeInfo.DiffReportPath)

// 2. Aggregated changes notification
changes := []models.FileChangeInfo{changeInfo, /* more changes */}
aggregatedReportPath := "./reports/aggregated-changes.html"

helper.SendAggregatedFileChangesNotification(ctx, changes, 
    aggregatedReportPath)

// 3. Monitor cycle completion
cycleData := models.MonitorCycleCompleteData{
    CycleID:        "cycle-20240101-001",
    ChangedURLs:    []string{"https://example.com/critical-config.js"},
    FileChanges:    changes,
    TotalMonitored: 100,
    SuccessfulChecks: 98,
    FailedChecks:   2,
    Timestamp:      time.Now(),
    ReportPath:     "./reports/cycle-complete.html",
}

helper.SendMonitorCycleCompleteNotification(ctx, cycleData)
```

### Custom Message Building

```go
// Build custom security alert
embed := notifier.NewDiscordEmbedBuilder().
    WithTitle("🚨 Critical Security Alert").
    WithDescription("High-severity vulnerability detected in monitored endpoint").
    WithColor(notifier.CriticalEmbedColor).
    WithTimestamp(time.Now()).
    AddField("🎯 Target", "https://api.example.com/auth/login", false).
    AddField("🔍 Issue", "Authentication bypass vulnerability", false).
    AddField("⚠️ Severity", "CRITICAL", true).
    AddField("🚀 Action Required", "Immediate patching required", true).
    AddField("📊 Confidence", "High (95%)", true).
    WithFooter("MonsterInc Security Monitor", "").
    Build()

// Send custom notification
payload := notifier.NewDiscordPayloadBuilder().
    WithUsername("MonsterInc Security").
    WithEmbeds([]*notifier.DiscordEmbed{embed}).
    Build()

err := discordNotifier.SendNotification(ctx, webhook, payload)
if err != nil {
    logger.Error().Err(err).Msg("Failed to send custom alert")
}
```

### Advanced File Handling

```go
// Handle large report files with compression
config := &notifier.FileUploadConfig{
    MaxFileSizeMB:     10,  // Discord limit
    CompressOversized: true,
    RetryCount:        3,
    CleanupAfterSend:  true,
}

// Send report with automatic compression if needed
err := helper.SendReportWithConfig(ctx, reportPath, webhook, config)
if err != nil {
    logger.Error().Err(err).Msg("Failed to send report")
}

// Handle multi-part reports (from Reporter splitting)
reportParts := []string{
    "./reports/scan-report-part1.html",
    "./reports/scan-report-part2.html",
    "./reports/scan-report-part3.html",
}

err = helper.SendMultiPartReport(ctx, reportParts, webhook, 
    "Scan Report (Multiple Parts)")
if err != nil {
    logger.Error().Err(err).Msg("Failed to send multi-part report")
}
```

## Configuration

### Notification Configuration

```yaml
notification_config:
  # Discord webhooks for different services
  scanner_webhook: "https://discord.com/api/webhooks/xxx/yyy"
  monitor_webhook: "https://discord.com/api/webhooks/aaa/bbb"
  
  # File upload settings
  max_file_size_mb: 10
  compress_large_files: true
  cleanup_after_send: true
  
  # Retry and error handling
  retry_count: 3
  retry_delay_seconds: 5
  enable_fallback: true
  
  # Message customization
  bot_username: "MonsterInc Security"
  enable_mentions: true
  mention_roles: ["@security-team"]
  
  # Rate limiting
  rate_limit_per_minute: 30
  burst_limit: 5
```

### Configuration Structure

```go
type NotificationConfig struct {
    ScannerWebhook     string   `yaml:"scanner_webhook"`
    MonitorWebhook     string   `yaml:"monitor_webhook"`
    MaxFileSizeMB      int64    `yaml:"max_file_size_mb"`
    CompressLargeFiles bool     `yaml:"compress_large_files"`
    CleanupAfterSend   bool     `yaml:"cleanup_after_send"`
    RetryCount         int      `yaml:"retry_count"`
    RetryDelaySeconds  int      `yaml:"retry_delay_seconds"`
    BotUsername        string   `yaml:"bot_username"`
    EnableMentions     bool     `yaml:"enable_mentions"`
    MentionRoles       []string `yaml:"mention_roles"`
}
```

## Message Formatting

### Color Coding System

```go
const (
    // Status colors
    SuccessEmbedColor = 0x00FF00  // Green - successful operations
    WarningEmbedColor = 0xFFFF00  // Yellow - warnings and non-critical issues
    ErrorEmbedColor   = 0xFF0000  // Red - errors and failures
    InfoEmbedColor    = 0x0099FF  // Blue - informational messages
    CriticalEmbedColor = 0xFF0066 // Pink - critical security alerts
)

// Usage in formatters
embed.WithColor(notifier.SuccessEmbedColor) // For successful scans
embed.WithColor(notifier.CriticalEmbedColor) // For security alerts
```

### Message Templates

**Scan Completion Template:**
```
🎯 Scan Completed Successfully
Target: file:targets.txt
Duration: 5m 30s
Results: 142/150 successful (94.7%)
📊 Report: [scan-report.html]
```

**File Change Template:**
```
📝 Content Change Detected
🔗 URL: https://example.com/config.js
🔄 Status: Modified
⏰ Time: 2024-01-01 12:00:00 UTC
📋 Diff Report: [config-js-diff.html]
```

## Dependencies

- **github.com/aleister1102/monsterinc/internal/models** - Data structures
- **github.com/aleister1102/monsterinc/internal/config** - Configuration management
- **github.com/aleister1102/monsterinc/internal/common** - HTTP client factory
- **github.com/rs/zerolog** - Structured logging
- **net/http** - HTTP client operations
- **encoding/json** - JSON marshaling for Discord API
- **archive/zip** - File compression functionality

## Best Practices

### Error Handling
- Implement retry logic with exponential backoff
- Log all notification attempts and failures
- Provide fallback mechanisms for critical notifications
- Handle Discord rate limits gracefully

### Performance Optimization
- Use connection pooling for HTTP clients
- Implement message batching for high-volume notifications
- Cache frequently used embed templates
- Monitor and optimize file upload performance

### Security Considerations
- Validate webhook URLs before use
- Sanitize user input in message content
- Implement proper authentication for webhook endpoints
- Monitor for potential sensitive data leakage in notifications

### File Management
- Always validate file sizes before upload attempts
- Implement proper cleanup of temporary files
- Use compression for large files to stay within Discord limits
- Provide clear error messages for file-related failures
