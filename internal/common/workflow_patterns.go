package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// WorkflowPhase represents a single phase in a workflow
type WorkflowPhase struct {
	Name        string
	Description string
	Required    bool
	Timeout     time.Duration
	Execute     func(context.Context) error
}

// WorkflowExecutor manages workflow execution with phase control
type WorkflowExecutor struct {
	name         string
	phases       []WorkflowPhase
	logger       zerolog.Logger
	ctx          context.Context
	sessionID    string
	stopOnError  bool
	mutex        sync.RWMutex
	currentPhase int
	results      map[string]interface{}
}

// WorkflowConfig holds configuration for workflow execution
type WorkflowConfig struct {
	Name        string
	SessionID   string
	Logger      zerolog.Logger
	StopOnError bool
	Timeout     time.Duration
}

// WorkflowResult represents the result of workflow execution
type WorkflowResult struct {
	Success         bool
	CompletedPhases int
	TotalPhases     int
	PhaseResults    map[string]interface{}
	Errors          map[string]error
	ExecutionTime   time.Duration
	FailedAt        string
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(config WorkflowConfig) *WorkflowExecutor {
	return &WorkflowExecutor{
		name:        config.Name,
		logger:      config.Logger.With().Str("workflow", config.Name).Str("session_id", config.SessionID).Logger(),
		sessionID:   config.SessionID,
		stopOnError: config.StopOnError,
		results:     make(map[string]interface{}),
	}
}

// AddPhase adds a phase to the workflow
func (we *WorkflowExecutor) AddPhase(phase WorkflowPhase) {
	we.mutex.Lock()
	defer we.mutex.Unlock()
	we.phases = append(we.phases, phase)
}

// Execute runs the complete workflow
func (we *WorkflowExecutor) Execute(ctx context.Context) WorkflowResult {
	startTime := time.Now()
	we.ctx = ctx

	result := WorkflowResult{
		PhaseResults:  make(map[string]interface{}),
		Errors:        make(map[string]error),
		TotalPhases:   len(we.phases),
		ExecutionTime: 0,
		Success:       true,
	}

	we.logger.Info().
		Int("total_phases", result.TotalPhases).
		Str("workflow", we.name).
		Msg("Starting workflow execution")

	for i, phase := range we.phases {
		we.mutex.Lock()
		we.currentPhase = i
		we.mutex.Unlock()

		// Check for cancellation before each phase
		if cancelled := CheckCancellationWithLog(ctx, we.logger, fmt.Sprintf("phase %s", phase.Name)); cancelled.Cancelled {
			result.Success = false
			result.FailedAt = phase.Name
			result.Errors[phase.Name] = cancelled.Error
			we.logger.Info().
				Str("phase", phase.Name).
				Int("completed_phases", i).
				Msg("Workflow cancelled")
			break
		}

		we.logger.Info().
			Str("phase", phase.Name).
			Str("description", phase.Description).
			Int("phase_number", i+1).
			Int("total_phases", result.TotalPhases).
			Bool("required", phase.Required).
			Msg("Executing workflow phase")

		phaseCtx := ctx
		var cancel context.CancelFunc

		// Apply phase timeout if specified
		if phase.Timeout > 0 {
			phaseCtx, cancel = context.WithTimeout(ctx, phase.Timeout)
			defer cancel()
		}

		// Execute the phase
		phaseStartTime := time.Now()
		err := phase.Execute(phaseCtx)
		phaseDuration := time.Since(phaseStartTime)

		if cancel != nil {
			cancel()
		}

		if err != nil {
			we.logger.Error().
				Err(err).
				Str("phase", phase.Name).
				Bool("required", phase.Required).
				Dur("duration", phaseDuration).
				Msg("Workflow phase failed")

			result.Errors[phase.Name] = err

			if phase.Required && we.stopOnError {
				result.Success = false
				result.FailedAt = phase.Name
				we.logger.Error().
					Str("phase", phase.Name).
					Msg("Workflow stopped due to required phase failure")
				break
			}

			if phase.Required {
				result.Success = false
				if result.FailedAt == "" {
					result.FailedAt = phase.Name
				}
			}
		} else {
			result.CompletedPhases++
			we.logger.Info().
				Str("phase", phase.Name).
				Dur("duration", phaseDuration).
				Msg("Workflow phase completed successfully")
		}

		// Store phase completion regardless of success/failure for tracking
		result.PhaseResults[phase.Name] = map[string]interface{}{
			"completed": err == nil,
			"duration":  phaseDuration,
			"error":     err,
		}
	}

	result.ExecutionTime = time.Since(startTime)

	if result.Success {
		we.logger.Info().
			Int("completed_phases", result.CompletedPhases).
			Dur("total_duration", result.ExecutionTime).
			Msg("Workflow completed successfully")
	} else {
		we.logger.Warn().
			Int("completed_phases", result.CompletedPhases).
			Int("total_phases", result.TotalPhases).
			Str("failed_at", result.FailedAt).
			Dur("total_duration", result.ExecutionTime).
			Msg("Workflow completed with errors")
	}

	return result
}

// GetCurrentPhase returns the current phase being executed
func (we *WorkflowExecutor) GetCurrentPhase() (int, string) {
	we.mutex.RLock()
	defer we.mutex.RUnlock()

	if we.currentPhase < len(we.phases) {
		return we.currentPhase, we.phases[we.currentPhase].Name
	}
	return we.currentPhase, "completed"
}

// GetResults returns the current workflow results
func (we *WorkflowExecutor) GetResults() map[string]interface{} {
	we.mutex.RLock()
	defer we.mutex.RUnlock()

	resultsCopy := make(map[string]interface{})
	for k, v := range we.results {
		resultsCopy[k] = v
	}
	return resultsCopy
}

// SetResult stores a result for the workflow
func (we *WorkflowExecutor) SetResult(key string, value interface{}) {
	we.mutex.Lock()
	defer we.mutex.Unlock()
	we.results[key] = value
}

// ScanWorkflowExecutor provides specialized workflow execution for scan operations
type ScanWorkflowExecutor struct {
	*WorkflowExecutor
	sessionID   string
	seedURLs    []string
	results     *ScanWorkflowResults
	resultMutex sync.RWMutex
}

// ScanWorkflowResults holds results from scan workflow execution
type ScanWorkflowResults struct {
	DiscoveredURLs []string
	ProbeResults   []interface{} // Using interface{} to avoid circular import
	DiffResults    map[string]interface{}
	SecretFindings []interface{}
	ReportPath     string
	Errors         []error
}

// ScanWorkflowConfig holds configuration for scan workflow
type ScanWorkflowConfig struct {
	WorkflowConfig
	SeedURLs []string
}

// NewScanWorkflowExecutor creates a new scan workflow executor
func NewScanWorkflowExecutor(config ScanWorkflowConfig) *ScanWorkflowExecutor {
	baseExecutor := NewWorkflowExecutor(config.WorkflowConfig)

	return &ScanWorkflowExecutor{
		WorkflowExecutor: baseExecutor,
		sessionID:        config.SessionID,
		seedURLs:         config.SeedURLs,
		results: &ScanWorkflowResults{
			DiffResults: make(map[string]interface{}),
		},
	}
}

// AddCrawlerPhase adds crawler phase to scan workflow
func (swe *ScanWorkflowExecutor) AddCrawlerPhase(crawlerFunc func(context.Context, []string) ([]string, error)) {
	phase := WorkflowPhase{
		Name:        "crawler",
		Description: "Discover URLs through crawling",
		Required:    false,
		Execute: func(ctx context.Context) error {
			if len(swe.seedURLs) == 0 {
				swe.logger.Info().Msg("No seed URLs provided, skipping crawler phase")
				return nil
			}

			discoveredURLs, err := crawlerFunc(ctx, swe.seedURLs)
			if err != nil {
				return fmt.Errorf("crawler phase failed: %w", err)
			}

			swe.resultMutex.Lock()
			swe.results.DiscoveredURLs = discoveredURLs
			swe.resultMutex.Unlock()

			swe.logger.Info().
				Int("discovered_count", len(discoveredURLs)).
				Int("seed_count", len(swe.seedURLs)).
				Msg("Crawler phase completed")

			return nil
		},
	}
	swe.AddPhase(phase)
}

// AddProbingPhase adds HTTP probing phase to scan workflow
func (swe *ScanWorkflowExecutor) AddProbingPhase(proberFunc func(context.Context, []string) ([]interface{}, error)) {
	phase := WorkflowPhase{
		Name:        "http_probing",
		Description: "Probe discovered URLs for HTTP responses",
		Required:    true,
		Execute: func(ctx context.Context) error {
			swe.resultMutex.RLock()
			urlsToProbe := swe.results.DiscoveredURLs
			swe.resultMutex.RUnlock()

			if len(urlsToProbe) == 0 {
				swe.logger.Info().Msg("No URLs to probe, skipping probing phase")
				return nil
			}

			probeResults, err := proberFunc(ctx, urlsToProbe)
			if err != nil {
				return fmt.Errorf("probing phase failed: %w", err)
			}

			swe.resultMutex.Lock()
			swe.results.ProbeResults = probeResults
			swe.resultMutex.Unlock()

			swe.logger.Info().
				Int("probe_results_count", len(probeResults)).
				Int("urls_probed", len(urlsToProbe)).
				Msg("Probing phase completed")

			return nil
		},
	}
	swe.AddPhase(phase)
}

// AddSecretDetectionPhase adds secret detection phase to scan workflow
func (swe *ScanWorkflowExecutor) AddSecretDetectionPhase(secretFunc func(context.Context, []interface{}) ([]interface{}, error)) {
	phase := WorkflowPhase{
		Name:        "secret_detection",
		Description: "Scan content for secrets and sensitive information",
		Required:    false,
		Execute: func(ctx context.Context) error {
			swe.resultMutex.RLock()
			probeResults := swe.results.ProbeResults
			swe.resultMutex.RUnlock()

			if len(probeResults) == 0 {
				swe.logger.Info().Msg("No probe results available, skipping secret detection phase")
				return nil
			}

			secretFindings, err := secretFunc(ctx, probeResults)
			if err != nil {
				return fmt.Errorf("secret detection phase failed: %w", err)
			}

			swe.resultMutex.Lock()
			swe.results.SecretFindings = secretFindings
			swe.resultMutex.Unlock()

			swe.logger.Info().
				Int("secret_findings_count", len(secretFindings)).
				Msg("Secret detection phase completed")

			return nil
		},
	}
	swe.AddPhase(phase)
}

// AddDiffingPhase adds diffing phase to scan workflow
func (swe *ScanWorkflowExecutor) AddDiffingPhase(diffFunc func(context.Context, []interface{}) (map[string]interface{}, error)) {
	phase := WorkflowPhase{
		Name:        "diffing",
		Description: "Compare current results with previous scan data",
		Required:    true,
		Execute: func(ctx context.Context) error {
			swe.resultMutex.RLock()
			probeResults := swe.results.ProbeResults
			swe.resultMutex.RUnlock()

			if len(probeResults) == 0 {
				swe.logger.Info().Msg("No probe results available, skipping diffing phase")
				return nil
			}

			diffResults, err := diffFunc(ctx, probeResults)
			if err != nil {
				return fmt.Errorf("diffing phase failed: %w", err)
			}

			swe.resultMutex.Lock()
			swe.results.DiffResults = diffResults
			swe.resultMutex.Unlock()

			swe.logger.Info().
				Int("diff_targets", len(diffResults)).
				Msg("Diffing phase completed")

			return nil
		},
	}
	swe.AddPhase(phase)
}

// AddReportingPhase adds report generation phase to scan workflow
func (swe *ScanWorkflowExecutor) AddReportingPhase(reportFunc func(context.Context, *ScanWorkflowResults) (string, error)) {
	phase := WorkflowPhase{
		Name:        "reporting",
		Description: "Generate scan report from results",
		Required:    false,
		Execute: func(ctx context.Context) error {
			swe.resultMutex.RLock()
			results := swe.results
			swe.resultMutex.RUnlock()

			reportPath, err := reportFunc(ctx, results)
			if err != nil {
				return fmt.Errorf("reporting phase failed: %w", err)
			}

			swe.resultMutex.Lock()
			swe.results.ReportPath = reportPath
			swe.resultMutex.Unlock()

			swe.logger.Info().
				Str("report_path", reportPath).
				Msg("Reporting phase completed")

			return nil
		},
	}
	swe.AddPhase(phase)
}

// GetScanResults returns the scan workflow results
func (swe *ScanWorkflowExecutor) GetScanResults() *ScanWorkflowResults {
	swe.resultMutex.RLock()
	defer swe.resultMutex.RUnlock()

	// Return a copy to prevent external modification
	return &ScanWorkflowResults{
		DiscoveredURLs: append([]string(nil), swe.results.DiscoveredURLs...),
		ProbeResults:   append([]interface{}(nil), swe.results.ProbeResults...),
		DiffResults:    copyMap(swe.results.DiffResults),
		SecretFindings: append([]interface{}(nil), swe.results.SecretFindings...),
		ReportPath:     swe.results.ReportPath,
		Errors:         append([]error(nil), swe.results.Errors...),
	}
}

// Pipeline represents a sequence of operations that can be executed
type Pipeline struct {
	name       string
	operations []PipelineOperation
	logger     zerolog.Logger
	ctx        context.Context
}

// PipelineOperation represents a single operation in a pipeline
type PipelineOperation struct {
	Name        string
	Description string
	Execute     func(context.Context, interface{}) (interface{}, error)
	Required    bool
	Timeout     time.Duration
}

// PipelineResult represents the result of pipeline execution
type PipelineResult struct {
	Success       bool
	FinalOutput   interface{}
	OperationLogs map[string]interface{}
	Errors        []error
	ExecutionTime time.Duration
}

// NewPipeline creates a new pipeline
func NewPipeline(name string, logger zerolog.Logger) *Pipeline {
	return &Pipeline{
		name:       name,
		operations: make([]PipelineOperation, 0),
		logger:     logger.With().Str("pipeline", name).Logger(),
	}
}

// AddOperation adds an operation to the pipeline
func (p *Pipeline) AddOperation(op PipelineOperation) {
	p.operations = append(p.operations, op)
}

// Execute runs the pipeline with the given input
func (p *Pipeline) Execute(ctx context.Context, input interface{}) PipelineResult {
	startTime := time.Now()
	result := PipelineResult{
		OperationLogs: make(map[string]interface{}),
		Errors:        make([]error, 0),
		Success:       true,
	}

	currentOutput := input
	p.logger.Info().
		Int("operation_count", len(p.operations)).
		Msg("Starting pipeline execution")

	for i, op := range p.operations {
		// Check for cancellation
		if cancelled := CheckCancellation(ctx); cancelled.Cancelled {
			result.Success = false
			result.Errors = append(result.Errors, cancelled.Error)
			p.logger.Info().
				Str("operation", op.Name).
				Int("completed_operations", i).
				Msg("Pipeline cancelled")
			break
		}

		p.logger.Info().
			Str("operation", op.Name).
			Str("description", op.Description).
			Int("operation_number", i+1).
			Int("total_operations", len(p.operations)).
			Msg("Executing pipeline operation")

		opCtx := ctx
		var cancel context.CancelFunc

		if op.Timeout > 0 {
			opCtx, cancel = context.WithTimeout(ctx, op.Timeout)
			defer cancel()
		}

		opStartTime := time.Now()
		output, err := op.Execute(opCtx, currentOutput)
		opDuration := time.Since(opStartTime)

		if cancel != nil {
			cancel()
		}

		result.OperationLogs[op.Name] = map[string]interface{}{
			"duration":   opDuration,
			"successful": err == nil,
			"error":      err,
		}

		if err != nil {
			p.logger.Error().
				Err(err).
				Str("operation", op.Name).
				Bool("required", op.Required).
				Dur("duration", opDuration).
				Msg("Pipeline operation failed")

			result.Errors = append(result.Errors, err)

			if op.Required {
				result.Success = false
				p.logger.Error().
					Str("operation", op.Name).
					Msg("Pipeline stopped due to required operation failure")
				break
			}
		} else {
			currentOutput = output
			p.logger.Info().
				Str("operation", op.Name).
				Dur("duration", opDuration).
				Msg("Pipeline operation completed successfully")
		}
	}

	result.FinalOutput = currentOutput
	result.ExecutionTime = time.Since(startTime)

	if result.Success {
		p.logger.Info().
			Dur("total_duration", result.ExecutionTime).
			Msg("Pipeline completed successfully")
	} else {
		p.logger.Warn().
			Int("error_count", len(result.Errors)).
			Dur("total_duration", result.ExecutionTime).
			Msg("Pipeline completed with errors")
	}

	return result
}

// copyMap creates a deep copy of a map[string]interface{}
func copyMap(original map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	for k, v := range original {
		copy[k] = v
	}
	return copy
}
