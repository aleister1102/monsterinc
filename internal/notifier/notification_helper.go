package notifier

import (
	"context"
	"fmt"
	"time"

	"monsterinc/internal/config"
	"monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// NotificationHelper provides a high-level interface for sending different types of scan notifications.
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
		logger:          logger.With().Str("component", "NotificationHelper").Logger(),
	}
}

// SendScanStartNotification sends a notification when a scan starts.
func (nh *NotificationHelper) SendScanStartNotification(ctx context.Context, scanID string, targets []string, totalTargets int) {
	if nh.discordNotifier == nil || !nh.cfg.NotifyOnScanStart {
		return
	}

	summary := models.ScanSummaryData{
		ScanID:       scanID,
		Targets:      targets,
		TotalTargets: totalTargets,
		Status:       string(models.ScanStatusStarted),
	}

	payload := FormatScanStartMessage(summary, nh.cfg)
	go func() {
		if err := nh.discordNotifier.SendNotification(context.Background(), payload, ""); err != nil {
			nh.logger.Error().Err(err).Msg("Failed to send scan start notification")
		}
	}()
}

// SendScanCompletionNotification sends a notification when a scan completes (successfully or with failures).
func (nh *NotificationHelper) SendScanCompletionNotification(ctx context.Context, summary models.ScanSummaryData) {
	if nh.discordNotifier == nil {
		return
	}

	shouldNotify := false
	switch models.ScanStatus(summary.Status) {
	case models.ScanStatusCompleted:
		shouldNotify = nh.cfg.NotifyOnSuccess
	case models.ScanStatusFailed, models.ScanStatusPartialComplete:
		shouldNotify = nh.cfg.NotifyOnFailure
	default:
		nh.logger.Warn().Str("status", summary.Status).Msg("Unknown scan status for completion notification")
		return // Don't notify for unknown or irrelevant statuses here
	}

	if !shouldNotify {
		return
	}

	payload := FormatScanCompleteMessage(summary, nh.cfg)
	reportPath := summary.ReportPath // This path is used by discordNotifier to attach the file

	go func() {
		if err := nh.discordNotifier.SendNotification(context.Background(), payload, reportPath); err != nil {
			nh.logger.Error().Err(err).Msg("Failed to send scan completion notification")
		}
	}()
}

// SendCriticalErrorNotification sends a notification for a critical application error.
func (nh *NotificationHelper) SendCriticalErrorNotification(ctx context.Context, componentName string, errorMessages []string) {
	if nh.discordNotifier == nil || !nh.cfg.NotifyOnCriticalError {
		return
	}

	summary := models.ScanSummaryData{
		Status:        string(models.ScanStatusCriticalError),
		Component:     componentName,
		ErrorMessages: errorMessages,
		ScanID:        fmt.Sprintf("CriticalError-%s", componentName),
		ScanDuration:  0, // Not applicable or can be set if relevant
	}

	payload := FormatCriticalErrorMessage(summary, nh.cfg)
	// No file to attach for critical errors usually
	// Use a blocking call or a new context if the main context (ctx) might be cancelled before sending.
	// For critical errors, it might be better to ensure it attempts to send.
	cancellableCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Give it a timeout
	defer cancel()

	if err := nh.discordNotifier.SendNotification(cancellableCtx, payload, ""); err != nil {
		nh.logger.Error().Err(err).Msg("Failed to send critical error notification")
	}
}
