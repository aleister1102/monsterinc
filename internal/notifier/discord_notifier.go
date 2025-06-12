package notifier

import (
	"context"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// DiscordNotifier handles sending notifications to Discord
type DiscordNotifier struct {
	logger     zerolog.Logger
	httpClient *common.HTTPClient
	webhookURL string
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

// SendSecretNotification sends a notification about a secret finding to Discord.
func (dn *DiscordNotifier) SendSecretNotification(finding models.SecretFinding) error {
	if dn.webhookURL == "" {
		dn.logger.Warn().Msg("Discord webhook URL for scan service is not configured, skipping notification")
		return nil
	}

	payload := dn.formatSecretMessage(finding)

	dn.logger.Info().Str("webhook_url", dn.webhookURL).Msg("Sending secret finding notification to Discord")

	// Context can be background for now as it's a fire-and-forget notification
	err := dn.httpClient.SendDiscordNotification(context.Background(), dn.webhookURL, payload, "")
	if err != nil {
		dn.logger.Error().Err(err).Str("webhook_url", dn.webhookURL).Msg("Failed to send Discord notification")
		return err
	}

	dn.logger.Info().Str("webhook_url", dn.webhookURL).Msg("Discord notification sent successfully")
	return nil
}

func (dn *DiscordNotifier) formatSecretMessage(finding models.SecretFinding) models.DiscordMessagePayload {
	return models.DiscordMessagePayload{
		Content: "🚨 Secret Detected!",
		Embeds: []models.DiscordEmbed{
			{
				Title:       "Secret Found: " + finding.RuleID,
				Description: "A secret was detected by the scanner.",
				Color:       15158332, // Red
				Fields: []models.DiscordEmbedField{
					{Name: "Rule", Value: finding.RuleID, Inline: true},
					{Name: "File/URL", Value: finding.SourceURL, Inline: true},
					{Name: "Secret", Value: "```" + finding.SecretText + "```"},
					// Severity is not available in the finding model, so it's omitted.
				},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
}
