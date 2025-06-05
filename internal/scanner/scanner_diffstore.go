package scanner

import (
	"context"
	"errors"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/models"
)

// diffAndPrepareStorageForTarget performs URL diffing for a specific target and prepares probe results for storage.
// It returns a new slice of ProbeResult with updated URLStatus and OldestScanTimestamp fields.
// Refactored to use DiffTargetInput for its parameters as per task 1.3.
// Error handling is reviewed as per task 1.6, ensuring errors are wrapped with context.
func (s *Scanner) diffAndPrepareStorageForTarget(input DiffTargetInput) (*models.URLDiffResult, []models.ProbeResult, error) {
	s.logDiffProcessingStart(input)

	probePointersForDiff := s.createProbePointers(input.ProbeResultsForTarget)
	diffResult, err := s.performURLDiffing(input, probePointersForDiff)
	if err != nil {
		return nil, nil, err
	}

	if diffResult == nil {
		s.logNilDiffResult(input)
		return nil, nil, nil
	}

	s.logDiffProcessingComplete(input, diffResult)
	updatedProbesToStore := s.extractUpdatedProbeResults(diffResult)

	return diffResult, updatedProbesToStore, nil
}

// logDiffProcessingStart logs the start of diff processing for a target.
func (s *Scanner) logDiffProcessingStart(input DiffTargetInput) {
	s.logger.Info().
		Str("root_target", input.RootTarget).
		Int("current_results_count", len(input.ProbeResultsForTarget)).
		Str("session_id", input.ScanSessionID).
		Msg("Processing diff for root target")
}

// createProbePointers creates a slice of pointers to copies for the differ.
func (s *Scanner) createProbePointers(probeResults []models.ProbeResult) []*models.ProbeResult {
	probePointersForDiff := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		copy := probeResults[i]
		probePointersForDiff[i] = &copy
	}
	return probePointersForDiff
}

// performURLDiffing executes the URL diffing process.
func (s *Scanner) performURLDiffing(input DiffTargetInput, probePointers []*models.ProbeResult) (*models.URLDiffResult, error) {
	diffResult, err := input.URLDiffer.Compare(probePointers, input.RootTarget)
	if err != nil {
		s.logDiffError(input, err)
		return nil, common.WrapError(err, "URL diffing failed")
	}
	return diffResult, nil
}

// logDiffError logs errors that occur during URL diffing.
func (s *Scanner) logDiffError(input DiffTargetInput, err error) {
	s.logger.Error().
		Err(err).
		Str("root_target", input.RootTarget).
		Str("session_id", input.ScanSessionID).
		Msg("Failed to compare URLs. Skipping storage and diff summary for this target")
}

// logNilDiffResult logs when diff result is nil.
func (s *Scanner) logNilDiffResult(input DiffTargetInput) {
	s.logger.Warn().
		Str("root_target", input.RootTarget).
		Str("session_id", input.ScanSessionID).
		Msg("DiffResult was nil, though no explicit error. Skipping further processing for this target")
}

// logDiffProcessingComplete logs the completion of diff processing.
func (s *Scanner) logDiffProcessingComplete(input DiffTargetInput, diffResult *models.URLDiffResult) {
	s.logger.Info().
		Str("root_target", input.RootTarget).
		Str("session_id", input.ScanSessionID).
		Int("new", diffResult.New).
		Int("old", diffResult.Old).
		Int("existing", diffResult.Existing).
		Int("total_diff_urls", len(diffResult.Results)).
		Msg("URL Diffing complete for target")
}

// extractUpdatedProbeResults extracts updated probe results from diff results.
func (s *Scanner) extractUpdatedProbeResults(diffResult *models.URLDiffResult) []models.ProbeResult {
	updatedProbesToStore := make([]models.ProbeResult, 0, len(diffResult.Results))
	for _, diffedURL := range diffResult.Results {
		updatedProbesToStore = append(updatedProbesToStore, diffedURL.ProbeResult)
	}
	return updatedProbesToStore
}

// groupProbeResultsByRootTarget groups probe results by their RootTargetURL.
// It also returns a map of original indices for later updates.
func (s *Scanner) groupProbeResultsByRootTarget(
	currentScanProbeResults []models.ProbeResult,
	primaryRootTargetURL string,
	seedURLs []string,
	scanSessionID string,
) (map[string][]models.ProbeResult, map[string][]int) {
	probeResultsByRootTarget := make(map[string][]models.ProbeResult)
	originalIndicesByRootTarget := make(map[string][]int)

	for i, probeResult := range currentScanProbeResults {
		rootTargetURL := s.determineRootTargetURL(probeResult, primaryRootTargetURL, seedURLs, scanSessionID)

		probeResultsByRootTarget[rootTargetURL] = append(probeResultsByRootTarget[rootTargetURL], probeResult)
		originalIndicesByRootTarget[rootTargetURL] = append(originalIndicesByRootTarget[rootTargetURL], i)
	}

	return probeResultsByRootTarget, originalIndicesByRootTarget
}

