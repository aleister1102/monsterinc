package models

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
)

// ScanSummaryValidator handles validation of scan summary data
type ScanSummaryValidator struct{}

// NewScanSummaryValidator creates a new validator
func NewScanSummaryValidator() *ScanSummaryValidator {
	return &ScanSummaryValidator{}
}

// ValidateSummary validates the scan summary data
func (ssv *ScanSummaryValidator) ValidateSummary(summary ScanSummaryData) error {
	if summary.ScanSessionID == "" {
		return common.NewValidationError("scan_session_id", summary.ScanSessionID, "scan session ID cannot be empty")
	}

	if summary.TotalTargets < 0 {
		return common.NewValidationError("total_targets", summary.TotalTargets, "total targets cannot be negative")
	}

	if summary.ProbeStats.TotalProbed < 0 {
		return common.NewValidationError("total_probed", summary.ProbeStats.TotalProbed, "total probed cannot be negative")
	}

	if summary.ProbeStats.SuccessfulProbes < 0 {
		return common.NewValidationError("successful_probes", summary.ProbeStats.SuccessfulProbes, "successful probes cannot be negative")
	}

	if summary.ProbeStats.FailedProbes < 0 {
		return common.NewValidationError("failed_probes", summary.ProbeStats.FailedProbes, "failed probes cannot be negative")
	}

	if summary.ScanDuration < 0 {
		return common.NewValidationError("scan_duration", summary.ScanDuration, "scan duration cannot be negative")
	}

	return nil
}

// ProbeStatsBuilder handles building probe stats
type ProbeStatsBuilder struct {
	stats ProbeStats
}

// NewProbeStatsBuilder creates a new probe stats builder
func NewProbeStatsBuilder() *ProbeStatsBuilder {
	return &ProbeStatsBuilder{
		stats: ProbeStats{},
	}
}

// WithTotalProbed sets total probed count
func (psb *ProbeStatsBuilder) WithTotalProbed(total int) *ProbeStatsBuilder {
	psb.stats.TotalProbed = total
	return psb
}

// WithSuccessfulProbes sets successful probes count
func (psb *ProbeStatsBuilder) WithSuccessfulProbes(successful int) *ProbeStatsBuilder {
	psb.stats.SuccessfulProbes = successful
	return psb
}

// WithFailedProbes sets failed probes count
func (psb *ProbeStatsBuilder) WithFailedProbes(failed int) *ProbeStatsBuilder {
	psb.stats.FailedProbes = failed
	return psb
}

// WithDiscoverableItems sets discoverable items count
func (psb *ProbeStatsBuilder) WithDiscoverableItems(items int) *ProbeStatsBuilder {
	psb.stats.DiscoverableItems = items
	return psb
}

// Build returns the constructed probe stats
func (psb *ProbeStatsBuilder) Build() ProbeStats {
	return psb.stats
}

// DiffStatsBuilder handles building diff stats
type DiffStatsBuilder struct {
	stats DiffStats
}

// NewDiffStatsBuilder creates a new diff stats builder
func NewDiffStatsBuilder() *DiffStatsBuilder {
	return &DiffStatsBuilder{
		stats: DiffStats{},
	}
}

// WithNew sets new count
func (dsb *DiffStatsBuilder) WithNew(count int) *DiffStatsBuilder {
	dsb.stats.New = count
	return dsb
}

// WithOld sets old count
func (dsb *DiffStatsBuilder) WithOld(count int) *DiffStatsBuilder {
	dsb.stats.Old = count
	return dsb
}

// WithExisting sets existing count
func (dsb *DiffStatsBuilder) WithExisting(count int) *DiffStatsBuilder {
	dsb.stats.Existing = count
	return dsb
}

// WithChanged sets changed count
func (dsb *DiffStatsBuilder) WithChanged(count int) *DiffStatsBuilder {
	dsb.stats.Changed = count
	return dsb
}

// Build returns the constructed diff stats
func (dsb *DiffStatsBuilder) Build() DiffStats {
	return dsb.stats
}

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

