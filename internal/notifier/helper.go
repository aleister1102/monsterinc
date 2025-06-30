package notifier

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common/summary"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/notifier/discord"
	"github.com/rs/zerolog"
)

// NotificationHelper provides a high-level interface for sending various scan-related notifications.
type NotificationHelper struct {
	discordNotifier *discord.DiscordNotifier
	cfg             config.NotificationConfig
	logger          zerolog.Logger
}

// NewNotificationHelper creates a new NotificationHelper.
func NewNotificationHelper(dn *discord.DiscordNotifier, cfg config.NotificationConfig, logger zerolog.Logger) *NotificationHelper {
	nh := &NotificationHelper{
		discordNotifier: dn,
		cfg:             cfg,
		logger:          logger.With().Str("module", "NotificationHelper").Logger(),
	}
	return nh
}

// getWebhookURL selects the appropriate webhook URL based on the service type.
func (nh *NotificationHelper) getWebhookURL() string {
	return nh.cfg.ScanServiceDiscordWebhookURL
}

// SendScanStartNotification sends a notification when a scan starts.
func (nh *NotificationHelper) SendScanStartNotification(ctx context.Context, summary summary.ScanSummaryData) {
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
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summaryData summary.ScanSummaryData, reportFilePaths []string) {
	if !nh.shouldSendScanCompletionNotification(summaryData) {
		return
	}

	webhookURL := nh.getWebhookURL()
	if webhookURL == "" {
		nh.logger.Warn().Msg("Webhook URL is not configured for this service type. Skipping scan completion notification.")
		return
	}

	// Always send only summary notification (no individual report parts)
	nh.sendSummaryOnlyReport(ctx, summaryData, webhookURL, reportFilePaths)
}

// shouldSendScanCompletionNotification checks if notification should be sent based on config and scan status
func (nh *NotificationHelper) shouldSendScanCompletionNotification(summaryData summary.ScanSummaryData) bool {
	if !nh.cfg.NotifyOnSuccess && summaryData.Status == string(summary.ScanStatusCompleted) {
		nh.logger.Info().Str("status", summaryData.Status).Msg("Scan success notification is disabled, skipping.")
		return false
	}
	if !nh.cfg.NotifyOnFailure && (summaryData.Status == string(summary.ScanStatusFailed) || summaryData.Status == string(summary.ScanStatusPartialComplete)) {
		nh.logger.Info().Str("status", summaryData.Status).Msg("Scan failure notification is disabled, skipping.")
		return false
	}
	return true
}

// sendSummaryOnlyReport sends a single notification with all report files attached
func (nh *NotificationHelper) sendSummaryOnlyReport(ctx context.Context, summary summary.ScanSummaryData, webhookURL string, reportFilePaths []string) {
	if len(reportFilePaths) > 0 {
		// Send single notification with all reports
		nh.sendSingleNotificationWithAllReports(ctx, summary, webhookURL, reportFilePaths)
	} else {
		// No reports to attach
		nh.sendSingleReport(ctx, summary, webhookURL)
	}
}

// sendSingleNotificationWithAllReports sends one notification with all report files attached
func (nh *NotificationHelper) sendSingleNotificationWithAllReports(ctx context.Context, summary summary.ScanSummaryData, webhookURL string, reportFilePaths []string) {
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

	// Track successfully sent report files for cleanup
	var sentReportFiles []string
	sentReportFiles = append(sentReportFiles, reportFilePaths[0]) // First report sent successfully

	// Send additional reports as follow-up messages if there are more than 1
	for i := 1; i < len(reportFilePaths); i++ {
		err := nh.sendAdditionalReport(ctx, summary, webhookURL, reportFilePaths[i], i+1, len(reportFilePaths))
		if err == nil {
			// Only add to cleanup list if sent successfully
			sentReportFiles = append(sentReportFiles, reportFilePaths[i])
		}

		// Small delay between sends
		if i < len(reportFilePaths)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Cleanup all sent report files after successful notification
	nh.cleanupReportFiles(sentReportFiles)
}

// sendAdditionalReport sends additional report files as simple attachments
func (nh *NotificationHelper) sendAdditionalReport(ctx context.Context, summary summary.ScanSummaryData, webhookURL string, reportPath string, partNum, totalParts int) error {
	payload := nh.buildSimpleReportPayload(summary.ScanSessionID, partNum, totalParts)

	nh.logger.Info().
		Str("session_id", summary.ScanSessionID).
		Int("part", partNum).
		Int("total_parts", totalParts).
		Msg("Sending additional report file.")

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, reportPath)
	if err != nil {
		nh.logger.Error().Err(err).Int("part", partNum).Msg("Failed to send additional report")
		return err
	}
	return nil
}

