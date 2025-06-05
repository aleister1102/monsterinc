package reporter

import (
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

//go:embed assets/img/favicon.ico
var faviconICODiff []byte

// FileHistoryStore defines an interface for accessing file history records.
// This avoids a direct dependency on the concrete ParquetFileHistoryStore and facilitates testing.
type FileHistoryStore interface {
	GetAllRecordsWithDiff() ([]*models.FileHistoryRecord, error)
	GetAllLatestDiffResultsForURLs(urls []string) (map[string]*models.ContentDiffResult, error)
	// GetLatestRecordsWithDiffForHost(host string) ([]*models.FileHistoryRecord, error) //  Potentially more granular
}

// HtmlDiffReporter creates HTML reports for content differences (refactored version)
type HtmlDiffReporter struct {
	logger       zerolog.Logger
	historyStore FileHistoryStore
	template     *template.Template
	assetManager *AssetManager
	directoryMgr *DirectoryManager
	diffUtils    *DiffUtils
}

// NewHtmlDiffReporter creates a new instance of NewHtmlDiffReporter
func NewHtmlDiffReporter(logger zerolog.Logger, historyStore FileHistoryStore) (*HtmlDiffReporter, error) {
	if historyStore == nil {
		logger.Warn().Msg("HistoryStore is nil in NewHtmlDiffReporter. Aggregated reports will not be available.")
	}

	reporter := &HtmlDiffReporter{
		logger:       logger,
		historyStore: historyStore,
		assetManager: NewAssetManager(logger),
		directoryMgr: NewDirectoryManager(logger),
		diffUtils:    NewDiffUtils(),
	}

	if err := reporter.initializeDirectories(); err != nil {
		return nil, err
	}

	if err := reporter.initializeTemplate(); err != nil {
		return nil, err
	}

	if err := reporter.copyAssets(); err != nil {
		logger.Warn().Err(err).Msg("Failed to copy assets for HTML diff reporter")
	}

	return reporter, nil
}

// initializeDirectories initializes required directories
func (r *HtmlDiffReporter) initializeDirectories() error {
	r.directoryMgr.LogWorkingDirectory(DefaultDiffReportDir)
	return r.directoryMgr.EnsureDiffReportDirectories()
}

// initializeTemplate initializes template with functions
func (r *HtmlDiffReporter) initializeTemplate() error {
	tmpl, err := template.New("").Funcs(GetDiffTemplateFunctions()).ParseFS(templateFS, "templates/diff_report.html.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse HTML diff template: %w", err)
	}

	r.logger.Info().Str("defined_templates", tmpl.DefinedTemplates()).Msg("HTML diff template parsed successfully")
	r.template = tmpl
	return nil
}

// copyAssets copies embedded assets to assets directory
func (r *HtmlDiffReporter) copyAssets() error {
	return r.assetManager.CopyEmbedDir(assetsFS, "assets", DefaultDiffReportAssetsDir)
}

// GenerateDiffReport creates HTML report for multiple URLs
func (r *HtmlDiffReporter) GenerateDiffReport(monitoredURLs []string, cycleID string) (string, error) {
	r.logger.Info().
		Strs("monitored_urls", monitoredURLs).
		Int("monitored_count", len(monitoredURLs)).
		Str("cycle_id", cycleID).
		Msg("Generating aggregated HTML diff report for monitored URLs")

	if r.historyStore == nil {
		r.logger.Error().Msg("HistoryStore is not available in HtmlDiffReporter")
		return "", errors.New("historyStore is not configured for HtmlDiffReporter")
	}

	diffResults, err := r.fetchLatestDiffResults(monitoredURLs)
	if err != nil {
		return "", err
	}

	displayResults := r.processDiffResults(diffResults)
	if len(displayResults) == 0 {
		r.logger.Info().Msg("No relevant (non-identical) diffs found for monitored URLs")
		return "", nil
	}

	return r.generateReport(displayResults, cycleID, true)
}