// determineRootTargetURL determines the root target URL for a probe result.
func (s *Scanner) determineRootTargetURL(probeResult models.ProbeResult, primaryRootTargetURL string, seedURLs []string, scanSessionID string) string {
	rootTargetURL := probeResult.RootTargetURL

	if rootTargetURL == "" {
		rootTargetURL = primaryRootTargetURL

		if rootTargetURL == "" && len(seedURLs) > 0 {
			rootTargetURL = seedURLs[0]
		} else if rootTargetURL == "" {
			rootTargetURL = probeResult.InputURL // Fallback
			s.logFallbackToInputURL(probeResult.InputURL, scanSessionID)
		}
	}

	return rootTargetURL
}

// logFallbackToInputURL logs when falling back to InputURL for grouping.
func (s *Scanner) logFallbackToInputURL(inputURL, scanSessionID string) {
	s.logger.Warn().
		Str("url", inputURL).
		Str("session_id", scanSessionID).
		Msg("RootTargetURL was empty, falling back to InputURL for grouping")
}

// updateProcessedProbeResults updates the main list of probe results with URLStatus and OldestScanTimestamp.
func updateProcessedProbeResults(
	processedProbeResults []models.ProbeResult,
	updatedProbesForTargetStorage []models.ProbeResult,
	originalIndicesForTarget []int,
) {
	updatedProbesMap := createUpdatedProbesMap(updatedProbesForTargetStorage)

	for _, originalIndex := range originalIndicesForTarget {
		if originalIndex < len(processedProbeResults) {
			updateSingleProbeResult(&processedProbeResults[originalIndex], updatedProbesMap)
		}
	}
}

// createUpdatedProbesMap creates a map for fast lookup of updated probe results.
func createUpdatedProbesMap(updatedProbes []models.ProbeResult) map[string]models.ProbeResult {
	updatedProbesMap := make(map[string]models.ProbeResult, len(updatedProbes))
	for _, p := range updatedProbes {
		updatedProbesMap[p.InputURL] = p
	}
	return updatedProbesMap
}

// updateSingleProbeResult updates a single probe result with diff information.
func updateSingleProbeResult(originalProbe *models.ProbeResult, updatedProbesMap map[string]models.ProbeResult) {
	if updatedProbe, ok := updatedProbesMap[originalProbe.InputURL]; ok {
		originalProbe.URLStatus = updatedProbe.URLStatus
		originalProbe.OldestScanTimestamp = updatedProbe.OldestScanTimestamp
	}
}

// processTargetGroup performs diffing, updates probe results, and writes to Parquet for a single target group.
func (s *Scanner) processTargetGroup(
	ctx context.Context,
	rootTarget string,
	resultsForRoot []models.ProbeResult,
	originalIndicesForTarget []int,
	scanSessionID string,
	urlDiffer *differ.UrlDiffer,
	processedProbeResults []models.ProbeResult,
	output *ProcessDiffingAndStorageOutput,
) error {
	if rootTarget == "" {
		s.logSkippingEmptyTarget(scanSessionID)
		return nil
	}

	if err := s.checkContextCancellation(ctx, "diff/store for target: "+rootTarget); err != nil {
		return err
	}

	diffResult, updatedProbes, err := s.processSingleTarget(rootTarget, resultsForRoot, scanSessionID, urlDiffer)
	if err != nil {
		return s.handleTargetProcessingError(err, rootTarget, scanSessionID)
	}

	if diffResult == nil {
		return nil
	}

	s.updateOutputWithResults(output, rootTarget, diffResult, updatedProbes)
	updateProcessedProbeResults(processedProbeResults, updatedProbes, originalIndicesForTarget)

	return s.writeProbeResultsToParquet(ctx, updatedProbes, scanSessionID, rootTarget)
}

// logSkippingEmptyTarget logs when skipping an empty target.
func (s *Scanner) logSkippingEmptyTarget(scanSessionID string) {
	s.logger.Warn().Str("session_id", scanSessionID).Msg("Skipping diffing/storage for empty root target")
}

// processSingleTarget processes diffing for a single target.
func (s *Scanner) processSingleTarget(rootTarget string, resultsForRoot []models.ProbeResult, scanSessionID string, urlDiffer *differ.UrlDiffer) (*models.URLDiffResult, []models.ProbeResult, error) {
	diffInput := DiffTargetInput{
		RootTarget:            rootTarget,
		ProbeResultsForTarget: resultsForRoot,
		ScanSessionID:         scanSessionID,
		URLDiffer:             urlDiffer,
	}

	return s.diffAndPrepareStorageForTarget(diffInput)
}

// handleTargetProcessingError handles errors that occur during target processing.
func (s *Scanner) handleTargetProcessingError(err error, rootTarget, scanSessionID string) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	s.logger.Error().
		Err(err).
		Str("root_target", rootTarget).
		Str("session_id", scanSessionID).
		Msg("Error in diffAndPrepareStorageForTarget, skipping target")

	return nil // Non-context error, log and continue with other targets
}

// updateOutputWithResults updates the output with diff results and probe data.
func (s *Scanner) updateOutputWithResults(output *ProcessDiffingAndStorageOutput, rootTarget string, diffResult *models.URLDiffResult, updatedProbes []models.ProbeResult) {
	output.URLDiffResults[rootTarget] = *diffResult
	output.AllProbesToStore = append(output.AllProbesToStore, updatedProbes...)
}

