package scanner

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/monsterinc/httpx"
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
	Write(ctx context.Context, probeResults []httpx.ProbeResult, scanSessionID string, rootTarget string) error
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
	ProbeResultsForTarget []httpx.ProbeResult
	ScanSessionID         string
	URLDiffer             *differ.UrlDiffer
}

// DiffTargetResult contains the results from processing a single target
type DiffTargetResult struct {
	URLDiffResult *models.URLDiffResult
	ProbeResults  []httpx.ProbeResult
	Error         error
}

// DiffHostnameInput contains the input for processing a single hostname
type DiffHostnameInput struct {
	Hostname                string
	ProbeResultsForHostname []httpx.ProbeResult
	ScanSessionID           string
	URLDiffer               *differ.UrlDiffer
}

// DiffHostnameResult contains the result of processing a single hostname
type DiffHostnameResult struct {
	URLDiffResult *models.URLDiffResult
	ProbeResults  []httpx.ProbeResult
	Error         error
}

// ProcessTarget performs diffing and prepares storage for a single target
func (dsp *DiffStorageProcessor) ProcessTarget(input DiffTargetInput) *DiffTargetResult {
	result := &DiffTargetResult{
		ProbeResults: input.ProbeResultsForTarget,
	}

	// Perform URL diffing for this target
	convertedProbes := make([]*httpx.ProbeResult, len(input.ProbeResultsForTarget))
	for i := range input.ProbeResultsForTarget {
		convertedProbes[i] = &input.ProbeResultsForTarget[i]
	}

	urlDiffResult, err := input.URLDiffer.Compare(convertedProbes, input.RootTarget, input.ScanSessionID)
	if err != nil {
		dsp.logger.Warn().Err(err).Str("root_target", input.RootTarget).Msg("URL diffing failed")
		result.Error = err
		return result
	}

	result.URLDiffResult = urlDiffResult

	// Apply diff status to probe results
	for i, probe := range result.ProbeResults {
		// Find corresponding diff result
		for _, diffURL := range urlDiffResult.Results {
			if diffURL.ProbeResult.InputURL == probe.InputURL {
				result.ProbeResults[i].URLStatus = string(diffURL.ProbeResult.URLStatus)
				break
			}
		}
	}

	return result
}

// ProcessHostname performs diffing and prepares storage for a single hostname
func (dsp *DiffStorageProcessor) ProcessHostname(input DiffHostnameInput) *DiffHostnameResult {
	result := &DiffHostnameResult{
		ProbeResults: input.ProbeResultsForHostname,
	}

	// Perform URL diffing for this hostname
	convertedProbes := make([]*httpx.ProbeResult, len(input.ProbeResultsForHostname))
	for i := range input.ProbeResultsForHostname {
		convertedProbes[i] = &input.ProbeResultsForHostname[i]
	}

	urlDiffResult, err := input.URLDiffer.Compare(convertedProbes, input.Hostname, input.ScanSessionID)
	if err != nil {
		dsp.logger.Warn().Err(err).Str("hostname", input.Hostname).Msg("URL diffing failed")
		result.Error = err
		return result
	}

	result.URLDiffResult = urlDiffResult

	// Apply diff status to probe results
	for i, probe := range result.ProbeResults {
		// Find corresponding diff result
		for _, diffURL := range urlDiffResult.Results {
			if diffURL.ProbeResult.InputURL == probe.InputURL {
				result.ProbeResults[i].URLStatus = string(diffURL.ProbeResult.URLStatus)
				break
			}
		}
	}

	return result
}

// ProcessDiffingAndStorageInput contains parameters for processing multiple targets
type ProcessDiffingAndStorageInput struct {
	Ctx                     context.Context
	CurrentScanProbeResults []httpx.ProbeResult
	SeedURLs                []string
	PrimaryRootTargetURL    string
	ScanSessionID           string
}

