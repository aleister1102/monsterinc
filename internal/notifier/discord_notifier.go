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

	"monsterinc/internal/config"
	"monsterinc/internal/models"

	"github.com/rs/zerolog"
)

const (
	defaultRetryAttempts = 2
	defaultRetryDelay    = 5 * time.Second
	maxContentLength     = 8 * 1024 * 1024 // Discord's file size limit (8MB)
)

// DiscordNotifier handles sending notifications to a Discord webhook.
type DiscordNotifier struct {
	cfg        config.NotificationConfig
	logger     zerolog.Logger
	httpClient *http.Client
}

// NewDiscordNotifier creates a new DiscordNotifier.
func NewDiscordNotifier(cfg config.NotificationConfig, logger zerolog.Logger, httpClient *http.Client) (*DiscordNotifier, error) {
	if cfg.DiscordWebhookURL == "" {
		logger.Info().Msg("DiscordWebhookURL is not configured. DiscordNotifier will be disabled.")
		// Return nil, nil to indicate it's disabled but not an error for program continuation
		return nil, nil
	}

	// Validate Webhook URL
	_, err := url.ParseRequestURI(cfg.DiscordWebhookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DiscordWebhookURL: %w", err)
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &DiscordNotifier{
		cfg:        cfg,
		logger:     logger.With().Str("component", "DiscordNotifier").Logger(),
		httpClient: httpClient,
	}, nil
}

// SendNotification sends a message payload and an optional file to the configured Discord webhook.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, payload models.DiscordMessagePayload, reportFilePath string) error {
	if dn == nil || dn.cfg.DiscordWebhookURL == "" {
		// Notifier is disabled or not configured
		return nil
	}

	// Prepare the multipart writer and the JSON payload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add JSON payload part
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		dn.logger.Error().Err(err).Msg("Failed to marshal Discord payload")
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}
	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to write payload_json to multipart writer")
		return fmt.Errorf("failed to write payload_json to multipart: %w", err)
	}

	// Add file part if reportFilePath is provided
	if reportFilePath != "" {
		file, err := os.Open(reportFilePath)
		if err != nil {
			dn.logger.Error().Err(err).Str("file_path", reportFilePath).Msg("Failed to open report file for Discord attachment")
			// Continue without attachment if file opening fails, but log it
		} else {
			defer file.Close()

			// Check file size
			fileInfo, err := file.Stat()
			if err != nil {
				dn.logger.Error().Err(err).Str("file_path", reportFilePath).Msg("Failed to get file stats for report file")
			} else if fileInfo.Size() > maxContentLength {
				dn.logger.Warn().
					Str("file_path", reportFilePath).
					Int64("file_size_bytes", fileInfo.Size()).
					Int("max_size_bytes", maxContentLength).
					Msg("Report file exceeds Discord's size limit, not attaching.")
				// Also, inform in the message that the file was too large.
				// This part can be improved by the formatter adding a note if reportPath was initially present but not attached.
			} else {
				part, err := writer.CreateFormFile("file", filepath.Base(reportFilePath))
				if err != nil {
					dn.logger.Error().Err(err).Msg("Failed to create form file for Discord attachment")
					return fmt.Errorf("failed to create form file: %w", err)
				}
				if _, err = io.Copy(part, file); err != nil {
					dn.logger.Error().Err(err).Msg("Failed to copy report file content to multipart writer")
					return fmt.Errorf("failed to copy file content: %w", err)
				}
				dn.logger.Info().Str("file_path", reportFilePath).Msg("Report file prepared for Discord attachment")
			}
		}
	}

	if err := writer.Close(); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to close multipart writer")
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	var resp *http.Response
	var reqErr error

	for i := 0; i < defaultRetryAttempts+1; i++ {
		select {
		case <-ctx.Done():
			dn.logger.Warn().Msg("Context cancelled, aborting Discord notification")
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, dn.cfg.DiscordWebhookURL, bytes.NewReader(body.Bytes()))
		if err != nil {
			dn.logger.Error().Err(err).Msg("Failed to create Discord webhook request")
			return fmt.Errorf("failed to create request: %w", err) // Non-retryable
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		dn.logger.Debug().Str("url", dn.cfg.DiscordWebhookURL).Msg("Sending notification to Discord")
		resp, reqErr = dn.httpClient.Do(req)

		if reqErr != nil {
			dn.logger.Error().Err(reqErr).Int("attempt", i+1).Msg("Failed to send Discord notification")
			if i < defaultRetryAttempts {
				time.Sleep(defaultRetryDelay)
				continue
			}
			return fmt.Errorf("failed to send discord notification after %d attempts: %w", defaultRetryAttempts+1, reqErr)
		}

		// Check response status
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			dn.logger.Info().Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully")
			if resp.Body != nil {
				resp.Body.Close()
			}
			return nil // Success
		}

		// Handle specific Discord error codes or rate limits if needed
		// For now, just log the error and retry for server-side errors.
		responseBody, _ := io.ReadAll(resp.Body)
		if resp.Body != nil {
			resp.Body.Close()
		}
		dn.logger.Error().
			Int("status_code", resp.StatusCode).
			Str("response_body", string(responseBody)).
			Int("attempt", i+1).
			Msg("Discord API returned an error")

		// Don't retry on 4xx client errors (except 429 Too Many Requests, though Discord might handle this with Retry-After header)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return fmt.Errorf("discord API error: status %d, body: %s", resp.StatusCode, string(responseBody))
		}

		if i < defaultRetryAttempts {
			// Check for Retry-After header (Discord specific for rate limits)
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if sleepDuration, err := time.ParseDuration(retryAfter + "s"); err == nil { // Discord sends seconds
					dn.logger.Info().Str("duration", sleepDuration.String()).Msg("Respecting Discord Retry-After header")
					time.Sleep(sleepDuration)
					continue
				}
			}
			time.Sleep(defaultRetryDelay)
		}
	}
	// If loop finishes, it means all retries failed
	return fmt.Errorf("failed to send discord notification after %d attempts, last status: %d", defaultRetryAttempts+1, resp.StatusCode)
}

// Helper to determine if a notification should be sent based on config and status
func (dn *DiscordNotifier) ShouldNotify(status models.ScanStatus) bool {
	if dn == nil || dn.cfg.DiscordWebhookURL == "" {
		return false
	}
	switch status {
	case models.ScanStatusStarted:
		return dn.cfg.NotifyOnScanStart
	case models.ScanStatusCompleted:
		return dn.cfg.NotifyOnSuccess
	case models.ScanStatusFailed:
		return dn.cfg.NotifyOnFailure
	case models.ScanStatusCriticalError: // This is a new status for clarity
		return dn.cfg.NotifyOnCriticalError
	}
	return false
}
