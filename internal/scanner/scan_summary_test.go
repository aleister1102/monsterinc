package scanner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanSummaryValidator_ValidateSummary(t *testing.T) {
	validator := NewScanSummaryValidator()

	tests := []struct {
		name        string
		summary     ScanSummaryData
		expectError bool
	}{
		{
			name: "valid summary",
			summary: ScanSummaryData{
				ScanSessionID: "20240101-120000",
				TargetSource:  "config_input_urls",
				ScanMode:      "onetime",
				Targets:       []string{"https://example.com"},
				TotalTargets:  1,
				Status:        "COMPLETED",
			},
			expectError: false,
		},
		{
			name: "empty scan session ID",
			summary: ScanSummaryData{
				ScanSessionID: "",
				TargetSource:  "config_input_urls",
				ScanMode:      "onetime",
				Targets:       []string{"https://example.com"},
				TotalTargets:  1,
				Status:        "COMPLETED",
			},
			expectError: true,
		},
		{
			name: "empty target source",
			summary: ScanSummaryData{
				ScanSessionID: "20240101-120000",
				TargetSource:  "",
				ScanMode:      "onetime",
				Targets:       []string{"https://example.com"},
				TotalTargets:  1,
				Status:        "COMPLETED",
			},
			expectError: true,
		},
		{
			name: "empty scan mode",
			summary: ScanSummaryData{
				ScanSessionID: "20240101-120000",
				TargetSource:  "config_input_urls",
				ScanMode:      "",
				Targets:       []string{"https://example.com"},
				TotalTargets:  1,
				Status:        "COMPLETED",
			},
			expectError: true,
		},
		{
			name: "empty status",
			summary: ScanSummaryData{
				ScanSessionID: "20240101-120000",
				TargetSource:  "config_input_urls",
				ScanMode:      "onetime",
				Targets:       []string{"https://example.com"},
				TotalTargets:  1,
				Status:        "",
			},
			expectError: true,
		},
		{
			name: "negative total targets",
			summary: ScanSummaryData{
				ScanSessionID: "20240101-120000",
				TargetSource:  "config_input_urls",
				ScanMode:      "onetime",
				Targets:       []string{"https://example.com"},
				TotalTargets:  -1,
				Status:        "COMPLETED",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSummary(tt.summary)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProbeStatsBuilder(t *testing.T) {
	builder := NewProbeStatsBuilder()

	stats := builder.
		WithTotalProbed(100).
		WithSuccessfulProbes(85).
		WithFailedProbes(15).
		WithDiscoverableItems(200).
		Build()

	assert.Equal(t, 100, stats.TotalProbed)
	assert.Equal(t, 85, stats.SuccessfulProbes)
	assert.Equal(t, 15, stats.FailedProbes)
	assert.Equal(t, 200, stats.DiscoverableItems)
}

func TestProbeStatsBuilder_DefaultValues(t *testing.T) {
	builder := NewProbeStatsBuilder()
	stats := builder.Build()

	assert.Equal(t, 0, stats.TotalProbed)
	assert.Equal(t, 0, stats.SuccessfulProbes)
	assert.Equal(t, 0, stats.FailedProbes)
	assert.Equal(t, 0, stats.DiscoverableItems)
}

func TestDiffStatsBuilder(t *testing.T) {
	builder := NewDiffStatsBuilder()

	stats := builder.
		WithNew(25).
		WithOld(10).
		WithExisting(65).
		WithChanged(5).
		Build()

	assert.Equal(t, 25, stats.New)
	assert.Equal(t, 10, stats.Old)
	assert.Equal(t, 65, stats.Existing)
	assert.Equal(t, 5, stats.Changed)
}

func TestDiffStatsBuilder_DefaultValues(t *testing.T) {
	builder := NewDiffStatsBuilder()
	stats := builder.Build()

	assert.Equal(t, 0, stats.New)
	assert.Equal(t, 0, stats.Old)
	assert.Equal(t, 0, stats.Existing)
	assert.Equal(t, 0, stats.Changed)
}

func TestScanSummaryDataBuilder_Complete(t *testing.T) {
	probeStats := NewProbeStatsBuilder().
		WithTotalProbed(100).
		WithSuccessfulProbes(90).
		WithFailedProbes(10).
		WithDiscoverableItems(150).
		Build()

	diffStats := NewDiffStatsBuilder().
		WithNew(30).
		WithOld(20).
		WithExisting(50).
		WithChanged(10).
		Build()

	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("config_input_urls").
		WithScanMode("onetime").
		WithTargets([]string{"https://example.com", "https://test.com"}).
		WithTotalTargets(2).
		WithProbeStats(probeStats).
		WithDiffStats(diffStats).
		WithScanDuration(5 * time.Minute).
		WithReportPath("/tmp/report.html").
		WithStatus(ScanStatusCompleted).
		WithErrorMessages([]string{}).
		WithComponent("scanner").
		WithRetriesAttempted(0).
		WithCycleMinutes(60).
		Build()

	require.NoError(t, err)

	assert.Equal(t, "20240101-120000", summary.ScanSessionID)
	assert.Equal(t, "config_input_urls", summary.TargetSource)
	assert.Equal(t, "onetime", summary.ScanMode)
	assert.Equal(t, []string{"https://example.com", "https://test.com"}, summary.Targets)
	assert.Equal(t, 2, summary.TotalTargets)
	assert.Equal(t, probeStats, summary.ProbeStats)
	assert.Equal(t, diffStats, summary.DiffStats)
	assert.Equal(t, 5*time.Minute, summary.ScanDuration)
	assert.Equal(t, "/tmp/report.html", summary.ReportPath)
	assert.Equal(t, string(ScanStatusCompleted), summary.Status)
	assert.Empty(t, summary.ErrorMessages)
	assert.Equal(t, "scanner", summary.Component)
	assert.Equal(t, 0, summary.RetriesAttempted)
	assert.Equal(t, 60, summary.CycleMinutes)
}

func TestScanSummaryDataBuilder_WithProbeStatsBuilder(t *testing.T) {
	probeStatsBuilder := NewProbeStatsBuilder().
		WithTotalProbed(50).
		WithSuccessfulProbes(45).
		WithFailedProbes(5)

	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("automated").
		WithProbeStatsBuilder(probeStatsBuilder).
		WithStatus(ScanStatusCompleted).
		Build()

	require.NoError(t, err)

	assert.Equal(t, 50, summary.ProbeStats.TotalProbed)
	assert.Equal(t, 45, summary.ProbeStats.SuccessfulProbes)
	assert.Equal(t, 5, summary.ProbeStats.FailedProbes)
}

func TestScanSummaryDataBuilder_WithDiffStatsBuilder(t *testing.T) {
	diffStatsBuilder := NewDiffStatsBuilder().
		WithNew(15).
		WithExisting(35).
		WithOld(5)

	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("automated").
		WithDiffStatsBuilder(diffStatsBuilder).
		WithStatus(ScanStatusCompleted).
		Build()

	require.NoError(t, err)

	assert.Equal(t, 15, summary.DiffStats.New)
	assert.Equal(t, 35, summary.DiffStats.Existing)
	assert.Equal(t, 5, summary.DiffStats.Old)
}

func TestScanSummaryDataBuilder_AddTarget(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("manual").
		WithScanMode("onetime").
		AddTarget("https://example.com").
		AddTarget("https://test.com").
		AddTarget("https://demo.com").
		WithStatus(ScanStatusCompleted).
		Build()

	require.NoError(t, err)

	expectedTargets := []string{"https://example.com", "https://test.com", "https://demo.com"}
	assert.Equal(t, expectedTargets, summary.Targets)
}

func TestScanSummaryDataBuilder_AddErrorMessage(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("onetime").
		AddErrorMessage("Network timeout").
		AddErrorMessage("DNS resolution failed").
		WithStatus(ScanStatusFailed).
		Build()

	require.NoError(t, err)

	expectedErrors := []string{"Network timeout", "DNS resolution failed"}
	assert.Equal(t, expectedErrors, summary.ErrorMessages)
}

func TestScanSummaryDataBuilder_WithScanDurationMs(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("onetime").
		WithScanDurationMs(120000). // 2 minutes in milliseconds
		WithStatus(ScanStatusCompleted).
		Build()

	require.NoError(t, err)

	assert.Equal(t, 120*time.Second, summary.ScanDuration)
}

func TestScanSummaryDataBuilder_WithStringStatus(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	summary, err := builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("onetime").
		WithStringStatus("COMPLETED").
		Build()

	require.NoError(t, err)

	assert.Equal(t, "COMPLETED", summary.Status)
}

func TestScanSummaryDataBuilder_Reset(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	// Build first summary
	builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("onetime").
		AddTarget("https://example.com").
		WithStatus(ScanStatusCompleted)

	// Reset and build second summary
	summary, err := builder.
		Reset().
		WithScanSessionID("20240102-130000").
		WithTargetSource("config").
		WithScanMode("automated").
		AddTarget("https://test.com").
		WithStatus(ScanStatusFailed).
		Build()

	require.NoError(t, err)

	// Verify reset worked
	assert.Equal(t, "20240102-130000", summary.ScanSessionID)
	assert.Equal(t, "config", summary.TargetSource)
	assert.Equal(t, "automated", summary.ScanMode)
	assert.Equal(t, []string{"https://test.com"}, summary.Targets)
	assert.Equal(t, string(ScanStatusFailed), summary.Status)
}

func TestScanSummaryDataBuilder_Clone(t *testing.T) {
	original := NewScanSummaryDataBuilder()
	original.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("onetime").
		AddTarget("https://example.com").
		WithStatus(ScanStatusCompleted)

	clone := original.Clone()

	// Modify clone
	cloneSummary, err := clone.
		WithScanSessionID("20240101-130000").
		AddTarget("https://test.com").
		Build()

	require.NoError(t, err)

	// Verify original is unchanged by building it
	originalSummary, err := original.Build()
	require.NoError(t, err)

	assert.Equal(t, "20240101-120000", originalSummary.ScanSessionID)
	assert.Equal(t, []string{"https://example.com"}, originalSummary.Targets)

	assert.Equal(t, "20240101-130000", cloneSummary.ScanSessionID)
	assert.Equal(t, []string{"https://example.com", "https://test.com"}, cloneSummary.Targets)
}

func TestScanSummaryDataBuilder_Validate(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	// Test validation with incomplete data
	err := builder.
		WithScanSessionID("").
		Validate()

	assert.Error(t, err)

	// Test validation with complete data
	err = builder.
		WithScanSessionID("20240101-120000").
		WithTargetSource("file").
		WithScanMode("onetime").
		WithStatus(ScanStatusCompleted).
		Validate()

	assert.NoError(t, err)
}

func TestScanSummaryDataBuilder_BuildUnsafe(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	// BuildUnsafe should not validate
	summary := builder.
		WithScanSessionID(""). // Invalid data
		WithTargetSource("file").
		BuildUnsafe()

	// Should have empty scan session ID (invalid data)
	assert.Empty(t, summary.ScanSessionID)
	assert.Equal(t, "file", summary.TargetSource)
}

func TestScanSummaryDataBuilder_ValidationFailure(t *testing.T) {
	builder := NewScanSummaryDataBuilder()

	// Try to build with missing required fields
	summary, err := builder.
		WithTargetSource("file").
		WithScanMode("onetime").
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan session ID is required")

	// Summary should be zero value when validation fails
	assert.Empty(t, summary.ScanSessionID)
}
