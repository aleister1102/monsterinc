package notifier

import (
	"context"
	"fmt"
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
func (nh *NotificationHelper) SendScanStartNotification(ctx context.Context, scanID string, targets []string, totalTargets int) {
	if nh.discordNotifier == nil || !nh.cfg.NotifyOnScanStart {
		nh.logger.Debug().Msg("DiscordNotifier not configured or NotifyOnScanStart is false, skipping start notification.")
		return
	}

	nh.logger.Info().Str("scan_id", scanID).Int("total_targets", totalTargets).Msg("Preparing to send scan start notification.")
	summary := models.ScanSummaryData{
		ScanID:       scanID,
		Targets:      targets,
		TotalTargets: totalTargets,
		Status:       string(models.ScanStatusStarted),
	}

	payload := FormatScanStartMessage(summary, nh.cfg)
	go func() {
		if err := nh.discordNotifier.SendNotification(context.Background(), payload, ""); err != nil {
			nh.logger.Error().Err(err).Str("scan_id", scanID).Msg("Failed to send scan start notification to Discord.")
		} else {
			nh.logger.Info().Str("scan_id", scanID).Msg("Scan start notification sent successfully.")
		}
	}()
}

// SendScanCompletionNotification sends a notification when a scan completes (successfully or with failures).
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData) {
	if nh.discordNotifier == nil {
		nh.logger.Debug().Msg("DiscordNotifier is nil, cannot send completion notification.")
		return
	}

	shouldNotify := false
	currentStatus := models.ScanStatus(summary.Status)
	switch currentStatus {
	case models.ScanStatusCompleted:
		shouldNotify = nh.cfg.NotifyOnSuccess
	case models.ScanStatusFailed, models.ScanStatusPartialComplete:
		shouldNotify = nh.cfg.NotifyOnFailure
	default:
		nh.logger.Warn().Str("scan_id", summary.ScanID).Str("status", summary.Status).Msg("Unknown or unhandled scan status for completion notification logic. Will not send notification.")
		return
	}

	if !shouldNotify {
		nh.logger.Info().Str("scan_id", summary.ScanID).Str("status", summary.Status).Msg("Notification for scan completion is disabled by config for this status.")
		return
	}

	nh.logger.Info().Str("scan_id", summary.ScanID).Str("status", summary.Status).Msg("Preparing to send scan completion notification.")
	payload := FormatScanCompleteMessage(summary, nh.cfg)
	reportPath := summary.ReportPath

	if err := nh.discordNotifier.SendNotification(ctx, payload, reportPath); err != nil {
		nh.logger.Error().Err(err).Str("scan_id", summary.ScanID).Msg("Failed to send scan completion notification to Discord.")
	} else {
		nh.logger.Info().Str("scan_id", summary.ScanID).Msg("Scan completion notification sent successfully.")
	}
}

// SendCriticalErrorNotification sends a notification for a critical application error.
func (nh *NotificationHelper) SendCriticalErrorNotification(ctx context.Context, componentName string, errorMessages []string) {
	if nh.discordNotifier == nil || !nh.cfg.NotifyOnCriticalError {
		nh.logger.Debug().Msg("DiscordNotifier not configured or NotifyOnCriticalError is false, skipping critical error notification.")
		return
	}

	nh.logger.Info().Str("component", componentName).Interface("errors", errorMessages).Msg("Preparing to send critical error notification.")
	summary := models.ScanSummaryData{
		Status:        string(models.ScanStatusCriticalError),
		Component:     componentName,
		ErrorMessages: errorMessages,
		ScanID:        fmt.Sprintf("CriticalError-%s-%d", componentName, time.Now().Unix()), // Add timestamp for uniqueness
		ScanDuration:  0,
	}

	payload := FormatCriticalErrorMessage(summary, nh.cfg)
	// Use a cancellableCtx with a timeout to ensure this notification attempts to send even if main ctx is ending.
	cancellableCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := nh.discordNotifier.SendNotification(cancellableCtx, payload, ""); err != nil {
		nh.logger.Error().Err(err).Str("component", componentName).Msg("Failed to send critical error notification to Discord.")
	} else {
		nh.logger.Info().Str("component", componentName).Msg("Critical error notification sent successfully.")
	}
}
