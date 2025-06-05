package monitor

import (
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
	uc.logger.Debug().Str("url", url).Str("cycle_id", cycleID).Msg("Starting URL check")

	// Fetch content from URL
	fetchResult, err := uc.fetcher.FetchFileContent(common.FetchFileContentInput{URL: url})
	if err != nil {
		errorInfo := &models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
			Source:     "fetch",
			OccurredAt: time.Now(),
			CycleID:    cycleID,
		}
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to fetch URL content")
		return CheckResult{ErrorInfo: errorInfo, Success: false}
	}

	// Process the fetched content
	processedUpdate, err := uc.processor.ProcessContent(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		errorInfo := &models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
			Source:     "process",
			OccurredAt: time.Now(),
			CycleID:    cycleID,
		}
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to process URL content")
		return CheckResult{ErrorInfo: errorInfo, Success: false}
	}

	// Detect changes
	fileChangeInfo, diffResult, err := uc.detectURLChanges(url, processedUpdate, fetchResult)
	if err != nil {
		errorInfo := &models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
			Source:     "change_detection",
			OccurredAt: time.Now(),
			CycleID:    cycleID,
		}
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to detect URL changes")
		return CheckResult{ErrorInfo: errorInfo, Success: false}
	}

	// Store the record
	err = uc.storeURLRecord(url, processedUpdate, fetchResult, diffResult, cycleID)
	if err != nil {
		errorInfo := &models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
			Source:     "store_history",
			OccurredAt: time.Now(),
			CycleID:    cycleID,
		}
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to store URL record")
		return CheckResult{ErrorInfo: errorInfo, Success: false}
	}

	if fileChangeInfo != nil {
		fileChangeInfo.CycleID = cycleID
		uc.logger.Info().Str("url", url).Str("cycle_id", cycleID).Msg("URL change detected and processed")
		return CheckResult{FileChangeInfo: fileChangeInfo, Success: true}
	}

	uc.logger.Debug().Str("url", url).Str("cycle_id", cycleID).Msg("URL check completed - no changes")
	return CheckResult{Success: true}
}

// detectURLChanges detects if a URL has changed
func (uc *URLChecker) detectURLChanges(
	url string,
	processedUpdate *models.MonitoredFileUpdate,
	fetchResult *common.FetchFileContentResult,
) (*models.FileChangeInfo, *models.ContentDiffResult, error) {
	// Get the last known record for this URL
	lastRecord, err := uc.historyStore.GetLastKnownRecord(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get last known record: %w", err)
	}

	// If no previous record exists, this is a new file
	if lastRecord == nil {
		uc.logger.Info().Str("url", url).Msg("New file detected (no previous record)")
		return &models.FileChangeInfo{
			URL:         url,
			OldHash:     "",
			NewHash:     processedUpdate.NewHash,
			ContentType: processedUpdate.ContentType,
			ChangeTime:  processedUpdate.FetchedAt,
		}, nil, nil
	}

	// If hash is the same, no change
	if lastRecord.Hash == processedUpdate.NewHash {
		uc.logger.Debug().Str("url", url).Msg("No changes detected - hash matches")
		return nil, nil, nil
	}

	uc.logger.Info().
		Str("url", url).
		Str("old_hash", lastRecord.Hash).
		Str("new_hash", processedUpdate.NewHash).
		Msg("Change detected - generating diff")

	// Generate content diff if differ is available
	var diffResult *models.ContentDiffResult
	if uc.contentDiffer != nil {
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
		} else {
			diffResult = generatedDiff
		}
	}

	// Extract paths if this is a JavaScript file and path extractor is available
	var extractedPaths []models.ExtractedPath
	if uc.pathExtractor != nil && uc.isJavaScriptContent(fetchResult.ContentType) {
		paths, err := uc.pathExtractor.ExtractPaths(url, fetchResult.Content, fetchResult.ContentType)
		if err != nil {
			uc.logger.Error().Err(err).Str("url", url).Msg("Failed to extract paths from JavaScript content")
		} else {
			extractedPaths = paths
			uc.logger.Debug().Str("url", url).Int("path_count", len(paths)).Msg("Extracted paths from JavaScript content")
		}
	}

	// Generate single diff report if HTML diff reporter is available
	var diffReportPath *string
	if uc.htmlDiffReporter != nil && diffResult != nil {
		reportPath, err := uc.htmlDiffReporter.GenerateSingleDiffReport(
			url,
			diffResult,
			lastRecord.Hash,
			processedUpdate.NewHash,
			fetchResult.Content,
		)
		if err != nil {
			uc.logger.Error().Err(err).Str("url", url).Msg("Failed to generate single diff report")
		} else {
			diffReportPath = &reportPath
			uc.logger.Debug().Str("url", url).Str("report_path", reportPath).Msg("Generated single diff report")
		}
	}

	fileChangeInfo := &models.FileChangeInfo{
		URL:            url,
		OldHash:        lastRecord.Hash,
		NewHash:        processedUpdate.NewHash,
		ContentType:    processedUpdate.ContentType,
		ChangeTime:     processedUpdate.FetchedAt,
		DiffReportPath: diffReportPath,
		ExtractedPaths: extractedPaths,
	}

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
	// Create file history record
	record := models.FileHistoryRecord{
		URL:          url,
		Timestamp:    processedUpdate.FetchedAt.UnixMilli(),
		Hash:         processedUpdate.NewHash,
		ContentType:  processedUpdate.ContentType,
		ETag:         fetchResult.ETag,
		LastModified: fetchResult.LastModified,
	}

	// Store full content if configured
	if uc.gCfg.MonitorConfig.StoreFullContentOnChange {
		record.Content = fetchResult.Content
	}

	// Store diff result as JSON if available
	if diffResult != nil {
		diffJSON, err := json.Marshal(diffResult)
		if err != nil {
			uc.logger.Error().Err(err).Str("url", url).Msg("Failed to marshal diff result to JSON")
		} else {
			diffJSONStr := string(diffJSON)
			record.DiffResultJSON = &diffJSONStr
		}
	}

	// Store extracted paths as JSON if available
	if record.ExtractedPathsJSON != nil && len(*record.ExtractedPathsJSON) > 0 {
		// This logic might need to be updated based on how ExtractedPaths are handled
		// For now, assuming we have extracted paths from somewhere
		// In the actual implementation, this would come from the change detection
	}

	// Store the record
	err := uc.historyStore.StoreFileRecord(record)
	if err != nil {
		return fmt.Errorf("failed to store file history record: %w", err)
	}

	uc.logger.Debug().
		Str("url", url).
		Str("hash", processedUpdate.NewHash).
		Str("cycle_id", cycleID).
		Msg("URL record stored successfully")

	return nil
}

// isJavaScriptContent checks if the content type indicates JavaScript
func (uc *URLChecker) isJavaScriptContent(contentType string) bool {
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
