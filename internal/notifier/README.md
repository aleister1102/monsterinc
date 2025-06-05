# Notifier Package

The notifier package provides a comprehensive Discord notification system for MonsterInc security scanner. It handles sending structured notifications for scan results, monitoring events, and system alerts.

## Overview

The notification system is built around Discord webhooks and provides formatted messages with embeds, file attachments, and role mentions. The package follows a modular design with separate components for different responsibilities.

## Architecture

```
┌─────────────────────────┐    ┌──────────────────────────┐
│   NotificationHelper    │ ──► │    DiscordNotifier       │
│   (High-level API)      │    │   (HTTP Client)          │
└─────────────────────────┘    └──────────────────────────┘
           │                              │
           ▼                              ▼
┌─────────────────────────┐    ┌──────────────────────────┐
│   Message Formatters    │    │    File Operations       │
│   (Content Creation)    │    │   (Zip, Upload)          │
└─────────────────────────┘    └──────────────────────────┘
           │
           ▼
┌─────────────────────────┐
│   Message Builders      │
│   (Discord Objects)     │
└─────────────────────────┘
```

## File Structure

### Core Components

- **`notification_service.go`** - Main service interface and NotificationHelper
- **`discord_client.go`** - Discord HTTP client and file operations
- **`message_formatters.go`** - Message formatting functions
- **`message_builders.go`** - Discord embed and payload builders

### Key Features

#### 1. **Multi-Service Support**
- **Scan Service**: Notifications for security scans
- **Monitor Service**: Notifications for file monitoring
- **Generic**: Fallback for critical errors

#### 2. **Message Types**
- Scan start/completion notifications
- Monitor cycle events
- File change alerts
- Error notifications
- System interruptions

#### 3. **File Handling**
- Automatic file size checking
- ZIP compression for large files
- Multi-part report support
- Auto-deletion after sending

## Usage Examples

### Basic Setup

```go
import (
    "github.com/aleister1102/monsterinc/internal/notifier"
    "github.com/aleister1102/monsterinc/internal/config"
)

// Create Discord notifier
discordNotifier, err := notifier.NewDiscordNotifier(logger, httpClient)
if err != nil {
    return err
}

// Create notification helper
helper := notifier.NewNotificationHelper(discordNotifier, cfg.NotificationConfig, logger)
```

### Sending Scan Notifications

```go
// Scan start notification
summary := models.ScanSummaryData{
    ScanSessionID: "20240101-120000",
    ScanMode:      "onetime",
    TargetSource:  "file:targets.txt",
    TotalTargets:  100,
}

helper.SendScanStartNotification(ctx, summary)

// Scan completion with report
helper.SendScanCompletionNotification(ctx, summary, 
    notifier.ScanServiceNotification, []string{"report.html"})
```

### Monitoring Notifications

```go
// File changes detected
changes := []models.FileChangeInfo{
    {
        URL:         "https://example.com/app.js",
        OldHash:     "abc123",
        NewHash:     "def456",
        ContentType: "application/javascript",
        ChangeTime:  time.Now(),
    },
}

helper.SendAggregatedFileChangesNotification(ctx, changes, "diff-report.html")
```

### Custom Message Building

```go
// Build custom embed
embed := notifier.NewDiscordEmbedBuilder().
    WithTitle("🔍 Custom Alert").
    WithDescription("Custom monitoring alert").
    WithColor(notifier.WarningEmbedColor).
    WithTimestamp(time.Now()).
    AddField("Status", "Active", true).
    Build()

// Build payload
payload := notifier.NewDiscordMessagePayloadBuilder().
    WithUsername("MonsterInc Scanner").
    AddEmbed(embed).
    Build()

// Send directly
err := discordNotifier.SendNotification(ctx, webhookURL, payload, "")
```

## Configuration

The notification system is configured through the main config:

```yaml
notification_config:
  scan_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  monitor_service_discord_webhook_url: "https://discord.com/api/webhooks/..."
  notify_on_scan_start: true
  notify_on_success: true
  notify_on_failure: true
  notify_on_critical_error: true
  mention_role_ids: ["123456789"]
  auto_delete_single_diff_reports_after_discord_notification: true
```

## Message Formatting

### Embed Colors
- **Success**: `0x5CB85C` (Green)
- **Error**: `0xD9534F` (Red) 
- **Warning**: `0xF0AD4E` (Orange)
- **Info**: `0x5BC0DE` (Blue)
- **Monitor**: `0x6F42C1` (Purple)
- **Critical**: `0xDC3545` (Dark Red)

### Standard Fields
- **Scan Statistics**: Probe counts, success/failure rates
- **Diff Statistics**: New/existing/old URL counts
- **Timestamps**: ISO8601 formatted
- **File Attachments**: Automatic size management

## Error Handling

### File Size Management
- Automatic detection of Discord size limits (7MB safe limit)
- ZIP compression for oversized files
- Graceful fallback to text-only messages

### Retry Logic
- Network error handling in HTTP client
- Webhook validation before sending
- Proper context cancellation support

### Notification Preferences
- Configurable notification types
- Service-specific webhook routing
- Role mention management

## Integration Points

### With Scanner
```go
// In scanner completion
helper.SendScanCompletionNotification(ctx, scanSummary, 
    notifier.ScanServiceNotification, reportPaths)
```

### With Monitor
```go
// File change detection
helper.SendAggregatedFileChangesNotification(ctx, changes, reportPath)

// Cycle completion
helper.SendMonitorCycleCompleteNotification(ctx, cycleData)
```

### With Scheduler
```go
// Critical errors
helper.SendCriticalErrorNotification(ctx, "SchedulerInit", errorSummary)
```

## Best Practices

1. **Context Management**: Always pass context for cancellation support
2. **File Cleanup**: Use auto-deletion for temporary reports
3. **Rate Limiting**: Add delays between multi-part messages
4. **Error Recovery**: Handle webhook failures gracefully
5. **Content Limits**: Respect Discord's message size limits

## Testing

```go
// Mock notifier for testing
type MockNotifier struct{}

func (m *MockNotifier) SendNotification(ctx context.Context, webhookURL string, 
    payload models.DiscordMessagePayload, reportPath string) error {
    // Test implementation
    return nil
}
```

## Dependencies

- **zerolog**: Structured logging
- **http**: Discord webhook API
- **archive/zip**: File compression
- **models**: Internal data structures
- **config**: Configuration management 