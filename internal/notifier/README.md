# Notifier Package

The notifier package provides comprehensive Discord notification capabilities for MonsterInc security scanner. It delivers structured notifications for scan results, monitoring events, and system alerts with rich formatting and file attachments.

## Overview

The notifier package enables:
- **Discord Integration**: Webhook-based notification delivery to Discord channels
- **Rich Formatting**: Structured embeds with colors, fields, and metadata
- **File Attachments**: Automatic report upload with size optimization
- **Message Building**: Fluent API for constructing Discord messages
- **Service Routing**: Different webhooks for scanner and monitor services
- **Error Handling**: Robust delivery with fallback mechanisms

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

## File Structure

### Core Components

- **`helper.go`** - Main notification service and high-level API
- **`notifier.go`** - Discord HTTP client and file operations
- **`scan_formatters.go`** - Scan-related message formatting
- **`monitor_formatters.go`** - Monitor-related message formatting
- **`builders.go`** - Discord embed and payload builders

### Supporting Components

- **`utils.go`** - Utility functions and validation
- **`constants.go`** - Color constants and configuration

## Features

### 1. Discord Integration

**Capabilities:**
- Webhook-based message delivery
- Rich embed formatting with colors and fields
- File attachment handling up to Discord limits
- Custom usernames and avatars
- Role mention support

### 2. Message Types

**Scan Notifications:**
- Scan start notifications with target information
- Completion notifications with statistics
- Failure notifications with error details
- Critical error alerts with system information

**Monitor Notifications:**
- File change notifications with diff reports
- Monitoring cycle completion summaries
- Error aggregation reports
- Fetch failure notifications

### 3. File Management

**Features:**
- Automatic file size detection and optimization
- ZIP compression for oversized reports
- Multi-part report handling
- Auto-cleanup after successful delivery
- Graceful fallback for upload failures

## Usage Examples

### Basic Setup

```go
import (
    "github.com/aleister1102/monsterinc/internal/notifier"
    "github.com/aleister1102/monsterinc/internal/config"
    "github.com/aleister1102/monsterinc/internal/common"
)

// Create HTTP client for Discord
httpClientFactory := common.NewHTTPClientFactory(logger)
discordClient, err := httpClientFactory.CreateDiscordClient(30 * time.Second)
if err != nil {
    return fmt.Errorf("failed to create Discord client: %w", err)
}

// Create Discord notifier
discordNotifier, err := notifier.NewDiscordNotifier(logger, discordClient)
if err != nil {
    return fmt.Errorf("failed to create Discord notifier: %w", err)
}

// Create notification helper
helper := notifier.NewNotificationHelper(
    discordNotifier,
    cfg.NotificationConfig,
    logger,
)
```

### Scan Notifications

```go
// Create scan summary
summary := models.ScanSummaryData{
    ScanSessionID: "20240101-120000",
    ScanMode:      "onetime",
    TargetSource:  "file:targets.txt",
    TotalTargets:  100,
    ProbeStats: models.ProbeStats{
        TotalProbed:      100,
        SuccessfulProbes: 85,
        FailedProbes:     15,
    },
    ScanDuration: 5 * time.Minute,
    Status:       "COMPLETED",
}

// Send scan start notification
ctx := context.Background()
helper.SendScanStartNotification(ctx, summary)

// Send completion notification with report
reportPaths := []string{"./reports/scan-report.html"}
helper.SendScanCompletionNotification(ctx, summary, 
    notifier.ScanServiceNotification, reportPaths)
```

### Monitor Notifications

```go
// File change notification
changes := []models.FileChangeInfo{
    {
        URL:            "https://example.com/app.js",
        OldHash:        "abc123",
        NewHash:        "def456",
        ContentType:    "application/javascript",
        ChangeTime:     time.Now(),
        DiffReportPath: stringPtr("./reports/app-js-diff.html"),
        CycleID:        "cycle-001",
    },
}

// Send aggregated changes notification
helper.SendAggregatedFileChangesNotification(ctx, changes, 
    "./reports/aggregated-diff.html")

// Monitor cycle completion
cycleData := models.MonitorCycleCompleteData{
    CycleID:        "cycle-001",
    ChangedURLs:    []string{"https://example.com/app.js"},
    FileChanges:    changes,
    TotalMonitored: 50,
    Timestamp:      time.Now(),
    ReportPath:     "./reports/cycle-report.html",
}

helper.SendMonitorCycleCompleteNotification(ctx, cycleData)
```

### Custom Message Building

