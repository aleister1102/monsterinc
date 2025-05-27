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
	"strconv"
	"time"

	"monsterinc/internal/config"
	"monsterinc/internal/models"

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
	cfg        config.NotificationConfig
	logger     zerolog.Logger
	httpClient *http.Client
	disabled   bool // Added to indicate if notifier is operational
}

// IsDisabled checks if the notifier is explicitly disabled or if the webhook URL is not set.
func (dn *DiscordNotifier) IsDisabled() bool {
	return dn.disabled || dn.cfg.DiscordWebhookURL == ""
}

// NewDiscordNotifier creates a new DiscordNotifier.
func NewDiscordNotifier(cfg config.NotificationConfig, logger zerolog.Logger, httpClient *http.Client) (*DiscordNotifier, error) {
	moduleLogger := logger.With().Str("module", "DiscordNotifier").Logger()
	if cfg.DiscordWebhookURL == "" {
		moduleLogger.Info().Msg("DiscordWebhookURL is not configured. Discord notifications will be disabled.")
		return &DiscordNotifier{cfg: cfg, logger: moduleLogger, httpClient: httpClient, disabled: true}, nil
	}

	_, errURL := url.ParseRequestURI(cfg.DiscordWebhookURL)
	if errURL != nil {
		moduleLogger.Error().Err(errURL).Str("url", cfg.DiscordWebhookURL).Msg("Invalid DiscordWebhookURL provided.")
		return nil, fmt.Errorf("invalid DiscordWebhookURL: %w", errURL)
	}

	if httpClient == nil {
		moduleLogger.Warn().Msg("HTTP client is nil, using default HTTP client with 20s timeout.")
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}

	moduleLogger.Info().Msg("DiscordNotifier initialized successfully.")
	return &DiscordNotifier{
		cfg:        cfg,
		logger:     moduleLogger,
		httpClient: httpClient,
		disabled:   false,
	}, nil
}

// SendNotification sends a message payload and an optional file to the configured Discord webhook.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, payload models.DiscordMessagePayload, reportFilePath string) error {
	if dn.disabled {
		dn.logger.Debug().Msg("DiscordNotifier is disabled, skipping notification send.")
		return nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		dn.logger.Error().Err(err).Msg("Failed to marshal Discord payload to JSON.")
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}
	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to write payload_json to multipart writer.")
		return fmt.Errorf("failed to write payload_json to multipart: %w", err)
	}

	if reportFilePath != "" {
		file, errOpen := os.Open(reportFilePath)
		if errOpen != nil {
			dn.logger.Error().Err(errOpen).Str("file_path", reportFilePath).Msg("Failed to open report file for Discord attachment. Notification will be sent without file.")
			// Continue without attachment
		} else {
			defer file.Close()
			fileInfo, errStat := file.Stat()
			if errStat != nil {
				dn.logger.Error().Err(errStat).Str("file_path", reportFilePath).Msg("Failed to get file stats for report file. Notification will be sent without file.")
			} else if fileInfo.Size() > maxDiscordFileSize {
				dn.logger.Warn().
					Str("file_path", reportFilePath).
					Int64("file_size_bytes", fileInfo.Size()).
					Int("max_size_bytes", maxDiscordFileSize).
					Msg("Report file exceeds Discord's size limit, not attaching.")
			} else {
				part, errCreate := writer.CreateFormFile("file", filepath.Base(reportFilePath))
				if errCreate != nil {
					dn.logger.Error().Err(errCreate).Msg("Failed to create form file for Discord attachment. Notification sent without file.")
				} else {
					if _, errCopy := io.Copy(part, file); errCopy != nil {
						dn.logger.Error().Err(errCopy).Msg("Failed to copy report file content to multipart writer. Notification sent without file.")
					} else {
						dn.logger.Info().Str("file_path", reportFilePath).Msg("Report file prepared for Discord attachment.")
					}
				}
			}
		}
	}

	if err := writer.Close(); err != nil {
		dn.logger.Error().Err(err).Msg("Failed to close multipart writer. This may result in a malformed request.")
		// Continue, but the request might fail or be malformed.
	}

	var resp *http.Response
	var reqErr error

	for i := 0; i < defaultRetryAttempts+1; i++ {
		select {
		case <-ctx.Done():
			dn.logger.Info().Err(ctx.Err()).Msg("Context cancelled, aborting Discord notification attempt.")
			return ctx.Err()
		default:
		}

		req, errReq := http.NewRequestWithContext(ctx, http.MethodPost, dn.cfg.DiscordWebhookURL, bytes.NewReader(body.Bytes()))
		if errReq != nil {
			dn.logger.Error().Err(errReq).Msg("Failed to create Discord webhook HTTP request.")
			return fmt.Errorf("failed to create request: %w", errReq) // Non-retryable
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		dn.logger.Debug().Str("webhook_url", dn.cfg.DiscordWebhookURL).Int("attempt", i+1).Msg("Sending notification to Discord.")
		resp, reqErr = dn.httpClient.Do(req)

		if reqErr != nil {
			dn.logger.Error().Err(reqErr).Int("attempt", i+1).Msg("HTTP request to Discord webhook failed.")
			if i < defaultRetryAttempts {
				time.Sleep(defaultRetryDelay)
				continue
			}
			return fmt.Errorf("failed to send discord notification after %d attempts: %w", defaultRetryAttempts+1, reqErr)
		}

		responseBodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			dn.logger.Warn().Err(readErr).Int("status_code", resp.StatusCode).Msg("Failed to read Discord response body.")
			// Continue processing the status code even if body read fails.
		}
		if errClose := resp.Body.Close(); errClose != nil {
			dn.logger.Warn().Err(errClose).Msg("Failed to close Discord response body.")
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			dn.logger.Info().Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully.")
			return nil // Success
		}

		dn.logger.Error().
			Int("status_code", resp.StatusCode).
			Bytes("response_body", responseBodyBytes).
			Int("attempt", i+1).
			Msg("Discord API returned an error or non-success status code.")

		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return fmt.Errorf("discord API client error: status %d, body: %s", resp.StatusCode, string(responseBodyBytes))
		}

		if i < defaultRetryAttempts {
			if retryAfterSeconds := resp.Header.Get("Retry-After"); retryAfterSeconds != "" {
				if sleepSeconds, parseErr := strconv.Atoi(retryAfterSeconds); parseErr == nil {
					sleepDuration := time.Duration(sleepSeconds) * time.Second
					dn.logger.Info().Dur("sleep_duration", sleepDuration).Msg("Respecting Discord Retry-After header.")
					time.Sleep(sleepDuration)
					continue
				} else {
					dn.logger.Warn().Err(parseErr).Str("retry_after_value", retryAfterSeconds).Msg("Could not parse Retry-After header from Discord.")
				}
			}
			time.Sleep(defaultRetryDelay)
		}
	}
	// If loop finishes, it means all retries failed.
	// Ensure resp is not nil before trying to access StatusCode if all attempts failed with reqErr != nil from the start.
	lastStatusCode := -1
	if resp != nil {
		lastStatusCode = resp.StatusCode
	}
	dn.logger.Error().Msgf("Failed to send Discord notification after %d attempts. Last HTTP error: %v. Last Status Code: %d", defaultRetryAttempts+1, reqErr, lastStatusCode)
	return fmt.Errorf("failed to send discord notification after %d attempts, last status: %d, last http error: %w", defaultRetryAttempts+1, lastStatusCode, reqErr)
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