// GenerateSingleDiffReport creates HTML report for single diff
func (r *HtmlDiffReporter) GenerateSingleDiffReport(urlStr string, diffResult *models.ContentDiffResult, oldHash, newHash string, currentContent []byte) (string, error) {
	if diffResult == nil {
		r.logger.Warn().Str("url", urlStr).Msg("Received nil diffResult")
		return "", errors.New("diffResult is nil")
	}

	r.logger.Info().Str("url", urlStr).Msg("Generating single HTML diff report")

	displayDiff := r.createSingleDiffDisplay(urlStr, diffResult, currentContent)
	reportPath := r.generateSingleReportPath(urlStr, diffResult)

	pageData := r.createSingleDiffPageData(displayDiff, urlStr)
	return r.writeReportToFile(pageData, reportPath)
}

// DeleteAllSingleDiffReports deletes all single diff reports
func (r *HtmlDiffReporter) DeleteAllSingleDiffReports() error {
	r.logger.Info().Str("diff_report_dir", DefaultDiffReportDir).Msg("Starting deletion of all single diff reports")

	files, err := os.ReadDir(DefaultDiffReportDir)
	if err != nil {
		r.logger.Error().Err(err).Str("dir", DefaultDiffReportDir).Msg("Failed to read diff report directory")
		return fmt.Errorf("failed to read diff report directory %s: %w", DefaultDiffReportDir, err)
	}

	return r.deleteSingleDiffFiles(files)
}

// fetchLatestDiffResults retrieves latest diff results from history store
func (r *HtmlDiffReporter) fetchLatestDiffResults(monitoredURLs []string) (map[string]*models.ContentDiffResult, error) {
	latestDiffsMap, err := r.historyStore.GetAllLatestDiffResultsForURLs(monitoredURLs)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to get latest diff results from history store")
		return nil, fmt.Errorf("failed to get latest diff results: %w", err)
	}

	r.logger.Info().
		Int("diff_results_retrieved", len(latestDiffsMap)).
		Int("monitored_urls_requested", len(monitoredURLs)).
		Msg("Retrieved latest diff results from history store")

	return latestDiffsMap, nil
}

// processDiffResults processes diff results into display format
func (r *HtmlDiffReporter) processDiffResults(latestDiffsMap map[string]*models.ContentDiffResult) []models.DiffResultDisplay {
	var diffResultsDisplay []models.DiffResultDisplay

	for url, diffResult := range latestDiffsMap {
		if diffResult == nil || diffResult.IsIdentical {
			continue
		}

		var summary string
		var diffHTML template.HTML
		if len(diffResult.Diffs) > 0 {
			// Regular diff case (including new files with content diffs)
			summary = r.diffUtils.CreateDiffSummary(diffResult.Diffs)
			diffHTML = r.diffUtils.GenerateDiffHTML(diffResult.Diffs)
		} else if diffResult.OldHash == "" {
			// New empty file case
			summary = "New empty file detected"
			diffHTML = template.HTML("<div class='new-file-notice'>✨ This is a newly discovered empty file.</div>")
		} else {
			// Other cases where no diffs available
			summary = "Changes detected but no diff available"
			diffHTML = template.HTML("<div class='no-diff-notice'>⚠️ Changes were detected but diff information is not available.</div>")
		}

		display := models.DiffResultDisplay{
			URL:            url,
			Timestamp:      time.UnixMilli(diffResult.Timestamp),
			ContentType:    diffResult.ContentType,
			OldHash:        diffResult.OldHash,
			NewHash:        diffResult.NewHash,
			Summary:        summary,
			DiffHTML:       diffHTML,
			Diffs:          diffResult.Diffs,
			IsIdentical:    diffResult.IsIdentical,
			ErrorMessage:   diffResult.ErrorMessage,
			ExtractedPaths: diffResult.ExtractedPaths,
		}
		diffResultsDisplay = append(diffResultsDisplay, display)
	}

	// Sort by URL for consistent output
	sort.Slice(diffResultsDisplay, func(i, j int) bool {
		return diffResultsDisplay[i].URL < diffResultsDisplay[j].URL
	})

	return diffResultsDisplay
}

