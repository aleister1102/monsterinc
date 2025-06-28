package scanner

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/rs/zerolog"
)

// ReportGenerator is responsible for creating HTML reports from probe results
// Separates report generation logic from main workflow according to single responsibility principle
type ReportGenerator struct {
	config *config.ReporterConfig
	logger zerolog.Logger
}

// NewReportGenerator creates a new ReportGenerator instance
func NewReportGenerator(config *config.ReporterConfig, logger zerolog.Logger) *ReportGenerator {
	return &ReportGenerator{
		config: config,
		logger: logger.With().Str("module", "ReportGenerator").Logger(),
	}
}

// ReportGenerationInput contains information needed to generate reports
type ReportGenerationInput struct {
	ProbeResults   []httpxrunner.ProbeResult
	URLDiffResults map[string]differ.URLDiffResult
	ScanSessionID  string
}

// NewReportGenerationInput creates input for report generation
func NewReportGenerationInput(probeResults []httpxrunner.ProbeResult, scanSessionID string) *ReportGenerationInput {
	return &ReportGenerationInput{
		ProbeResults:  probeResults,
		ScanSessionID: scanSessionID,
	}
}

// NewReportGenerationInputWithDiff creates input for report generation including URL diff results
func NewReportGenerationInputWithDiff(probeResults []httpxrunner.ProbeResult, urlDiffResults map[string]differ.URLDiffResult, scanSessionID string) *ReportGenerationInput {
	return &ReportGenerationInput{
		ProbeResults:   probeResults,
		URLDiffResults: urlDiffResults,
		ScanSessionID:  scanSessionID,
	}
}

// GenerateReports creates HTML reports from probe results
// Returns list of generated file paths or error if any
func (rg *ReportGenerator) GenerateReports(ctx context.Context, input *ReportGenerationInput) ([]string, error) {
	rg.logger.Info().
		Int("probe_results", len(input.ProbeResults)).
		Int("diff_results", len(input.URLDiffResults)).
		Msg("Starting report generation")

	if len(input.ProbeResults) == 0 {
		rg.logger.Info().Msg("No results found, skipping report generation.")
		return nil, nil
	}

	reporter, err := rg.createHTMLReporter()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize HTML reporter: %w", err)
	}

	baseReportPath := rg.buildBaseReportPath(input.ScanSessionID)

	// Combine current scan results with old URLs from diff results
	allProbeResults := rg.combineProbeResultsWithOldURLs(input.ProbeResults, input.URLDiffResults)

	// OPTIMIZATION: Direct pointer conversion to avoid intermediate allocation
	probeResultsPtr := rg.convertToPointersOptimized(allProbeResults)

	reportPaths, err := reporter.GenerateReport(probeResultsPtr, baseReportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML report(s): %w", err)
	}

	rg.logReportGeneration(input.ScanSessionID, reportPaths)
	return reportPaths, nil
}

// createHTMLReporter creates and initializes a new HTML reporter
func (rg *ReportGenerator) createHTMLReporter() (*reporter.HtmlReporter, error) {
	return reporter.NewHtmlReporter(rg.config, rg.logger)
}

// buildBaseReportPath creates base path for report file
func (rg *ReportGenerator) buildBaseReportPath(scanSessionID string) string {
	baseReportFilename := fmt.Sprintf("%s_scan_report.html", scanSessionID)
	return filepath.Join(rg.config.OutputDir, baseReportFilename)
}

// convertToPointersOptimized converts ProbeResult slice to pointer slice efficiently
func (rg *ReportGenerator) convertToPointersOptimized(probeResults []httpxrunner.ProbeResult) []*httpxrunner.ProbeResult {
	if len(probeResults) == 0 {
		return nil
	}

	// Pre-allocate with exact capacity
	probeResultsPtr := make([]*httpxrunner.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}
	return probeResultsPtr
}

// logReportGeneration logs report generation results
func (rg *ReportGenerator) logReportGeneration(scanSessionID string, reportPaths []string) {
	if len(reportPaths) == 0 {
		rg.logger.Info().
			Str("session_id", scanSessionID).
			Msg("HTML report generation resulted in no files")
		return
	}

	rg.logger.Info().
		Str("session_id", scanSessionID).
		Strs("paths", reportPaths).
		Msg("HTML report(s) generated successfully")
}

// combineProbeResultsWithOldURLs combines current scan results with old URLs from diff results
func (rg *ReportGenerator) combineProbeResultsWithOldURLs(probeResults []httpxrunner.ProbeResult, urlDiffResults map[string]differ.URLDiffResult) []httpxrunner.ProbeResult {
	if len(urlDiffResults) == 0 {
		return probeResults
	}
	// Calculate total capacity for pre-allocation
	totalOldResults := 0
	for _, urlDiffResult := range urlDiffResults {
		for _, diffedURL := range urlDiffResult.Results {
			if diffedURL.ProbeResult.URLStatus == string(models.StatusOld) {
				totalOldResults++
			}
		}
	}

	allProbeResults := make([]httpxrunner.ProbeResult, 0, len(probeResults)+totalOldResults)
	allProbeResults = append(allProbeResults, probeResults...)

	// Add old URLs from diff results
	for _, urlDiffResult := range urlDiffResults {
		for _, diffedURL := range urlDiffResult.Results {
			if diffedURL.ProbeResult.URLStatus == string(models.StatusOld) {
				allProbeResults = append(allProbeResults, diffedURL.ProbeResult)
			}
		}
	}

	return allProbeResults
}

func (rg *ReportGenerator) getReportTimestamp(probeResults []httpxrunner.ProbeResult) string {
	if len(probeResults) == 0 {
		return ""
	}
	// Implementation of getReportTimestamp method
	return ""
}

func (rg *ReportGenerator) getReportRootURL(probeResults []httpxrunner.ProbeResult) string {
	if len(probeResults) > 0 {
		return probeResults[0].RootTargetURL
	}
	return ""
}
