package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
