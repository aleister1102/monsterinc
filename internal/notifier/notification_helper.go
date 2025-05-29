package notifier

import (
	"context"
	"os"

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
	webhookURL := nh.getWebhookURL(ScanServiceNotification)
	if !nh.cfg.NotifyOnScanStart || nh.discordNotifier == nil || webhookURL == "" {
		nh.logger.Debug().Msg("Scan start notification disabled or notifier/webhook not configured.")
		return
	}

	nh.logger.Info().Str("scan_session_id", summary.ScanSessionID).Str("target_source", summary.TargetSource).Int("total_targets", summary.TotalTargets).Msg("Preparing to send scan start notification.")

	payload := FormatScanStartMessage(summary, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "") // No report file
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
	webhookURL := nh.getWebhookURL(GenericNotification) // Use generic as critical errors might not be service-specific
	if !nh.cfg.NotifyOnCriticalError || nh.discordNotifier == nil || webhookURL == "" {
		nh.logger.Debug().Msg("Critical error notification disabled or notifier/webhook not configured.")
		return
	}
	if summaryData.Component == "" {
		summaryData.Component = componentName
	}

	nh.logger.Info().Str("component", summaryData.Component).Strs("errors", summaryData.ErrorMessages).Msg("Preparing to send critical error notification.")

	payload := FormatCriticalErrorMessage(summaryData, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "") // No report file
	if err != nil {
		nh.logger.Error().Err(err).Str("component", summaryData.Component).Msg("Failed to send critical error notification")
	} else {
		nh.logger.Info().Str("component", summaryData.Component).Msg("Critical error notification sent successfully.")
	}
}

// SendAggregatedFileChangesNotification sends an aggregated notification for multiple file changes from MonitorService.
func (nh *NotificationHelper) SendAggregatedFileChangesNotification(ctx context.Context, changes []models.FileChangeInfo, reportFilePath string) {
	webhookURL := nh.getWebhookURL(MonitorServiceNotification)
	if nh.discordNotifier == nil || webhookURL == "" || len(changes) == 0 {
		nh.logger.Debug().Msg("Aggregated file changes notification not sent: notifier/webhook not configured or no changes.")
		return
	}
	nh.logger.Info().Int("change_count", len(changes)).Str("report_file", reportFilePath).Msg("Preparing to send aggregated file changes notification.")
	payload := FormatAggregatedFileChangesMessage(changes, nh.cfg)
	if err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportFilePath); err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send aggregated file changes notification")
	} else {
		nh.logger.Info().Int("change_count", len(changes)).Msg("Aggregated file changes notification sent successfully.")

		// Auto-delete single diff reports if configured
		if nh.cfg.AutoDeleteReportAfterDiscordNotification {
			// Collect single diff report paths from changes
			singleReportPaths := make([]string, 0)
			for _, change := range changes {
				if change.DiffReportPath != nil && *change.DiffReportPath != "" {
					singleReportPaths = append(singleReportPaths, *change.DiffReportPath)
				}
			}

			// Delete single diff reports
			for _, path := range singleReportPaths {
				nh.logger.Info().Str("single_report_path", path).Msg("Attempting to auto-delete single diff report file after successful aggregated notification.")
				if errDel := os.Remove(path); errDel != nil {
					nh.logger.Error().Err(errDel).Str("single_report_path", path).Msg("Failed to auto-delete single diff report file.")
				} else {
					nh.logger.Info().Str("single_report_path", path).Msg("Successfully auto-deleted single diff report file.")
				}
			}
		}

		// Auto-delete aggregated report file if configured and path exists
		if nh.cfg.AutoDeleteReportAfterDiscordNotification && reportFilePath != "" {
			nh.logger.Info().Str("report_path", reportFilePath).Msg("Attempting to auto-delete aggregated diff report file after successful Discord notification.")
			if errDel := os.Remove(reportFilePath); errDel != nil {
				nh.logger.Error().Err(errDel).Str("report_path", reportFilePath).Msg("Failed to auto-delete aggregated diff report file.")
			} else {
				nh.logger.Info().Str("report_path", reportFilePath).Msg("Successfully auto-deleted aggregated diff report file.")
			}

			// Also delete the assets directory if it exists
			assetsDir := "reports/diff/assets"
			nh.logger.Info().Str("assets_dir", assetsDir).Msg("Attempting to auto-delete diff assets directory after successful Discord notification.")
			if errDelAssets := os.RemoveAll(assetsDir); errDelAssets != nil {
				nh.logger.Error().Err(errDelAssets).Str("assets_dir", assetsDir).Msg("Failed to auto-delete diff assets directory.")
			} else {
				nh.logger.Info().Str("assets_dir", assetsDir).Msg("Successfully auto-deleted diff assets directory.")
			}
		}
	}
}

