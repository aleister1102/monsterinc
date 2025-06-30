package summary

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
)

// ScanSummaryDataBuilder helps in constructing ScanSummaryData objects
type ScanSummaryDataBuilder struct {
	summary   ScanSummaryData
	validator *ScanSummaryValidator
}

// NewScanSummaryDataBuilder creates a new instance of ScanSummaryDataBuilder
func NewScanSummaryDataBuilder() *ScanSummaryDataBuilder {
	return &ScanSummaryDataBuilder{
		summary:   GetDefaultScanSummaryData(),
		validator: NewScanSummaryValidator(),
	}
}

// WithScanSessionID sets the ScanSessionID for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithScanSessionID(scanSessionID string) *ScanSummaryDataBuilder {
	b.summary.ScanSessionID = scanSessionID
	return b
}

// WithTargetSource sets the TargetSource for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithTargetSource(targetSource string) *ScanSummaryDataBuilder {
	b.summary.TargetSource = targetSource
	return b
}

// WithScanMode sets the ScanMode for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithScanMode(scanMode string) *ScanSummaryDataBuilder {
	b.summary.ScanMode = scanMode
	return b
}

// WithTargets sets the Targets for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithTargets(targets []string) *ScanSummaryDataBuilder {
	// Create a copy to avoid external modifications
	b.summary.Targets = make([]string, len(targets))
	copy(b.summary.Targets, targets)
	return b
}

// AddTarget adds a single target to the targets list
func (b *ScanSummaryDataBuilder) AddTarget(target string) *ScanSummaryDataBuilder {
	b.summary.Targets = append(b.summary.Targets, target)
	return b
}

// WithTotalTargets sets the TotalTargets for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithTotalTargets(totalTargets int) *ScanSummaryDataBuilder {
	b.summary.TotalTargets = totalTargets
	return b
}

// WithProbeStats sets the ProbeStats for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithProbeStats(probeStats ProbeStats) *ScanSummaryDataBuilder {
	b.summary.ProbeStats = probeStats
	return b
}

// WithDiffStats sets the DiffStats for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithDiffStats(diffStats DiffStats) *ScanSummaryDataBuilder {
	b.summary.DiffStats = diffStats
	return b
}

// WithScanDuration sets the ScanDuration for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithScanDuration(scanDuration time.Duration) *ScanSummaryDataBuilder {
	b.summary.ScanDuration = scanDuration
	return b
}

// WithReportPath sets the ReportPath for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithReportPath(reportPath string) *ScanSummaryDataBuilder {
	b.summary.ReportPath = reportPath
	return b
}

// WithStatus sets the Status for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithStatus(status ScanStatus) *ScanSummaryDataBuilder {
	b.summary.Status = string(status)
	return b
}

// WithErrorMessages sets error messages for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithErrorMessages(errorMessages []string) *ScanSummaryDataBuilder {
	// Create a copy to avoid external modifications
	b.summary.ErrorMessages = make([]string, len(errorMessages))
	copy(b.summary.ErrorMessages, errorMessages)
	return b
}

// WithRetriesAttempted sets the RetriesAttempted for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithRetriesAttempted(retriesAttempted int) *ScanSummaryDataBuilder {
	b.summary.RetriesAttempted = retriesAttempted
	return b
}

// Validate validates the current summary data
func (b *ScanSummaryDataBuilder) Validate() error {
	return b.validator.ValidateSummary(b.summary)
}

// Build returns the constructed ScanSummaryData object with validation
func (b *ScanSummaryDataBuilder) Build() (ScanSummaryData, error) {
	if err := b.Validate(); err != nil {
		return ScanSummaryData{}, errorwrapper.WrapError(err, "validation failed")
	}

	// Ensure total targets matches targets count if targets are provided
	if len(b.summary.Targets) > 0 && b.summary.TotalTargets == 0 {
		b.summary.TotalTargets = len(b.summary.Targets)
	}

	return b.summary, nil
}

// BuildUnsafe returns the constructed ScanSummaryData object without validation
func (b *ScanSummaryDataBuilder) BuildUnsafe() ScanSummaryData {
	return b.summary
}
