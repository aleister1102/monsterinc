package reporter

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
)

// DiffUtils contains utility functions for diff processing
type DiffUtils struct{}

// NewDiffUtils creates a new DiffUtils
func NewDiffUtils() *DiffUtils {
	return &DiffUtils{}
}

// GenerateDiffHTML creates HTML representation of diffs
func (du *DiffUtils) GenerateDiffHTML(diffs []models.ContentDiff) template.HTML {
	var htmlBuilder strings.Builder

	// Check if this is a large single diff (like minified JS)
	isLargeContent := false
	totalLength := 0
	for _, d := range diffs {
		totalLength += len(d.Text)
	}

	// If content is very large and mostly in one operation, format it better
	if totalLength > 10000 && len(diffs) <= 3 {
		isLargeContent = true
	}

	for i, d := range diffs {
		// Escape HTML characters to prevent XSS and rendering issues
		escapedText := template.HTMLEscapeString(d.Text)

		// For large content, add line breaks for better readability
		if isLargeContent && len(escapedText) > 120 {
			escapedText = du.formatLargeContent(escapedText)
		}

		switch d.Operation {
		case models.DiffInsert:
			if isLargeContent {
				htmlBuilder.WriteString(fmt.Sprintf(`<div class="diff-line-insert"><ins style="background:#e6ffe6; text-decoration: none; display: block; padding: 2px 4px; margin: 1px 0;">%s</ins></div>`, escapedText))
			} else {
				htmlBuilder.WriteString(fmt.Sprintf(`<ins style="background:#e6ffe6; text-decoration: none;">%s</ins>`, escapedText))
			}
		case models.DiffDelete:
			if isLargeContent {
				htmlBuilder.WriteString(fmt.Sprintf(`<div class="diff-line-delete"><del style="background:#f8d7da; text-decoration: none; display: block; padding: 2px 4px; margin: 1px 0;">%s</del></div>`, escapedText))
			} else {
				htmlBuilder.WriteString(fmt.Sprintf(`<del style="background:#f8d7da; text-decoration: none;">%s</del>`, escapedText))
			}
		case models.DiffEqual:
			// For large equal content, truncate in middle to show context
			if isLargeContent && len(escapedText) > 200 {
				start := escapedText[:100]
				end := escapedText[len(escapedText)-100:]
				truncated := fmt.Sprintf(`%s<span style="color: #666; font-style: italic;">... [%d characters truncated] ...</span>%s`,
					start, len(escapedText)-200, end)
				htmlBuilder.WriteString(truncated)
			} else {
				htmlBuilder.WriteString(escapedText)
			}
		}

		// Add spacing between diffs for readability
		if isLargeContent && i < len(diffs)-1 {
			htmlBuilder.WriteString("\n")
		}
	}
	return template.HTML(htmlBuilder.String())
}

