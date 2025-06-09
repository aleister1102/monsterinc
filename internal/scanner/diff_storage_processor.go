package scanner

import (
	"context"
	"fmt"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// DiffStorageProcessor handles diffing and storage operations
// Separates diff and storage logic from the main scanner
type DiffStorageProcessor struct {
	logger        zerolog.Logger
	parquetWriter ParquetWriter
	urlDiffer     *differ.UrlDiffer
}

// ParquetWriter interface for dependency injection and better testing
type ParquetWriter interface {
	Write(ctx context.Context, probeResults []models.ProbeResult, scanSessionID string, rootTarget string) error
}

// NewDiffStorageProcessor creates a new diff storage processor
func NewDiffStorageProcessor(logger zerolog.Logger, parquetWriter ParquetWriter, urlDiffer *differ.UrlDiffer) *DiffStorageProcessor {
	return &DiffStorageProcessor{
		logger:        logger.With().Str("module", "DiffStorageProcessor").Logger(),
		parquetWriter: parquetWriter,
		urlDiffer:     urlDiffer,
	}
}

// DiffTargetInput contains parameters for processing a single target
type DiffTargetInput struct {
	RootTarget            string
	ProbeResultsForTarget []models.ProbeResult
	ScanSessionID         string
	URLDiffer             *differ.UrlDiffer
}

// DiffTargetResult contains the results from processing a single target
type DiffTargetResult struct {
	URLDiffResult *models.URLDiffResult
	ProbeResults  []models.ProbeResult
	Error         error
}

// ProcessTarget performs diffing and prepares storage for a single target
// Renamed from diffAndPrepareStorageForTarget for clarity
func (dsp *DiffStorageProcessor) ProcessTarget(input DiffTargetInput) *DiffTargetResult {
	result := &DiffTargetResult{}

	if len(input.ProbeResultsForTarget) == 0 {
		return result
	}

	// Convert to pointer slice for differ - optimized allocation
	probeResultPtrs := make([]*models.ProbeResult, len(input.ProbeResultsForTarget))
	for i := range input.ProbeResultsForTarget {
		probeResultPtrs[i] = &input.ProbeResultsForTarget[i]
	}

	urlDiffResult, err := input.URLDiffer.Compare(probeResultPtrs, input.RootTarget)
	if err != nil {
		dsp.logger.Error().
			Err(err).
			Str("root_target", input.RootTarget).
			Str("session_id", input.ScanSessionID).
			Msg("URL diffing failed")
		result.Error = fmt.Errorf("URL diffing failed for target %s: %w", input.RootTarget, err)
		return result
	}

	// Only log if there are interesting changes (new/old URLs)
	if urlDiffResult.New > 0 || urlDiffResult.Old > 0 {
		dsp.logger.Info().
			Int("new", urlDiffResult.New).
			Int("old", urlDiffResult.Old).
			Int("existing", urlDiffResult.Existing).
			Str("root_target", input.RootTarget).
			Msg("URL changes detected")
	}

	result.URLDiffResult = urlDiffResult
	result.ProbeResults = input.ProbeResultsForTarget
	return result
}

// ProcessDiffingAndStorageInput contains parameters for processing multiple targets
type ProcessDiffingAndStorageInput struct {
	Ctx                     context.Context
	CurrentScanProbeResults []models.ProbeResult
	SeedURLs                []string
	PrimaryRootTargetURL    string
	ScanSessionID           string
}

// ProcessDiffingAndStorageOutput contains results from processing multiple targets
type ProcessDiffingAndStorageOutput struct {
	URLDiffResults          map[string]models.URLDiffResult
	UpdatedScanProbeResults []models.ProbeResult
	AllProbesToStore        []models.ProbeResult
}

// ProcessDiffingAndStorage handles diffing and storage for all targets
func (dsp *DiffStorageProcessor) ProcessDiffingAndStorage(input ProcessDiffingAndStorageInput) (ProcessDiffingAndStorageOutput, error) {
	// Early context cancellation check
	if cancelled := common.CheckCancellation(input.Ctx); cancelled.Cancelled {
		dsp.logger.Info().Msg("Context cancelled before diff processing")
		return ProcessDiffingAndStorageOutput{}, cancelled.Error
	}

	// Group probe results by root target for efficient processing
	probeResultsByRoot, originalIndicesMapByRoot := dsp.groupProbeResultsByRootTarget(input.CurrentScanProbeResults, input.PrimaryRootTargetURL)

	// Initialize output structure
	output := ProcessDiffingAndStorageOutput{
		URLDiffResults:          make(map[string]models.URLDiffResult),
		UpdatedScanProbeResults: make([]models.ProbeResult, len(input.CurrentScanProbeResults)),
		AllProbesToStore:        make([]models.ProbeResult, 0, len(input.CurrentScanProbeResults)),
	}

	// Copy original results to avoid mutation
	copy(output.UpdatedScanProbeResults, input.CurrentScanProbeResults)

	// Process each target group
	for rootTarget, resultsForRoot := range probeResultsByRoot {
		// Check context cancellation before processing each target group
		if cancelled := common.CheckCancellation(input.Ctx); cancelled.Cancelled {
			dsp.logger.Info().Str("root_target", rootTarget).Msg("Context cancelled during target group processing")
			return output, cancelled.Error
		}

		originalIndicesForTarget := originalIndicesMapByRoot[rootTarget]

		if err := dsp.processTargetGroup(
			input.Ctx,
			rootTarget,
			resultsForRoot,
			originalIndicesForTarget,
			input.ScanSessionID,
			output.UpdatedScanProbeResults,
			&output,
		); err != nil {
			// Check if error is due to context cancellation
			if input.Ctx.Err() != nil {
				dsp.logger.Info().Str("root_target", rootTarget).Msg("Target group processing cancelled")
				return output, input.Ctx.Err()
			}

			dsp.logger.Error().Err(err).Str("root_target", rootTarget).Msg("Failed to process target group")
			continue // Continue with other targets
		}
	}

	// Final context check before returning results
	if cancelled := common.CheckCancellation(input.Ctx); cancelled.Cancelled {
		dsp.logger.Info().Msg("Context cancelled after diff processing completion")
		return output, cancelled.Error
	}

	dsp.logger.Info().
		Int("total_targets", len(probeResultsByRoot)).
		Int("total_probes", len(output.AllProbesToStore)).
		Msg("Diff processing and storage completed")

	return output, nil
}

// processTargetGroup processes a single target group
func (dsp *DiffStorageProcessor) processTargetGroup(
	ctx context.Context,
	rootTarget string,
	resultsForRoot []models.ProbeResult,
	originalIndicesForTarget []int,
	scanSessionID string,
	processedProbeResults []models.ProbeResult,
	output *ProcessDiffingAndStorageOutput,
) error {
	// Check context cancellation at the start of target processing
	if cancelled := common.CheckCancellation(ctx); cancelled.Cancelled {
		dsp.logger.Info().Str("root_target", rootTarget).Msg("Context cancelled at start of target processing")
		return cancelled.Error
	}

	dsp.logger.Debug().
		Str("root_target", rootTarget).
		Int("probe_count", len(resultsForRoot)).
		Msg("Processing target group")

	// Perform URL diffing for this target
	diffInput := DiffTargetInput{
		RootTarget:            rootTarget,
		ProbeResultsForTarget: resultsForRoot,
		ScanSessionID:         scanSessionID,
		URLDiffer:             dsp.urlDiffer,
	}

	diffResult := dsp.ProcessTarget(diffInput)
	if diffResult.Error != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			dsp.logger.Info().Str("root_target", rootTarget).Msg("URL diffing cancelled")
			return ctx.Err()
		}

		dsp.logger.Warn().Err(diffResult.Error).Str("root_target", rootTarget).Msg("URL diffing failed, continuing")
	} else {
		output.URLDiffResults[rootTarget] = *diffResult.URLDiffResult
	}

	// Check context cancellation before storage operations
	if cancelled := common.CheckCancellation(ctx); cancelled.Cancelled {
		dsp.logger.Info().Str("root_target", rootTarget).Msg("Context cancelled before storage operations")
		return cancelled.Error
	}

	// Update processed results with diff information
	updatedProbesForTargetStorage := diffResult.ProbeResults
	dsp.updateProcessedProbeResults(processedProbeResults, updatedProbesForTargetStorage, originalIndicesForTarget)

	// Store results in Parquet
	if err := dsp.writeProbeResultsToParquet(ctx, updatedProbesForTargetStorage, scanSessionID, rootTarget); err != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			dsp.logger.Info().Str("root_target", rootTarget).Msg("Parquet storage cancelled")
			return ctx.Err()
		}

		dsp.logger.Warn().Err(err).Str("root_target", rootTarget).Msg("Failed to write to Parquet, continuing")
	}

	// Add to all probes for potential reporting
	output.AllProbesToStore = append(output.AllProbesToStore, updatedProbesForTargetStorage...)

	// Final context check
	if cancelled := common.CheckCancellation(ctx); cancelled.Cancelled {
		dsp.logger.Info().Str("root_target", rootTarget).Msg("Context cancelled after target processing")
		return cancelled.Error
	}

	return nil
}

