package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

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
		// Use common HTTP client factory for creating default client
		factory := common.NewHTTPClientFactory(moduleLogger)
		var err error
		httpClient, err = factory.CreateDiscordClient(20 * time.Second)
		if err != nil {
			return nil, common.WrapError(err, "failed to create default Discord HTTP client")
		}
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
	errURL := urlhandler.ValidateURLFormat(webhookURL)
	if errURL != nil {
		dn.logger.Error().Err(errURL).Str("url", webhookURL).Msg("Invalid DiscordWebhookURL provided for this notification.")
		return common.WrapError(errURL, "invalid DiscordWebhookURL for send")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	var req *http.Request
	var err error

	// Add JSON payload
	payloadJSON, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		dn.logger.Error().Err(jsonErr).Msg("Failed to marshal Discord payload to JSON")
		return common.WrapError(jsonErr, "failed to marshal discord payload")
	}

	if err = writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to write payload_json field to multipart writer")
		return common.WrapError(err, "failed to write payload_json to multipart")
	}

	// Add file if reportFilePath is provided
	if reportFilePath != "" {
		// Use common file utilities for reading file
		fileManager := common.NewFileManager(dn.logger)
		fileData, readErr := fileManager.ReadFile(reportFilePath, common.DefaultFileReadOptions())
		if readErr != nil {
			dn.logger.Error().Err(readErr).Str("file_path", reportFilePath).Msg("Failed to read report file for attachment")
			return common.WrapError(readErr, fmt.Sprintf("failed to read report file '%s'", reportFilePath))
		}

		part, partErr := writer.CreateFormFile("file[0]", filepath.Base(reportFilePath))
		if partErr != nil {
			dn.logger.Error().Err(partErr).Msg("Failed to create form file for report attachment")
			return common.WrapError(partErr, "failed to create form file")
		}
		_, copyErr := io.Copy(part, bytes.NewReader(fileData))
		if copyErr != nil {
			dn.logger.Error().Err(copyErr).Msg("Failed to copy report file data to multipart form")
			return common.WrapError(copyErr, "failed to copy file data to form")
		}
	}

	if err = writer.Close(); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to close multipart writer")
		return common.WrapError(err, "failed to close multipart writer")
	}

	req, err = http.NewRequestWithContext(ctx, "POST", webhookURL, body) // Use the passed webhookURL
	if err != nil {
		dn.logger.Error().Err(err).Msg("Failed to create new HTTP request for Discord with attachment")
		return common.WrapError(err, "failed to create discord request")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// HTTP Client Execution (common for both cases)
	resp, err := dn.httpClient.Do(req)
	if err != nil {
		dn.logger.Error().Err(err).Str("webhook_url", webhookURL).Msg("Failed to send Discord notification with attachment")
		return common.WrapError(err, fmt.Sprintf("failed to send discord notification to %s", webhookURL))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		dn.logger.Error().Int("status_code", resp.StatusCode).Str("response_body", string(respBody)).Str("webhook_url", webhookURL).Msg("Discord notification failed")
		return common.NewHTTPError(resp.StatusCode, fmt.Sprintf("discord notification to %s failed with status %d: %s", webhookURL, resp.StatusCode, string(respBody)))
	}

	dn.logger.Info().Int("status_code", resp.StatusCode).Str("webhook_url", webhookURL).Msg("Discord notification sent successfully.")
	return nil
}

// ShouldNotify function is removed as this logic now resides in NotificationHelper directly,
// which has access to the full NotificationConfig and can select the appropriate webhook URL.
