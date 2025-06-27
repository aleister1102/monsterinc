package reporter

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// GenerateDiffReport creates HTML report for multiple URLs with automatic file size-based splitting
func (r *HtmlDiffReporter) GenerateDiffReport(changedURLs []string, cycleID string) ([]string, error) {
	r.logger.Info().
		Strs("changed_urls", changedURLs).
		Int("changed_count", len(changedURLs)).
		Str("cycle_id", cycleID).
		Msg("Generating aggregated HTML diff report for changed URLs only")

	if r.historyStore == nil {
		r.logger.Error().Msg("HistoryStore is not available in HtmlDiffReporter")
		return nil, errors.New("historyStore is not configured for HtmlDiffReporter")
	}

	// Fetch only changed URLs instead of all monitored URLs
	diffResults, err := r.fetchLatestDiffResults(changedURLs)
	if err != nil {
		return nil, err
	}

	displayResults := r.processDiffResults(diffResults)
	if len(displayResults) == 0 {
		return []string{}, nil
	}

	// Always generate single report first and check file size for automatic splitting
	reportPath, err := r.generateSingleReport(displayResults, cycleID, true)
	if err != nil {
		return nil, err
	}

	// Check file size and split if necessary
	return r.checkFileSizeAndSplit(reportPath, displayResults, cycleID)
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
func (r *HtmlDiffReporter) fetchLatestDiffResults(changedURLs []string) (map[string]*models.ContentDiffResult, error) {
	latestDiffsMap, err := r.historyStore.GetAllLatestDiffResultsForURLs(changedURLs)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to get latest diff results from history store")
		return nil, fmt.Errorf("failed to get latest diff results: %w", err)
	}

	r.logger.Info().
		Int("diff_results_retrieved", len(latestDiffsMap)).
		Int("changed_urls_requested", len(changedURLs)).
		Msg("Retrieved latest diff results from history store for changed URLs only")

	return latestDiffsMap, nil
}

// processDiffResults processes diff results into display format
func (r *HtmlDiffReporter) processDiffResults(latestDiffsMap map[string]*models.ContentDiffResult) []models.DiffResultDisplay {
	var diffResultsDisplay []models.DiffResultDisplay
	skippedIdentical := 0

	for url, diffResult := range latestDiffsMap {
		if diffResult == nil { // Added nil check for diffResult itself
			r.logger.Warn().Str("url", url).Msg("Skipping nil diffResult in processDiffResults")
			continue
		}
		// Skip URLs that have no changes (identical content)
		if diffResult.IsIdentical {
			skippedIdentical++
			continue
		}

		var summary string
		var diffHTML template.HTML
		var displayTimestamp time.Time

		if diffResult.Timestamp > 0 {
			displayTimestamp = time.UnixMilli(diffResult.Timestamp)
		} else {
			displayTimestamp = time.Now() // Default to current time if timestamp is invalid
			r.logger.Warn().Str("url", url).Int64("timestamp", diffResult.Timestamp).Msg("Invalid or zero timestamp for diffResult, using current time")
		}

		if len(diffResult.Diffs) > 0 {
			summary = r.diffUtils.CreateDiffSummary(diffResult.Diffs)
			diffHTML = r.diffUtils.GenerateDiffHTML(diffResult.Diffs)
			if summary == "No textual changes detected." { // Be more explicit if diffs are present but summary is generic
				summary = "Content changed (see details)"
			}
		} else if diffResult.OldHash == "" {
			summary = "New file detected"
			diffHTML = template.HTML("<div class='new-file-notice'>✨ This is a newly discovered file. Full content should be available.</div>")
		} else {
			summary = "Changes detected but no detailed diff available"
			if diffResult.ErrorMessage != "" {
				summary = fmt.Sprintf("Error during diff: %s", diffResult.ErrorMessage)
				diffHTML = template.HTML(fmt.Sprintf("<div class='error-notice'>⚠️ Error generating diff: %s</div>", template.HTMLEscapeString(diffResult.ErrorMessage)))
			} else {
				diffHTML = template.HTML("<div class='no-diff-notice'>⚠️ Changes were detected but detailed diff information is not available. This could be due to file type or size.</div>")
			}
		}

		// Ensure summary is never empty if changes are present
		if summary == "" {
			summary = "Summary not available"
		}

		display := models.DiffResultDisplay{
			URL:          url,
			Timestamp:    displayTimestamp,
			ContentType:  diffResult.ContentType,
			OldHash:      diffResult.OldHash,
			NewHash:      diffResult.NewHash,
			Summary:      summary,
			DiffHTML:     diffHTML,
			Diffs:        diffResult.Diffs,
			IsIdentical:  diffResult.IsIdentical,
			ErrorMessage: diffResult.ErrorMessage,
		}
		diffResultsDisplay = append(diffResultsDisplay, display)
	}

	// Sort by URL for consistent output
	sort.Slice(diffResultsDisplay, func(i, j int) bool {
		return diffResultsDisplay[i].URL < diffResultsDisplay[j].URL
	})

	r.logger.Info().
		Int("total_diff_results", len(latestDiffsMap)).
		Int("changed_urls_included", len(diffResultsDisplay)).
		Int("identical_urls_skipped", skippedIdentical).
		Msg("Processed diff results - including only URLs with changes or new URLs")

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

	// Create JSON data for client-side rendering
	r.populateDiffResultsJSON(&pageData)

	return pageData
}

