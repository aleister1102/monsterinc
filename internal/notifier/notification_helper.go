package notifier

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
// It now accepts a slice of reportFilePaths for multi-part reports.
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData, serviceType NotificationServiceType, reportFilePaths []string) {
	if !nh.cfg.NotifyOnSuccess && summary.Status == string(models.ScanStatusCompleted) {
		nh.logger.Info().Str("status", summary.Status).Msg("Scan success notification is disabled, skipping.")
		return
	}
	if !nh.cfg.NotifyOnFailure && (summary.Status == string(models.ScanStatusFailed) || summary.Status == string(models.ScanStatusPartialComplete)) {
		nh.logger.Info().Str("status", summary.Status).Msg("Scan failure notification is disabled, skipping.")
		return
	}

	webhookURL := nh.getWebhookURL(serviceType)
	if webhookURL == "" {
		nh.logger.Warn().Str("service_type", fmt.Sprintf("%d", serviceType)).Msg("Webhook URL is not configured for this service type. Skipping scan completion notification.")
		return
	}

	// If there are multiple report files, the first message will be the main one with full embeds.
	// Subsequent messages will be minimal, just attaching the file.
	if len(reportFilePaths) > 0 {
		for i, reportPath := range reportFilePaths {
			var payload models.DiscordMessagePayload
			isPrimaryMessage := (i == 0)

			if isPrimaryMessage {
				payload = FormatScanCompleteMessage(summary, nh.cfg) // Full message for the first part
				// Adjust embed to note if report is split
				if len(reportFilePaths) > 1 {
					for embedIdx := range payload.Embeds {
						payload.Embeds[embedIdx].Description += fmt.Sprintf("\nReport is split into %d parts. This is part 1.", len(reportFilePaths))
						// Remove or adjust the generic "Report is attached below" field if FormatScanCompleteMessage adds it
						// and replace with a more specific one for part 1, or rely on the loop to explain attachments.
						foundReportField := false
						for fieldIdx, field := range payload.Embeds[embedIdx].Fields {
							if field.Name == "📄 Report" { // Assuming this is how the formatter names it
								payload.Embeds[embedIdx].Fields[fieldIdx].Value = fmt.Sprintf("Report part 1 of %d is attached.", len(reportFilePaths))
								foundReportField = true
								break
							}
						}
						if !foundReportField && payload.Embeds[embedIdx].Title != "" { // Add field if not present and embed exists
							payload.Embeds[embedIdx].Fields = append(payload.Embeds[embedIdx].Fields, models.DiscordEmbedField{
								Name:   "📄 Report",
								Value:  fmt.Sprintf("Report part 1 of %d is attached.", len(reportFilePaths)),
								Inline: false,
							})
						}
					}
				}
			} else {
				// Minimal message for subsequent parts
				payload = FormatSecondaryReportPartMessage(summary.ScanSessionID, i+1, len(reportFilePaths), &nh.cfg)
			}

			nh.logger.Info().Str("status", summary.Status).Str("session_id", summary.ScanSessionID).Int("part", i+1).Int("total_parts", len(reportFilePaths)).Msgf("Attempting to send scan completion notification (part %d/%d).", i+1, len(reportFilePaths))
			err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportPath) // reportPath is the current part
			if err != nil {
				nh.logger.Error().Err(err).Int("part", i+1).Msgf("Failed to send scan completion notification (part %d/%d)", i+1, len(reportFilePaths))
				// Optionally, decide if we should stop sending further parts on error
			} else {
				nh.logger.Info().Int("part", i+1).Msgf("Scan completion notification (part %d/%d) sent successfully.", i+1, len(reportFilePaths))
				if reportPath != "" && nh.cfg.AutoDeleteReportAfterDiscordNotification {
					// Logic for deleting the specific reportPath part
					nh.logger.Info().Str("report_part_path", reportPath).Msg("Auto-deleting report file part after successful Discord notification.")
					if errDel := os.Remove(reportPath); errDel != nil {
						nh.logger.Error().Err(errDel).Str("report_part_path", reportPath).Msg("Failed to auto-delete report file part.")
					}
				}
			}
			// Add a small delay between sending parts if there are more to send
			if i < len(reportFilePaths)-1 {
				time.Sleep(1 * time.Second) // 1-second delay
			}
		}
	} else {
		// No report files to attach, send a single notification
		payload := FormatScanCompleteMessage(summary, nh.cfg)
		// Adjust embed if ReportPath was set to a multi-part message but somehow no paths were given here
		if strings.HasPrefix(summary.ReportPath, "Multiple report files generated") {
			for embedIdx := range payload.Embeds {
				for fieldIdx, field := range payload.Embeds[embedIdx].Fields {
					if field.Name == "📄 Report" {
						payload.Embeds[embedIdx].Fields[fieldIdx].Value = "Report generated, but not attached (check local files or configuration)."
						break
					}
				}
			}
		}

		nh.logger.Info().Str("status", summary.Status).Str("session_id", summary.ScanSessionID).Msg("Attempting to send scan completion notification (no report attachments).")
		err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "") // No report path
		if err != nil {
			nh.logger.Error().Err(err).Msg("Failed to send scan completion notification")
		} else {
			nh.logger.Info().Msg("Scan completion notification sent successfully.")
			// No auto-delete logic here as no file was attached
		}
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
	if len(changes) == 0 {
		nh.logger.Info().Msg("No file changes to notify.")
		return
	}

	webhookURL := nh.getWebhookURL(MonitorServiceNotification) // Monitor service typically sends these
	if webhookURL == "" {
		nh.logger.Warn().Msg("Monitor service webhook URL is not configured. Skipping aggregated file changes notification.")
		return
	}

	payload := FormatAggregatedFileChangesMessage(changes, nh.cfg)

	// Determine if report should be attached or if it's too large/etc.
	finalReportPath := reportFilePath
	// Note: Logic for zipping large reports will be handled in discordNotifier.SendNotification

	nh.logger.Info().Int("changes_count", len(changes)).Msg("Attempting to send aggregated file changes notification.")
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, finalReportPath)
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send aggregated file changes notification")
	} else {
		nh.logger.Info().Msg("Aggregated file changes notification sent successfully.")
		// Auto-delete logic for the aggregated report
		if finalReportPath != "" && nh.cfg.AutoDeleteReportAfterDiscordNotification {
			// Similar to scan completion, assume temporary zip (if any) is handled by SendNotification
			if finalReportPath == reportFilePath { // Check if it's the original path
				nh.logger.Info().Str("report_path", reportFilePath).Msg("Auto-deleting aggregated diff report file after successful Discord notification.")
				if errDel := os.Remove(reportFilePath); errDel != nil {
					nh.logger.Error().Err(errDel).Str("report_path", reportFilePath).Msg("Failed to auto-delete aggregated diff report file.")
				}
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

// SendMonitorCycleCompleteNotification sends a notification when a monitor cycle completes.
func (nh *NotificationHelper) SendMonitorCycleCompleteNotification(ctx context.Context, data models.MonitorCycleCompleteData) {
	webhookURL := nh.getWebhookURL(MonitorServiceNotification)
	if webhookURL == "" {
		nh.logger.Warn().Msg("Monitor service webhook URL is not configured. Skipping monitor cycle complete notification.")
		return
	}

	// This function also needs to be adapted if it can send multi-part reports.
	// For now, assuming monitor cycle report is a single file.
	// If MonitorCycleCompleteData.ReportPath could become multiple paths, this needs similar logic to SendScanCompletionNotification.
	reportFilePaths := []string{}
	if data.ReportPath != "" {
		// This assumes ReportPath is a single path. If it could represent multiple (e.g. comma-separated or a placeholder for split reports),
		// then logic to split/interpret it would be needed here, or GenerateReport for monitor cycle should return []string.
		// For now, assume single path for simplicity, matching current structure.
		reportFilePaths = append(reportFilePaths, data.ReportPath)
	}

	if len(reportFilePaths) > 0 {
		for i, reportPath := range reportFilePaths { // Loop even for one, to keep structure similar
			var payload models.DiscordMessagePayload
			isPrimaryMessage := (i == 0)

			if isPrimaryMessage {
				payload = FormatMonitorCycleCompleteMessage(data, nh.cfg)
				if len(reportFilePaths) > 1 { // Should not happen with current assumption, but good for future proofing
					for embedIdx := range payload.Embeds {
						payload.Embeds[embedIdx].Description += fmt.Sprintf("\nReport is split into %d parts. This is part 1.", len(reportFilePaths))
					}
				}
			} else {
				// This part of logic would only be hit if data.ReportPath somehow implied multiple files AND we decided to split messages here.
				// Keeping it minimal as it's not the current primary path for this function.
				payload = FormatSecondaryReportPartMessage("MonitorCycle-"+time.Now().Format("20060102150405"), i+1, len(reportFilePaths), &nh.cfg)
			}

			nh.logger.Info().Int("monitored_urls", data.TotalMonitored).Int("changed_urls", len(data.ChangedURLs)).Int("part", i+1).Msgf("Attempting to send monitor cycle complete notification (part %d/%d).", i+1, len(reportFilePaths))
			err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportPath)
			if err != nil {
				nh.logger.Error().Err(err).Int("part", i+1).Msgf("Failed to send monitor cycle complete notification (part %d/%d)", i+1, len(reportFilePaths))
			} else {
				nh.logger.Info().Int("part", i+1).Msgf("Monitor cycle complete notification (part %d/%d) sent successfully.", i+1, len(reportFilePaths))
				if reportPath != "" && nh.cfg.AutoDeleteReportAfterDiscordNotification {
					nh.logger.Info().Str("report_path", reportPath).Msg("Auto-deleting monitor cycle report file after successful Discord notification.")
					if errDel := os.Remove(reportPath); errDel != nil {
						nh.logger.Error().Err(errDel).Str("report_path", reportPath).Msg("Failed to auto-delete monitor cycle report file.")
					}
				}
			}
			if i < len(reportFilePaths)-1 {
				time.Sleep(1 * time.Second)
			}
		}
	} else {
		// No report file attached
		payload := FormatMonitorCycleCompleteMessage(data, nh.cfg)
		nh.logger.Info().Int("monitored_urls", data.TotalMonitored).Int("changed_urls", len(data.ChangedURLs)).Msg("Attempting to send monitor cycle complete notification (no report attachment).")
		err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
		if err != nil {
			nh.logger.Error().Err(err).Msg("Failed to send monitor cycle complete notification")
		} else {
			nh.logger.Info().Msg("Monitor cycle complete notification sent successfully.")
		}
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
