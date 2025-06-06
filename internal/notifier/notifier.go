package notifier

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"

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
	logger     zerolog.Logger
	httpClient *common.FastHTTPClient
}

// NewDiscordNotifier creates a new DiscordNotifier.
func NewDiscordNotifier(logger zerolog.Logger, httpClient *common.FastHTTPClient) (*DiscordNotifier, error) {
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

	return &DiscordNotifier{
		logger:     moduleLogger,
		httpClient: httpClient,
	}, nil
}

// AttachmentProcessingResult holds the result of processing report attachments
type AttachmentProcessingResult struct {
	FinalReportPath  string
	ShouldCleanupZip bool
	TempZipPath      string
	UpdatedPayload   models.DiscordMessagePayload
}

// SendNotification sends a DiscordMessagePayload to the specified webhookURL.
// It handles both regular JSON payloads and multipart/form-data for file uploads.
func (dn *DiscordNotifier) SendNotification(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload, reportFilePath string) error {
	if err := dn.validateWebhookURL(webhookURL); err != nil {
		return err
	}

	// Process attachment if provided
	attachmentResult, err := dn.processAttachment(reportFilePath, payload)
	if err != nil {
		return err
	}
	defer dn.cleanupTempFiles(attachmentResult)

	// Send the request using fasthttp
	return dn.sendHTTPRequest(ctx, webhookURL, attachmentResult)
}

// validateWebhookURL validates the provided webhook URL
func (dn *DiscordNotifier) validateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		dn.logger.Warn().Msg("DiscordNotifier: SendNotification called with empty webhookURL. Notification skipped.")
		return fmt.Errorf("webhook URL is empty")
	}
	return nil
}

// processAttachment handles report file attachment processing including zipping if needed
func (dn *DiscordNotifier) processAttachment(reportFilePath string, payload models.DiscordMessagePayload) (*AttachmentProcessingResult, error) {
	result := &AttachmentProcessingResult{
		UpdatedPayload: payload,
	}

	if reportFilePath == "" {
		return result, nil
	}

	fileInfo, err := os.Stat(reportFilePath)
	if err != nil {
		dn.logger.Warn().Err(err).Str("report_path", reportFilePath).Msg("Report file not found or inaccessible, sending notification without attachment.")
		return result, nil
	}

	if fileInfo.Size() <= discordAttachmentSizeLimitBytes {
		result.FinalReportPath = reportFilePath
		return result, nil
	}

	// File is too large, attempt to zip it
	return dn.handleOversizedFile(reportFilePath, fileInfo.Size(), result)
}

// handleOversizedFile processes files that exceed Discord's size limit
func (dn *DiscordNotifier) handleOversizedFile(reportFilePath string, fileSize int64, result *AttachmentProcessingResult) (*AttachmentProcessingResult, error) {
	dn.logger.Info().Str("report_path", reportFilePath).Int64("size_bytes", fileSize).Msg("Report file exceeds Discord size limit, attempting to zip.")

	tempZipPath, err := dn.zipReportFile(reportFilePath)
	if err != nil {
		dn.logger.Error().Err(err).Str("report_path", reportFilePath).Msg("Failed to zip report file. Sending notification without attachment.")
		result.UpdatedPayload = dn.addReportStatusField(result.UpdatedPayload, "Report file was too large and zipping failed. Not attached.")
		return result, nil
	}

	result.TempZipPath = tempZipPath
	return dn.validateZippedFile(tempZipPath, result)
}

