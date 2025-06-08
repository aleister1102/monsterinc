package reporter

import (
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// GenerateDiffReport creates HTML report for multiple URLs with paging/splitting support
func (r *HtmlDiffReporter) GenerateDiffReport(monitoredURLs []string, cycleID string) ([]string, error) {
	r.logger.Info().
		Strs("monitored_urls", monitoredURLs).
		Int("monitored_count", len(monitoredURLs)).
		Str("cycle_id", cycleID).
		Msg("Generating aggregated HTML diff report for monitored URLs")

	if r.historyStore == nil {
		r.logger.Error().Msg("HistoryStore is not available in HtmlDiffReporter")
		return nil, errors.New("historyStore is not configured for HtmlDiffReporter")
	}

	diffResults, err := r.fetchLatestDiffResults(monitoredURLs)
	if err != nil {
		return nil, err
	}

	displayResults := r.processDiffResults(diffResults)
	if len(displayResults) == 0 {
		return []string{}, nil
	}

	maxResults := r.getMaxDiffResultsPerFile()

	// If maxResults is 0, it means no limit - generate single report with all results
	if maxResults == 0 || len(displayResults) <= maxResults {
		reportPath, err := r.generateSingleReport(displayResults, cycleID, true)
		if err != nil {
			return nil, err
		}
		return []string{reportPath}, nil
	}

	return r.generateChunkedReports(displayResults, cycleID, maxResults)
}

// TODO: Check if this function is used
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

// createAggregatedPageData creates page data for aggregated report
func (r *HtmlDiffReporter) createAggregatedPageData(displayResults []models.DiffResultDisplay, partInfo string) models.DiffReportPageData {
	reportTitle := DefaultDiffReportTitle
	if partInfo != "" {
		reportTitle = fmt.Sprintf("%s - %s", DefaultDiffReportTitle, partInfo)
	}

	pageData := models.DiffReportPageData{
		ReportTitle:    reportTitle,
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05 MST"),
		DiffResults:    displayResults,
		TotalDiffs:     len(displayResults),
		ReportType:     "aggregated",
		ReportPartInfo: partInfo,
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
	defer func() {
		if err := file.Close(); err != nil {
			r.logger.Error().Err(err).Str("path", outputFilePath).Msg("Failed to close diff report file")
		}
	}()

	if err := r.template.ExecuteTemplate(file, "diff_report.html.tmpl", pageData); err != nil {
		r.logger.Error().Err(err).Msg("Failed to execute template for diff report")
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	r.logger.Info().Str("path", outputFilePath).Int("diff_count", len(pageData.DiffResults)).Msg("Successfully generated HTML diff report")
	return outputFilePath, nil
}

// generateSingleReport creates a single HTML diff report file
func (r *HtmlDiffReporter) generateSingleReport(displayResults []models.DiffResultDisplay, cycleID string, isAggregated bool) (string, error) {
	pageData := r.createAggregatedPageData(displayResults, "")
	outputFilePath := r.buildOutputPath(cycleID, 0, 1, isAggregated)

	if err := r.directoryMgr.EnsureOutputDirectories(filepath.Dir(outputFilePath)); err != nil {
		return "", err
	}

	return r.writeReportToFile(pageData, outputFilePath)
}

// generateChunkedReports creates multiple HTML diff report files for large result sets
func (r *HtmlDiffReporter) generateChunkedReports(displayResults []models.DiffResultDisplay, cycleID string, maxResults int) ([]string, error) {
	totalChunks := (len(displayResults) + maxResults - 1) / maxResults
	outputPaths := make([]string, 0, totalChunks)

	for i := range totalChunks {
		start := i * maxResults
		end := start + maxResults
		if end > len(displayResults) {
			end = len(displayResults)
		}

		chunk := displayResults[start:end]
		partInfo := fmt.Sprintf("Part %d of %d", i+1, totalChunks)

		pageData := r.createAggregatedPageData(chunk, partInfo)
		outputPath := r.buildOutputPath(cycleID, i+1, totalChunks, true)

		if err := r.directoryMgr.EnsureOutputDirectories(filepath.Dir(outputPath)); err != nil {
			return outputPaths, fmt.Errorf("failed to ensure output directory for chunk %d: %w", i+1, err)
		}

		reportPath, err := r.writeReportToFile(pageData, outputPath)
		if err != nil {
			return outputPaths, fmt.Errorf("failed to write chunk %d: %w", i+1, err)
		}

		outputPaths = append(outputPaths, reportPath)
	}

	return outputPaths, nil
}
