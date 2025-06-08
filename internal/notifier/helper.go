package notifier

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// NotificationServiceType defines the type of notification service.
type NotificationServiceType int

const (
	ScanServiceNotification NotificationServiceType = iota
	MonitorServiceNotification
	GenericNotification
)

// DiffReportCleaner defines an interface for cleaning up diff reports.
type DiffReportCleaner interface {
	DeleteAllSingleDiffReports() error
}

// NotificationHelper provides a high-level interface for sending various scan-related notifications.
type NotificationHelper struct {
	discordNotifier   *DiscordNotifier
	cfg               config.NotificationConfig
	logger            zerolog.Logger
	diffReportCleaner DiffReportCleaner // Added for cleaning up single diff reports
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

// SetDiffReportCleaner sets the diff report cleaner for auto-deletion functionality.
func (nh *NotificationHelper) SetDiffReportCleaner(cleaner DiffReportCleaner) {
	nh.diffReportCleaner = cleaner
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
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData, serviceType NotificationServiceType, reportFilePaths []string) {
	if !nh.shouldSendScanCompletionNotification(summary) {
		return
	}

	webhookURL := nh.getWebhookURL(serviceType)
	if webhookURL == "" {
		nh.logger.Warn().Str("service_type", fmt.Sprintf("%d", serviceType)).Msg("Webhook URL is not configured for this service type. Skipping scan completion notification.")
		return
	}

	// Always send only summary notification (no individual report parts)
	nh.sendSummaryOnlyReport(ctx, summary, webhookURL, reportFilePaths)
}

// shouldSendScanCompletionNotification checks if notification should be sent based on config and scan status
func (nh *NotificationHelper) shouldSendScanCompletionNotification(summary models.ScanSummaryData) bool {
	if !nh.cfg.NotifyOnSuccess && summary.Status == string(models.ScanStatusCompleted) {
		nh.logger.Info().Str("status", summary.Status).Msg("Scan success notification is disabled, skipping.")
		return false
	}
	if !nh.cfg.NotifyOnFailure && (summary.Status == string(models.ScanStatusFailed) || summary.Status == string(models.ScanStatusPartialComplete)) {
		nh.logger.Info().Str("status", summary.Status).Msg("Scan failure notification is disabled, skipping.")
		return false
	}
	return true
}

// sendSummaryOnlyReport sends a single notification with all report files attached
func (nh *NotificationHelper) sendSummaryOnlyReport(ctx context.Context, summary models.ScanSummaryData, webhookURL string, reportFilePaths []string) {
	if len(reportFilePaths) > 0 {
		// Send single notification with all reports
		nh.sendSingleNotificationWithAllReports(ctx, summary, webhookURL, reportFilePaths)
	} else {
		// No reports to attach
		nh.sendSingleReport(ctx, summary, webhookURL)
	}
}

// sendSingleNotificationWithAllReports sends one notification with all report files attached
func (nh *NotificationHelper) sendSingleNotificationWithAllReports(ctx context.Context, summary models.ScanSummaryData, webhookURL string, reportFilePaths []string) {
	payload := FormatScanCompleteMessageWithReports(summary, nh.cfg, true)

	// Update payload to indicate multiple reports in single notification
	if len(reportFilePaths) > 1 {
		nh.adjustPayloadForMultipleReports(payload, len(reportFilePaths))
	}

	nh.logger.Info().
		Str("status", summary.Status).
		Str("session_id", summary.ScanSessionID).
		Int("report_count", len(reportFilePaths)).
		Msg("Attempting to send scan completion notification with all reports.")

	// Send notification with first report attached, then send additional reports separately
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportFilePaths[0])
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send scan completion notification")
		return
	}

	nh.logger.Info().Msg("Scan completion notification sent successfully.")

	// Send additional reports as follow-up messages if there are more than 1
	for i := 1; i < len(reportFilePaths); i++ {
		nh.sendAdditionalReport(ctx, summary, webhookURL, reportFilePaths[i], i+1, len(reportFilePaths))

		// Small delay between sends
		if i < len(reportFilePaths)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// sendAdditionalReport sends additional report files as simple attachments
func (nh *NotificationHelper) sendAdditionalReport(ctx context.Context, summary models.ScanSummaryData, webhookURL string, reportPath string, partNum, totalParts int) {
	payload := nh.buildSimpleReportPayload(summary.ScanSessionID, partNum, totalParts)

	nh.logger.Info().
		Str("session_id", summary.ScanSessionID).
		Int("part", partNum).
		Int("total_parts", totalParts).
		Msg("Sending additional report file.")

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportPath)
	if err != nil {
		nh.logger.Error().Err(err).Int("part", partNum).Msg("Failed to send additional report")
	}
}