// ProcessDiffingAndStorageOutput contains results from processing multiple targets
type ProcessDiffingAndStorageOutput struct {
	URLDiffResults          map[string]models.URLDiffResult
	UpdatedScanProbeResults []httpx.ProbeResult
	AllProbesToStore        []httpx.ProbeResult
}

// ProcessDiffingAndStorage handles diffing and storage for all targets
func (dsp *DiffStorageProcessor) ProcessDiffingAndStorage(input ProcessDiffingAndStorageInput) (ProcessDiffingAndStorageOutput, error) {
	// Early context cancellation check
	if cancelled := CheckCancellation(input.Ctx); cancelled.Cancelled {
		dsp.logger.Info().Msg("Context cancelled before diff processing")
		return ProcessDiffingAndStorageOutput{}, cancelled.Error
	}

	// Group probe results by hostname for efficient processing
	probeResultsByHostname, originalIndicesMapByHostname := dsp.groupProbeResultsByHostname(input.CurrentScanProbeResults)

	// Initialize output structure
	output := ProcessDiffingAndStorageOutput{
		URLDiffResults:          make(map[string]models.URLDiffResult),
		UpdatedScanProbeResults: make([]httpx.ProbeResult, len(input.CurrentScanProbeResults)),
		AllProbesToStore:        make([]httpx.ProbeResult, 0, len(input.CurrentScanProbeResults)),
	}

	// Copy original results to avoid mutation
	copy(output.UpdatedScanProbeResults, input.CurrentScanProbeResults)

	// Process each hostname group
	for hostname, resultsForHostname := range probeResultsByHostname {
		// Check context cancellation before processing each hostname group
		if cancelled := CheckCancellation(input.Ctx); cancelled.Cancelled {
			dsp.logger.Info().Str("hostname", hostname).Msg("Context cancelled during hostname group processing")
			return output, cancelled.Error
		}

		originalIndicesForHostname := originalIndicesMapByHostname[hostname]

		if err := dsp.processHostnameGroup(
			input.Ctx,
			hostname,
			resultsForHostname,
			originalIndicesForHostname,
			input.ScanSessionID,
			output.UpdatedScanProbeResults,
			&output,
		); err != nil {
			// Check if error is due to context cancellation
			if input.Ctx.Err() != nil {
				dsp.logger.Info().Str("hostname", hostname).Msg("Hostname group processing cancelled")
				return output, input.Ctx.Err()
			}

			dsp.logger.Error().Err(err).Str("hostname", hostname).Msg("Failed to process hostname group")
			continue // Continue with other hostnames
		}
	}

	// Final context check before returning results
	if cancelled := CheckCancellation(input.Ctx); cancelled.Cancelled {
		dsp.logger.Info().Msg("Context cancelled after diff processing completion")
		return output, cancelled.Error
	}

	dsp.logger.Info().
		Int("total_hostnames", len(probeResultsByHostname)).
		Int("total_probes", len(output.AllProbesToStore)).
		Msg("Diff processing and storage completed")

	return output, nil
}