// formatLargeContent formats large content by adding line breaks for readability
func (du *DiffUtils) formatLargeContent(content string) string {
	// Don't format if content already has line breaks
	if strings.Contains(content, "\n") {
		return content
	}

	// For very long single lines (like minified JS), add breaks at logical points
	var result strings.Builder
	chunkSize := 120

	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := content[i:end]
		result.WriteString(chunk)

		// Add line break if not at the end
		if end < len(content) {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// CreateDiffSummary creates text summary of diff
func (du *DiffUtils) CreateDiffSummary(diffs []models.ContentDiff) string {
	insertions := 0
	deletions := 0
	for _, d := range diffs {
		switch d.Operation {
		case models.DiffInsert:
			insertions++
		case models.DiffDelete:
			deletions++
		}
	}
	if insertions == 0 && deletions == 0 {
		return "No textual changes detected."
	}
	return fmt.Sprintf("%d insertions (+), %d deletions (-).", insertions, deletions)
}

// TruncateHash truncates hash for shorter display
func (du *DiffUtils) TruncateHash(hash string) string {
	if len(hash) <= HashLength {
		return hash
	}
	return hash[:HashLength]
}

// checkFileSizeAndSplit checks if file exceeds Discord size limit and splits if necessary
func (r *HtmlDiffReporter) checkFileSizeAndSplit(filePath string, displayResults []models.DiffResultDisplay, cycleID string) ([]string, error) {
	const maxDiscordFileSize = 10 * 1024 * 1024 // 10MB in bytes

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return []string{filePath}, nil // Return original file if can't get size
	}

	if fileInfo.Size() <= maxDiscordFileSize {
		return []string{filePath}, nil // File is within limit
	}

	r.logger.Warn().
		Str("file_path", filePath).
		Float64("file_size_mb", float64(fileInfo.Size())/(1024*1024)).
		Msg("Report file exceeds Discord size limit, splitting into smaller files")

	// Remove the original oversized file
	if err := os.Remove(filePath); err != nil {
		r.logger.Warn().Err(err).Str("file_path", filePath).Msg("Failed to remove oversized file")
	}

	// Calculate split parameters with safety margin
	avgSizePerResult := fileInfo.Size() / int64(len(displayResults))

	// For client-side templates, use different safety margin since JSON is more compact
	templateName := r.selectOptimalTemplate(models.DiffReportPageData{DiffResults: displayResults})
	var safetyMargin float64
	if strings.Contains(templateName, "client_side") {
		safetyMargin = 0.7 // 70% for client-side (JSON is more compact)
		r.logger.Info().Msg("Using client-side template for splitting - more aggressive limits")
	} else {
		safetyMargin = 0.5 // 50% for server-side (full HTML)
		r.logger.Info().Msg("Using server-side template for splitting - conservative limits")
	}

	safeDiscordLimit := int64(float64(maxDiscordFileSize) * safetyMargin)
	maxResultsPerFile := int(safeDiscordLimit / avgSizePerResult)

	// Ensure minimum viable split (at least 1 result per file, max 10 results for very large files)
	if maxResultsPerFile < 1 {
		maxResultsPerFile = 1
	} else if maxResultsPerFile > 10 && avgSizePerResult > 500*1024 { // If avg > 500KB, limit to 10 items
		maxResultsPerFile = 10
	}

	r.logger.Info().
		Int64("avg_size_per_result", avgSizePerResult).
		Int64("safe_limit_bytes", safeDiscordLimit).
		Int("max_results_per_file", maxResultsPerFile).
		Int("total_results", len(displayResults)).
		Float64("safety_margin", safetyMargin).
		Str("template_type", templateName).
		Msg("Calculated file splitting parameters with optimized safety margin")

	// Generate chunked reports with iterative size checking and aggressive splitting
	return r.generateChunkedReportsWithSizeCheck(displayResults, cycleID, maxResultsPerFile)
}

// generateChunkedReportsWithSizeCheck generates chunked reports and checks each file size
func (r *HtmlDiffReporter) generateChunkedReportsWithSizeCheck(displayResults []models.DiffResultDisplay, cycleID string, initialMaxResults int) ([]string, error) {
	const maxDiscordFileSize = 10 * 1024 * 1024 // 10MB in bytes
	var allOutputPaths []string

	maxResultsPerFile := initialMaxResults
	remainingResults := displayResults
	partNum := 1

	for len(remainingResults) > 0 {
		// Determine chunk size for this iteration
		chunkSize := maxResultsPerFile
		if chunkSize > len(remainingResults) {
			chunkSize = len(remainingResults)
		}

		chunk := remainingResults[:chunkSize]
		partInfo := fmt.Sprintf("Part %d", partNum) // Will update total later

		pageData := r.createAggregatedPageData(chunk, partInfo)
		outputPath := r.buildOutputPath(cycleID, partNum, 999, true) // Use 999 as placeholder

		if err := r.directoryMgr.EnsureOutputDirectories(filepath.Dir(outputPath)); err != nil {
			return allOutputPaths, fmt.Errorf("failed to ensure output directory for chunk %d: %w", partNum, err)
		}

		reportPath, err := r.writeReportToFile(pageData, outputPath)
		if err != nil {
			return allOutputPaths, fmt.Errorf("failed to write chunk %d: %w", partNum, err)
		}

		// Check file size after generation
		fileInfo, err := os.Stat(reportPath)
		if err != nil {
			return allOutputPaths, fmt.Errorf("failed to check size of generated file %s: %w", reportPath, err)
		}

		if fileInfo.Size() > maxDiscordFileSize {
			// File is still too big, split further
			r.logger.Warn().
				Str("file_path", reportPath).
				Float64("file_size_mb", float64(fileInfo.Size())/(1024*1024)).
				Int("chunk_size", chunkSize).
				Msg("Generated file still exceeds limit, splitting further")

			// Remove the oversized file
			if err := os.Remove(reportPath); err != nil {
				r.logger.Warn().Err(err).Str("file_path", reportPath).Msg("Failed to remove oversized chunk")
			}

			// Reduce chunk size very aggressively and try again
			newMaxResults := chunkSize / 3 // More aggressive than /2
			if newMaxResults < 1 {
				newMaxResults = 1
			}

			r.logger.Info().
				Int("old_chunk_size", chunkSize).
				Int("new_chunk_size", newMaxResults).
				Msg("Reducing chunk size for next iteration")

			maxResultsPerFile = newMaxResults
			continue // Retry with smaller chunk
		}

		// File size is acceptable
		allOutputPaths = append(allOutputPaths, reportPath)
		remainingResults = remainingResults[chunkSize:]
		partNum++

		r.logger.Info().
			Str("report_path", reportPath).
			Float64("file_size_mb", float64(fileInfo.Size())/(1024*1024)).
			Int("chunk_size", chunkSize).
			Int("remaining_results", len(remainingResults)).
			Msg("Successfully generated report chunk within size limit")
	}

	// Update part info in filenames to reflect actual total
	totalParts := len(allOutputPaths)
	for i, oldPath := range allOutputPaths {
		newPath := r.buildOutputPath(cycleID, i+1, totalParts, true)
		if oldPath != newPath {
			if err := os.Rename(oldPath, newPath); err != nil {
				r.logger.Warn().Err(err).Str("old_path", oldPath).Str("new_path", newPath).Msg("Failed to rename report file")
			} else {
				allOutputPaths[i] = newPath
			}
		}
	}

	return allOutputPaths, nil
}