// WithProbeStatsBuilder uses ProbeStatsBuilder to set probe stats
func (b *ScanSummaryDataBuilder) WithProbeStatsBuilder(builder *ProbeStatsBuilder) *ScanSummaryDataBuilder {
	b.summary.ProbeStats = builder.Build()
	return b
}

// WithDiffStats sets the DiffStats for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithDiffStats(diffStats DiffStats) *ScanSummaryDataBuilder {
	b.summary.DiffStats = diffStats
	return b
}

// WithDiffStatsBuilder uses DiffStatsBuilder to set diff stats
func (b *ScanSummaryDataBuilder) WithDiffStatsBuilder(builder *DiffStatsBuilder) *ScanSummaryDataBuilder {
	b.summary.DiffStats = builder.Build()
	return b
}

// WithScanDuration sets the ScanDuration for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithScanDuration(scanDuration time.Duration) *ScanSummaryDataBuilder {
	b.summary.ScanDuration = scanDuration
	return b
}

// WithScanDurationMs sets scan duration in milliseconds
func (b *ScanSummaryDataBuilder) WithScanDurationMs(durationMs int64) *ScanSummaryDataBuilder {
	b.summary.ScanDuration = time.Duration(durationMs) * time.Millisecond
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

// WithStringStatus sets status as string
func (b *ScanSummaryDataBuilder) WithStringStatus(status string) *ScanSummaryDataBuilder {
	b.summary.Status = status
	return b
}

// WithErrorMessages sets error messages for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithErrorMessages(errorMessages []string) *ScanSummaryDataBuilder {
	// Create a copy to avoid external modifications
	b.summary.ErrorMessages = make([]string, len(errorMessages))
	copy(b.summary.ErrorMessages, errorMessages)
	return b
}

// AddErrorMessage adds a single error message
func (b *ScanSummaryDataBuilder) AddErrorMessage(errorMessage string) *ScanSummaryDataBuilder {
	b.summary.ErrorMessages = append(b.summary.ErrorMessages, errorMessage)
	return b
}

// WithComponent sets the component field
func (b *ScanSummaryDataBuilder) WithComponent(component string) *ScanSummaryDataBuilder {
	b.summary.Component = component
	return b
}

// WithRetriesAttempted sets the RetriesAttempted for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithRetriesAttempted(retriesAttempted int) *ScanSummaryDataBuilder {
	b.summary.RetriesAttempted = retriesAttempted
	return b
}

// WithCycleMinutes sets the CycleMinutes for the ScanSummaryData
func (b *ScanSummaryDataBuilder) WithCycleMinutes(cycleMinutes int) *ScanSummaryDataBuilder {
	b.summary.CycleMinutes = cycleMinutes
	return b
}

// Reset resets the builder to default state
func (b *ScanSummaryDataBuilder) Reset() *ScanSummaryDataBuilder {
	b.summary = GetDefaultScanSummaryData()
	return b
}

// Clone creates a copy of the current builder
func (b *ScanSummaryDataBuilder) Clone() *ScanSummaryDataBuilder {
	newBuilder := NewScanSummaryDataBuilder()
	newBuilder.summary = b.summary

	// Deep copy slices
	if len(b.summary.Targets) > 0 {
		newBuilder.summary.Targets = make([]string, len(b.summary.Targets))
		copy(newBuilder.summary.Targets, b.summary.Targets)
	}

	if len(b.summary.ErrorMessages) > 0 {
		newBuilder.summary.ErrorMessages = make([]string, len(b.summary.ErrorMessages))
		copy(newBuilder.summary.ErrorMessages, b.summary.ErrorMessages)
	}

	return newBuilder
}

// Validate validates the current summary data
func (b *ScanSummaryDataBuilder) Validate() error {
	return b.validator.ValidateSummary(b.summary)
}

// Build returns the constructed ScanSummaryData object with validation
func (b *ScanSummaryDataBuilder) Build() (ScanSummaryData, error) {
	if err := b.Validate(); err != nil {
		return ScanSummaryData{}, common.WrapError(err, "validation failed")
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