// buildSimpleReportPayload creates a minimal payload for additional reports
func (nh *NotificationHelper) buildSimpleReportPayload(sessionID string, partNum, totalParts int) models.DiscordMessagePayload {
	embed := NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("📎 Report %d/%d", partNum, totalParts)).
		WithDescription(fmt.Sprintf("**Session:** `%s`", sessionID)).
		WithColor(DefaultEmbedColor).
		WithTimestamp(time.Now()).
		Build()

	return NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		AddEmbed(embed).
		Build()
}

// adjustPayloadForMultipleReports modifies payload to indicate multiple reports are being sent
func (nh *NotificationHelper) adjustPayloadForMultipleReports(payload models.DiscordMessagePayload, reportCount int) {
	for embedIdx := range payload.Embeds {
		foundReportField := false
		for fieldIdx, field := range payload.Embeds[embedIdx].Fields {
			if field.Name == "📄 Report" {
				payload.Embeds[embedIdx].Fields[fieldIdx].Value = fmt.Sprintf("%d reports will be sent (main report attached, additional files follow).", reportCount)
				foundReportField = true
				break
			}
		}

		if !foundReportField && payload.Embeds[embedIdx].Title != "" {
			payload.Embeds[embedIdx].Fields = append(payload.Embeds[embedIdx].Fields, models.DiscordEmbedField{
				Name:   "📄 Report",
				Value:  fmt.Sprintf("%d reports will be sent (main report attached, additional files follow).", reportCount),
				Inline: false,
			})
		}
	}
}

// sendSingleReport sends a single report without attachments
func (nh *NotificationHelper) sendSingleReport(ctx context.Context, summary models.ScanSummaryData, webhookURL string) {
	payload := FormatScanCompleteMessageWithReports(summary, nh.cfg, false)
	nh.adjustPayloadForNoAttachments(payload, summary)

	nh.logger.Info().Str("status", summary.Status).Str("session_id", summary.ScanSessionID).Msg("Attempting to send scan completion notification (no report attachments).")

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send scan completion notification")
	}
}

// adjustPayloadForNoAttachments modifies payload when no attachments are present
func (nh *NotificationHelper) adjustPayloadForNoAttachments(payload models.DiscordMessagePayload, summary models.ScanSummaryData) {
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
}

// canSendMonitorNotification checks if monitor notifications can be sent
func (nh *NotificationHelper) canSendMonitorNotification() bool {
	return nh.discordNotifier != nil && nh.cfg.MonitorServiceDiscordWebhookURL != ""
}

// sendSimpleMonitorNotification sends a monitor notification without file attachment
func (nh *NotificationHelper) sendSimpleMonitorNotification(ctx context.Context, payload models.DiscordMessagePayload, notificationType string) {
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.MonitorServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msgf("Failed to send %s notification", notificationType)
	}
}

// SendCriticalErrorNotification sends a notification for critical application errors.
func (nh *NotificationHelper) SendCriticalErrorNotification(ctx context.Context, componentName string, summaryData models.ScanSummaryData) {
	if !nh.canSendCriticalErrorNotification() {
		return
	}

	webhookURL := nh.getWebhookURL(ScanServiceNotification)
	if webhookURL == "" {
		nh.logger.Warn().Msg("Scan service webhook URL is not configured. Skipping critical error notification.")
		return
	}

	summaryData.Component = componentName
	nh.logger.Error().Str("component", componentName).Interface("summary", summaryData).Msg("Preparing to send critical error notification.")

	payload := FormatCriticalErrorMessage(summaryData, nh.cfg)
	nh.sendCriticalErrorNotification(ctx, webhookURL, payload, componentName)
}

