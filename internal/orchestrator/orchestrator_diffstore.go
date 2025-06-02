package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/models"
	// Note: config and datastore.ParquetWriter might be needed if ParquetWriter interaction is complex
	// and not fully handled by existing parameters or ScanOrchestrator fields.
)

// DiffTargetInput holds parameters for diffAndPrepareStorageForTarget.
// This encapsulates inputs for clarity and easier future modification.
// Aligns with task 1.3 for reducing direct parameters.
type DiffTargetInput struct {
	RootTarget            string
	ProbeResultsForTarget []models.ProbeResult
	ScanSessionID         string
	URLDiffer             *differ.UrlDiffer
}

// diffAndPrepareStorageForTarget performs URL diffing for a specific target and prepares probe results for storage.
// It returns a new slice of ProbeResult with updated URLStatus and OldestScanTimestamp fields.
// Refactored to use DiffTargetInput for its parameters as per task 1.3.
// Error handling is reviewed as per task 1.6, ensuring errors are wrapped with context.
func (so *Orchestrator) diffAndPrepareStorageForTarget(
	input DiffTargetInput,
) (*models.URLDiffResult, []models.ProbeResult, error) { // Return []models.ProbeResult
	so.logger.Info().Str("root_target", input.RootTarget).Int("current_results_count", len(input.ProbeResultsForTarget)).Str("session_id", input.ScanSessionID).Msg("Processing diff for root target")

	// Create a slice of pointers to copies for the differ, as it might modify them.
	probePointersForDiff := make([]*models.ProbeResult, len(input.ProbeResultsForTarget))
	for i := range input.ProbeResultsForTarget {
		copy := input.ProbeResultsForTarget[i]
		probePointersForDiff[i] = &copy
	}

	diffResult, err := input.URLDiffer.Compare(probePointersForDiff, input.RootTarget)
	if err != nil {
		so.logger.Error().Err(err).Str("root_target", input.RootTarget).Str("session_id", input.ScanSessionID).Msg("Failed to compare URLs. Skipping storage and diff summary for this target.")
		return nil, nil, fmt.Errorf("urlDiffer.Compare failed for target %s, session %s: %w", input.RootTarget, input.ScanSessionID, err)
	}

	if diffResult == nil {
		so.logger.Warn().Str("root_target", input.RootTarget).Str("session_id", input.ScanSessionID).Msg("DiffResult was nil, though no explicit error. Skipping further processing for this target.")
		return nil, nil, nil // No error, but nothing to process
	}

	so.logger.Info().Str("root_target", input.RootTarget).Str("session_id", input.ScanSessionID).Int("new", diffResult.New).Int("old", diffResult.Old).Int("existing", diffResult.Existing).Int("total_diff_urls", len(diffResult.Results)).Msg("URL Diffing complete for target.")

	updatedProbesToStore := make([]models.ProbeResult, 0, len(diffResult.Results))
	for _, diffedURL := range diffResult.Results {
		updatedProbesToStore = append(updatedProbesToStore, diffedURL.ProbeResult) // This ProbeResult has updated fields
	}

	return diffResult, updatedProbesToStore, nil
}

// ProcessDiffingAndStorageInput holds parameters for processDiffingAndStorage.
// This follows task 1.3 to group parameters into a struct.
type ProcessDiffingAndStorageInput struct {
	Ctx                     context.Context
	CurrentScanProbeResults []models.ProbeResult
	SeedURLs                []string
	PrimaryRootTargetURL    string
	ScanSessionID           string
}

// ProcessDiffingAndStorageOutput holds the results from processDiffingAndStorage.
// It includes the URLDiffResults and the updated probe results, removing the side effect
// of modifying the input slice directly in this function.
// This addresses task 1.4 (Minimize side effects).
type ProcessDiffingAndStorageOutput struct {
	URLDiffResults          map[string]models.URLDiffResult
	UpdatedScanProbeResults []models.ProbeResult // The original slice with updated URLStatus and OldestScanTimestamp
	AllProbesToStore        []models.ProbeResult // All probes that are candidates for writing, after diffing and status updates
}

