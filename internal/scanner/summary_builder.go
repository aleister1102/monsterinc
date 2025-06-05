package scanner

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// SummaryBuilder handles the creation of scan summary data
// Encapsulates the logic for building comprehensive scan summaries
type SummaryBuilder struct {
	logger zerolog.Logger
}

// NewSummaryBuilder creates a new SummaryBuilder instance
func NewSummaryBuilder(logger zerolog.Logger) *SummaryBuilder {
	return &SummaryBuilder{
		logger: logger.With().Str("module", "SummaryBuilder").Logger(),
	}
}

// SummaryInput contains data needed to build scan summary
type SummaryInput struct {
	ScanSessionID   string
	TargetSource    string
	ScanMode        string
	Targets         []string
	StartTime       time.Time
	ProbeResults    []models.ProbeResult
	URLDiffResults  map[string]models.URLDiffResult
	WorkflowError   error
	ErrorMessages   []string
	ReportFilePaths []string
}

// BuildSummary creates comprehensive scan summary from workflow results
// Follows single responsibility principle by focusing only on summary creation
func (sb *SummaryBuilder) BuildSummary(input *SummaryInput) models.ScanSummaryData {
	summary := models.GetDefaultScanSummaryData()

	// Set basic information
	summary.ScanSessionID = input.ScanSessionID
	summary.TargetSource = input.TargetSource
	summary.ScanMode = input.ScanMode
	summary.Targets = input.Targets
	summary.TotalTargets = len(input.Targets)

	// Calculate statistics in single pass
	sb.calculateStats(&summary, input.ProbeResults, input.URLDiffResults)

	// Set timing information
	summary.ScanDuration = time.Since(input.StartTime)

	// Determine status and handle errors
	sb.determineStatus(&summary, input)

	// Set report information
	sb.setReportInfo(&summary, input.ReportFilePaths)

	return summary
}

// calculateStats calculates both probe and diff statistics efficiently
func (sb *SummaryBuilder) calculateStats(summary *models.ScanSummaryData, probeResults []models.ProbeResult, urlDiffResults map[string]models.URLDiffResult) {
	// Calculate probe stats
	totalProbed := len(probeResults)
	successCount := 0

	// Single pass through probe results
	for _, result := range probeResults {
		if result.Error == "" && result.StatusCode >= 200 && result.StatusCode < 400 {
			successCount++
		}
	}

	summary.ProbeStats = models.ProbeStats{
		TotalProbed:       totalProbed,
		SuccessfulProbes:  successCount,
		FailedProbes:      totalProbed - successCount,
		DiscoverableItems: totalProbed,
	}

	// Calculate diff stats efficiently
	var totalNew, totalOld, totalExisting int
	for _, diffResult := range urlDiffResults {
		totalNew += diffResult.New
		totalOld += diffResult.Old
		totalExisting += diffResult.Existing
	}

	summary.DiffStats = models.DiffStats{
		New:      totalNew,
		Old:      totalOld,
		Existing: totalExisting,
	}
}

// determineStatus determines the final scan status based on workflow results
func (sb *SummaryBuilder) determineStatus(summary *models.ScanSummaryData, input *SummaryInput) {
	// Handle error messages
	summary.ErrorMessages = append(summary.ErrorMessages, input.ErrorMessages...)

	// Determine status based on workflow error and context
	if input.WorkflowError != nil {
		if common.ContainsCancellationError(summary.ErrorMessages) {
			summary.Status = string(models.ScanStatusInterrupted)
		} else {
			summary.Status = string(models.ScanStatusFailed)
			summary.ErrorMessages = append(summary.ErrorMessages, input.WorkflowError.Error())
		}
	} else {
		if len(summary.ErrorMessages) == 0 {
			summary.Status = string(models.ScanStatusCompleted)
		} else {
			summary.Status = string(models.ScanStatusPartialComplete)
		}
	}
}

// setReportInfo sets report-related information in summary
func (sb *SummaryBuilder) setReportInfo(summary *models.ScanSummaryData, reportFilePaths []string) {
	switch len(reportFilePaths) {
	case 0:
		summary.ReportPath = ""
	case 1:
		summary.ReportPath = reportFilePaths[0]
	default:
		summary.ReportPath = "Multiple report files generated"
	}
}
