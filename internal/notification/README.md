# Notification Package

This package is responsible for sending notifications to various services based on scan events.

## Features

- Send notifications for scan start, success, and failure events.
- Send notifications for critical application errors.
- Support for Discord webhooks.
- Configurable notification settings through `config.yaml` (see `NotificationConfig` in `internal/config/config.go`).

## Usage

Initialize a `NotificationHelper` with the notification configuration and a logger:

```go
import (
    "monsterinc/internal/config"
    "monsterinc/internal/notification"
    "log"
)

// ...

logger := log.Default()
notificationCfg := &config.NotificationConfig{
    // ... populate from global config ...
}

notificationHelper := notification.NewNotificationHelper(notificationCfg, logger)

// Example: Send a scan start notification
err := notificationHelper.SendScanStartNotification("targets.txt")
if err != nil {
    logger.Printf("Error sending notification: %v", err)
}
``` 