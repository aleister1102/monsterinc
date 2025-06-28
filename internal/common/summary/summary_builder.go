package summary

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/common/contextutils"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
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
	ProbeResults    []httpxrunner.ProbeResult
	URLDiffResults  map[string]differ.URLDiffResult
	WorkflowError   error
	ErrorMessages   []string
	ReportFilePaths []string
}

// BuildSummary creates comprehensive scan summary from workflow results
// Follows single responsibility principle by focusing only on summary creation
func (sb *SummaryBuilder) BuildSummary(input *SummaryInput) ScanSummaryData {
	summary := GetDefaultScanSummaryData()

	// Set basic information
	summary.ScanSessionID = input.ScanSessionID
	summary.TargetSource = input.TargetSource
	summary.ScanMode = input.ScanMode
	summary.Targets = input.Targets
	summary.TotalTargets = len(input.Targets)

	// Calculate statistics in single pass
	sb.calculateStats(&summary, input.ProbeResults, input.URLDiffResults)

	// Set timing information
	if !input.StartTime.IsZero() {
		summary.ScanDuration = time.Since(input.StartTime)
	} else {
		summary.ScanDuration = 0
	}

	// Determine status and handle errors
	sb.determineStatus(&summary, input)

	// Set report information
	sb.setReportInfo(&summary, input.ReportFilePaths)

	return summary
}

// calculateStats calculates both probe and diff statistics efficiently
func (sb *SummaryBuilder) calculateStats(summary *ScanSummaryData, probeResults []httpxrunner.ProbeResult, urlDiffResults map[string]differ.URLDiffResult) {
	// Calculate probe stats
	totalProbed := len(probeResults)
	successCount := 0

	// Single pass through probe results
	for _, result := range probeResults {
		if result.Error == "" && result.StatusCode >= 200 && result.StatusCode < 400 {
			successCount++
		}
	}

	summary.ProbeStats = ProbeStats{
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

	summary.DiffStats = DiffStats{
		New:      totalNew,
		Old:      totalOld,
		Existing: totalExisting,
	}
}

// determineStatus determines the final scan status based on workflow results
func (sb *SummaryBuilder) determineStatus(summary *ScanSummaryData, input *SummaryInput) {
	// Handle error messages
	summary.ErrorMessages = append(summary.ErrorMessages, input.ErrorMessages...)

	// Determine status based on workflow error and context
	if input.WorkflowError != nil {
		if contextutils.ContainsCancellationError(summary.ErrorMessages) {
			summary.Status = string(ScanStatusInterrupted)
		} else {
			summary.Status = string(ScanStatusFailed)
			summary.ErrorMessages = append(summary.ErrorMessages, input.WorkflowError.Error())
		}
	} else {
		if len(summary.ErrorMessages) == 0 {
			summary.Status = string(ScanStatusCompleted)
		} else {
			summary.Status = string(ScanStatusPartialComplete)
		}
	}
}

// setReportInfo sets report-related information in summary
func (sb *SummaryBuilder) setReportInfo(summary *ScanSummaryData, reportFilePaths []string) {
	switch len(reportFilePaths) {
	case 0:
		summary.ReportPath = ""
	case 1:
		summary.ReportPath = reportFilePaths[0]
	default:
		summary.ReportPath = "Multiple report files generated"
	}
}