// writeProbeResultsToParquet handles the persistence of probe results to Parquet
func (dsp *DiffStorageProcessor) writeProbeResultsToParquet(ctx context.Context, probesToStore []models.ProbeResult, scanSessionID, rootTarget string) error {
	if dsp.parquetWriter == nil {
		return nil
	}

	if len(probesToStore) == 0 {
		return nil
	}

	if err := dsp.parquetWriter.Write(ctx, probesToStore, scanSessionID, rootTarget); err != nil {
		dsp.logger.Error().
			Err(err).
			Str("root_target", rootTarget).
			Str("session_id", scanSessionID).
			Msg("Failed to write Parquet data")
		return err
	}

	return nil
}

// updateProcessedProbeResults updates the processed results with changes from diffing
func (dsp *DiffStorageProcessor) updateProcessedProbeResults(
	processedProbeResults []models.ProbeResult,
	updatedProbesForTargetStorage []models.ProbeResult,
	originalIndicesForTarget []int,
) {
	// OPTIMIZATION: Direct assignment instead of loop where possible
	minLen := len(updatedProbesForTargetStorage)
	if len(originalIndicesForTarget) < minLen {
		minLen = len(originalIndicesForTarget)
	}

	for i := 0; i < minLen; i++ {
		originalIndex := originalIndicesForTarget[i]
		if originalIndex < len(processedProbeResults) {
			processedProbeResults[originalIndex] = updatedProbesForTargetStorage[i]
		}
	}
}

// groupProbeResultsByRootTarget groups probe results by their root target URL
func (dsp *DiffStorageProcessor) groupProbeResultsByRootTarget(
	currentScanProbeResults []models.ProbeResult,
	primaryRootTargetURL string,
) (map[string][]models.ProbeResult, map[string][]int) {
	targetGroups := make(map[string][]models.ProbeResult)
	originalIndices := make(map[string][]int)

	for i, result := range currentScanProbeResults {
		rootTarget := result.RootTargetURL
		if rootTarget == "" {
			rootTarget = primaryRootTargetURL
		}

		targetGroups[rootTarget] = append(targetGroups[rootTarget], result)
		originalIndices[rootTarget] = append(originalIndices[rootTarget], i)
	}

	return targetGroups, originalIndices
}
