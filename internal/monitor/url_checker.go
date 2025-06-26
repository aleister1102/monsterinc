package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/monsterinc/httpclient"

	"github.com/rs/zerolog"
)

// CheckResult represents the result of checking a URL
type CheckResult struct {
	URL            string
	Changed        bool
	NewHash        string
	OldHash        string
	ContentType    string
	Content        []byte
	Error          error
	ProcessedAt    time.Time
	DiffResult     *models.ContentDiffResult
	ExtractedPaths []models.ExtractedPath
}

// URLChecker handles the checking of individual URLs with memory optimization
type URLChecker struct {
	logger           zerolog.Logger
	gCfg             *config.GlobalConfig
	historyStore     models.FileHistoryStore
	fetcher          *httpclient.Fetcher
	processor        *ContentProcessor
	contentDiffer    *differ.ContentDiffer
	pathExtractor    *extractor.PathExtractor
	htmlDiffReporter *reporter.HtmlDiffReporter

	// Memory optimization components
	bufferPool *common.BufferPool
	slicePool  *common.SlicePool
}

// NewURLChecker creates a new URLChecker with memory optimization
func NewURLChecker(
	logger zerolog.Logger,
	gCfg *config.GlobalConfig,
	historyStore models.FileHistoryStore,
	fetcher *httpclient.Fetcher,
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
		// Initialize memory pools
		bufferPool: common.NewBufferPool(64 * 1024), // 64KB buffers
		slicePool:  common.NewSlicePool(32 * 1024),  // 32KB slices
	}
}

// CheckURL checks a single URL for changes with memory optimization
func (uc *URLChecker) CheckURL(ctx context.Context, url string) CheckResult {
	result := CheckResult{
		URL:         url,
		ProcessedAt: time.Now(),
	}

	// Get buffer from pool for processing
	buffer := uc.bufferPool.Get()
	defer uc.bufferPool.Put(buffer)

	// Fetch content with context and memory optimization
	fetchResult, err := uc.fetchContentWithOptimization(ctx, url)
	if err != nil {
		result.Error = fmt.Errorf("failed to fetch content: %w", err)
		return result
	}

	// Process content to get hash and type
	update, err := uc.processor.ProcessContent(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		result.Error = fmt.Errorf("failed to process content: %w", err)
		return result
	}

	result.NewHash = update.NewHash
	result.ContentType = update.ContentType

	// Store limited content if configured
	if uc.gCfg.MonitorConfig.StoreFullContentOnChange {
		// Use slice pool for content storage
		contentSlice := uc.slicePool.Get()
		defer uc.slicePool.Put(contentSlice)

		if len(fetchResult.Content) <= cap(contentSlice) {
			contentSlice = contentSlice[:len(fetchResult.Content)]
			copy(contentSlice, fetchResult.Content)
			result.Content = make([]byte, len(contentSlice))
			copy(result.Content, contentSlice)
		} else {
			// Content too large for pool, use direct allocation
			result.Content = make([]byte, len(fetchResult.Content))
			copy(result.Content, fetchResult.Content)
		}
	}

	// Get last known record to compare
	lastRecord, err := uc.historyStore.GetLastKnownRecord(url)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to get last known record")
		// Continue with empty last record
	}

	if lastRecord != nil {
		result.OldHash = lastRecord.Hash
		result.Changed = lastRecord.Hash != update.NewHash
	} else {
		result.Changed = true // New URL, consider as changed
	}

	// Generate diff if content changed and differ is available
	if result.Changed && uc.contentDiffer != nil {
		if lastRecord != nil {
			// Normal diff between old and new content
			diffResult := uc.generateContentDiff(lastRecord, fetchResult.Content, result.ContentType, result.OldHash, result.NewHash)
			result.DiffResult = diffResult
		} else {
			// First time monitoring this URL - treat all content as insertion
			diffResult := uc.generateFirstTimeDiff(fetchResult.Content, result.ContentType, result.NewHash)
			result.DiffResult = diffResult
		}
	}

	// Extract paths if path extractor is available and content type is suitable
	if uc.pathExtractor != nil && uc.shouldExtractPaths(result.ContentType) {
		extractedPaths := uc.extractPathsWithOptimization(url, fetchResult.Content, result.ContentType)
		result.ExtractedPaths = extractedPaths
	}

	// Store the new record
	if err := uc.storeFileRecord(url, result, fetchResult); err != nil {
		uc.logger.Error().Err(err).Str("url", url).Msg("Failed to store file record")
		result.Error = fmt.Errorf("failed to store record: %w", err)
	}

	return result
}