// canSendCriticalErrorNotification checks if critical error notifications can be sent
func (nh *NotificationHelper) canSendCriticalErrorNotification() bool {
	return nh.cfg.NotifyOnCriticalError && nh.discordNotifier != nil
}

// sendCriticalErrorNotification sends a critical error notification to the specified webhook
func (nh *NotificationHelper) sendCriticalErrorNotification(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload, componentName string) {
	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Str("component", componentName).Msg("Failed to send critical error notification")
	} else {
		nh.logger.Info().Str("component", componentName).Msg("Critical error notification sent successfully.")
	}
}

// SendMonitorCycleCompleteNotification sends a notification when a monitor cycle completes.
func (nh *NotificationHelper) SendMonitorCycleCompleteNotification(ctx context.Context, data models.MonitorCycleCompleteData) {
	if !nh.canSendMonitorNotification() {
		nh.logger.Warn().Str("cycle_id", data.CycleID).Msg("❌ MONITOR NOTIFICATION DISABLED OR WEBHOOK NOT CONFIGURED")
		return
	}

	nh.logger.Info().Str("cycle_id", data.CycleID).Int("total_monitored", data.TotalMonitored).Int("changed_count", len(data.ChangedURLs)).Msg("📤 SENDING MONITOR CYCLE COMPLETE NOTIFICATION TO DISCORD")

	payload := FormatMonitorCycleCompleteMessage(data, nh.cfg)

	// Handle multiple report paths similar to scan service
	if len(data.ReportPaths) == 0 {
		nh.sendSimpleMonitorNotification(ctx, payload, "monitor cycle complete")
		nh.logger.Info().Str("cycle_id", data.CycleID).Msg("✅ MONITOR CYCLE COMPLETE NOTIFICATION SENT SUCCESSFULLY (NO REPORTS)")
	} else {
		// Send notification with first report attached, then send additional reports
		nh.sendMonitorNotificationWithAllReports(ctx, data, payload)
	}

	// Clean up ONLY partial diff reports if cleanup is enabled
	if nh.cfg.AutoDeletePartialDiffReports && nh.diffReportCleaner != nil {
		if err := nh.diffReportCleaner.DeleteAllSingleDiffReports(); err != nil {
			nh.logger.Error().Err(err).Msg("Failed to cleanup partial diff reports")
		}
	}
}

