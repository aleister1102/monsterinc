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
