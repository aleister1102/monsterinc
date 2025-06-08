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
	for _, d := range diffs {
		// Escape HTML characters to prevent XSS and rendering issues
		escapedText := template.HTMLEscapeString(d.Text)

		switch d.Operation {
		case models.DiffInsert:
			htmlBuilder.WriteString(fmt.Sprintf(`<ins style="background:#e6ffe6; text-decoration: none;">%s</ins>`, escapedText))
		case models.DiffDelete:
			htmlBuilder.WriteString(fmt.Sprintf(`<del style="background:#f8d7da; text-decoration: none;">%s</del>`, escapedText))
		case models.DiffEqual:
			htmlBuilder.WriteString(escapedText)
		}
	}
	return template.HTML(htmlBuilder.String())
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
		r.logger.Warn().Err(err).Str("file_path", filePath).Msg("Failed to remove oversized report file")
	}

	// Split results into smaller chunks based on estimated size per result
	avgSizePerResult := fileInfo.Size() / int64(len(displayResults))
	maxResultsPerFile := int(maxDiscordFileSize / avgSizePerResult * 80 / 100) // Use 80% of limit as safety margin

	if maxResultsPerFile <= 0 {
		maxResultsPerFile = 1 // Ensure at least 1 result per file
	}

	r.logger.Info().
		Int("avg_size_per_result", int(avgSizePerResult)).
		Int("max_results_per_file", maxResultsPerFile).
		Msg("Calculated split parameters for oversized report")

	// Generate chunked reports
	totalChunks := (len(displayResults) + maxResultsPerFile - 1) / maxResultsPerFile
	var allOutputPaths []string

	for i := range totalChunks {
		start := i * maxResultsPerFile
		end := start + maxResultsPerFile
		if end > len(displayResults) {
			end = len(displayResults)
		}

		chunk := displayResults[start:end]
		partInfo := fmt.Sprintf("Part %d of %d", i+1, totalChunks)

		pageData := r.createAggregatedPageData(chunk, partInfo)
		outputPath := r.buildOutputPath(cycleID, i+1, totalChunks, true)

		if err := r.directoryMgr.EnsureOutputDirectories(filepath.Dir(outputPath)); err != nil {
			return allOutputPaths, fmt.Errorf("failed to ensure output directory for chunk %d: %w", i+1, err)
		}

		reportPath, err := r.writeReportToFile(pageData, outputPath)
		if err != nil {
			return allOutputPaths, fmt.Errorf("failed to write chunk %d: %w", i+1, err)
		}

		allOutputPaths = append(allOutputPaths, reportPath)
	}

	return allOutputPaths, nil
}