// sendMonitorNotificationWithAllReports sends monitor notification with all report files
func (nh *NotificationHelper) sendMonitorNotificationWithAllReports(ctx context.Context, data models.MonitorCycleCompleteData, payload models.DiscordMessagePayload) {
	webhookURL := nh.getWebhookURL(MonitorServiceNotification)

	// Update payload to indicate multiple reports if there are more than 1
	if len(data.ReportPaths) > 1 {
		nh.adjustPayloadForMultipleReports(payload, len(data.ReportPaths))
	}

	// Try to send notification with first report attached
	if err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, data.ReportPaths[0]); err != nil {
		nh.logger.Error().Err(err).Str("webhook_url", webhookURL).Str("notification_type", "monitor cycle complete").Msg("❌ FAILED TO SEND MONITOR NOTIFICATION WITH ATTACHMENT")

		// Fallback: Send notification without attachment
		nh.logger.Info().Str("cycle_id", data.CycleID).Msg("📤 FALLING BACK TO SEND NOTIFICATION WITHOUT ATTACHMENT")

		// Modify payload to indicate reports are available but not attached
		nh.adjustPayloadForAttachmentFailure(payload, strings.Join(data.ReportPaths, ", "))

		if fallbackErr := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, ""); fallbackErr != nil {
			nh.logger.Error().Err(fallbackErr).Str("cycle_id", data.CycleID).Msg("❌ FAILED TO SEND FALLBACK NOTIFICATION WITHOUT ATTACHMENT")
		} else {
			nh.logger.Info().Str("cycle_id", data.CycleID).Int("report_count", len(data.ReportPaths)).Msg("✅ MONITOR CYCLE COMPLETE NOTIFICATION SENT SUCCESSFULLY (FALLBACK WITHOUT ATTACHMENT)")
		}
		return
	}

	nh.logger.Info().Str("cycle_id", data.CycleID).Str("first_report_path", data.ReportPaths[0]).Msg("✅ MONITOR CYCLE COMPLETE NOTIFICATION WITH REPORT SENT SUCCESSFULLY TO DISCORD")

	// Send additional reports as follow-up messages if there are more than 1
	for i := 1; i < len(data.ReportPaths); i++ {
		nh.sendAdditionalMonitorReport(ctx, data, webhookURL, data.ReportPaths[i], i+1, len(data.ReportPaths))

		// Small delay between sends
		if i < len(data.ReportPaths)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// sendAdditionalMonitorReport sends additional monitor report files as simple attachments
func (nh *NotificationHelper) sendAdditionalMonitorReport(ctx context.Context, data models.MonitorCycleCompleteData, webhookURL string, reportPath string, partNum, totalParts int) {
	payload := nh.buildSimpleMonitorReportPayload(data.CycleID, partNum, totalParts)

	nh.logger.Info().
		Str("cycle_id", data.CycleID).
		Int("part", partNum).
		Int("total_parts", totalParts).
		Msg("Sending additional monitor report file.")

	if err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportPath); err != nil {
		nh.logger.Error().Err(err).Int("part", partNum).Msg("Failed to send additional monitor report")

		// Try fallback without attachment for additional reports too
		nh.logger.Info().Str("cycle_id", data.CycleID).Int("part", partNum).Msg("📤 FALLING BACK TO SEND ADDITIONAL REPORT WITHOUT ATTACHMENT")

		// Modify payload to indicate report is available but not attached
		nh.adjustPayloadForAttachmentFailure(payload, reportPath)

		if fallbackErr := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, ""); fallbackErr != nil {
			nh.logger.Error().Err(fallbackErr).Str("cycle_id", data.CycleID).Int("part", partNum).Msg("❌ FAILED TO SEND FALLBACK ADDITIONAL REPORT")
		} else {
			nh.logger.Info().Str("cycle_id", data.CycleID).Int("part", partNum).Msg("✅ ADDITIONAL MONITOR REPORT SENT SUCCESSFULLY (FALLBACK WITHOUT ATTACHMENT)")
		}
	} else {
		nh.logger.Info().Str("cycle_id", data.CycleID).Int("part", partNum).Msg("✅ Additional monitor report sent successfully")
	}
}

// buildSimpleMonitorReportPayload creates a minimal payload for additional monitor reports
func (nh *NotificationHelper) buildSimpleMonitorReportPayload(cycleID string, partNum, totalParts int) models.DiscordMessagePayload {
	embed := NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("📎 Monitor Report %d/%d", partNum, totalParts)).
		WithDescription(fmt.Sprintf("**Cycle ID:** `%s`", cycleID)).
		WithColor(DefaultEmbedColor).
		WithTimestamp(time.Now()).
		Build()

	return NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		AddEmbed(embed).
		Build()
}

// SendMonitoredUrlsNotification sends a notification about monitored URLs.
func (nh *NotificationHelper) SendMonitoredUrlsNotification(ctx context.Context, monitoredURLs []string, cycleID string) {
	if !nh.canSendMonitorNotification() {
		return
	}

	payload := FormatInitialMonitoredURLsMessage(monitoredURLs, cycleID, nh.cfg)
	nh.sendSimpleMonitorNotification(ctx, payload, "monitored URLs")
}

// SendScanInterruptNotification sends a notification when a scan is interrupted.
func (nh *NotificationHelper) SendScanInterruptNotification(ctx context.Context, summaryData models.ScanSummaryData) {
	if !nh.canSendScanFailureNotification() {
		return
	}

	nh.logger.Info().Str("session_id", summaryData.ScanSessionID).Str("component", summaryData.Component).Msg("Preparing to send scan interrupt notification.")

	payload := FormatInterruptNotificationMessage(summaryData, nh.cfg)
	nh.sendSimpleScanNotification(ctx, payload, "scan interrupt")
}

