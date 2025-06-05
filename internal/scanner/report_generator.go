package scanner

import (
	"fmt"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
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
	ProbeResults   []models.ProbeResult
	ScanSessionID  string
	ShouldGenerate bool // Determines whether to generate reports
}

// NewReportGenerationInput creates input for report generation
func NewReportGenerationInput(probeResults []models.ProbeResult, scanSessionID string) *ReportGenerationInput {
	return &ReportGenerationInput{
		ProbeResults:   probeResults,
		ScanSessionID:  scanSessionID,
		ShouldGenerate: true,
	}
}

// GenerateReports creates HTML reports from probe results
// Returns list of generated file paths or error if any
func (rg *ReportGenerator) GenerateReports(input *ReportGenerationInput) ([]string, error) {
	if !rg.shouldGenerateReport(input) {
		rg.logger.Info().
			Str("session_id", input.ScanSessionID).
			Msg("Report generation skipped - no results and GenerateEmptyReport is false")
		return nil, nil
	}

	htmlReporter, err := rg.createHTMLReporter()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize HTML reporter: %w", err)
	}

	baseReportPath := rg.buildBaseReportPath(input.ScanSessionID)

	// OPTIMIZATION: Direct pointer conversion to avoid intermediate allocation
	probeResultsPtr := rg.convertToPointersOptimized(input.ProbeResults)

	reportPaths, err := htmlReporter.GenerateReport(probeResultsPtr, baseReportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML report(s): %w", err)
	}

	rg.logReportGeneration(input.ScanSessionID, reportPaths)
	return reportPaths, nil
}

// shouldGenerateReport determines whether to generate reports
func (rg *ReportGenerator) shouldGenerateReport(input *ReportGenerationInput) bool {
	if !input.ShouldGenerate {
		return false
	}

	hasResults := len(input.ProbeResults) > 0
	shouldGenerateEmpty := rg.config.GenerateEmptyReport

	return hasResults || shouldGenerateEmpty
}

// createHTMLReporter creates HTML reporter instance
func (rg *ReportGenerator) createHTMLReporter() (*reporter.HtmlReporter, error) {
	return reporter.NewHtmlReporter(rg.config, rg.logger)
}

// buildBaseReportPath creates base path for report file
func (rg *ReportGenerator) buildBaseReportPath(scanSessionID string) string {
	baseReportFilename := fmt.Sprintf("%s_scan_report.html", scanSessionID)
	return filepath.Join(rg.config.OutputDir, baseReportFilename)
}

// convertToPointersOptimized converts ProbeResult slice to pointer slice efficiently
func (rg *ReportGenerator) convertToPointersOptimized(probeResults []models.ProbeResult) []*models.ProbeResult {
	if len(probeResults) == 0 {
		return nil
	}

	// Pre-allocate with exact capacity
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
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