// createSingleDiffDisplay creates display data for single diff
func (r *HtmlDiffReporter) createSingleDiffDisplay(urlStr string, diffResult *models.ContentDiffResult, currentContent []byte) models.DiffResultDisplay {
	return models.DiffResultDisplay{
		URL:          urlStr,
		Timestamp:    time.UnixMilli(diffResult.Timestamp),
		ContentType:  diffResult.ContentType,
		OldHash:      diffResult.OldHash,
		NewHash:      diffResult.NewHash,
		Summary:      r.diffUtils.CreateDiffSummary(diffResult.Diffs),
		DiffHTML:     r.diffUtils.GenerateDiffHTML(diffResult.Diffs),
		Diffs:        diffResult.Diffs,
		IsIdentical:  diffResult.IsIdentical,
		ErrorMessage: diffResult.ErrorMessage,
		FullContent:  string(currentContent),
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

	// Create JSON data for client-side rendering
	r.populateDiffResultsJSON(&pageData)

	return pageData
}

// writeReportToFile writes page data to file
func (r *HtmlDiffReporter) writeReportToFile(pageData models.DiffReportPageData, outputFilePath string) (string, error) {
	// Embed assets into page data if configured
	if r.assetManager != nil {
		r.assetManager.EmbedAssetsIntoPageDataWithPaths(&pageData, assetsFS, assetsFS, EmbeddedDiffCSSPath, EmbeddedDiffJSPath, true)
	}

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

	// Choose template based on expected file size - use client-side template for large reports
	templateName := r.selectOptimalTemplate(pageData)

	if err := r.template.ExecuteTemplate(file, templateName, pageData); err != nil {
		r.logger.Error().Err(err).Str("template", templateName).Msg("Failed to execute template for diff report")
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	r.logger.Info().
		Str("path", outputFilePath).
		Str("template", templateName).
		Int("diff_count", len(pageData.DiffResults)).
		Msg("Successfully generated HTML diff report")
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

// populateDiffResultsJSON creates JSON representation of diff results for client-side rendering
func (r *HtmlDiffReporter) populateDiffResultsJSON(pageData *models.DiffReportPageData) {
	// Create a simplified version of diff results for JSON serialization
	type SimplifiedDiffResult struct {
		URL          string               `json:"url"`
		ContentType  string               `json:"content_type"`
		Timestamp    time.Time            `json:"timestamp"`
		IsIdentical  bool                 `json:"is_identical"`
		ErrorMessage string               `json:"error_message"`
		Diffs        []models.ContentDiff `json:"diffs"`
		OldHash      string               `json:"old_hash"`
		NewHash      string               `json:"new_hash"`
		Summary      string               `json:"summary"`
	}

	// Convert DiffResultDisplay to simplified version
	var simplifiedResults []SimplifiedDiffResult
	for _, result := range pageData.DiffResults {
		simplified := SimplifiedDiffResult{
			URL:          result.URL,
			ContentType:  result.ContentType,
			Timestamp:    result.Timestamp,
			IsIdentical:  result.IsIdentical,
			ErrorMessage: result.ErrorMessage,
			Diffs:        result.Diffs, // Use raw diffs instead of pre-rendered HTML
			OldHash:      result.OldHash,
			NewHash:      result.NewHash,
			Summary:      result.Summary,
		}
		simplifiedResults = append(simplifiedResults, simplified)
	}

	// Serialize to JSON
	if jsonData, err := json.Marshal(simplifiedResults); err != nil {
		r.logger.Error().Err(err).Msg("Failed to marshal diff results to JSON")
		pageData.DiffResultsJSON = template.JS("[]") // Fallback to empty array
	} else {
		pageData.DiffResultsJSON = template.JS(jsonData)
	}
}

// getTemplateName chooses between client-side and server-side templates based on data size
func (r *HtmlDiffReporter) getTemplateName(useClientSide bool) string {
	return "diff_report_client_side.html.tmpl"
}

// selectOptimalTemplate chooses between client-side and server-side templates based on data size
func (r *HtmlDiffReporter) selectOptimalTemplate(pageData models.DiffReportPageData) string {
	// Estimate the size impact of diff content
	totalDiffSize := 0
	for _, result := range pageData.DiffResults {
		totalDiffSize += len(string(result.DiffHTML))
		totalDiffSize += len(result.Summary)
		totalDiffSize += len(result.ErrorMessage)
	}

	// Use client-side template if:
	// 1. More than 15 diff results OR (reduced from 20 for better file size optimization)
	// 2. Total diff content size > 80KB OR (reduced from 100KB)
	// 3. Individual result has very large diff content OR
	// 4. This is a multi-part report (splitting scenario)
	useClientSide := false

	if len(pageData.DiffResults) > 15 {
		useClientSide = true
		r.logger.Info().
			Int("diff_count", len(pageData.DiffResults)).
			Msg("Using client-side template due to number of diffs")
	} else if totalDiffSize > 80*1024 { // 80KB
		useClientSide = true
		r.logger.Info().
			Int("total_diff_size_bytes", totalDiffSize).
			Msg("Using client-side template due to large diff content size")
	} else if strings.Contains(pageData.ReportPartInfo, "Part") {
		// Multi-part reports benefit from client-side rendering
		useClientSide = true
		r.logger.Info().
			Str("part_info", pageData.ReportPartInfo).
			Msg("Using client-side template for multi-part report")
	} else {
		// Check for any individual very large diff
		for _, result := range pageData.DiffResults {
			if len(string(result.DiffHTML)) > 40*1024 { // 40KB for single diff (reduced from 50KB)
				useClientSide = true
				r.logger.Info().
					Str("url", result.URL).
					Int("diff_size_bytes", len(string(result.DiffHTML))).
					Msg("Using client-side template due to large individual diff")
				break
			}
		}
	}

	return r.getTemplateName(useClientSide)
}