```go
// Build custom embed with fluent API
embed := notifier.NewDiscordEmbedBuilder().
    WithTitle("🔍 Security Alert").
    WithDescription("Critical vulnerability detected in monitored file").
    WithColor(notifier.CriticalEmbedColor).
    WithTimestamp(time.Now()).
    AddField("Affected File", "https://example.com/critical-config.js", false).
    AddField("Severity", "HIGH", true).
    AddField("Action Required", "Immediate Review", true).
    WithFooter("MonsterInc Security Scanner", "").
    Build()

// Build complete message payload
allowedMentions := models.NewAllowedMentionsBuilder().
    WithRoles([]string{"123456789"}).
    Build()

payload := notifier.NewDiscordMessagePayloadBuilder().
    WithUsername("MonsterInc Security Alert").
    WithContent("🚨 **SECURITY ALERT** 🚨").
    AddEmbed(embed).
    WithAllowedMentions(allowedMentions).
    Build()

// Send directly via notifier
err := discordNotifier.SendNotification(ctx, webhookURL, payload, "")
```

### Error Notifications

```go
// Critical error notification
helper.SendCriticalErrorNotification(ctx, 
    "Database connection failed", 
    "scanner", 
    notifier.ScanServiceNotification)

// Monitor fetch errors
fetchErrors := []models.MonitorFetchErrorInfo{
    {
        URL:        "https://example.com/unavailable.js",
        Error:      "connection timeout",
        Source:     "fetch",
        OccurredAt: time.Now(),
        CycleID:    "cycle-001",
    },
}

helper.SendAggregatedFetchErrorsNotification(ctx, fetchErrors)
```

## Configuration

### Notification Configuration

```yaml
notification_config:
  scan_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  monitor_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  notify_on_scan_start: true             # Send scan start notifications
  notify_on_success: true                # Send successful completion notifications
  notify_on_failure: true                # Send failure notifications
  notify_on_critical_error: true         # Send critical error alerts
  mention_role_ids:                      # Role IDs to mention in notifications
    - "123456789012345678"
    - "987654321098765432"
  auto_delete_partial_diff_reports: true # Delete partial reports after sending
```

### Configuration Options

- **`scan_service_discord_webhook_url`**: Webhook for scan-related notifications
- **`monitor_service_discord_webhook_url`**: Webhook for monitor-related notifications
- **`notify_on_*`**: Boolean flags to control notification types
- **`mention_role_ids`**: Discord role IDs to mention in notifications
- **`auto_delete_partial_diff_reports`**: Cleanup partial reports after delivery

## Message Formatting

### Embed Colors

```go
const (
    SuccessEmbedColor  = 0x5CB85C  // Green - successful operations
    ErrorEmbedColor    = 0xD9534F  // Red - failed operations
    WarningEmbedColor  = 0xF0AD4E  // Orange - warnings
    InfoEmbedColor     = 0x5BC0DE  // Blue - informational
    MonitorEmbedColor  = 0x6F42C1  // Purple - monitoring events
    CriticalEmbedColor = 0xDC3545  // Dark Red - critical alerts
)
```

### Standard Message Structure

```go
// Scan completion embed structure
type ScanCompletionEmbed struct {
    Title       string    // "🎯 Scan Completed"
    Description string    // Scan summary
    Color       int       // Based on success/failure
    Fields      []Field   // Statistics and details
    Timestamp   time.Time // Completion time
    Footer      string    // Scanner identification
}
```

### Field Formatting

```go
// Common embed fields
fields := []models.DiscordEmbedField{
    {Name: "📊 Targets Processed", Value: "100", Inline: true},
    {Name: "✅ Successful Probes", Value: "85", Inline: true},
    {Name: "❌ Failed Probes", Value: "15", Inline: true},
    {Name: "⏱️ Duration", Value: "5m 30s", Inline: true},
    {Name: "🔗 Report", Value: "[View Report](./report.html)", Inline: false},
}
```

## File Handling

### Size Optimization

```go
// Automatic file size management
func (dn *DiscordNotifier) handleFileAttachment(ctx context.Context, filePath string) error {
    fileInfo, err := os.Stat(filePath)
    if err != nil {
        return err
    }
    
    // Discord file size limit (safe margin)
    if fileInfo.Size() > discordFileSizeLimit {
        return dn.compressAndUpload(ctx, filePath)
    }
    
    return dn.uploadFile(ctx, filePath)
}
```

### ZIP Compression

```go
// Compress large files for Discord
func (dn *DiscordNotifier) compressFile(sourceFile string) (string, error) {
    zipPath := sourceFile + ".zip"
    
    archive, err := os.Create(zipPath)
    if err != nil {
        return "", err
    }
    defer archive.Close()
    
    zipWriter := zip.NewWriter(archive)
    defer zipWriter.Close()
    
    // Add file to ZIP
    return dn.addFileToZip(zipWriter, sourceFile)
}
```

### Cleanup Management

