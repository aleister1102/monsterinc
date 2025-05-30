package notifier

import (
	"context"
	"os"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// NotificationServiceType helps determine which webhook URL to use.
// This is an internal type for NotificationHelper.
type NotificationServiceType int

const (
	ScanServiceNotification NotificationServiceType = iota
	MonitorServiceNotification
	GenericNotification // For critical errors that might not originate from a specific service
)

// NotificationHelper provides a high-level interface for sending various scan-related notifications.
type NotificationHelper struct {
	discordNotifier *DiscordNotifier
	cfg             config.NotificationConfig
	logger          zerolog.Logger
}

// NewNotificationHelper creates a new NotificationHelper.
func NewNotificationHelper(dn *DiscordNotifier, cfg config.NotificationConfig, logger zerolog.Logger) *NotificationHelper {
	nh := &NotificationHelper{
		discordNotifier: dn,
		cfg:             cfg,
		logger:          logger.With().Str("module", "NotificationHelper").Logger(),
	}
	return nh
}

// getWebhookURL selects the appropriate webhook URL based on the service type.
func (nh *NotificationHelper) getWebhookURL(serviceType NotificationServiceType) string {
	switch serviceType {
	case ScanServiceNotification:
		return nh.cfg.ScanServiceDiscordWebhookURL
	case MonitorServiceNotification:
		return nh.cfg.MonitorServiceDiscordWebhookURL
	case GenericNotification: // Fallback or for general critical errors
		// Prefer scan service webhook for general errors, or make this configurable
		if nh.cfg.ScanServiceDiscordWebhookURL != "" {
			return nh.cfg.ScanServiceDiscordWebhookURL
		}
		return nh.cfg.MonitorServiceDiscordWebhookURL // Fallback to monitor if scan isn't set
	default:
		nh.logger.Warn().Int("service_type", int(serviceType)).Msg("Unknown service type for webhook URL, defaulting to scan service webhook.")
		return nh.cfg.ScanServiceDiscordWebhookURL
	}
}

// SendScanStartNotification sends a notification when a scan starts.
// It now accepts ScanSummaryData for a more structured approach.
func (nh *NotificationHelper) SendScanStartNotification(ctx context.Context, summary models.ScanSummaryData) {
	if !nh.cfg.NotifyOnScanStart || nh.discordNotifier == nil || nh.cfg.ScanServiceDiscordWebhookURL == "" {
		return
	}

	nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Str("target_source", summary.TargetSource).Int("total_targets", summary.TotalTargets).Msg("Preparing to send scan start notification.")

	payload := FormatScanStartMessage(summary, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.ScanServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send scan start notification")
	} else {
		nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Scan start notification sent successfully.")
	}
}

// SendScanCompletionNotification sends a notification when a scan completes (successfully or with failure).
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData, serviceType NotificationServiceType) {
	// Determine if notification should be sent based on success/failure and config
	shouldSend := false
	successStatus := summary.Status == string(models.ScanStatusCompleted) || summary.Status == string(models.ScanStatusCompletedWithIssues)

	if successStatus && nh.cfg.NotifyOnSuccess {
		shouldSend = true
	} else if !successStatus && nh.cfg.NotifyOnFailure {
		// This covers FAILED, INTERRUPTED, NO_TARGETS etc. as non-success cases
		shouldSend = true
	}

	if shouldSend {
		nh.logger.Info().
			Str("scan_session_id", summary.ScanSessionID).
			Str("status", summary.Status).
			Str("target_source", summary.TargetSource).
			Msg("Preparing to send scan completion notification.")

		payload := FormatScanCompleteMessage(summary, nh.cfg)
		webhookURL := nh.getWebhookURL(serviceType) // Use the passed serviceType

		if webhookURL == "" {
			nh.logger.Warn().Msg("Webhook URL is empty for the determined service type, skipping scan completion notification.")
			return
		}

		reportFilePath := "" // Default to no file
		// Only attach report if the scan was completed (or partially) and a path exists.
		// For interruptions or critical errors before report generation, this path will be empty.
		if (summary.Status == string(models.ScanStatusCompleted) || summary.Status == string(models.ScanStatusPartialComplete) || summary.Status == string(models.ScanStatusCompletedWithIssues)) && summary.ReportPath != "" {
			reportFilePath = summary.ReportPath
		}

		err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportFilePath)
		if err != nil {
			nh.logger.Error().Err(err).Msg("Failed to send scan completion notification")
		} else {
			nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Scan completion notification sent successfully.")
			// Auto-delete report file if configured and path exists
			if nh.cfg.AutoDeleteReportAfterDiscordNotification && reportFilePath != "" {
				nh.logger.Info().Str("report_path", reportFilePath).Msg("Attempting to auto-delete report file after successful Discord notification.")
				if errDel := os.Remove(reportFilePath); errDel != nil {
					nh.logger.Error().Err(errDel).Str("report_path", reportFilePath).Msg("Failed to auto-delete report file.")
				} else {
					nh.logger.Info().Str("report_path", reportFilePath).Msg("Successfully auto-deleted report file.")
				}
			}
		}
	} else {
		nh.logger.Debug().
			Str("scan_session_id", summary.ScanSessionID).
			Str("status", summary.Status).
			Bool("notify_on_success", nh.cfg.NotifyOnSuccess).
			Bool("notify_on_failure", nh.cfg.NotifyOnFailure).
			Msg("Scan completion notification skipped based on status and configuration.")
	}
}

