package notifier

import (
	"archive/zip"
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
	httpClient *http.Client
}

// NewDiscordNotifier creates a new DiscordNotifier.
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

	httpReqCtx, httpReqCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer httpReqCancel()

	// Process attachment if provided
	attachmentResult, err := dn.processAttachment(reportFilePath, payload)
	if err != nil {
		return err
	}
	defer dn.cleanupTempFiles(attachmentResult)

	// Prepare HTTP request
	req, err := dn.prepareHTTPRequest(httpReqCtx, webhookURL, attachmentResult)
	if err != nil {
		return err
	}

	// Send the request
	return dn.sendHTTPRequest(req)
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
		os.Remove(tempZipPath)
		return result, nil
	}

	if zipInfo.Size() > discordAttachmentSizeLimitBytes {
		dn.logger.Warn().Str("zip_path", tempZipPath).Int64("size_bytes", zipInfo.Size()).Msg("Zipped report file still exceeds Discord size limit. Sending notification without attachment.")
		result.UpdatedPayload = dn.addReportStatusField(result.UpdatedPayload, "Report file was too large, and the zipped version is also too large. Not attached.")
		os.Remove(tempZipPath)
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

// prepareHTTPRequest creates the HTTP request with appropriate content type and body
func (dn *DiscordNotifier) prepareHTTPRequest(ctx context.Context, webhookURL string, attachmentResult *AttachmentProcessingResult) (*http.Request, error) {
	var body *bytes.Buffer
	var contentType string
	var err error

	if attachmentResult.FinalReportPath != "" {
		body, contentType, err = dn.createMultipartRequest(attachmentResult)
	} else {
		body, contentType, err = dn.createJSONRequest(attachmentResult.UpdatedPayload)
	}

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "MonsterInc/1.0")
	return req, nil
}

// createMultipartRequest creates a multipart form request with file attachment
func (dn *DiscordNotifier) createMultipartRequest(attachmentResult *AttachmentProcessingResult) (*bytes.Buffer, string, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	defer writer.Close()

	// Add JSON payload
	if err := dn.addJSONPayloadToForm(writer, attachmentResult.UpdatedPayload); err != nil {
		return nil, "", err
	}

	// Add file attachment
	if err := dn.addFileToForm(writer, attachmentResult.FinalReportPath); err != nil {
		// Fallback to JSON-only request if file attachment fails
		dn.logger.Error().Err(err).Str("file_path", attachmentResult.FinalReportPath).Msg("Failed to add file to form, falling back to JSON-only request.")
		return dn.createJSONRequest(attachmentResult.UpdatedPayload)
	}

	return body, writer.FormDataContentType(), nil
}

// createJSONRequest creates a JSON-only request
func (dn *DiscordNotifier) createJSONRequest(payload models.DiscordMessagePayload) (*bytes.Buffer, string, error) {
	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return nil, "", fmt.Errorf("failed to encode JSON payload: %w", err)
	}
	return body, "application/json", nil
}

// addJSONPayloadToForm adds the JSON payload to the multipart form
func (dn *DiscordNotifier) addJSONPayloadToForm(writer *multipart.Writer, payload models.DiscordMessagePayload) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload to JSON: %w", err)
	}

	part, err := writer.CreateFormField("payload_json")
	if err != nil {
		return fmt.Errorf("failed to create form field for json_payload: %w", err)
	}

	if _, err = part.Write(jsonPayload); err != nil {
		return fmt.Errorf("failed to write json_payload: %w", err)
	}

	return nil
}

// addFileToForm adds the file attachment to the multipart form
func (dn *DiscordNotifier) addFileToForm(writer *multipart.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file[0]", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err = io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file to form: %w", err)
	}

	return nil
}

// sendHTTPRequest sends the HTTP request and processes the response
func (dn *DiscordNotifier) sendHTTPRequest(req *http.Request) error {
	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	return dn.processHTTPResponse(resp)
}

// processHTTPResponse processes the HTTP response and handles errors
func (dn *DiscordNotifier) processHTTPResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		dn.logger.Debug().Int("status_code", resp.StatusCode).Msg("Discord notification sent successfully")
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("discord webhook responded with status %d: %s", resp.StatusCode, string(respBody))
}

// cleanupTempFiles removes temporary files if needed
func (dn *DiscordNotifier) cleanupTempFiles(attachmentResult *AttachmentProcessingResult) {
	if attachmentResult.ShouldCleanupZip && attachmentResult.TempZipPath != "" {
		os.Remove(attachmentResult.TempZipPath)
	}
}

// zipReportFile creates a zip file from the source file and returns the path to the zip file.
func (dn *DiscordNotifier) zipReportFile(sourceFilePath string) (string, error) {
	zipFilePath := dn.generateZipFilePath(sourceFilePath)

	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return zipFilePath, dn.addFileToZip(zipWriter, sourceFilePath)
}

// generateZipFilePath generates the zip file path from source file path
func (dn *DiscordNotifier) generateZipFilePath(sourceFilePath string) string {
	dir := filepath.Dir(sourceFilePath)
	baseName := filepath.Base(sourceFilePath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]
	zipFileName := fmt.Sprintf("%s.zip", nameWithoutExt)
	return filepath.Join(dir, zipFileName)
}

// addFileToZip adds a file to the zip archive
func (dn *DiscordNotifier) addFileToZip(zipWriter *zip.Writer, sourceFilePath string) error {
	fileToZip, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer fileToZip.Close()

	info, err := fileToZip.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("failed to create zip file header: %w", err)
	}

	header.Name = filepath.Base(sourceFilePath)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip writer: %w", err)
	}

	if _, err = io.Copy(writer, fileToZip); err != nil {
		return fmt.Errorf("failed to copy file to zip: %w", err)
	}

	return nil
}