// groupProbeResultsByRootTarget groups probe results by their RootTargetURL.
// It also returns a map of original indices for later updates.
func (so *Orchestrator) groupProbeResultsByRootTarget(
	currentScanProbeResults []models.ProbeResult,
	primaryRootTargetURL string,
	seedURLs []string,
	scanSessionID string,
) (map[string][]models.ProbeResult, map[string][]int) {
	probeResultsByRootTarget := make(map[string][]models.ProbeResult)
	originalIndicesByRootTarget := make(map[string][]int)

	for i, probeResult := range currentScanProbeResults {
		rootTargetURL := probeResult.RootTargetURL
		if rootTargetURL == "" {
			rootTargetURL = primaryRootTargetURL
			if rootTargetURL == "" && len(seedURLs) > 0 {
				rootTargetURL = seedURLs[0]
			} else if rootTargetURL == "" {
				rootTargetURL = probeResult.InputURL // Fallback
				so.logger.Warn().Str("url", probeResult.InputURL).Str("session_id", scanSessionID).Msg("RootTargetURL was empty, falling back to InputURL for grouping.")
			}
		}
		probeResultsByRootTarget[rootTargetURL] = append(probeResultsByRootTarget[rootTargetURL], probeResult)
		originalIndicesByRootTarget[rootTargetURL] = append(originalIndicesByRootTarget[rootTargetURL], i)
	}
	return probeResultsByRootTarget, originalIndicesByRootTarget
}

// updateProcessedProbeResults updates the main list of probe results with URLStatus and OldestScanTimestamp
// based on the results from diffing.
func updateProcessedProbeResults(
	processedProbeResults []models.ProbeResult,
	updatedProbesForTargetStorage []models.ProbeResult,
	originalIndicesForTarget []int,
) {
	updatedProbesMap := make(map[string]models.ProbeResult, len(updatedProbesForTargetStorage))
	for _, p := range updatedProbesForTargetStorage {
		updatedProbesMap[p.InputURL] = p
	}

	for _, originalIndex := range originalIndicesForTarget {
		if originalIndex < len(processedProbeResults) { // Boundary check
			originalProbe := &processedProbeResults[originalIndex]
			if updatedProbe, ok := updatedProbesMap[originalProbe.InputURL]; ok {
				originalProbe.URLStatus = updatedProbe.URLStatus
				originalProbe.OldestScanTimestamp = updatedProbe.OldestScanTimestamp
			}
		}
	}
}

// processTargetGroup performs diffing, updates probe results, and writes to Parquet for a single target group.
// It modifies processedProbeResults in place and contributes to the overall output.
func (so *Orchestrator) processTargetGroup(
	inputCtx context.Context, // Renamed to avoid conflict with ProcessDiffingAndStorageInput.Ctx
	rootTarget string,
	resultsForRoot []models.ProbeResult,
	originalIndicesForTarget []int, // Indices in the main processedProbeResults slice
	scanSessionID string,
	urlDiffer *differ.UrlDiffer,
	processedProbeResults []models.ProbeResult, // The slice to update
	output *ProcessDiffingAndStorageOutput, // Pointer to the main output struct to append results
) error {
	if rootTarget == "" {
		so.logger.Warn().Str("session_id", scanSessionID).Msg("Skipping diffing/storage for empty root target.")
		return nil
	}

	if cancelled := common.CheckCancellationWithLog(inputCtx, so.logger, "diff/store for target: "+rootTarget); cancelled.Cancelled {
		return cancelled.Error
	}

	diffInput := DiffTargetInput{
		RootTarget:            rootTarget,
		ProbeResultsForTarget: resultsForRoot,
		ScanSessionID:         scanSessionID,
		URLDiffer:             urlDiffer,
	}
	diffResultData, updatedProbesForTargetStorage, err := so.diffAndPrepareStorageForTarget(diffInput)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err // Propagate context errors
		}
		so.logger.Error().Err(err).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Error in diffAndPrepareStorageForTarget, skipping target.")
		return nil // Non-context error, log and continue with other targets
	}
	if diffResultData == nil {
		return nil // No diff data, nothing more to do for this target
	}

	output.URLDiffResults[rootTarget] = *diffResultData
	output.AllProbesToStore = append(output.AllProbesToStore, updatedProbesForTargetStorage...)

	// Update the main processedProbeResults slice
	updateProcessedProbeResults(processedProbeResults, updatedProbesForTargetStorage, originalIndicesForTarget)

	// Call the extracted Parquet writing function
	if err := so.writeProbeResultsToParquet(inputCtx, updatedProbesForTargetStorage, scanSessionID, rootTarget); err != nil {
		// If writing fails (e.g., due to context cancellation), propagate the error.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		// For other Parquet write errors, we've already logged in the helper.
		// The error is not returned here to allow processing of other targets, but logged.
		so.logger.Warn().Err(err).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Parquet write failed for target, but continuing with other targets.")
	}
	return nil
}