// SendCriticalErrorNotification sends a notification for critical application errors.
// componentName helps identify where the error occurred (e.g., "SchedulerInitialization", "ConfigLoad").
// summaryData contains error messages and other relevant info.
func (nh *NotificationHelper) SendCriticalErrorNotification(ctx context.Context, componentName string, summaryData models.ScanSummaryData) {
	if !nh.cfg.NotifyOnCriticalError || nh.discordNotifier == nil {
		return
	}

	webhookURL := nh.getWebhookURL(ScanServiceNotification)
	if webhookURL == "" {
		return
	}

	summaryData.Component = componentName
	nh.logger.Info().Str("component", summaryData.Component).Strs("errors", summaryData.ErrorMessages).Msg("Preparing to send critical error notification.")

	payload := FormatCriticalErrorMessage(summaryData, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Str("component", summaryData.Component).Msg("Failed to send critical error notification")
	} else {
		nh.logger.Info().Str("component", summaryData.Component).Msg("Critical error notification sent successfully.")
	}
}

// SendAggregatedFileChangesNotification sends an aggregated notification for multiple file changes from MonitorService.
func (nh *NotificationHelper) SendAggregatedFileChangesNotification(ctx context.Context, changes []models.FileChangeInfo, reportFilePath string) {
	if nh.discordNotifier == nil || nh.cfg.MonitorServiceDiscordWebhookURL == "" || len(changes) == 0 {
		return
	}

	nh.logger.Info().Int("change_count", len(changes)).Str("report_file", reportFilePath).Msg("Preparing to send aggregated file changes notification.")

	payload := FormatAggregatedFileChangesMessage(changes, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.MonitorServiceDiscordWebhookURL, payload, reportFilePath)
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send aggregated file changes notification")
	} else {
		nh.logger.Info().Int("change_count", len(changes)).Msg("Aggregated file changes notification sent successfully.")

		if nh.cfg.AutoDeleteReportAfterDiscordNotification && reportFilePath != "" {
			for _, change := range changes {
				if change.DiffReportPath != nil {
					path := *change.DiffReportPath
					if err := os.Remove(path); err != nil {
						nh.logger.Error().Err(err).Str("single_report_path", path).Msg("Failed to auto-delete single diff report file.")
					} else {
						nh.logger.Info().Str("single_report_path", path).Msg("Successfully auto-deleted single diff report file.")
					}
				}
			}

			if err := os.Remove(reportFilePath); err != nil {
				nh.logger.Error().Err(err).Str("report_path", reportFilePath).Msg("Failed to auto-delete aggregated diff report file.")
			} else {
				nh.logger.Info().Str("report_path", reportFilePath).Msg("Successfully auto-deleted aggregated diff report file.")
			}

			assetsDir := filepath.Join(filepath.Dir(reportFilePath), "assets")
			if err := os.RemoveAll(assetsDir); err != nil {
				nh.logger.Error().Err(err).Str("assets_dir", assetsDir).Msg("Failed to auto-delete diff assets directory.")
			} else {
				nh.logger.Info().Str("assets_dir", assetsDir).Msg("Successfully auto-deleted diff assets directory.")
			}
		}
	}
}

// SendInitialMonitoredURLsNotification sends a notification listing the initial set of URLs being monitored from MonitorService.
func (nh *NotificationHelper) SendInitialMonitoredURLsNotification(ctx context.Context, monitoredURLs []string) {
	if nh.discordNotifier == nil || nh.cfg.MonitorServiceDiscordWebhookURL == "" || len(monitoredURLs) == 0 {
		return
	}

	nh.logger.Info().Int("url_count", len(monitoredURLs)).Msg("Preparing to send initial monitored URLs notification.")

	payload := FormatInitialMonitoredURLsMessage(monitoredURLs, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.MonitorServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Int("url_count", len(monitoredURLs)).Msg("Failed to send initial monitored URLs notification.")
	} else {
		nh.logger.Info().Int("url_count", len(monitoredURLs)).Msg("Initial monitored URLs notification sent successfully.")
	}
}

