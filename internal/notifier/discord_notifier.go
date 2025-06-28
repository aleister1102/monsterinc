package notifier

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/rs/zerolog"
)

// DiscordNotifier handles sending notifications to Discord
type DiscordNotifier struct {
	logger     zerolog.Logger
	httpClient *common.HTTPClient
	webhookURL string
	fileStore  datastore.FileHistoryStore
}

// NewDiscordNotifier creates a new DiscordNotifier instance
func NewDiscordNotifier(cfg *config.NotificationConfig, logger zerolog.Logger, httpClient *common.HTTPClient) (*DiscordNotifier, error) {
	return &DiscordNotifier{
		logger:     logger.With().Str("module", "DiscordNotifier").Logger(),
		httpClient: httpClient,
		webhookURL: cfg.ScanServiceDiscordWebhookURL,
	}, nil
}

// SendNotification sends a generic notification to Discord.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload, attachmentPath string) error {
	if webhookURL == "" {
		dn.logger.Warn().Msg("Discord webhook URL is not configured, skipping notification")
		return nil
	}

	err := dn.httpClient.SendDiscordNotification(ctx, webhookURL, payload, attachmentPath)
	if err != nil {
		dn.logger.Error().Err(err).Str("webhook_url", webhookURL).Msg("Failed to send Discord notification")
		return err
	}

	dn.logger.Info().Str("webhook_url", webhookURL).Msg("Discord notification sent successfully")
	return nil
}

func (dn *DiscordNotifier) Send(ctx context.Context, notification Notification) error {
	// ... implementation
	return nil
}

func (dn *DiscordNotifier) SendFileContentChangeNotification(ctx context.Context, fileURL, newHash string, diffResult *differ.ContentDiffResult) error {
	// ... implementation
	return nil
}

func (dn *DiscordNotifier) SendURLScanCompletionNotification(ctx context.Context, summary ScanSummaryData, reportFilePaths []string) error {
	// ... implementation
	return nil
}

func (dn *DiscordNotifier) GetFileHistoryStore() datastore.FileHistoryStore {
	return dn.fileStore
}