// canSendScanFailureNotification checks if scan failure notifications can be sent
func (nh *NotificationHelper) canSendScanFailureNotification() bool {
	return nh.cfg.NotifyOnFailure && nh.discordNotifier != nil && nh.cfg.ScanServiceDiscordWebhookURL != ""
}

// sendSimpleScanNotification sends a scan notification without file attachment
func (nh *NotificationHelper) sendSimpleScanNotification(ctx context.Context, payload models.DiscordMessagePayload, notificationType string) {
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.ScanServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msgf("Failed to send %s notification", notificationType)
	}
}

// handleMonitorContextCancellation handles monitor context cancellation
func (nh *NotificationHelper) handleMonitorContextCancellation() {
	nh.logger.Info().Msg("Context cancelled, stopping monitor service")
}

// SendMonitorStartNotification sends a notification when monitor service starts.
func (nh *NotificationHelper) SendMonitorStartNotification(ctx context.Context, data models.MonitorStartData) {
	if !nh.canSendMonitorNotification() {
		nh.logger.Info().Str("cycle_id", data.CycleID).Msg("Monitor start notification disabled or webhook not configured")
		return
	}

	nh.logger.Info().
		Str("cycle_id", data.CycleID).
		Int("total_targets", data.TotalTargets).
		Msg("Preparing to send monitor start notification")

	payload := FormatMonitorStartMessage(data, nh.cfg)
	nh.sendSimpleMonitorNotification(ctx, payload, "monitor start")

	nh.logger.Info().Str("cycle_id", data.CycleID).Msg("Monitor start notification sent successfully")
}

// SendMonitorInterruptNotification sends a notification when monitor service is interrupted.
func (nh *NotificationHelper) SendMonitorInterruptNotification(ctx context.Context, data models.MonitorInterruptData) {
	if !nh.canSendMonitorNotification() {
		nh.logger.Info().Str("cycle_id", data.CycleID).Msg("Monitor interrupt notification disabled or webhook not configured")
		return
	}

	nh.logger.Info().
		Str("cycle_id", data.CycleID).
		Str("reason", data.Reason).
		Int("processed_targets", data.ProcessedTargets).
		Msg("Preparing to send monitor interrupt notification")

	payload := FormatMonitorInterruptMessage(data, nh.cfg)
	nh.sendSimpleMonitorNotification(ctx, payload, "monitor interrupt")

	nh.logger.Info().Str("cycle_id", data.CycleID).Msg("Monitor interrupt notification sent successfully")
}

// SendMonitorErrorNotification sends a notification when monitor service encounters an error.
func (nh *NotificationHelper) SendMonitorErrorNotification(ctx context.Context, data models.MonitorErrorData) {
	if !nh.canSendMonitorNotification() {
		nh.logger.Info().Str("cycle_id", data.CycleID).Msg("Monitor error notification disabled or webhook not configured")
		return
	}

	nh.logger.Info().
		Str("cycle_id", data.CycleID).
		Str("error_type", data.ErrorType).
		Str("component", data.Component).
		Bool("recoverable", data.Recoverable).
		Msg("Preparing to send monitor error notification")

	payload := FormatMonitorErrorMessage(data, nh.cfg)
	nh.sendSimpleMonitorNotification(ctx, payload, "monitor error")

	nh.logger.Info().Str("cycle_id", data.CycleID).Msg("Monitor error notification sent successfully")
}

// adjustPayloadForAttachmentFailure modifies payload to indicate report is available but not attached
func (nh *NotificationHelper) adjustPayloadForAttachmentFailure(payload models.DiscordMessagePayload, reportPath string) {
	for embedIdx := range payload.Embeds {
		for fieldIdx, field := range payload.Embeds[embedIdx].Fields {
			if field.Name == "📄 Report" {
				payload.Embeds[embedIdx].Fields[fieldIdx].Value = fmt.Sprintf("Report generated, but not attached (check local files or configuration). Report path: %s", reportPath)
				break
			}
		}
	}
}