// validateZippedFile checks if the zipped file is within size limits
func (dn *DiscordNotifier) validateZippedFile(tempZipPath string, result *AttachmentProcessingResult) (*AttachmentProcessingResult, error) {
	zipInfo, err := os.Stat(tempZipPath)
	if err != nil {
		dn.logger.Error().Err(err).Str("zip_path", tempZipPath).Msg("Failed to stat temporary zip file. Sending notification without attachment.")
		result.UpdatedPayload = dn.addReportStatusField(result.UpdatedPayload, "Report file was too large, zipping seemed to work but could not verify zip. Not attached.")
		err = os.Remove(tempZipPath)
		if err != nil {
			dn.logger.Error().Err(err).Str("zip_path", tempZipPath).Msg("Failed to remove temporary zip file.")
		}
		return result, nil
	}

	if zipInfo.Size() > discordAttachmentSizeLimitBytes {
		dn.logger.Warn().Str("zip_path", tempZipPath).Int64("size_bytes", zipInfo.Size()).Msg("Zipped report file still exceeds Discord size limit. Sending notification without attachment.")
		result.UpdatedPayload = dn.addReportStatusField(result.UpdatedPayload, "Report file was too large, and the zipped version is also too large. Not attached.")
		err = os.Remove(tempZipPath)
		if err != nil {
			dn.logger.Error().Err(err).Str("zip_path", tempZipPath).Msg("Failed to remove temporary zip file.")
		}
		return result, nil
	}

	dn.logger.Info().Str("zip_path", tempZipPath).Int64("size_bytes", zipInfo.Size()).Msg("Successfully zipped report file within size limits.")
	result.FinalReportPath = tempZipPath
	result.ShouldCleanupZip = true
	result.UpdatedPayload = dn.updatePayloadForZipFile(result.UpdatedPayload)
	return result, nil
}

// addReportStatusField adds a status field to the payload about report attachment
func (dn *DiscordNotifier) addReportStatusField(payload models.DiscordMessagePayload, statusMessage string) models.DiscordMessagePayload {
	if len(payload.Embeds) > 0 {
		payload.Embeds[0].Fields = append(payload.Embeds[0].Fields, models.DiscordEmbedField{
			Name:   "📄 Report Status",
			Value:  statusMessage,
			Inline: false,
		})
	}
	return payload
}

// updatePayloadForZipFile updates payload to indicate the file was zipped
func (dn *DiscordNotifier) updatePayloadForZipFile(payload models.DiscordMessagePayload) models.DiscordMessagePayload {
	if len(payload.Embeds) > 0 {
		for i, field := range payload.Embeds[0].Fields {
			if field.Name == "📄 Report" {
				payload.Embeds[0].Fields[i].Value = "Report was too large, sent as ZIP file attached below."
				break
			}
		}
	}
	return payload
}

// sendHTTPRequest sends the HTTP request using fasthttp
func (dn *DiscordNotifier) sendHTTPRequest(ctx context.Context, webhookURL string, attachmentResult *AttachmentProcessingResult) error {
	if attachmentResult.FinalReportPath == "" {
		// Send JSON payload only
		return dn.sendJSONRequest(ctx, webhookURL, attachmentResult.UpdatedPayload)
	}

	// Send multipart request with file attachment
	return dn.sendMultipartRequest(ctx, webhookURL, attachmentResult)
}

// sendJSONRequest sends a JSON-only request
func (dn *DiscordNotifier) sendJSONRequest(ctx context.Context, webhookURL string, payload models.DiscordMessagePayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return common.WrapError(err, "failed to marshal Discord payload to JSON")
	}

	req := &common.HTTPRequest{
		URL:     webhookURL,
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    bytes.NewReader(jsonData),
		Context: ctx,
	}

	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return common.WrapError(err, "failed to send Discord notification")
	}

	return dn.processHTTPResponse(resp)
}

// sendMultipartRequest sends a multipart request with file attachment
func (dn *DiscordNotifier) sendMultipartRequest(ctx context.Context, webhookURL string, attachmentResult *AttachmentProcessingResult) error {
	body, contentType, err := dn.createMultipartRequest(attachmentResult)
	if err != nil {
		return err
	}

	req := &common.HTTPRequest{
		URL:     webhookURL,
		Method:  "POST",
		Headers: map[string]string{"Content-Type": contentType},
		Body:    body,
		Context: ctx,
	}

	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return common.WrapError(err, "failed to send Discord notification with attachment")
	}

	return dn.processHTTPResponse(resp)
}

// createMultipartRequest creates a multipart/form-data request
func (dn *DiscordNotifier) createMultipartRequest(attachmentResult *AttachmentProcessingResult) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add JSON payload as "payload_json" field
	if err := dn.addJSONPayloadToForm(writer, attachmentResult.UpdatedPayload); err != nil {
		return nil, "", err
	}

	// Add file attachment
	if err := dn.addFileToForm(writer, attachmentResult.FinalReportPath); err != nil {
		return nil, "", err
	}

	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return nil, "", common.WrapError(err, "failed to close multipart writer")
	}

	return body, contentType, nil
}

