package models

import "time"

// ScanSummaryDataBuilder helps in constructing ScanSummaryData objects.
type ScanSummaryDataBuilder struct {
	summary ScanSummaryData
}

// NewScanSummaryDataBuilder creates a new instance of ScanSummaryDataBuilder.
// It initializes the ScanSummaryData with default values using GetDefaultScanSummaryData.
func NewScanSummaryDataBuilder() *ScanSummaryDataBuilder {
	return &ScanSummaryDataBuilder{
		summary: GetDefaultScanSummaryData(),
	}
}

// WithScanSessionID sets the ScanSessionID for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithScanSessionID(scanSessionID string) *ScanSummaryDataBuilder {
	b.summary.ScanSessionID = scanSessionID
	return b
}

// WithTargetSource sets the TargetSource for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithTargetSource(targetSource string) *ScanSummaryDataBuilder {
	b.summary.TargetSource = targetSource
	return b
}

// WithScanMode sets the ScanMode for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithScanMode(scanMode string) *ScanSummaryDataBuilder {
	b.summary.ScanMode = scanMode
	return b
}

// WithTargets sets the Targets for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithTargets(targets []string) *ScanSummaryDataBuilder {
	b.summary.Targets = targets
	return b
}

// WithTotalTargets sets the TotalTargets for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithTotalTargets(totalTargets int) *ScanSummaryDataBuilder {
	b.summary.TotalTargets = totalTargets
	return b
}

// WithProbeStats sets the ProbeStats for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithProbeStats(probeStats ProbeStats) *ScanSummaryDataBuilder {
	b.summary.ProbeStats = probeStats
	return b
}

// WithDiffStats sets the DiffStats for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithDiffStats(diffStats DiffStats) *ScanSummaryDataBuilder {
	b.summary.DiffStats = diffStats
	return b
}

// WithScanDuration sets the ScanDuration for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithScanDuration(scanDuration time.Duration) *ScanSummaryDataBuilder {
	b.summary.ScanDuration = scanDuration
	return b
}

// WithReportPath sets the ReportPath for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithReportPath(reportPath string) *ScanSummaryDataBuilder {
	b.summary.ReportPath = reportPath
	return b
}

// WithStatus sets the Status for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithStatus(status ScanStatus) *ScanSummaryDataBuilder {
	b.summary.Status = string(status)
	return b
}

// WithErrorMessages appends error messages to the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithErrorMessages(errorMessages []string) *ScanSummaryDataBuilder {
	b.summary.ErrorMessages = append(b.summary.ErrorMessages, errorMessages...)
	return b
}

// WithRetriesAttempted sets the RetriesAttempted for the ScanSummaryData.
func (b *ScanSummaryDataBuilder) WithRetriesAttempted(retriesAttempted int) *ScanSummaryDataBuilder {
	b.summary.RetriesAttempted = retriesAttempted
	return b
}

// Build returns the constructed ScanSummaryData object.
func (b *ScanSummaryDataBuilder) Build() ScanSummaryData {
	return b.summary
}
