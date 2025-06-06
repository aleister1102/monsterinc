package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/reporter"

	"github.com/rs/zerolog"
)

// URLChecker handles checking individual URLs for changes
type URLChecker struct {
	logger           zerolog.Logger
	gCfg             *config.GlobalConfig
	historyStore     models.FileHistoryStore
	fetcher          *common.Fetcher
	processor        *ContentProcessor
	contentDiffer    *differ.ContentDiffer
	pathExtractor    *extractor.PathExtractor
	htmlDiffReporter *reporter.HtmlDiffReporter
}

// NewURLChecker creates a new URLChecker
func NewURLChecker(
	logger zerolog.Logger,
	gCfg *config.GlobalConfig,
	historyStore models.FileHistoryStore,
	fetcher *common.Fetcher,
	processor *ContentProcessor,
	contentDiffer *differ.ContentDiffer,
	pathExtractor *extractor.PathExtractor,
	htmlDiffReporter *reporter.HtmlDiffReporter,
) *URLChecker {
	return &URLChecker{
		logger:           logger.With().Str("component", "URLChecker").Logger(),
		gCfg:             gCfg,
		historyStore:     historyStore,
		fetcher:          fetcher,
		processor:        processor,
		contentDiffer:    contentDiffer,
		pathExtractor:    pathExtractor,
		htmlDiffReporter: htmlDiffReporter,
	}
}

// CheckResult represents the result of checking a URL
type CheckResult struct {
	FileChangeInfo *models.FileChangeInfo
	ErrorInfo      *models.MonitorFetchErrorInfo
	Success        bool
}

// CheckURL checks a single URL for changes
func (uc *URLChecker) CheckURL(url string, cycleID string) CheckResult {
	return uc.CheckURLWithContext(nil, url, cycleID)
}

// CheckURLWithContext checks a single URL for changes with context support
func (uc *URLChecker) CheckURLWithContext(ctx context.Context, url string, cycleID string) CheckResult {
	uc.logger.Debug().Str("url", url).Str("cycle_id", cycleID).Msg("Starting URL check")

	// Check for context cancellation before starting
	if ctx != nil {
		select {
		case <-ctx.Done():
			uc.logger.Debug().Str("url", url).Msg("URL check cancelled due to context cancellation")
			return uc.createErrorResult(url, cycleID, "context_cancelled", ctx.Err())
		default:
		}
	}

	// Fetch content from URL
	fetchResult, err := uc.fetchURLContentWithContext(ctx, url)
	if err != nil {
		return uc.createErrorResult(url, cycleID, "fetch", err)
	}

	// Process the fetched content
	processedUpdate, err := uc.processURLContent(url, fetchResult)
	if err != nil {
		return uc.createErrorResult(url, cycleID, "process", err)
	}

	// Detect changes
	fileChangeInfo, diffResult, err := uc.detectURLChanges(url, processedUpdate, fetchResult)
	if err != nil {
		return uc.createErrorResult(url, cycleID, "change_detection", err)
	}

	// Store the record
	err = uc.storeURLRecord(url, processedUpdate, fetchResult, diffResult, cycleID)
	if err != nil {
		return uc.createErrorResult(url, cycleID, "store_history", err)
	}

	return uc.createSuccessResult(url, cycleID, fileChangeInfo)
}

