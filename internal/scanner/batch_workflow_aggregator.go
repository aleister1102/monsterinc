package scanner

import (
	"github.com/aleister1102/monsterinc/internal/common/summary"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
)

// aggregateBatchResults aggregates results from individual batches
func (bwo *BatchWorkflowOrchestrator) aggregateBatchResults(
	aggregated *summary.ScanSummaryData,
	batchSummary summary.ScanSummaryData,
) {
	// Aggregate probe stats
	aggregated.ProbeStats.TotalProbed += batchSummary.ProbeStats.TotalProbed
	aggregated.ProbeStats.SuccessfulProbes += batchSummary.ProbeStats.SuccessfulProbes
	aggregated.ProbeStats.FailedProbes += batchSummary.ProbeStats.FailedProbes
	aggregated.ProbeStats.DiscoverableItems += batchSummary.ProbeStats.DiscoverableItems

	// Aggregate diff stats
	aggregated.DiffStats.New += batchSummary.DiffStats.New
	aggregated.DiffStats.Old += batchSummary.DiffStats.Old
	aggregated.DiffStats.Existing += batchSummary.DiffStats.Existing
	aggregated.DiffStats.Changed += batchSummary.DiffStats.Changed

	// Aggregate scan duration
	aggregated.ScanDuration += batchSummary.ScanDuration

	// Collect error messages
	aggregated.ErrorMessages = append(aggregated.ErrorMessages, batchSummary.ErrorMessages...)

	// Add retries attempted
	aggregated.RetriesAttempted += batchSummary.RetriesAttempted
}

// finalizeBatchSummary finalizes the aggregated summary
func (bwo *BatchWorkflowOrchestrator) finalizeBatchSummary(
	summaryData *summary.ScanSummaryData,
	processedBatches int,
	totalBatches int,
	lastError error,
	wasInterrupted bool,
) {
	// Determine final status
	if wasInterrupted {
		if processedBatches == 0 {
			summaryData.Status = string(summary.ScanStatusFailed)
		} else {
			summaryData.Status = string(summary.ScanStatusPartialComplete)
		}
	} else {
		summaryData.Status = string(summary.ScanStatusCompleted)
	}

	// Don't add batch processing info to error messages as it's not an error
	// This information will be displayed in other parts of the notification

	if lastError != nil && !wasInterrupted {
		summaryData.Status = string(summary.ScanStatusFailed)
	}
}

type AggregatedBatchResult struct {
	ProbeResults    []httpxrunner.ProbeResult
	URLDiffResults  map[string]differ.URLDiffResult
	ScanSummaryData summary.ScanSummaryData
	ReportFilePaths []string
	Error           error
}
