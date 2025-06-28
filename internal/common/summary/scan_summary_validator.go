package summary

import "github.com/aleister1102/monsterinc/internal/common/errorwrapper"

// ScanSummaryValidator handles validation of scan summary data
type ScanSummaryValidator struct{}

// NewScanSummaryValidator creates a new validator
func NewScanSummaryValidator() *ScanSummaryValidator {
	return &ScanSummaryValidator{}
}

// ValidateSummary validates the scan summary data
func (ssv *ScanSummaryValidator) ValidateSummary(summaryData ScanSummaryData) error {
	if summaryData.ScanSessionID == "" {
		return errorwrapper.NewValidationError("scan_session_id", summaryData.ScanSessionID, "scan session ID is required")
	}

	if summaryData.TargetSource == "" {
		return errorwrapper.NewValidationError("target_source", summaryData.TargetSource, "target source is required")
	}

	if summaryData.ScanMode == "" {
		return errorwrapper.NewValidationError("scan_mode", summaryData.ScanMode, "scan mode is required")
	}

	if summaryData.Status == "" {
		return errorwrapper.NewValidationError("status", summaryData.Status, "status is required")
	}

	if summaryData.TotalTargets < 0 {
		return errorwrapper.NewValidationError("total_targets", summaryData.TotalTargets, "total targets cannot be negative")
	}

	if summaryData.ProbeStats.TotalProbed < 0 {
		return errorwrapper.NewValidationError("total_probed", summaryData.ProbeStats.TotalProbed, "total probed cannot be negative")
	}

	if summaryData.ProbeStats.SuccessfulProbes < 0 {
		return errorwrapper.NewValidationError("successful_probes", summaryData.ProbeStats.SuccessfulProbes, "successful probes cannot be negative")
	}

	if summaryData.ProbeStats.FailedProbes < 0 {
		return errorwrapper.NewValidationError("failed_probes", summaryData.ProbeStats.FailedProbes, "failed probes cannot be negative")
	}

	if summaryData.ScanDuration < 0 {
		return errorwrapper.NewValidationError("scan_duration", summaryData.ScanDuration, "scan duration cannot be negative")
	}

	return nil
}