// SendAggregatedMonitorErrorsNotification sends an aggregated notification for monitor service fetch errors.
func (nh *NotificationHelper) SendAggregatedMonitorErrorsNotification(ctx context.Context, errors []models.MonitorFetchErrorInfo) {
	if nh.discordNotifier == nil || nh.cfg.MonitorServiceDiscordWebhookURL == "" {
		return
	}

	if !nh.cfg.NotifyOnCriticalError {
		return
	}

	if len(errors) == 0 {
		return
	}

	nh.logger.Info().Int("error_count", len(errors)).Msg("Preparing to send aggregated monitor error notification.")

	payload := FormatAggregatedMonitorErrorsMessage(errors, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.MonitorServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send aggregated monitor error notification")
	}
}

// SendHighSeveritySecretNotification sends a notification for high-severity secret findings.
func (nh *NotificationHelper) SendHighSeveritySecretNotification(ctx context.Context, finding models.SecretFinding, serviceType NotificationServiceType) {
	webhookURL := nh.getWebhookURL(serviceType)
	if !nh.cfg.NotifyOnHighSeverity || nh.discordNotifier == nil || webhookURL == "" {
		return
	}

	nh.logger.Info().Str("rule_id", finding.RuleID).Str("source_url", finding.SourceURL).Msg("Preparing to send high severity secret notification.")

	payload := FormatHighSeveritySecretNotification(finding, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send high severity secret notification")
	} else {
		nh.logger.Info().Str("rule_id", finding.RuleID).Msg("High severity secret notification sent successfully.")
	}
}

// SendMonitorCycleCompleteNotification sends a notification when a monitor cycle completes.
func (nh *NotificationHelper) SendMonitorCycleCompleteNotification(ctx context.Context, data models.MonitorCycleCompleteData) {
	if nh.discordNotifier == nil || nh.cfg.MonitorServiceDiscordWebhookURL == "" {
		return
	}

	nh.logger.Info().Int("changed_urls", len(data.ChangedURLs)).Int("total_monitored", data.TotalMonitored).Msg("Preparing to send monitor cycle complete notification.")

	payload := FormatMonitorCycleCompleteMessage(data, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.MonitorServiceDiscordWebhookURL, payload, data.ReportPath)
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send monitor cycle complete notification")
	} else {
		nh.logger.Info().Msg("Monitor cycle complete notification sent successfully.")
	}
}

// SendMonitorInterruptNotification sends a notification when monitoring service is interrupted.
func (nh *NotificationHelper) SendMonitorInterruptNotification(ctx context.Context, summaryData models.ScanSummaryData) {
	if !nh.cfg.NotifyOnCriticalError || nh.discordNotifier == nil {
		return
	}

	webhookURL := nh.getWebhookURL(MonitorServiceNotification)
	if webhookURL == "" {
		return
	}

	nh.logger.Info().Str("component", summaryData.Component).Strs("errors", summaryData.ErrorMessages).Msg("Preparing to send monitor interrupt notification.")

	payload := FormatInterruptNotificationMessage(summaryData, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Str("component", summaryData.Component).Msg("Failed to send monitor interrupt notification")
	} else {
		nh.logger.Info().Str("component", summaryData.Component).Msg("Monitor interrupt notification sent successfully.")
	}
}

// SendMonitorStartNotification sends a notification when monitoring service starts.
func (nh *NotificationHelper) SendMonitorStartNotification(ctx context.Context, summary models.ScanSummaryData) {
	if !nh.cfg.NotifyOnScanStart || nh.discordNotifier == nil || nh.cfg.MonitorServiceDiscordWebhookURL == "" {
		return
	}

	nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Str("target_source", summary.TargetSource).Int("total_targets", summary.TotalTargets).Msg("Preparing to send monitor start notification.")

	payload := FormatScanStartMessage(summary, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.MonitorServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send monitor start notification")
	} else {
		nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Monitor start notification sent successfully.")
	}
}

// SendScanInterruptNotification sends a notification when scan service is interrupted.
func (nh *NotificationHelper) SendScanInterruptNotification(ctx context.Context, summaryData models.ScanSummaryData) {
	if !nh.cfg.NotifyOnCriticalError || nh.discordNotifier == nil {
		return
	}

	webhookURL := nh.getWebhookURL(ScanServiceNotification)
	if webhookURL == "" {
		return
	}

	nh.logger.Info().Str("component", summaryData.Component).Strs("errors", summaryData.ErrorMessages).Msg("Preparing to send scan interrupt notification.")

	payload := FormatInterruptNotificationMessage(summaryData, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Str("component", summaryData.Component).Msg("Failed to send scan interrupt notification")
	} else {
		nh.logger.Info().Str("component", summaryData.Component).Msg("Scan interrupt notification sent successfully.")
	}
}
