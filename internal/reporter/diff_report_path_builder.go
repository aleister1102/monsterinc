package reporter

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

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

// buildOutputPath constructs the output file path with paging support
func (r *HtmlDiffReporter) buildOutputPath(cycleID string, partNum, totalParts int, isAggregated bool) string {
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

		if totalParts == 1 {
			filename = fmt.Sprintf("%s_monitor_report.html", timestamp)
		} else {
			filename = fmt.Sprintf("%s_monitor_report-part%d.html", timestamp, partNum)
		}
	} else {
		if totalParts == 1 {
			filename = "aggregated_diff_report.html"
		} else {
			filename = fmt.Sprintf("aggregated_diff_report-part%d.html", partNum)
		}
	}

	return filepath.Join(DefaultDiffReportDir, filename)
}