// SendInitialMonitoredURLsNotification sends a notification listing the initial set of URLs being monitored from MonitorService.
func (nh *NotificationHelper) SendInitialMonitoredURLsNotification(ctx context.Context, monitoredURLs []string) {
	webhookURL := nh.getWebhookURL(MonitorServiceNotification)
	if nh.discordNotifier == nil || webhookURL == "" || len(monitoredURLs) == 0 {
		nh.logger.Debug().Msg("Initial monitored URLs notification not sent: notifier/webhook not configured or no URLs.")
		return
	}

	payload := FormatInitialMonitoredURLsMessage(monitoredURLs, nh.cfg)
	nh.logger.Info().Int("url_count", len(monitoredURLs)).Msg("Preparing to send initial monitored URLs notification.")

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "") // No report file
	if err != nil {
		nh.logger.Error().Err(err).Int("url_count", len(monitoredURLs)).Msg("Failed to send initial monitored URLs notification.")
	} else {
		nh.logger.Info().Int("url_count", len(monitoredURLs)).Msg("Initial monitored URLs notification sent successfully.")
	}
}

// SendAggregatedMonitorErrorsNotification sends an aggregated notification for monitor service fetch errors.
func (nh *NotificationHelper) SendAggregatedMonitorErrorsNotification(ctx context.Context, errors []models.MonitorFetchErrorInfo) {
	webhookURL := nh.getWebhookURL(MonitorServiceNotification)
	if nh.discordNotifier == nil || webhookURL == "" {
		nh.logger.Debug().Int("error_count", len(errors)).Msg("Discord notifier or monitor webhook is disabled, skipping aggregated monitor error notification.")
		return
	}

	if !nh.cfg.NotifyOnCriticalError { // Assuming these errors fall under critical
		nh.logger.Debug().Int("error_count", len(errors)).Msg("Aggregated monitor error notifications are disabled in config (via NotifyOnCriticalError).")
		return
	}

	if len(errors) == 0 {
		return
	}

	nh.logger.Info().Int("error_count", len(errors)).Msg("Preparing to send aggregated monitor error notification.")
	payload := FormatAggregatedMonitorErrorsMessage(errors, nh.cfg)

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "") // No report file
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send aggregated monitor error notification")
	}
}

// SendHighSeveritySecretNotification sends a notification for high-severity secret findings.
func (nh *NotificationHelper) SendHighSeveritySecretNotification(ctx context.Context, finding models.SecretFinding, serviceType NotificationServiceType) {
	webhookURL := nh.getWebhookURL(serviceType)
	if !nh.cfg.NotifyOnHighSeverity || nh.discordNotifier == nil || webhookURL == "" {
		nh.logger.Debug().Msg("High severity secret notification disabled or notifier/webhook not configured.")
		return
	}

	nh.logger.Info().Str("rule_id", finding.RuleID).Str("source_url", finding.SourceURL).Msg("Preparing to send high severity secret notification.")

	payload := FormatHighSeveritySecretNotification(finding, nh.cfg)
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "") // No report file for individual secret findings usually
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send high severity secret notification")
	} else {
		nh.logger.Info().Str("rule_id", finding.RuleID).Msg("High severity secret notification sent successfully.")
	}
}

// SendMonitorCycleCompleteNotification sends a notification when a monitoring cycle is complete.
func (nh *NotificationHelper) SendMonitorCycleCompleteNotification(ctx context.Context, data models.MonitorCycleCompleteData) {
	webhookURL := nh.getWebhookURL(MonitorServiceNotification)
	if nh.discordNotifier == nil || webhookURL == "" {
		nh.logger.Debug().Msg("Monitor cycle complete notification not sent: notifier/webhook not configured.")
		return
	}

	nh.logger.Info().Int("changed_urls_count", len(data.ChangedURLs)).Str("report_path", data.ReportPath).Msg("Preparing to send monitor cycle complete notification.")

	payload := FormatMonitorCycleCompleteMessage(data, nh.cfg)
	if err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, data.ReportPath); err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send monitor cycle complete notification")
	} else {
		nh.logger.Info().Msg("Monitor cycle complete notification sent successfully.")
		// Auto-delete aggregated report file if configured and path exists
		if nh.cfg.AutoDeleteReportAfterDiscordNotification && data.ReportPath != "" {
			nh.logger.Info().Str("report_path", data.ReportPath).Msg("Attempting to auto-delete aggregated monitor report file after successful Discord notification.")
			if errDel := os.Remove(data.ReportPath); errDel != nil {
				nh.logger.Error().Err(errDel).Str("report_path", data.ReportPath).Msg("Failed to auto-delete aggregated monitor report file.")
			} else {
				nh.logger.Info().Str("report_path", data.ReportPath).Msg("Successfully auto-deleted aggregated monitor report file.")
			}
		}
	}
}