// addJSONPayloadToForm adds the JSON payload to the multipart form
func (dn *DiscordNotifier) addJSONPayloadToForm(writer *multipart.Writer, payload models.DiscordMessagePayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return common.WrapError(err, "failed to marshal Discord payload to JSON")
	}

	payloadWriter, err := writer.CreateFormField("payload_json")
	if err != nil {
		return common.WrapError(err, "failed to create payload_json form field")
	}

	if _, err := payloadWriter.Write(jsonData); err != nil {
		return common.WrapError(err, "failed to write JSON payload to form")
	}

	return nil
}

// addFileToForm adds a file to the multipart form
func (dn *DiscordNotifier) addFileToForm(writer *multipart.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return common.WrapError(err, fmt.Sprintf("failed to open file %s", filePath))
	}
	defer file.Close()

	fileName := filepath.Base(filePath)
	fileWriter, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return common.WrapError(err, "failed to create form file field")
	}

	if _, err := io.Copy(fileWriter, file); err != nil {
		return common.WrapError(err, "failed to copy file content to form")
	}

	dn.logger.Debug().Str("file_path", filePath).Str("file_name", fileName).Msg("Successfully added file to multipart form")
	return nil
}

// processHTTPResponse processes the HTTP response from Discord
func (dn *DiscordNotifier) processHTTPResponse(resp *common.HTTPResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		dn.logger.Debug().Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully")
		return nil
	}

	dn.logger.Error().Int("status_code", resp.StatusCode).Str("response_body", string(resp.Body)).Msg("Discord notification failed")
	return fmt.Errorf("Discord webhook returned status %d: %s", resp.StatusCode, string(resp.Body))
}

// cleanupTempFiles removes temporary files created during processing
func (dn *DiscordNotifier) cleanupTempFiles(attachmentResult *AttachmentProcessingResult) {
	if attachmentResult.ShouldCleanupZip && attachmentResult.TempZipPath != "" {
		if err := os.Remove(attachmentResult.TempZipPath); err != nil {
			dn.logger.Error().Err(err).Str("zip_path", attachmentResult.TempZipPath).Msg("Failed to remove temporary zip file")
		} else {
			dn.logger.Debug().Str("zip_path", attachmentResult.TempZipPath).Msg("Successfully removed temporary zip file")
		}
	}
}

// zipReportFile creates a ZIP file containing the report
func (dn *DiscordNotifier) zipReportFile(sourceFilePath string) (string, error) {
	zipFilePath := dn.generateZipFilePath(sourceFilePath)

	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return "", common.WrapError(err, fmt.Sprintf("failed to create zip file %s", zipFilePath))
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	if err := dn.addFileToZip(zipWriter, sourceFilePath); err != nil {
		// Clean up the incomplete zip file
		os.Remove(zipFilePath)
		return "", err
	}

	dn.logger.Debug().Str("source_path", sourceFilePath).Str("zip_path", zipFilePath).Msg("Successfully created zip file")
	return zipFilePath, nil
}

// generateZipFilePath generates a path for the temporary ZIP file
func (dn *DiscordNotifier) generateZipFilePath(sourceFilePath string) string {
	dir := filepath.Dir(sourceFilePath)
	baseName := filepath.Base(sourceFilePath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]

	return filepath.Join(dir, fmt.Sprintf("%s.zip", nameWithoutExt))
}

// addFileToZip adds a file to the ZIP archive
func (dn *DiscordNotifier) addFileToZip(zipWriter *zip.Writer, sourceFilePath string) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return common.WrapError(err, fmt.Sprintf("failed to open source file %s", sourceFilePath))
	}
	defer sourceFile.Close()

	fileName := filepath.Base(sourceFilePath)
	zipFileWriter, err := zipWriter.Create(fileName)
	if err != nil {
		return common.WrapError(err, fmt.Sprintf("failed to create zip entry for %s", fileName))
	}

	if _, err := io.Copy(zipFileWriter, sourceFile); err != nil {
		return common.WrapError(err, fmt.Sprintf("failed to copy file content to zip for %s", fileName))
	}

	return nil
}