// detectURLChanges detects if a URL has changed
func (uc *URLChecker) detectURLChanges(
	url string,
	processedUpdate *models.MonitoredFileUpdate,
	fetchResult *common.FetchFileContentResult,
) (*models.FileChangeInfo, *models.ContentDiffResult, error) {
	// Get the last known record for this URL
	lastRecord, err := uc.getLastKnownRecord(url)
	if err != nil {
		return nil, nil, err
	}

	// Handle new file case
	if lastRecord == nil {
		fileChangeInfo := uc.createNewFileChangeInfo(url, processedUpdate)
		// Create a special diff result for new files
		newFileDiffResult := uc.createNewFileDiffResult(url, fetchResult, processedUpdate)
		return fileChangeInfo, newFileDiffResult, nil
	}

	// Check if content has changed
	if !uc.hasContentChanged(lastRecord, processedUpdate) {
		uc.logger.Debug().Str("url", url).Msg("No changes detected - hash matches")
		return nil, nil, nil
	}

	uc.logChangeDetected(url, lastRecord.Hash, processedUpdate.NewHash)

	// Generate comprehensive change information
	diffResult := uc.generateContentDiff(url, lastRecord, fetchResult, processedUpdate)
	extractedPaths := uc.extractPathsIfJavaScript(url, fetchResult)
	diffReportPath := uc.generateSingleDiffReport(url, diffResult, lastRecord, processedUpdate, fetchResult)

	fileChangeInfo := uc.createFileChangeInfo(url, lastRecord, processedUpdate, diffReportPath, extractedPaths)

	return fileChangeInfo, diffResult, nil
}

// storeURLRecord stores the URL record in the history store
func (uc *URLChecker) storeURLRecord(
	url string,
	processedUpdate *models.MonitoredFileUpdate,
	fetchResult *common.FetchFileContentResult,
	diffResult *models.ContentDiffResult,
	cycleID string,
) error {
	// Create base file history record
	record := uc.createBaseHistoryRecord(url, processedUpdate, fetchResult)

	// Add optional content
	uc.addContentToRecord(&record, fetchResult)

	// Add diff result if available
	uc.addDiffResultToRecord(&record, diffResult, url)

	// Store the record
	err := uc.historyStore.StoreFileRecord(record)
	if err != nil {
		return fmt.Errorf("failed to store file history record: %w", err)
	}

	uc.logRecordStored(url, processedUpdate.NewHash, cycleID)
	return nil
}

// Private helper methods for URL fetching and processing

func (uc *URLChecker) fetchURLContentWithContext(ctx context.Context, url string) (*common.FetchFileContentResult, error) {
	fetchResult, err := uc.fetcher.FetchFileContent(common.FetchFileContentInput{
		URL:     url,
		Context: ctx,
	})
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to fetch URL content")
		return nil, err
	}
	return fetchResult, nil
}

func (uc *URLChecker) processURLContent(url string, fetchResult *common.FetchFileContentResult) (*models.MonitoredFileUpdate, error) {
	processedUpdate, err := uc.processor.ProcessContent(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to process URL content")
		return nil, err
	}
	return processedUpdate, nil
}

// Private helper methods for change detection

func (uc *URLChecker) getLastKnownRecord(url string) (*models.FileHistoryRecord, error) {
	lastRecord, err := uc.historyStore.GetLastKnownRecord(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get last known record: %w", err)
	}
	return lastRecord, nil
}

func (uc *URLChecker) createNewFileChangeInfo(url string, processedUpdate *models.MonitoredFileUpdate) *models.FileChangeInfo {
	uc.logger.Info().Str("url", url).Msg("New file detected (no previous record)")
	return &models.FileChangeInfo{
		URL:         url,
		OldHash:     "",
		NewHash:     processedUpdate.NewHash,
		ContentType: processedUpdate.ContentType,
		ChangeTime:  processedUpdate.FetchedAt,
	}
}

func (uc *URLChecker) createNewFileDiffResult(url string, fetchResult *common.FetchFileContentResult, processedUpdate *models.MonitoredFileUpdate) *models.ContentDiffResult {
	uc.logger.Debug().Str("url", url).Msg("Creating diff result for new file")

	// Extract paths if it's JavaScript content
	extractedPaths := uc.extractPathsIfJavaScript(url, fetchResult)

	// Create diffs showing entire content as insertion for new files
	content := string(fetchResult.Content)
	var diffs []models.ContentDiff
	if len(strings.TrimSpace(content)) > 0 {
		diffs = []models.ContentDiff{
			{
				Operation: models.DiffInsert,
				Text:      content,
			},
		}
	}

	return &models.ContentDiffResult{
		Timestamp:      processedUpdate.FetchedAt.UnixMilli(),
		ContentType:    fetchResult.ContentType,
		OldHash:        "",
		NewHash:        processedUpdate.NewHash,
		IsIdentical:    false, // New files are considered as changes
		LinesAdded:     len(strings.Split(content, "\n")),
		LinesDeleted:   0,
		LinesChanged:   0,
		Diffs:          diffs, // Show entire content as insertion
		ErrorMessage:   "",
		ExtractedPaths: extractedPaths,
	}
}

