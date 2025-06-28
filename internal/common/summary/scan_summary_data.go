package summary

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/common/contextutils"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/rs/zerolog"
)

// Discord color constants for different types of notifications
const (
	DiscordColorSuccess   = 0x00ff00 // Green
	DiscordColorError     = 0xff0000 // Red
	DiscordColorWarning   = 0xffa500 // Orange
	DiscordColorInfo      = 0x0099ff // Blue
	DiscordColorDefault   = 0x36393f // Discord default gray
	DiscordColorCritical  = 0x8b0000 // Dark red
	DiscordColorCompleted = 0x228b22 // Forest green
)

// ScanSummaryData holds all relevant information about a scan to be used in notifications.
type ScanSummaryData struct {
	ScanSessionID    string        // Unique identifier for the scan session (e.g., YYYYMMDD-HHMMSS timestamp)
	TargetSource     string        // The source of the targets (e.g., file path, "config_input_urls")
	ScanMode         string        // Mode of the scan (e.g., "onetime", "automated")
	Targets          []string      // List of original target URLs/identifiers
	TotalTargets     int           // Total number of targets processed or attempted
	ProbeStats       ProbeStats    // Statistics from the probing phase
	DiffStats        DiffStats     // Statistics from the diffing phase (New, Old, Existing)
	ScanDuration     time.Duration // Total duration of the scan
	ReportPath       string        // Filesystem path to the generated report (used by notifier to attach)
	Status           string        // Overall status: "COMPLETED", "FAILED", "STARTED", "INTERRUPTED", "PARTIAL_COMPLETE"
	ErrorMessages    []string      // Any critical errors encountered during the scan
	Component        string        // Component where an error might have occurred (for critical errors)
	RetriesAttempted int           // Number of retries, if applicable
	CycleMinutes     int           // Cycle interval in minutes (only for automated mode)
}

// GetDefaultScanSummaryData initializes a ScanSummaryData with default/empty values.
func GetDefaultScanSummaryData() ScanSummaryData {
	return ScanSummaryData{
		ScanSessionID: "",
		TargetSource:  "Unknown",
		ScanMode:      "Unknown",
		Targets:       []string{},
		TotalTargets:  0,
		ProbeStats:    ProbeStats{},
		DiffStats:     DiffStats{},
		Status:        string(ScanStatusUnknown), // Default to unknown status
	}
}

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