// fetchContentWithOptimization fetches content using memory-optimized approach
func (uc *URLChecker) fetchContentWithOptimization(ctx context.Context, url string) (*httpclient.FetchFileContentResult, error) {
	var previousETag, previousLastModified string

	// Only get previous cache headers if not bypassing cache
	if !uc.gCfg.MonitorConfig.BypassCache {
		if lastRecord, err := uc.historyStore.GetLastKnownRecord(url); err == nil && lastRecord != nil {
			previousETag = lastRecord.ETag
			previousLastModified = lastRecord.LastModified
		}
	}

	fetchInput := httpclient.FetchFileContentInput{
		URL:                  url,
		PreviousETag:         previousETag,
		PreviousLastModified: previousLastModified,
		Context:              ctx,
		BypassCache:          uc.gCfg.MonitorConfig.BypassCache,
	}

	return uc.fetcher.FetchFileContent(fetchInput)
}

// generateContentDiff generates diff between old and new content
func (uc *URLChecker) generateContentDiff(lastRecord *models.FileHistoryRecord, newContent []byte, contentType, oldHash, newHash string) *models.ContentDiffResult {
	var oldContent []byte
	if lastRecord.Content != nil {
		oldContent = lastRecord.Content
	}

	diffResult, err := uc.contentDiffer.GenerateDiff(oldContent, newContent, contentType, oldHash, newHash)
	if err != nil {
		uc.logger.Error().Err(err).Msg("Failed to generate content diff")
		return nil
	}

	return diffResult
}

// generateFirstTimeDiff generates diff result for first-time monitoring, treating all content as insertion
func (uc *URLChecker) generateFirstTimeDiff(newContent []byte, contentType, newHash string) *models.ContentDiffResult {
	// For first time monitoring, treat empty content as old content
	emptyContent := []byte{}

	diffResult, err := uc.contentDiffer.GenerateDiff(emptyContent, newContent, contentType, "", newHash)
	if err != nil {
		uc.logger.Error().Err(err).Msg("Failed to generate first-time content diff")
		return nil
	}

	return diffResult
}

// shouldExtractPaths determines if paths should be extracted from the content type
func (uc *URLChecker) shouldExtractPaths(contentType string) bool {
	// Extract paths from JavaScript and HTML files
	return contentType == "application/javascript" ||
		contentType == "text/javascript" ||
		contentType == "text/html" ||
		contentType == "application/json"
}

// extractPathsWithOptimization extracts paths using memory-optimized approach
func (uc *URLChecker) extractPathsWithOptimization(sourceURL string, content []byte, contentType string) []models.ExtractedPath {
	// Use buffer pool for path extraction processing
	buffer := uc.bufferPool.Get()
	defer uc.bufferPool.Put(buffer)

	paths, err := uc.pathExtractor.ExtractPaths(sourceURL, content, contentType)
	if err != nil {
		uc.logger.Error().Err(err).Str("url", sourceURL).Msg("Failed to extract paths")
		return nil
	}

	return paths
}