func (uc *URLChecker) hasContentChanged(lastRecord *models.FileHistoryRecord, processedUpdate *models.MonitoredFileUpdate) bool {
	return lastRecord.Hash != processedUpdate.NewHash
}

func (uc *URLChecker) logChangeDetected(url, oldHash, newHash string) {
	uc.logger.Info().
		Str("url", url).
		Str("old_hash", oldHash).
		Str("new_hash", newHash).
		Msg("Change detected - generating diff")
}

func (uc *URLChecker) generateContentDiff(
	url string,
	lastRecord *models.FileHistoryRecord,
	fetchResult *common.FetchFileContentResult,
	processedUpdate *models.MonitoredFileUpdate,
) *models.ContentDiffResult {
	if uc.contentDiffer == nil {
		return nil
	}

	var previousContent []byte
	if lastRecord.Content != nil {
		previousContent = lastRecord.Content
	}

	generatedDiff, err := uc.contentDiffer.GenerateDiff(
		previousContent,
		fetchResult.Content,
		fetchResult.ContentType,
		lastRecord.Hash,
		processedUpdate.NewHash,
	)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to generate content diff")
		return nil
	}

	return generatedDiff
}

func (uc *URLChecker) extractPathsIfJavaScript(url string, fetchResult *common.FetchFileContentResult) []models.ExtractedPath {
	if uc.pathExtractor == nil || !uc.shouldExtractPaths(url, fetchResult.ContentType) {
		return nil
	}

	paths, err := uc.pathExtractor.ExtractPaths(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to extract paths from JavaScript content")
		return nil
	}

	uc.logger.Debug().Str("url", url).Int("path_count", len(paths)).Msg("Extracted paths from JavaScript content")
	return paths
}

func (uc *URLChecker) generateSingleDiffReport(
	url string,
	diffResult *models.ContentDiffResult,
	lastRecord *models.FileHistoryRecord,
	processedUpdate *models.MonitoredFileUpdate,
	fetchResult *common.FetchFileContentResult,
) *string {
	if uc.htmlDiffReporter == nil || diffResult == nil {
		return nil
	}

	reportPath, err := uc.htmlDiffReporter.GenerateSingleDiffReport(
		url,
		diffResult,
		lastRecord.Hash,
		processedUpdate.NewHash,
		fetchResult.Content,
	)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to generate single diff report")
		return nil
	}

	uc.logger.Debug().Str("url", url).Str("report_path", reportPath).Msg("Generated single diff report")
	return &reportPath
}

func (uc *URLChecker) createFileChangeInfo(
	url string,
	lastRecord *models.FileHistoryRecord,
	processedUpdate *models.MonitoredFileUpdate,
	diffReportPath *string,
	extractedPaths []models.ExtractedPath,
) *models.FileChangeInfo {
	return &models.FileChangeInfo{
		URL:            url,
		OldHash:        lastRecord.Hash,
		NewHash:        processedUpdate.NewHash,
		ContentType:    processedUpdate.ContentType,
		ChangeTime:     processedUpdate.FetchedAt,
		DiffReportPath: diffReportPath,
		ExtractedPaths: extractedPaths,
	}
}

// Private helper methods for record storage

