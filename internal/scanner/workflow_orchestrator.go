package scanner

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// WorkflowOrchestrator coordinates the complete scan workflow
// Separates orchestration logic from individual component operations
type WorkflowOrchestrator struct {
	scanner         *Scanner
	reportGenerator *ReportGenerator
	summaryBuilder  *SummaryBuilder
	logger          zerolog.Logger
	gCfg            *config.GlobalConfig
}

// NewWorkflowOrchestrator creates a new workflow orchestrator
func NewWorkflowOrchestrator(scanner *Scanner, config *config.GlobalConfig, logger zerolog.Logger) (*WorkflowOrchestrator, error) {
	return &WorkflowOrchestrator{
		scanner:         scanner,
		reportGenerator: NewReportGenerator(&config.ReporterConfig, logger),
		summaryBuilder:  NewSummaryBuilder(logger),
		logger:          logger.With().Str("module", "WorkflowOrchestrator").Logger(),
		gCfg:            config,
	}, nil
}

// ExecuteCompleteWorkflow executes the full scan workflow with reporting
// This is the main entry point for running a complete scan cycle
func (wo *WorkflowOrchestrator) ExecuteCompleteWorkflow(input *ScanWorkflowInput) (*ScanWorkflowResult, error) {
	if err := wo.validateInput(input); err != nil {
		return wo.createFailureResult(input, err), err
	}

	// Execute core scan workflow
	probeResults, urlDiffResults, workflowError := wo.scanner.ExecuteScanWorkflow(
		input.Ctx,
		input.SeedURLs,
		input.ScanSessionID,
	)

	// Generate reports if needed
	reportPaths, reportError := wo.generateReports(input.Ctx, probeResults, urlDiffResults, input.ScanSessionID)
	if reportError != nil && workflowError == nil {
		workflowError = reportError
	}

	// Build comprehensive summary
	summary := wo.buildSummary(input, probeResults, urlDiffResults, reportPaths, workflowError)

	result := &ScanWorkflowResult{
		ProbeResults:    probeResults,
		URLDiffResults:  urlDiffResults,
		ReportFilePaths: reportPaths,
		SummaryData:     summary,
		WorkflowError:   workflowError,
		Duration:        summary.ScanDuration,
	}

	return result, workflowError
}

// validateInput validates the workflow input parameters
func (wo *WorkflowOrchestrator) validateInput(input *ScanWorkflowInput) error {
	if input == nil {
		return common.NewError("workflow input cannot be nil")
	}

	if len(input.SeedURLs) == 0 {
		return common.NewError("no seed URLs provided for scan workflow")
	}

	if input.ScanSessionID == "" {
		return common.NewError("scan session ID cannot be empty")
	}

	if input.Ctx == nil {
		return common.NewError("context cannot be nil")
	}

	return nil
}

// generateReports handles report generation with error handling
func (wo *WorkflowOrchestrator) generateReports(ctx context.Context, probeResults []httpxrunner.ProbeResult, urlDiffResults map[string]differ.URLDiffResult, scanSessionID string) ([]string, error) {
	reportInput := NewReportGenerationInputWithDiff(probeResults, urlDiffResults, scanSessionID)
	return wo.reportGenerator.Generate(wo.gCfg, reportInput)
}

// buildSummary creates comprehensive scan summary
func (wo *WorkflowOrchestrator) buildSummary(
	input *ScanWorkflowInput,
	probeResults []httpxrunner.ProbeResult,
	urlDiffResults map[string]differ.URLDiffResult,
	reportPaths []string,
	workflowError error,
) models.ScanSummaryData {
	summaryInput := &SummaryInput{
		ScanSessionID:   input.ScanSessionID,
		TargetSource:    input.TargetSource,
		ScanMode:        input.ScanMode,
		Targets:         input.SeedURLs,
		StartTime:       input.StartTime,
		ProbeResults:    probeResults,
		URLDiffResults:  urlDiffResults,
		WorkflowError:   workflowError,
		ReportFilePaths: reportPaths,
	}

	return wo.summaryBuilder.BuildSummary(summaryInput)
}

// createFailureResult creates a failure result for invalid input
func (wo *WorkflowOrchestrator) createFailureResult(input *ScanWorkflowInput, err error) *ScanWorkflowResult {
	summary := models.GetDefaultScanSummaryData()

	if input != nil {
		summary.ScanSessionID = input.ScanSessionID
		summary.TargetSource = input.TargetSource
		summary.ScanMode = input.ScanMode
		summary.Targets = input.SeedURLs
		summary.TotalTargets = len(input.SeedURLs)
	}

	summary.Status = string(models.ScanStatusFailed)
	summary.ErrorMessages = []string{err.Error()}

	return &ScanWorkflowResult{
		SummaryData:   summary,
		WorkflowError: err,
	}
}

// ExecuteCoreWorkflow executes only the core scanning workflow without reporting
// Useful for cases where only scan results are needed
func (wo *WorkflowOrchestrator) ExecuteCoreWorkflow(ctx context.Context, seedURLs []string, scanSessionID string) ([]models.ProbeResult, map[string]models.URLDiffResult, error) {
	return wo.scanner.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
}

func (wo *WorkflowOrchestrator) getSeedURLs(inputFile string) ([]string, string, error) {
	return wo.targetManager.LoadAndSelectTargets(inputFile)
}