// cleanupReportFiles removes report files after successful notification
func (nh *NotificationHelper) cleanupReportFiles(reportFilePaths []string) {
	for _, filePath := range reportFilePaths {
		if filePath == "" {
			continue
		}

		err := os.Remove(filePath)
		if err != nil {
			nh.logger.Error().Err(err).Str("file_path", filePath).Msg("Failed to cleanup report file")
		} else {
			nh.logger.Info().Str("file_path", filePath).Msg("Report file cleaned up successfully")
		}
	}
}

// buildSimpleReportPayload creates a minimal payload for additional reports
func (nh *NotificationHelper) buildSimpleReportPayload(sessionID string, partNum, totalParts int) discord.DiscordMessagePayload {
	embed := discord.NewDiscordEmbedBuilder().
		WithTitle(fmt.Sprintf("📎 Report %d/%d", partNum, totalParts)).
		WithDescription(fmt.Sprintf("**Session:** `%s`", sessionID)).
		WithColor(DefaultEmbedColor).
		WithTimestamp(time.Now()).
		Build()

	return discord.NewDiscordMessagePayloadBuilder().
		WithUsername(DiscordUsername).
		WithAvatarURL(DiscordAvatarURL).
		AddEmbed(embed).
		Build()
}

// adjustPayloadForMultipleReports modifies payload to indicate multiple reports are being sent
func (nh *NotificationHelper) adjustPayloadForMultipleReports(payload discord.DiscordMessagePayload, reportCount int) {
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
			payload.Embeds[embedIdx].Fields = append(payload.Embeds[embedIdx].Fields, discord.DiscordEmbedField{
				Name:   "📄 Report",
				Value:  fmt.Sprintf("%d reports will be sent (main report attached, additional files follow).", reportCount),
				Inline: false,
			})
		}
	}
}

// sendSingleReport sends a single report without attachments
func (nh *NotificationHelper) sendSingleReport(ctx context.Context, summary summary.ScanSummaryData, webhookURL string) {
	payload := FormatScanCompleteMessageWithReports(summary, nh.cfg, false)
	nh.adjustPayloadForNoAttachments(payload, summary)

	nh.logger.Info().Str("status", summary.Status).Str("session_id", summary.ScanSessionID).Msg("Attempting to send scan completion notification (no report attachments).")

	err := nh.discordNotifier.SendNotification(ctx, webhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send scan completion notification")
	}
}

// adjustPayloadForNoAttachments modifies payload when no attachments are present
func (nh *NotificationHelper) adjustPayloadForNoAttachments(payload discord.DiscordMessagePayload, summary summary.ScanSummaryData) {
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

// SendScanInterruptNotification sends a notification when a scan is interrupted.
func (nh *NotificationHelper) SendScanInterruptNotification(ctx context.Context, summary summary.ScanSummaryData) {
	if !nh.canSendScanFailureNotification() {
		return
	}

	nh.logger.Info().Str("session_id", summary.ScanSessionID).Str("component", summary.Component).Msg("Preparing to send scan interrupt notification.")

	payload := FormatInterruptNotificationMessage(summary, nh.cfg)
	nh.sendSimpleScanNotification(ctx, payload, "scan interrupt")
}

// canSendScanFailureNotification checks if scan failure notifications can be sent
func (nh *NotificationHelper) canSendScanFailureNotification() bool {
	return nh.cfg.NotifyOnFailure && nh.discordNotifier != nil && nh.cfg.ScanServiceDiscordWebhookURL != ""
}

// sendSimpleScanNotification sends a scan notification without file attachment
func (nh *NotificationHelper) sendSimpleScanNotification(ctx context.Context, payload discord.DiscordMessagePayload, notificationType string) {
	err := nh.discordNotifier.SendNotification(ctx, nh.cfg.ScanServiceDiscordWebhookURL, payload, "")
	if err != nil {
		nh.logger.Error().Err(err).Msgf("Failed to send %s notification", notificationType)
	}
}