// processDiffingAndStorage processes URL diffing and stores results to Parquet.
// Refactored to use ProcessDiffingAndStorageInput and return ProcessDiffingAndStorageOutput.
// This addresses tasks 1.2 (single responsibility by better defining inputs/outputs),
// 1.3 (parameter reduction), and 1.4 (minimizing side effects).
func (so *Orchestrator) processDiffingAndStorage(input ProcessDiffingAndStorageInput) (ProcessDiffingAndStorageOutput, error) {
	// Make a copy of the input slice to avoid modifying the original one passed by the caller directly.
	// This copy will be updated and returned in ProcessDiffingAndStorageOutput.
	processedProbeResults := make([]models.ProbeResult, len(input.CurrentScanProbeResults))
	copy(processedProbeResults, input.CurrentScanProbeResults)

	probeResultsByRootTarget, originalIndicesByRootTarget := so.groupProbeResultsByRootTarget(
		processedProbeResults, // Pass the copy
		input.PrimaryRootTargetURL,
		input.SeedURLs,
		input.ScanSessionID,
	)

	urlDiffer := differ.NewUrlDiffer(so.parquetReader, so.logger)
	output := ProcessDiffingAndStorageOutput{
		URLDiffResults: make(map[string]models.URLDiffResult),
		// AllProbesToStore will be populated by processTargetGroup
	}

	for rootTarget, resultsForRoot := range probeResultsByRootTarget {
		originalIndicesForTarget := originalIndicesByRootTarget[rootTarget]
		err := so.processTargetGroup(
			input.Ctx,
			rootTarget,
			resultsForRoot,
			originalIndicesForTarget,
			input.ScanSessionID,
			urlDiffer,
			processedProbeResults, // Pass the slice to be updated
			&output,               // Pass a pointer to the output struct
		)
		if err != nil {
			// If processTargetGroup returns an error (e.g. context cancellation), propagate it.
			// The output struct might be partially filled.
			output.UpdatedScanProbeResults = processedProbeResults // Return the current state of processed results
			return output, err
		}
	}

	output.UpdatedScanProbeResults = processedProbeResults
	return output, nil
}

// writeProbeResultsToParquet handles the persistence of probe results to Parquet.
// This function is extracted from processDiffingAndStorage to improve separation of concerns (Task 1.2).
func (so *Orchestrator) writeProbeResultsToParquet(ctx context.Context, probesToStore []models.ProbeResult, scanSessionID string, rootTarget string) error {
	if so.parquetWriter == nil {
		so.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("ParquetWriter is not initialized. Skipping Parquet storage for target.")
		return nil
	}
	if len(probesToStore) == 0 {
		so.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("No probe results to store to Parquet for target.")
		return nil
	}

	so.logger.Info().Int("count", len(probesToStore)).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Writing probe results to Parquet...")
	if err := so.parquetWriter.Write(ctx, probesToStore, scanSessionID, rootTarget); err != nil {
		so.logger.Error().Err(err).Str("root_target", rootTarget).Str("session_id", scanSessionID).Msg("Failed to write Parquet data")
		// Return the error so the caller can decide how to handle it (e.g., context cancellation)
		return err
	}
	return nil
}
