package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"

	"archive/zip"

	"github.com/rs/zerolog"
)

const (
	defaultRetryAttempts            = 2
	defaultRetryDelay               = 5 * time.Second
	maxContentLength                = 8 * 1024 * 1024  // Discord's file size limit (8MB)
	maxDiscordFileSize              = 50 * 1024 * 1024 // 50MB, Discord's typical limit without Nitro
	maxFileSizeDefault              = 8 * 1024 * 1024  // 8MB default, Discord non-Nitro limit
	discordAttachmentSizeLimitBytes = 7 * 1024 * 1024  // 7MB to be safe, Nitro is 50MB, free is 8MB, but embeds count too
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

// SendNotification sends a DiscordMessagePayload to the specified webhookURL.
// It handles both regular JSON payloads and multipart/form-data for file uploads.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload, reportFilePath string) error {
	if webhookURL == "" {
		// This case should ideally be caught by the NotificationHelper before calling this.
		dn.logger.Warn().Msg("DiscordNotifier: SendNotification called with empty webhookURL. Notification skipped.")
		return fmt.Errorf("webhook URL is empty")
	}

	// Create a new context for the HTTP request itself with a timeout.
	// This ensures the send operation has its own lifecycle, independent of the caller's context potentially being cancelled too soon.
	httpReqCtx, httpReqCancel := context.WithTimeout(context.Background(), 30*time.Second) // 30-second timeout for the HTTP request
	defer httpReqCancel()

	var req *http.Request
	var err error

	body := new(bytes.Buffer)
	var writer *multipart.Writer
	var contentType string
	finalReportPathToSend := reportFilePath
	shouldCleanupTempZip := false
	tempZipPath := ""

	if reportFilePath != "" {
		fileInfo, statErr := os.Stat(reportFilePath)
		if statErr != nil {
			dn.logger.Warn().Err(statErr).Str("report_path", reportFilePath).Msg("Report file not found or inaccessible, sending notification without attachment.")
			reportFilePath = "" // Clear it so we don't try to attach
			finalReportPathToSend = ""
		} else if fileInfo.Size() > discordAttachmentSizeLimitBytes {
			dn.logger.Info().Str("report_path", reportFilePath).Int64("size_bytes", fileInfo.Size()).Msg("Report file exceeds Discord size limit, attempting to zip.")
			tempZipPath, err = dn.zipReportFile(reportFilePath)
			if err != nil {
				dn.logger.Error().Err(err).Str("report_path", reportFilePath).Msg("Failed to zip report file. Sending notification without attachment.")
				payload.Embeds[0].Fields = append(payload.Embeds[0].Fields, models.DiscordEmbedField{
					Name:   "📄 Report Status",
					Value:  "Report file was too large and zipping failed. Not attached.",
					Inline: false,
				})
				reportFilePath = "" // Clear it
				finalReportPathToSend = ""
			} else {
				zipInfo, zipStatErr := os.Stat(tempZipPath)
				if zipStatErr != nil {
					dn.logger.Error().Err(zipStatErr).Str("zip_path", tempZipPath).Msg("Failed to stat temporary zip file. Sending notification without attachment.")
					payload.Embeds[0].Fields = append(payload.Embeds[0].Fields, models.DiscordEmbedField{
						Name:   "📄 Report Status",
						Value:  "Report file was too large, zipping seemed to work but could not verify zip. Not attached.",
						Inline: false,
					})
					reportFilePath = "" // Clear it
					finalReportPathToSend = ""
					os.Remove(tempZipPath) // Attempt to clean up bad zip
				} else if zipInfo.Size() > discordAttachmentSizeLimitBytes {
					dn.logger.Warn().Str("zip_path", tempZipPath).Int64("size_bytes", zipInfo.Size()).Msg("Zipped report file still exceeds Discord size limit. Sending notification without attachment.")
					payload.Embeds[0].Fields = append(payload.Embeds[0].Fields, models.DiscordEmbedField{
						Name:   "📄 Report Status",
						Value:  "Report file was too large, and the zipped version is also too large. Not attached.",
						Inline: false,
					})
					reportFilePath = "" // Clear it
					finalReportPathToSend = ""
					os.Remove(tempZipPath) // Clean up the large zip
				} else {
					dn.logger.Info().Str("zip_path", tempZipPath).Int64("size_bytes", zipInfo.Size()).Msg("Successfully zipped report file within size limits.")
					finalReportPathToSend = tempZipPath // Use the zip file path
					shouldCleanupTempZip = true
					// Modify payload to indicate it's a zip
					for i, field := range payload.Embeds[0].Fields {
						if field.Name == "📄 Report" {
							payload.Embeds[0].Fields[i].Value = "Report was too large, sent as ZIP file attached below."
							break
						}
					}
				}
			}
		}
	} else {
		finalReportPathToSend = "" // Ensure it's empty if original reportFilePath was empty
	}

	if finalReportPathToSend != "" {
		writer = multipart.NewWriter(body)
		contentType = writer.FormDataContentType()

		// Write JSON payload for embeds
		jsonPayload, jsonErr := json.Marshal(payload)
		if jsonErr != nil {
			if shouldCleanupTempZip {
				os.Remove(tempZipPath)
			}
			return fmt.Errorf("failed to marshal discord payload to JSON: %w", jsonErr)
		}
		part, err := writer.CreateFormField("payload_json")
		if err != nil {
			if shouldCleanupTempZip {
				os.Remove(tempZipPath)
			}
			return fmt.Errorf("failed to create form field for json_payload: %w", err)
		}
		_, err = part.Write(jsonPayload)
		if err != nil {
			if shouldCleanupTempZip {
				os.Remove(tempZipPath)
			}
			return fmt.Errorf("failed to write json_payload: %w", err)
		}

		file, errFile := os.Open(finalReportPathToSend)
		if errFile != nil {
			dn.logger.Error().Err(errFile).Str("file_path", finalReportPathToSend).Msg("Failed to open file for attachment, sending without it.")
			// Reset to send as plain JSON if file opening fails unexpectedly after deciding to attach
			body.Reset()
			json.NewEncoder(body).Encode(payload)
			contentType = "application/json"
			if shouldCleanupTempZip {
				os.Remove(tempZipPath)
			}
		} else {
			defer file.Close()
			part, err := writer.CreateFormFile("file[0]", filepath.Base(finalReportPathToSend))
			if err != nil {
				if shouldCleanupTempZip {
					os.Remove(tempZipPath)
				}
				return fmt.Errorf("failed to create form file: %w", err)
			}
			_, err = io.Copy(part, file)
			if err != nil {
				if shouldCleanupTempZip {
					os.Remove(tempZipPath)
				}
				return fmt.Errorf("failed to copy file to form: %w", err)
			}
		}
		writer.Close() // Close multipart writer before sending
	} else {
		// No file to attach, just encode JSON payload
		contentType = "application/json" // Set Content-Type for JSON payload
		err = json.NewEncoder(body).Encode(payload)
		if err != nil {
			return fmt.Errorf("failed to encode discord payload to JSON: %w", err)
		}
	}

	// TODO: use http_client.go
	req, err = http.NewRequestWithContext(httpReqCtx, http.MethodPost, webhookURL, body)
	if err != nil {
		if shouldCleanupTempZip {
			os.Remove(tempZipPath)
		}
		return fmt.Errorf("failed to create discord request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := dn.httpClient.Do(req)
	if err != nil {
		if shouldCleanupTempZip {
			os.Remove(tempZipPath)
		}
		return fmt.Errorf("failed to send discord notification to %s: %w", webhookURL, err)
	}
	defer resp.Body.Close()

	if shouldCleanupTempZip {
		dn.logger.Debug().Str("temp_zip_path", tempZipPath).Msg("Cleaning up temporary zip file.")
		os.Remove(tempZipPath)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		dn.logger.Debug().Str("webhook_url", webhookURL).Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully")
		return nil
	}

	respBodyBytes, _ := io.ReadAll(resp.Body)
	dn.logger.Error().Int("status_code", resp.StatusCode).Str("webhook_url", webhookURL).Str("response_body", string(respBodyBytes)).Msg("Discord notification failed")
	return fmt.Errorf("HTTP %d error: discord notification to %s failed with status %d: %s", resp.StatusCode, webhookURL, resp.StatusCode, string(respBodyBytes))
}

// zipReportFile creates a zip archive of the given report file.
// It returns the path to the temporary zip file and an error if any.
func (dn *DiscordNotifier) zipReportFile(sourceFilePath string) (string, error) {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file for zipping '%s': %w", sourceFilePath, err)
	}
	defer sourceFile.Close()

	// Create a temporary zip file
	tempZipFile, err := os.CreateTemp("", "report-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary zip file: %w", err)
	}
	defer tempZipFile.Close() // Close it in case of error during zipping

	zipWriter := zip.NewWriter(tempZipFile)

	fileInZip, err := zipWriter.Create(filepath.Base(sourceFilePath)) // Use the original file name inside the zip
	if err != nil {
		return "", fmt.Errorf("failed to create entry in zip file: %w", err)
	}

	_, err = io.Copy(fileInZip, sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy source file content to zip: %w", err)
	}

	err = zipWriter.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close zip writer: %w", err)
	}

	// tempZipFile is closed by defer. We need to return its name.
	return tempZipFile.Name(), nil
}