func (uc *URLChecker) createBaseHistoryRecord(
	url string,
	processedUpdate *models.MonitoredFileUpdate,
	fetchResult *common.FetchFileContentResult,
) models.FileHistoryRecord {
	return models.FileHistoryRecord{
		URL:          url,
		Timestamp:    processedUpdate.FetchedAt.UnixMilli(),
		Hash:         processedUpdate.NewHash,
		ContentType:  processedUpdate.ContentType,
		ETag:         fetchResult.ETag,
		LastModified: fetchResult.LastModified,
	}
}

func (uc *URLChecker) addContentToRecord(record *models.FileHistoryRecord, fetchResult *common.FetchFileContentResult) {
	if uc.gCfg.MonitorConfig.StoreFullContentOnChange {
		record.Content = fetchResult.Content
	}
}

func (uc *URLChecker) addDiffResultToRecord(record *models.FileHistoryRecord, diffResult *models.ContentDiffResult, url string) {
	if diffResult == nil {
		return
	}

	diffJSON, err := json.Marshal(diffResult)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to marshal diff result to JSON")
		return
	}

	diffJSONStr := string(diffJSON)
	record.DiffResultJSON = &diffJSONStr
}

func (uc *URLChecker) logRecordStored(url, hash, cycleID string) {
	uc.logger.Debug().
		Str("url", url).
		Str("hash", hash).
		Str("cycle_id", cycleID).
		Msg("URL record stored successfully")
}

// Private helper methods for result creation

func (uc *URLChecker) createErrorResult(url, cycleID, source string, err error) CheckResult {
	errorInfo := &models.MonitorFetchErrorInfo{
		URL:        url,
		Error:      err.Error(),
		Source:     source,
		OccurredAt: time.Now(),
		CycleID:    cycleID,
	}
	return CheckResult{ErrorInfo: errorInfo, Success: false}
}

func (uc *URLChecker) createSuccessResult(url, cycleID string, fileChangeInfo *models.FileChangeInfo) CheckResult {
	if fileChangeInfo != nil {
		fileChangeInfo.CycleID = cycleID
		uc.logger.Info().Str("url", url).Str("cycle_id", cycleID).Msg("URL change detected and processed")
		return CheckResult{FileChangeInfo: fileChangeInfo, Success: true}
	}

	uc.logger.Debug().Str("url", url).Str("cycle_id", cycleID).Msg("URL check completed - no changes")
	return CheckResult{Success: true}
}

// isJavaScriptContent checks if the content type indicates JavaScript or URL has JS extension
func (uc *URLChecker) isJavaScriptContent(contentType string) bool {
	// First check content type
	jsContentTypes := []string{
		"application/javascript",
		"application/x-javascript",
		"text/javascript",
		"application/ecmascript",
		"text/ecmascript",
	}

	contentTypeLower := strings.ToLower(contentType)
	for _, jsType := range jsContentTypes {
		if strings.Contains(contentTypeLower, jsType) {
			return true
		}
	}

	return false
}

// isJavaScriptFile checks if URL has JavaScript file extension based on config
func (uc *URLChecker) isJavaScriptFile(url string) bool {
	if uc.gCfg == nil {
		return false
	}

	urlLower := strings.ToLower(url)
	for _, ext := range uc.gCfg.MonitorConfig.JSFileExtensions {
		if strings.HasSuffix(urlLower, ext) {
			return true
		}
	}
	return false
}

// isHTMLFile checks if URL has HTML file extension based on config
func (uc *URLChecker) isHTMLFile(url string) bool {
	if uc.gCfg == nil {
		return false
	}

	urlLower := strings.ToLower(url)
	for _, ext := range uc.gCfg.MonitorConfig.HTMLFileExtensions {
		if strings.HasSuffix(urlLower, ext) {
			return true
		}
	}
	return false
}

// shouldExtractPaths determines if paths should be extracted from content
func (uc *URLChecker) shouldExtractPaths(url string, contentType string) bool {
	return uc.isJavaScriptContent(contentType) || uc.isJavaScriptFile(url)
}