// generateReport creates and writes report file
func (r *HtmlDiffReporter) generateReport(displayResults []models.DiffResultDisplay, cycleID string, isAggregated bool) (string, error) {
	outputFilePath := r.generateReportPath(cycleID, isAggregated)

	if err := r.directoryMgr.EnsureOutputDirectories(filepath.Dir(outputFilePath)); err != nil {
		return "", err
	}

	pageData := r.createAggregatedPageData(displayResults)
	return r.writeReportToFile(pageData, outputFilePath)
}

// generateReportPath creates path for report file
func (r *HtmlDiffReporter) generateReportPath(cycleID string, isAggregated bool) string {
	var filename string
	if isAggregated && cycleID != "" {
		// Extract timestamp from cycleID format: monitor-init-20241231-161213 or monitor-20241231-161213
		timestamp := time.Now().Format("20060102-150405") // fallback to current time

		// Split by '-' and look for timestamp parts
		if strings.Contains(cycleID, "-") {
			parts := strings.Split(cycleID, "-")
			// Look for the last two parts that could form YYYYMMDD-HHMMSS
			if len(parts) >= 2 {
				lastTwoParts := parts[len(parts)-2:]
				if len(lastTwoParts) == 2 &&
					len(lastTwoParts[0]) == 8 && len(lastTwoParts[1]) == 6 {
					// Validate that these look like date and time
					if _, err := time.Parse("20060102", lastTwoParts[0]); err == nil {
						if _, err := time.Parse("150405", lastTwoParts[1]); err == nil {
							timestamp = lastTwoParts[0] + "-" + lastTwoParts[1]
						}
					}
				}
			}
		}
		filename = fmt.Sprintf("%s_monitor_report.html", timestamp)
	} else {
		filename = "aggregated_diff_report.html"
	}
	return filepath.Join(DefaultDiffReportDir, filename)
}

// generateSingleReportPath creates path for single diff report
func (r *HtmlDiffReporter) generateSingleReportPath(urlStr string, diffResult *models.ContentDiffResult) string {
	sanitizedURL := urlhandler.SanitizeFilename(urlStr)
	var reportFilename string

	if diffResult.OldHash != "" && diffResult.NewHash != "" {
		oldHashTrunc := r.diffUtils.TruncateHash(diffResult.OldHash)
		newHashTrunc := r.diffUtils.TruncateHash(diffResult.NewHash)
		reportFilename = fmt.Sprintf("diff_%s_%s_vs_%s_%d.html", sanitizedURL, oldHashTrunc, newHashTrunc, diffResult.Timestamp)
	} else {
		reportFilename = fmt.Sprintf("diff_%s_%d.html", sanitizedURL, diffResult.Timestamp)
	}

	return filepath.Join(DefaultDiffReportDir, reportFilename)
}

// createAggregatedPageData creates page data for aggregated report
func (r *HtmlDiffReporter) createAggregatedPageData(displayResults []models.DiffResultDisplay) models.DiffReportPageData {
	pageData := models.DiffReportPageData{
		ReportTitle: DefaultDiffReportTitle,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
		DiffResults: displayResults,
		TotalDiffs:  len(displayResults),
		ReportType:  "aggregated",
	}

	// Set favicon base64 data
	if len(faviconICODiff) > 0 {
		pageData.FaviconBase64 = base64.StdEncoding.EncodeToString(faviconICODiff)
	}

	return pageData
}

