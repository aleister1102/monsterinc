package notifier

import (
	"context"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"time"

	"github.com/rs/zerolog"
)

// NotificationHelper provides a high-level interface for sending various scan-related notifications.
type NotificationHelper struct {
	discordNotifier *DiscordNotifier
	cfg             config.NotificationConfig
	logger          zerolog.Logger
}

// NewNotificationHelper creates a new NotificationHelper.
func NewNotificationHelper(dn *DiscordNotifier, cfg config.NotificationConfig, logger zerolog.Logger) *NotificationHelper {
	return &NotificationHelper{
		discordNotifier: dn,
		cfg:             cfg,
		logger:          logger.With().Str("module", "NotificationHelper").Logger(),
	}
}

// SendScanStartNotification sends a notification when a scan starts.
// It now accepts ScanSummaryData for a more structured approach.
func (nh *NotificationHelper) SendScanStartNotification(ctx context.Context, summary models.ScanSummaryData) {
	if !nh.cfg.NotifyOnScanStart || nh.discordNotifier == nil || nh.discordNotifier.disabled {
		nh.logger.Debug().Msg("Scan start notification disabled or notifier not configured.")
		return
	}

	nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Str("target_source", summary.TargetSource).Int("total_targets", summary.TotalTargets).Msg("Preparing to send scan start notification.")

	payload := FormatScanStartMessage(summary, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, payload, "") // No report file for start notification
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send scan start notification")
	} else {
		nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Scan start notification sent successfully.")
	}
}

// SendScanCompletionNotification sends a notification when a scan completes (successfully or with failure).
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData) {
	if nh.discordNotifier == nil || nh.discordNotifier.disabled {
		nh.logger.Debug().Msg("DiscordNotifier not configured or completion notifications disabled, skipping.")
		return
	}

	notify := false
	switch models.ScanStatus(summary.Status) {
	case models.ScanStatusCompleted, models.ScanStatusPartialComplete:
		if nh.cfg.NotifyOnSuccess {
			notify = true
		}
	case models.ScanStatusFailed, models.ScanStatusInterrupted:
		if nh.cfg.NotifyOnFailure {
			notify = true
		}
	default:
		nh.logger.Warn().Str("status", summary.Status).Msg("Unknown scan status for notification, skipping.")
		return
	}

	if !notify {
		nh.logger.Debug().Str("status", summary.Status).Msg("Notification for this scan status is disabled, skipping.")
		return
	}

	nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Str("target_source", summary.TargetSource).Str("status", summary.Status).Msg("Preparing to send scan completion notification.")

	payload := FormatScanCompleteMessage(summary, nh.cfg)
	// Use a new context for sending completion notification to avoid issues if the original context is already cancelled.
	notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := nh.discordNotifier.SendNotification(notificationCtx, payload, summary.ReportPath)
	if err != nil {
		nh.logger.Error().Err(err).Str("scan_session_id", summary.ScanSessionID).Msg("Failed to send scan completion notification")
	} else {
		nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Scan completion notification sent successfully.")
	}
}

// SendCriticalErrorNotification sends a notification for critical application errors.
// componentName helps identify where the error occurred (e.g., "SchedulerInitialization", "ConfigLoad").
// summaryData contains error messages and other relevant info.
func (nh *NotificationHelper) SendCriticalErrorNotification(ctx context.Context, componentName string, summaryData models.ScanSummaryData) {
	if !nh.cfg.NotifyOnCriticalError || nh.discordNotifier == nil || nh.discordNotifier.disabled {
		nh.logger.Debug().Msg("Critical error notification disabled or notifier not configured.")
		return
	}
	// Ensure component name is set in summary if not already
	if summaryData.Component == "" {
		summaryData.Component = componentName
	}

	nh.logger.Info().Str("component", summaryData.Component).Strs("errors", summaryData.ErrorMessages).Msg("Preparing to send critical error notification.")

	payload := FormatCriticalErrorMessage(summaryData, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, payload, "") // No report file for critical errors
	if err != nil {
		nh.logger.Error().Err(err).Str("component", summaryData.Component).Msg("Failed to send critical error notification")
	} else {
		nh.logger.Info().Str("component", summaryData.Component).Msg("Critical error notification sent successfully.")
	}
}

func (nh *NotificationHelper) SendAggregatedFileChangesNotification(ctx context.Context, changes []models.FileChangeInfo) {
	if nh.discordNotifier == nil || nh.discordNotifier.IsDisabled() || len(changes) == 0 {
		return
	}

	payload := FormatAggregatedFileChangesMessage(changes, nh.cfg)
	nh.logger.Info().Int("change_count", len(changes)).Msg("Preparing to send aggregated file changes notification.")

	err := nh.discordNotifier.SendNotification(ctx, payload, "") // No specific report file for aggregated changes
	if err != nil {
		nh.logger.Error().Err(err).Int("change_count", len(changes)).Msg("Failed to send aggregated file changes notification.")
	} else {
		nh.logger.Info().Int("change_count", len(changes)).Msg("Aggregated file changes notification sent successfully.")
	}
}

func (nh *NotificationHelper) SendInitialMonitoredURLsNotification(ctx context.Context, monitoredURLs []string) {
	if nh.discordNotifier == nil || nh.discordNotifier.IsDisabled() || len(monitoredURLs) == 0 {
		return
	}

	payload := FormatInitialMonitoredURLsMessage(monitoredURLs, nh.cfg)
	nh.logger.Info().Int("url_count", len(monitoredURLs)).Msg("Preparing to send initial monitored URLs notification.")

	err := nh.discordNotifier.SendNotification(ctx, payload, "") // No specific report file
	if err != nil {
		nh.logger.Error().Err(err).Int("url_count", len(monitoredURLs)).Msg("Failed to send initial monitored URLs notification.")
	} else {
		nh.logger.Info().Int("url_count", len(monitoredURLs)).Msg("Initial monitored URLs notification sent successfully.")
	}
}

// SendFileChangeNotification is removed as changes are aggregated.
// func (nh *NotificationHelper) SendFileChangeNotification(ctx context.Context, ...) { ... }

func (nh *NotificationHelper) SendAggregatedMonitorErrorsNotification(ctx context.Context, errors []models.MonitorFetchErrorInfo) {
	if nh.discordNotifier == nil || nh.discordNotifier.IsDisabled() {
		nh.logger.Debug().Int("error_count", len(errors)).Msg("Discord notifier is disabled, skipping aggregated monitor error notification.")
		return
	}

	if !nh.cfg.NotifyOnCriticalError { // Assuming these errors fall under critical, or add a specific config flag
		nh.logger.Debug().Int("error_count", len(errors)).Msg("Aggregated monitor error notifications are disabled in config.")
		return
	}

	if len(errors) == 0 {
		return
	}

	nh.logger.Info().Int("error_count", len(errors)).Msg("Preparing to send aggregated monitor error notification.")
	payload := FormatAggregatedMonitorErrorsMessage(errors, nh.cfg)

	err := nh.discordNotifier.SendNotification(ctx, payload, "") // No report file for this type of notification
	if err != nil {
		nh.logger.Error().Err(err).Str("webhook_url", nh.cfg.DiscordWebhookURL).Msg("Failed to send aggregated monitor error notification")
	}
}
