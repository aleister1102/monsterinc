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

	if len(reportFilePaths) > 0 {
		nh.sendMultiPartReports(ctx, summary, webhookURL, reportFilePaths)
	} else {
		nh.sendSingleReport(ctx, summary, webhookURL)
	}
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

// sendMultiPartReports sends multiple report parts sequentially
func (nh *NotificationHelper) sendMultiPartReports(ctx context.Context, summary models.ScanSummaryData, webhookURL string, reportFilePaths []string) {
	for i, reportPath := range reportFilePaths {
		payload := nh.buildReportPayload(summary, i, len(reportFilePaths))
		nh.sendReportPart(ctx, summary, webhookURL, reportPath, payload, i+1, len(reportFilePaths))

		if i < len(reportFilePaths)-1 {
			time.Sleep(1 * time.Second)
		}
	}
}

// buildReportPayload creates the appropriate payload for a report part
func (nh *NotificationHelper) buildReportPayload(summary models.ScanSummaryData, partIndex, totalParts int) models.DiscordMessagePayload {
	isPrimaryMessage := (partIndex == 0)

	if isPrimaryMessage {
		payload := FormatScanCompleteMessage(summary, nh.cfg)
		if totalParts > 1 {
			nh.adjustPayloadForMultiPart(payload, partIndex+1, totalParts)
		}
		return payload
	}

	return FormatSecondaryReportPartMessage(summary.ScanSessionID, partIndex+1, totalParts, &nh.cfg)
}

// adjustPayloadForMultiPart modifies the payload to indicate it's part of a multi-part report
func (nh *NotificationHelper) adjustPayloadForMultiPart(payload models.DiscordMessagePayload, partNum, totalParts int) {
	for embedIdx := range payload.Embeds {
		payload.Embeds[embedIdx].Description += fmt.Sprintf("\nReport is split into %d parts. This is part %d.", totalParts, partNum)

		foundReportField := false
		for fieldIdx, field := range payload.Embeds[embedIdx].Fields {
			if field.Name == "📄 Report" {
				payload.Embeds[embedIdx].Fields[fieldIdx].Value = fmt.Sprintf("Report part %d of %d is attached.", partNum, totalParts)
				foundReportField = true
				break
			}
		}

		if !foundReportField && payload.Embeds[embedIdx].Title != "" {
			payload.Embeds[embedIdx].Fields = append(payload.Embeds[embedIdx].Fields, models.DiscordEmbedField{
				Name:   "📄 Report",
				Value:  fmt.Sprintf("Report part %d of %d is attached.", partNum, totalParts),
				Inline: false,
			})
		}
	}
}

// sendReportPart sends a single report part and handles cleanup
func (nh *NotificationHelper) sendReportPart(ctx context.Context, summary models.ScanSummaryData, webhookURL, reportPath string, payload models.DiscordMessagePayload, partNum, totalParts int) {
	nh.logger.Info().Str("status", summary.Status).Str("session_id", summary.ScanSessionID).Int("part", partNum).Int("total_parts", totalParts).Msgf("Attempting to send scan completion notification (part %d/%d).", partNum, totalParts)

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportPath)
	if err != nil {
		nh.logger.Error().Err(err).Int("part", partNum).Msgf("Failed to send scan completion notification (part %d/%d)", partNum, totalParts)
	} else {
		nh.logger.Info().Int("part", partNum).Msgf("Scan completion notification (part %d/%d) sent successfully.", partNum, totalParts)
	}
}

// sendSingleReport sends a single report without attachments
func (nh *NotificationHelper) sendSingleReport(ctx context.Context, summary models.ScanSummaryData, webhookURL string) {
	payload := FormatScanCompleteMessage(summary, nh.cfg)
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
		return
	}

	nh.logger.Info().Str("cycle_id", data.CycleID).Int("total_monitored", data.TotalMonitored).Int("changed_count", len(data.ChangedURLs)).Msg("Preparing to send monitor cycle complete notification.")

	payload := FormatMonitorCycleCompleteMessage(data, nh.cfg)

	// Send notification with report attachment but DON'T cleanup the aggregated report
	if data.ReportPath == "" {
		nh.sendSimpleMonitorNotification(ctx, payload, "monitor cycle complete")
	} else {
		// Send notification with attachment but don't auto-delete the aggregated report
		webhookURL := nh.getWebhookURL(MonitorServiceNotification)
		if err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, data.ReportPath); err != nil {
			nh.logger.Error().Err(err).Str("webhook_url", webhookURL).Str("notification_type", "monitor cycle complete").Msg("Failed to send monitor notification with attachment")
		}
	}

	// Clean up ONLY partial diff reports if cleanup is enabled
	if nh.cfg.AutoDeletePartialDiffReports && nh.diffReportCleaner != nil {
		if err := nh.diffReportCleaner.DeleteAllSingleDiffReports(); err != nil {
			nh.logger.Error().Err(err).Msg("Failed to cleanup partial diff reports")
		}
	}
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
