package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	// "github.com/aleister1102/monsterinc/internal/config" // No longer needed directly by DiscordNotifier for WebhookURL
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

const (
	defaultRetryAttempts = 2
	defaultRetryDelay    = 5 * time.Second
	maxContentLength     = 8 * 1024 * 1024 // Discord's file size limit (8MB)
	maxDiscordFileSize   = 8 * 1024 * 1024 // 8MB, Discord's typical limit without Nitro
)

// DiscordNotifier handles sending notifications to a Discord webhook.
type DiscordNotifier struct {
	// cfg        config.NotificationConfig // Config is now managed by NotificationHelper
	logger     zerolog.Logger
	httpClient *http.Client
	// disabled   bool // Logic for disabling will be based on provided webhookURL in SendNotification
}

// NewDiscordNotifier creates a new DiscordNotifier.
// It no longer takes NotificationConfig as it doesn't store the webhook URL directly.
func NewDiscordNotifier(logger zerolog.Logger, httpClient *http.Client) (*DiscordNotifier, error) {
	moduleLogger := logger.With().Str("module", "DiscordNotifier").Logger()

	if httpClient == nil {
		moduleLogger.Warn().Msg("HTTP client is nil, using default HTTP client with 20s timeout.")
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	moduleLogger.Info().Msg("DiscordNotifier initialized (webhook URL will be provided per send call).")
	return &DiscordNotifier{
		logger:     moduleLogger,
		httpClient: httpClient,
	}, nil
}

// SendNotification sends a message payload and an optional file to the specified Discord webhook URL.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload, reportFilePath string) error {
	if webhookURL == "" {
		dn.logger.Info().Msg("Webhook URL is empty. Skipping Discord notification.")
		return nil
	}

	// Validate webhookURL on each call (could be cached if performance becomes an issue)
	_, errURL := url.ParseRequestURI(webhookURL)
	if errURL != nil {
		dn.logger.Error().Err(errURL).Str("url", webhookURL).Msg("Invalid DiscordWebhookURL provided for this notification.")
		return fmt.Errorf("invalid DiscordWebhookURL for send: %w", errURL)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	var req *http.Request
	var err error

	// Add JSON payload
	payloadJSON, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		dn.logger.Error().Err(jsonErr).Msg("Failed to marshal Discord payload to JSON")
		return fmt.Errorf("failed to marshal discord payload: %w", jsonErr)
	}

	if err = writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to write payload_json field to multipart writer")
		return fmt.Errorf("failed to write payload_json to multipart: %w", err)
	}

	// Add file if reportFilePath is provided
	if reportFilePath != "" {
		fileData, readErr := os.ReadFile(reportFilePath)
		if readErr != nil {
			dn.logger.Error().Err(readErr).Str("file_path", reportFilePath).Msg("Failed to read report file for attachment")
			return fmt.Errorf("failed to read report file '%s': %w", reportFilePath, readErr)
		}

		part, partErr := writer.CreateFormFile("file[0]", filepath.Base(reportFilePath))
		if partErr != nil {
			dn.logger.Error().Err(partErr).Msg("Failed to create form file for report attachment")
			return fmt.Errorf("failed to create form file: %w", partErr)
		}
		_, copyErr := io.Copy(part, bytes.NewReader(fileData))
		if copyErr != nil {
			dn.logger.Error().Err(copyErr).Msg("Failed to copy report file data to multipart form")
			return fmt.Errorf("failed to copy file data to form: %w", copyErr)
		}
	}

	if err = writer.Close(); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to close multipart writer")
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err = http.NewRequestWithContext(ctx, "POST", webhookURL, body) // Use the passed webhookURL
	if err != nil {
		dn.logger.Error().Err(err).Msg("Failed to create new HTTP request for Discord with attachment")
		return fmt.Errorf("failed to create discord request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// HTTP Client Execution (common for both cases)
	resp, err := dn.httpClient.Do(req)
	if err != nil {
		dn.logger.Error().Err(err).Str("webhook_url", webhookURL).Msg("Failed to send Discord notification with attachment")
		return fmt.Errorf("failed to send discord notification to %s: %w", webhookURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		dn.logger.Error().Int("status_code", resp.StatusCode).Str("response_body", string(respBody)).Str("webhook_url", webhookURL).Msg("Discord notification failed")
		return fmt.Errorf("discord notification to %s failed with status %d: %s", webhookURL, resp.StatusCode, string(respBody))
	}

	dn.logger.Info().Int("status_code", resp.StatusCode).Str("webhook_url", webhookURL).Msg("Discord notification sent successfully.")
	return nil
}

// ShouldNotify function is removed as this logic now resides in NotificationHelper directly,
// which has access to the full NotificationConfig and can select the appropriate webhook URL.