// createSingleDiffDisplay creates display data for single diff
func (r *HtmlDiffReporter) createSingleDiffDisplay(urlStr string, diffResult *models.ContentDiffResult, currentContent []byte) models.DiffResultDisplay {
	return models.DiffResultDisplay{
		URL:            urlStr,
		Timestamp:      time.UnixMilli(diffResult.Timestamp),
		ContentType:    diffResult.ContentType,
		OldHash:        diffResult.OldHash,
		NewHash:        diffResult.NewHash,
		Summary:        r.diffUtils.CreateDiffSummary(diffResult.Diffs),
		DiffHTML:       r.diffUtils.GenerateDiffHTML(diffResult.Diffs),
		Diffs:          diffResult.Diffs,
		IsIdentical:    diffResult.IsIdentical,
		ErrorMessage:   diffResult.ErrorMessage,
		FullContent:    string(currentContent),
		ExtractedPaths: diffResult.ExtractedPaths,
	}
}

// createSingleDiffPageData creates page data for single diff report
func (r *HtmlDiffReporter) createSingleDiffPageData(displayDiff models.DiffResultDisplay, urlStr string) models.DiffReportPageData {
	pageData := models.DiffReportPageData{
		ReportTitle: fmt.Sprintf("Content Diff Report: %s", urlStr),
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
		DiffResults: []models.DiffResultDisplay{displayDiff},
		TotalDiffs:  1,
		ReportType:  "single",
	}

	// Set favicon base64 data
	if len(faviconICODiff) > 0 {
		pageData.FaviconBase64 = base64.StdEncoding.EncodeToString(faviconICODiff)
	}

	return pageData
}

// writeReportToFile writes page data to file
func (r *HtmlDiffReporter) writeReportToFile(pageData models.DiffReportPageData, outputFilePath string) (string, error) {
	file, err := os.Create(outputFilePath)
	if err != nil {
		r.logger.Error().Err(err).Str("path", outputFilePath).Msg("Failed to create diff report file")
		return "", fmt.Errorf("failed to create file %s: %w", outputFilePath, err)
	}
	defer file.Close()

	if err := r.template.ExecuteTemplate(file, "diff_report.html.tmpl", pageData); err != nil {
		r.logger.Error().Err(err).Msg("Failed to execute template for diff report")
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	r.logger.Info().Str("path", outputFilePath).Int("diff_count", len(pageData.DiffResults)).Msg("Successfully generated HTML diff report")
	return outputFilePath, nil
}

// deleteSingleDiffFiles deletes single diff files
func (r *HtmlDiffReporter) deleteSingleDiffFiles(files []os.DirEntry) error {
	deletedCount := 0
	var deletionErrors []string

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()

		// Skip aggregated reports
		if strings.Contains(fileName, "aggregated-report") {
			r.logger.Debug().Str("file", fileName).Msg("Skipping aggregated report file")
			continue
		}

		// Only delete files with single diff report format
		if r.isSingleDiffFile(fileName) {
			if err := r.deleteSingleFile(fileName); err != nil {
				deletionErrors = append(deletionErrors, err.Error())
			} else {
				deletedCount++
			}
		}
	}

	r.logger.Info().
		Int("deleted_count", deletedCount).
		Int("error_count", len(deletionErrors)).
		Msg("Completed deletion of single diff reports")

	if len(deletionErrors) > 0 {
		return fmt.Errorf("encountered %d errors while deleting single diff reports: %s", len(deletionErrors), strings.Join(deletionErrors, "; "))
	}

	return nil
}

// isSingleDiffFile checks if file is a single diff file
func (r *HtmlDiffReporter) isSingleDiffFile(fileName string) bool {
	return strings.HasPrefix(fileName, "diff_") && strings.HasSuffix(fileName, ".html")
}

// deleteSingleFile deletes a single file
func (r *HtmlDiffReporter) deleteSingleFile(fileName string) error {
	filePath := filepath.Join(DefaultDiffReportDir, fileName)

	if err := os.Remove(filePath); err != nil {
		r.logger.Error().Err(err).Str("file", filePath).Msg("Failed to delete single diff report file")
		return fmt.Errorf("failed to delete %s: %v", fileName, err)
	}

	r.logger.Debug().Str("file", fileName).Msg("Successfully deleted single diff report file")
	return nil
}
