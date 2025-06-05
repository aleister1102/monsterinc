package scanner

import (
	"context"
	"fmt"

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

// ProcessDiffingAndStorage performs diffing and storage for all targets
// Main orchestration function for diff and storage operations
func (dsp *DiffStorageProcessor) ProcessDiffingAndStorage(input ProcessDiffingAndStorageInput) (ProcessDiffingAndStorageOutput, error) {
	output := ProcessDiffingAndStorageOutput{
		URLDiffResults: make(map[string]models.URLDiffResult),
	}

	// Group probe results by root target
	targetGroups, originalIndices := dsp.groupProbeResultsByRootTarget(
		input.CurrentScanProbeResults,
		input.PrimaryRootTargetURL,
	)

	// OPTIMIZATION: Work with original slice directly instead of copying
	// This reduces memory allocation and improves performance
	output.UpdatedScanProbeResults = input.CurrentScanProbeResults

	// Process each target group
	for rootTarget, resultsForRoot := range targetGroups {
		originalIndicesForTarget := originalIndices[rootTarget]

		err := dsp.processTargetGroup(
			input.Ctx,
			rootTarget,
			resultsForRoot,
			originalIndicesForTarget,
			input.ScanSessionID,
			output.UpdatedScanProbeResults,
			&output,
		)
		if err != nil {
			return output, err
		}
	}

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
	// Perform diffing for this target
	targetInput := DiffTargetInput{
		RootTarget:            rootTarget,
		ProbeResultsForTarget: resultsForRoot,
		ScanSessionID:         scanSessionID,
		URLDiffer:             dsp.urlDiffer,
	}

	targetResult := dsp.ProcessTarget(targetInput)
	if targetResult.Error != nil {
		return targetResult.Error
	}

	// Update the processed results with any changes from diffing
	dsp.updateProcessedProbeResults(
		processedProbeResults,
		targetResult.ProbeResults,
		originalIndicesForTarget,
	)

	// Store the URL diff result
	if targetResult.URLDiffResult != nil {
		output.URLDiffResults[rootTarget] = *targetResult.URLDiffResult
	}

	// Add to storage candidates - only if needed
	if len(targetResult.ProbeResults) > 0 {
		output.AllProbesToStore = append(output.AllProbesToStore, targetResult.ProbeResults...)
	}

	// Write to Parquet storage
	if err := dsp.writeProbeResultsToParquet(ctx, targetResult.ProbeResults, scanSessionID, rootTarget); err != nil {
		return err
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