// processHostnameGroup processes a single hostname group
func (dsp *DiffStorageProcessor) processHostnameGroup(
	ctx context.Context,
	hostname string,
	resultsForHostname []httpx.ProbeResult,
	originalIndicesForHostname []int,
	scanSessionID string,
	processedProbeResults []httpx.ProbeResult,
	output *ProcessDiffingAndStorageOutput,
) error {
	// Check context cancellation at the start of hostname processing
	if cancelled := CheckCancellation(ctx); cancelled.Cancelled {
		dsp.logger.Info().Str("hostname", hostname).Msg("Context cancelled at start of hostname processing")
		return cancelled.Error
	}

	dsp.logger.Debug().
		Str("hostname", hostname).
		Int("probe_count", len(resultsForHostname)).
		Msg("Processing hostname group")

	// Perform URL diffing for this hostname
	diffInput := DiffHostnameInput{
		Hostname:                hostname,
		ProbeResultsForHostname: resultsForHostname,
		ScanSessionID:           scanSessionID,
		URLDiffer:               dsp.urlDiffer,
	}

	diffResult := dsp.ProcessHostname(diffInput)
	if diffResult.Error != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			dsp.logger.Info().Str("hostname", hostname).Msg("URL diffing cancelled")
			return ctx.Err()
		}

		dsp.logger.Warn().Err(diffResult.Error).Str("hostname", hostname).Msg("URL diffing failed, continuing")
	} else {
		output.URLDiffResults[hostname] = *diffResult.URLDiffResult
	}

	// Check context cancellation before storage operations
	if cancelled := CheckCancellation(ctx); cancelled.Cancelled {
		dsp.logger.Info().Str("hostname", hostname).Msg("Context cancelled before storage operations")
		return cancelled.Error
	}

	// Update processed results with diff information
	updatedProbesForHostnameStorage := diffResult.ProbeResults
	dsp.updateProcessedProbeResults(processedProbeResults, updatedProbesForHostnameStorage, originalIndicesForHostname)

	// Store results in Parquet - use hostname instead of root target
	if err := dsp.writeProbeResultsToParquet(ctx, updatedProbesForHostnameStorage, scanSessionID, hostname); err != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			dsp.logger.Info().Str("hostname", hostname).Msg("Parquet storage cancelled")
			return ctx.Err()
		}

		dsp.logger.Warn().Err(err).Str("hostname", hostname).Msg("Failed to write to Parquet, continuing")
	}

	// Add to all probes for potential reporting
	output.AllProbesToStore = append(output.AllProbesToStore, updatedProbesForHostnameStorage...)

	// Final context check
	if cancelled := CheckCancellation(ctx); cancelled.Cancelled {
		dsp.logger.Info().Str("hostname", hostname).Msg("Context cancelled after hostname processing")
		return cancelled.Error
	}

	return nil
}

// writeProbeResultsToParquet handles the persistence of probe results to Parquet
func (dsp *DiffStorageProcessor) writeProbeResultsToParquet(ctx context.Context, probesToStore []httpx.ProbeResult, scanSessionID, hostname string) error {
	if dsp.parquetWriter == nil {
		return nil
	}

	if len(probesToStore) == 0 {
		return nil
	}

	if err := dsp.parquetWriter.Write(ctx, probesToStore, scanSessionID, hostname); err != nil {
		dsp.logger.Error().
			Err(err).
			Str("hostname", hostname).
			Str("session_id", scanSessionID).
			Msg("Failed to write Parquet data")
		return err
	}

	return nil
}

// updateProcessedProbeResults updates the processed results with changes from diffing
func (dsp *DiffStorageProcessor) updateProcessedProbeResults(
	processedProbeResults []httpx.ProbeResult,
	updatedProbesForTargetStorage []httpx.ProbeResult,
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
	currentScanProbeResults []httpx.ProbeResult,
	primaryRootTargetURL string,
) (map[string][]httpx.ProbeResult, map[string][]int) {
	targetGroups := make(map[string][]httpx.ProbeResult)
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

// groupProbeResultsByHostname groups probe results by their hostname
func (dsp *DiffStorageProcessor) groupProbeResultsByHostname(
	currentScanProbeResults []httpx.ProbeResult,
) (map[string][]httpx.ProbeResult, map[string][]int) {
	targetGroups := make(map[string][]httpx.ProbeResult)
	originalIndices := make(map[string][]int)

	for i, result := range currentScanProbeResults {
		// Extract hostname from InputURL
		hostname := dsp.extractHostnameFromURL(result.InputURL)
		if hostname == "" {
			// Fallback to "unknown" if hostname extraction fails
			hostname = "unknown"
		}

		targetGroups[hostname] = append(targetGroups[hostname], result)
		originalIndices[hostname] = append(originalIndices[hostname], i)
	}

	return targetGroups, originalIndices
}

// extractHostnameFromURL extracts hostname from URL
func (dsp *DiffStorageProcessor) extractHostnameFromURL(urlStr string) string {
	if hostname, err := urlhandler.ExtractHostname(urlStr); err == nil {
		return hostname
	}
	return ""
}
