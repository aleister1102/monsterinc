package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	httpclient "github.com/aleister1102/go-comet"
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
func (dn *DiscordNotifier) SendNotification(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload, attachmentPath string) error {
	if webhookURL == "" {
		dn.logger.Warn().Msg("Discord webhook URL is not configured, skipping notification")
		return nil
	}

	err := dn.sendDiscordNotification(ctx, webhookURL, payload, attachmentPath)
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
	err := dn.sendDiscordNotification(context.Background(), dn.webhookURL, payload, "")
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

// sendDiscordNotification sends a notification to Discord webhook
func (dn *DiscordNotifier) sendDiscordNotification(ctx context.Context, webhookURL string, payload interface{}, filePath string) error {
	if filePath == "" {
		// Send JSON payload only
		return dn.sendDiscordJSON(ctx, webhookURL, payload)
	}

	// Send multipart form-data with file attachment
	return dn.sendDiscordMultipart(ctx, webhookURL, payload, filePath)
}

// sendDiscordJSON sends JSON payload to Discord webhook
func (dn *DiscordNotifier) sendDiscordJSON(ctx context.Context, webhookURL string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return WrapError(err, "failed to marshal Discord payload")
	}

	req := &httpclient.HTTPRequest{
		URL:    webhookURL,
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:    bytes.NewReader(jsonData),
		Context: ctx,
	}

	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return WrapError(err, "failed to send Discord notification")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	dn.logger.Debug().Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully")
	return nil
}

// sendDiscordMultipart sends multipart form-data to Discord webhook with file attachment
func (dn *DiscordNotifier) sendDiscordMultipart(ctx context.Context, webhookURL string, payload interface{}, filePath string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add JSON payload as form field
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return WrapError(err, "failed to marshal Discord payload")
	}

	if err := writer.WriteField("payload_json", string(jsonData)); err != nil {
		return WrapError(err, "failed to write payload_json field")
	}

	// Add file attachment
	file, err := os.Open(filePath)
	if err != nil {
		return WrapError(err, "failed to open file for Discord attachment")
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return WrapError(err, "failed to create form file")
	}

	if _, err := io.Copy(part, file); err != nil {
		return WrapError(err, "failed to copy file content")
	}

	if err := writer.Close(); err != nil {
		return WrapError(err, "failed to close multipart writer")
	}

	req := &httpclient.HTTPRequest{
		URL:    webhookURL,
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": writer.FormDataContentType(),
		},
		Body:    &buf,
		Context: ctx,
	}

	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return WrapError(err, "failed to send Discord notification with attachment")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned status %d: %s", resp.StatusCode, string(resp.Body))
	}

	dn.logger.Debug().Int("status_code", resp.StatusCode).Msg("Discord notification with attachment sent successfully")
	return nil
}
