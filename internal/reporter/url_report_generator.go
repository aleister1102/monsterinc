package reporter

import (
	"fmt"

	"github.com/aleister1102/monsterinc/internal/models"
	httpx "github.com/aleister1102/go-telescope"
)

// GenerateReport generates HTML reports from probe results and secret findings
func (r *HtmlReporter) GenerateReport(probeResults []*httpx.ProbeResult, secretFindings []models.SecretFinding, baseOutputPath string) ([]string, error) {
	if len(probeResults) == 0 && len(secretFindings) == 0 {
		r.logger.Warn().Msg("No probe results or secret findings provided for report generation.")
		return []string{}, nil
	}

	if baseOutputPath == "" {
		baseOutputPath = "report"
	}

	maxResults := r.cfg.MaxProbeResultsPerReportFile

	// If maxResults is 0, it means no limit - generate single report with all results
	if maxResults == 0 {
		return r.generateSingleReport(probeResults, secretFindings, baseOutputPath)
	}

	// If maxResults is negative, use default value
	if maxResults < 0 {
		maxResults = DefaultMaxResultsPerFile
	}

	if len(probeResults) <= maxResults {
		return r.generateSingleReport(probeResults, secretFindings, baseOutputPath)
	}

	return r.generateChunkedReports(probeResults, secretFindings, baseOutputPath, maxResults)
}

// generateSingleReport creates a single HTML report file
func (r *HtmlReporter) generateSingleReport(probeResults []*httpx.ProbeResult, secretFindings []models.SecretFinding, baseOutputPath string) ([]string, error) {
	pageData, err := r.prepareReportData(probeResults, secretFindings, "")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare report data: %w", err)
	}

	outputPath := r.buildOutputPath(baseOutputPath, 0, 1)
	if err := r.executeAndWriteReport(*pageData, outputPath); err != nil {
		return nil, fmt.Errorf("failed to write report: %w", err)
	}

	return []string{outputPath}, nil
}

// generateChunkedReports creates multiple HTML report files for large result sets
func (r *HtmlReporter) generateChunkedReports(probeResults []*httpx.ProbeResult, secretFindings []models.SecretFinding, baseOutputPath string, maxResults int) ([]string, error) {
	totalChunks := (len(probeResults) + maxResults - 1) / maxResults
	outputPaths := make([]string, 0, totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * maxResults
		end := start + maxResults
		if end > len(probeResults) {
			end = len(probeResults)
		}

		chunk := probeResults[start:end]
		partInfo := fmt.Sprintf("Part %d of %d", i+1, totalChunks)

		// Secret findings are passed whole to each chunk, as they are not paginated.
		pageData, err := r.prepareReportData(chunk, secretFindings, partInfo)
		if err != nil {
			return outputPaths, fmt.Errorf("failed to prepare data for chunk %d: %w", i+1, err)
		}

		outputPath := r.buildOutputPath(baseOutputPath, i+1, totalChunks)
		if err := r.executeAndWriteReport(*pageData, outputPath); err != nil {
			return outputPaths, fmt.Errorf("failed to write chunk %d: %w", i+1, err)
		}

		outputPaths = append(outputPaths, outputPath)
	}

	return outputPaths, nil
}