// writeProbeResultsToParquet handles the persistence of probe results to Parquet.
// This function is extracted from processDiffingAndStorage to improve separation of concerns (Task 1.2).
func (s *Scanner) writeProbeResultsToParquet(ctx context.Context, probesToStore []models.ProbeResult, scanSessionID string, rootTarget string) error {
	if !s.shouldWriteToParquet(probesToStore, rootTarget, scanSessionID) {
		return nil
	}

	s.logParquetWriteStart(probesToStore, rootTarget, scanSessionID)

	if err := s.parquetWriter.Write(ctx, probesToStore, scanSessionID, rootTarget); err != nil {
		s.logParquetWriteError(err, rootTarget, scanSessionID)
		return err
	}

	return nil
}

// shouldWriteToParquet determines if data should be written to Parquet.
func (s *Scanner) shouldWriteToParquet(probesToStore []models.ProbeResult, rootTarget, scanSessionID string) bool {
	if s.parquetWriter == nil {
		s.logger.Info().
			Str("root_target", rootTarget).
			Str("session_id", scanSessionID).
			Msg("ParquetWriter is not initialized. Skipping Parquet storage for target")
		return false
	}

	if len(probesToStore) == 0 {
		s.logger.Info().
			Str("root_target", rootTarget).
			Str("session_id", scanSessionID).
			Msg("No probe results to store to Parquet for target")
		return false
	}

	return true
}

// logParquetWriteStart logs the start of Parquet writing.
func (s *Scanner) logParquetWriteStart(probesToStore []models.ProbeResult, rootTarget, scanSessionID string) {
	s.logger.Info().
		Int("count", len(probesToStore)).
		Str("root_target", rootTarget).
		Str("session_id", scanSessionID).
		Msg("Writing probe results to Parquet...")
}

// logParquetWriteError logs errors that occur during Parquet writing.
func (s *Scanner) logParquetWriteError(err error, rootTarget, scanSessionID string) {
	s.logger.Error().
		Err(err).
		Str("root_target", rootTarget).
		Str("session_id", scanSessionID).
		Msg("Failed to write Parquet data")
}

// processDiffingAndStorage processes URL diffing and stores results to Parquet.
// Refactored to use ProcessDiffingAndStorageInput and return ProcessDiffingAndStorageOutput.
// This addresses tasks 1.2 (single responsibility by better defining inputs/outputs),
// 1.3 (parameter reduction), and 1.4 (minimizing side effects).
func (s *Scanner) processDiffingAndStorage(input ProcessDiffingAndStorageInput) (ProcessDiffingAndStorageOutput, error) {
	// Create a copy to avoid modifying the original input slice
	processedProbeResults := s.copyProbeResults(input.CurrentScanProbeResults)

	probeResultsByRootTarget, originalIndicesByRootTarget := s.groupProbeResultsByRootTarget(
		processedProbeResults,
		input.PrimaryRootTargetURL,
		input.SeedURLs,
		input.ScanSessionID,
	)

	output := ProcessDiffingAndStorageOutput{
		URLDiffResults: make(map[string]models.URLDiffResult),
	}

	urlDiffer, err := s.createURLDiffer()
	if err != nil {
		return output, err
	}

	err = s.processAllTargetGroups(input, probeResultsByRootTarget, originalIndicesByRootTarget, urlDiffer, processedProbeResults, &output)
	if err != nil {
		output.UpdatedScanProbeResults = processedProbeResults
		return output, err
	}

	output.UpdatedScanProbeResults = processedProbeResults
	return output, nil
}

// copyProbeResults creates a copy of the probe results slice.
func (s *Scanner) copyProbeResults(original []models.ProbeResult) []models.ProbeResult {
	copied := make([]models.ProbeResult, len(original))
	copy(copied, original)
	return copied
}

// createURLDiffer creates a new URL differ instance.
func (s *Scanner) createURLDiffer() (*differ.UrlDiffer, error) {
	urlDiffer, err := differ.NewUrlDiffer(s.parquetReader, s.logger)
	if err != nil {
		return nil, common.WrapError(err, "failed to create URL differ")
	}
	return urlDiffer, nil
}

// processAllTargetGroups processes all target groups for diffing and storage.
func (s *Scanner) processAllTargetGroups(
	input ProcessDiffingAndStorageInput,
	probeResultsByRootTarget map[string][]models.ProbeResult,
	originalIndicesByRootTarget map[string][]int,
	urlDiffer *differ.UrlDiffer,
	processedProbeResults []models.ProbeResult,
	output *ProcessDiffingAndStorageOutput,
) error {
	for rootTarget, resultsForRoot := range probeResultsByRootTarget {
		originalIndicesForTarget := originalIndicesByRootTarget[rootTarget]

		err := s.processTargetGroup(
			input.Ctx,
			rootTarget,
			resultsForRoot,
			originalIndicesForTarget,
			input.ScanSessionID,
			urlDiffer,
			processedProbeResults,
			output,
		)

		if err != nil {
			return err
		}
	}
	return nil
}