```go
// Auto-cleanup after sending
func (dn *DiscordNotifier) SendWithCleanup(ctx context.Context, 
    webhookURL string, payload models.DiscordMessagePayload, filePath string) error {
    
    err := dn.SendNotification(ctx, webhookURL, payload, filePath)
    
    // Cleanup on success or configured auto-delete
    if err == nil || dn.shouldAutoDelete(filePath) {
        dn.cleanupFile(filePath)
    }
    
    return err
}
```

## Integration Examples

### With Scanner Service

```go
// Scanner completion hook
scanner.OnScanComplete(func(summary models.ScanSummaryData, reportPaths []string) {
    ctx := context.Background()
    
    // Send notification based on scan result
    if summary.Status == string(models.ScanStatusCompleted) {
        helper.SendScanCompletionNotification(ctx, summary, 
            notifier.ScanServiceNotification, reportPaths)
    } else {
        helper.SendScanFailureNotification(ctx, summary, 
            notifier.ScanServiceNotification)
    }
})
```

### With Monitor Service

```go
// Monitor change detection hook
monitor.OnChangesDetected(func(changes []models.FileChangeInfo, reportPath string) {
    ctx := context.Background()
    
    // Send aggregated notification for multiple changes
    if len(changes) > 1 {
        helper.SendAggregatedFileChangesNotification(ctx, changes, reportPath)
    } else {
        // Send individual notification for single change
        helper.SendFileChangeNotification(ctx, changes[0])
    }
})
```

### With Scheduler

```go
// Scheduled task completion
scheduler.OnTaskComplete(func(task scheduler.Task, result scheduler.TaskResult) {
    ctx := context.Background()
    
    // Send notification based on task type and result
    switch task.Type {
    case scheduler.TaskTypeScan:
        helper.SendScanCompletionNotification(ctx, result.ScanSummary, 
            notifier.ScanServiceNotification, result.ReportPaths)
    case scheduler.TaskTypeMonitor:
        helper.SendMonitorCycleCompleteNotification(ctx, result.CycleData)
    }
})
```

## Error Handling

### Network Retry Logic

```go
// HTTP client with retry capabilities
func (dn *DiscordNotifier) sendWithRetry(ctx context.Context, 
    req *common.HTTPRequest, maxRetries int) (*common.HTTPResponse, error) {
    
    var lastErr error
    for attempt := 0; attempt <= maxRetries; attempt++ {
        resp, err := dn.httpClient.Do(req)
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }
        
        lastErr = err
        if attempt < maxRetries {
            backoff := time.Duration(attempt+1) * time.Second
            time.Sleep(backoff)
        }
    }
    
    return nil, lastErr
}
```

### Validation

```go
// Webhook URL validation
func (dn *DiscordNotifier) validateWebhookURL(webhookURL string) error {
    if webhookURL == "" {
        return errors.New("webhook URL is required")
    }
    
    if !strings.HasPrefix(webhookURL, "https://discord.com/api/webhooks/") {
        return errors.New("invalid Discord webhook URL format")
    }
    
    return nil
}
```

### Fallback Mechanisms

```go
// Graceful degradation for file upload failures
func (dn *DiscordNotifier) sendWithFallback(ctx context.Context, 
    webhookURL string, payload models.DiscordMessagePayload, filePath string) error {
    
    // Try with file attachment first
    err := dn.SendNotification(ctx, webhookURL, payload, filePath)
    if err == nil {
        return nil
    }
    
    // Fallback to text-only message
    dn.logger.Warn().Err(err).Msg("File upload failed, sending text-only")
    return dn.SendNotification(ctx, webhookURL, payload, "")
}
```

## Thread Safety

- All public methods are thread-safe
- HTTP client uses connection pooling safely
- File operations use proper locking mechanisms
- Context propagation for cancellation support
- Concurrent notification sending supported

## Dependencies

- **github.com/aleister1102/monsterinc/internal/common** - HTTP client functionality
- **github.com/aleister1102/monsterinc/internal/models** - Data models and structures
- **github.com/aleister1102/monsterinc/internal/config** - Configuration management
- **github.com/rs/zerolog** - Structured logging

## Best Practices

### Message Design
- Use appropriate embed colors for different message types
- Include relevant context and actionable information
- Keep descriptions concise but informative
- Use inline fields for related data grouping

### File Management
- Monitor file sizes and optimize for Discord limits
- Implement proper cleanup to prevent disk space issues
- Use meaningful filenames for uploaded reports
- Consider compression for large report files

### Error Handling
- Implement retry logic for transient network failures
- Provide fallback mechanisms for file upload issues
- Log notification failures for debugging
- Use appropriate error recovery strategies

### Performance Optimization
- Batch notifications when appropriate to reduce API calls
- Use efficient HTTP client with connection pooling
- Implement reasonable timeouts for webhook requests
- Monitor notification delivery success rates 