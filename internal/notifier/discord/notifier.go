package discord

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/common/httpclient"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// DiscordNotifier handles sending notifications to Discord
type DiscordNotifier struct {
	logger     zerolog.Logger
	httpClient *httpclient.HTTPClient
	webhookURL string
}

// NewDiscordNotifier creates a new DiscordNotifier instance
func NewDiscordNotifier(cfg *config.NotificationConfig, logger zerolog.Logger, httpClient *httpclient.HTTPClient) (*DiscordNotifier, error) {
	return &DiscordNotifier{
		logger:     logger.With().Str("module", "DiscordNotifier").Logger(),
		httpClient: httpClient,
		webhookURL: cfg.ScanServiceDiscordWebhookURL,
	}, nil
}

// SendNotification sends a generic notification to Discord.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, webhookURL string, payload DiscordMessagePayload, attachmentPath string) error {
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