// storeFileRecord stores the file record with extracted paths and diff results
func (uc *URLChecker) storeFileRecord(url string, result CheckResult, fetchResult *httpclient.FetchFileContentResult) error {
	record := models.FileHistoryRecord{
		URL:          url,
		Timestamp:    result.ProcessedAt.UnixMilli(),
		Hash:         result.NewHash,
		ContentType:  result.ContentType,
		ETag:         fetchResult.ETag,
		LastModified: fetchResult.LastModified,
	}

	// Store content if configured and changed
	if result.Changed && uc.gCfg.MonitorConfig.StoreFullContentOnChange {
		record.Content = result.Content
	}

	// Store diff result as JSON if available
	if result.DiffResult != nil {
		if diffJSON, err := uc.serializeDiffResult(result.DiffResult); err == nil {
			record.DiffResultJSON = &diffJSON
		} else {
			uc.logger.Warn().Err(err).Msg("Failed to serialize diff result")
		}
	}

	// Store extracted paths as JSON if available
	if len(result.ExtractedPaths) > 0 {
		if pathsJSON, err := uc.serializeExtractedPaths(result.ExtractedPaths); err == nil {
			record.ExtractedPathsJSON = &pathsJSON
		} else {
			uc.logger.Warn().Err(err).Msg("Failed to serialize extracted paths")
		}
	}

	return uc.historyStore.StoreFileRecord(record)
}

// serializeDiffResult serializes diff result to JSON string
func (uc *URLChecker) serializeDiffResult(diffResult *models.ContentDiffResult) (string, error) {
	if diffResult == nil {
		return "", nil
	}

	jsonData, err := json.Marshal(diffResult)
	if err != nil {
		return "", fmt.Errorf("failed to marshal diff result: %w", err)
	}

	return string(jsonData), nil
}

// serializeExtractedPaths serializes extracted paths to JSON string
func (uc *URLChecker) serializeExtractedPaths(paths []models.ExtractedPath) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	jsonData, err := json.Marshal(paths)
	if err != nil {
		return "", fmt.Errorf("failed to marshal extracted paths: %w", err)
	}

	return string(jsonData), nil
}

// CheckURLWithContext checks a URL with legacy result format for backwards compatibility
func (uc *URLChecker) CheckURLWithContext(ctx context.Context, url string, cycleID string) LegacyCheckResult {
	return uc.CheckURLWithBatchContext(ctx, url, cycleID, nil)
}

// CheckURLWithBatchContext checks a URL with batch context information
func (uc *URLChecker) CheckURLWithBatchContext(ctx context.Context, url string, cycleID string, batchInfo *models.BatchInfo) LegacyCheckResult {
	result := uc.CheckURL(ctx, url)

	if result.Error != nil {
		return LegacyCheckResult{
			ErrorInfo: &models.MonitorFetchErrorInfo{
				URL:        url,
				Error:      result.Error.Error(),
				Source:     "fetch",
				OccurredAt: result.ProcessedAt,
				CycleID:    cycleID,
				BatchInfo:  batchInfo,
			},
			Success: false,
		}
	}

	if !result.Changed {
		return LegacyCheckResult{Success: true}
	}

	// File changed - create change info
	changeInfo := &models.FileChangeInfo{
		URL:            url,
		OldHash:        result.OldHash,
		NewHash:        result.NewHash,
		ContentType:    result.ContentType,
		ChangeTime:     result.ProcessedAt,
		ExtractedPaths: result.ExtractedPaths,
		CycleID:        cycleID,
		BatchInfo:      batchInfo,
	}

	// Note: HTML diff report will be generated later at cycle level for all changed URLs
	// Individual URL diff results are stored in the file history

	return LegacyCheckResult{
		FileChangeInfo: changeInfo,
		Success:        true,
	}
}

// LegacyCheckResult represents the legacy result format for compatibility
type LegacyCheckResult struct {
	FileChangeInfo *models.FileChangeInfo
	ErrorInfo      *models.MonitorFetchErrorInfo
	Success        bool
}

// UpdateLogger updates the logger for this component
func (uc *URLChecker) UpdateLogger(newLogger zerolog.Logger) {
	uc.logger = newLogger.With().Str("component", "URLChecker").Logger()
}
