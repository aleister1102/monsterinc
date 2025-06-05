package scanner

import (
	"context"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// ScanWorkflowInput contains all necessary information to execute scan workflow
// Helps reduce number of function parameters according to refactor principles
type ScanWorkflowInput struct {
	Ctx           context.Context
	SeedURLs      []string
	ScanSessionID string
	TargetSource  string
	ScanMode      string
	StartTime     time.Time
}

// NewScanWorkflowInput creates ScanWorkflowInput with reasonable default values
func NewScanWorkflowInput(ctx context.Context, seedURLs []string, scanSessionID string) *ScanWorkflowInput {
	return &ScanWorkflowInput{
		Ctx:           ctx,
		SeedURLs:      seedURLs,
		ScanSessionID: scanSessionID,
		StartTime:     time.Now(),
	}
}

// WithTargetSource sets target source
func (swi *ScanWorkflowInput) WithTargetSource(targetSource string) *ScanWorkflowInput {
	swi.TargetSource = targetSource
	return swi
}

// WithScanMode sets scan mode
func (swi *ScanWorkflowInput) WithScanMode(scanMode string) *ScanWorkflowInput {
	swi.ScanMode = scanMode
	return swi
}

// ScanWorkflowResult contains the results of scan workflow
// Separates output data for easier management and testing
type ScanWorkflowResult struct {
	ProbeResults    []models.ProbeResult
	URLDiffResults  map[string]models.URLDiffResult
	ReportFilePaths []string
	SummaryData     models.ScanSummaryData
	WorkflowError   error
	Duration        time.Duration
}

// IsSuccessful checks if workflow was successful
func (swr *ScanWorkflowResult) IsSuccessful() bool {
	return swr.WorkflowError == nil
}

// HasResults checks if there are probe results
func (swr *ScanWorkflowResult) HasResults() bool {
	return len(swr.ProbeResults) > 0
}

// HasReports checks if reports were generated
func (swr *ScanWorkflowResult) HasReports() bool {
	return len(swr.ReportFilePaths) > 0
}
