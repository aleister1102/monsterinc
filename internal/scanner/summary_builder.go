package scanner

import (
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// buildScanSummary populates a ScanSummaryData object based on scan execution details.
func (s *Scanner) buildScanSummary(
	input ScanWorkflowInput,
	result ScanWorkflowResult,
) models.ScanSummaryData {
	scanDuration := time.Since(input.StartTime)
	summaryData := models.GetDefaultScanSummaryData()

	s.populateBasicSummaryData(&summaryData, input, scanDuration)
	s.populateProbeStats(&summaryData, result.ProbeResults)
	s.populateDiffStats(&summaryData, result.URLDiffResults)
	s.setSummaryStatus(&summaryData, result.WorkflowError)

	return summaryData
}

// populateBasicSummaryData sets the basic information in the summary data.
func (s *Scanner) populateBasicSummaryData(summaryData *models.ScanSummaryData, input ScanWorkflowInput, scanDuration time.Duration) {
	summaryData.ScanSessionID = input.ScanSessionID
	summaryData.TargetSource = input.TargetSource
	summaryData.Targets = input.Targets
	summaryData.TotalTargets = len(input.Targets)
	summaryData.ScanDuration = scanDuration
}

// populateProbeStats calculates and sets probe statistics.
func (s *Scanner) populateProbeStats(summaryData *models.ScanSummaryData, probeResults []models.ProbeResult) {
	if probeResults == nil {
		return
	}

	summaryData.ProbeStats.DiscoverableItems = len(probeResults)

	for _, probeResult := range probeResults {
		if s.isSuccessfulProbe(probeResult) {
			summaryData.ProbeStats.SuccessfulProbes++
		} else {
			summaryData.ProbeStats.FailedProbes++
		}
	}
}

// isSuccessfulProbe determines if a probe result is considered successful.
func (s *Scanner) isSuccessfulProbe(probeResult models.ProbeResult) bool {
	return probeResult.Error == "" &&
		(probeResult.StatusCode < 400 || (probeResult.StatusCode >= 300 && probeResult.StatusCode < 400))
}

// populateDiffStats calculates and sets diff statistics.
func (s *Scanner) populateDiffStats(summaryData *models.ScanSummaryData, urlDiffResults map[string]models.URLDiffResult) {
	if urlDiffResults == nil {
		return
	}

	for _, diffResult := range urlDiffResults {
		summaryData.DiffStats.New += diffResult.New
		summaryData.DiffStats.Old += diffResult.Old
		summaryData.DiffStats.Existing += diffResult.Existing
	}
}

// setSummaryStatus sets the status of the summary based on workflow error.
func (s *Scanner) setSummaryStatus(summaryData *models.ScanSummaryData, workflowError error) {
	if workflowError != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{
			fmt.Sprintf("Scan workflow execution failed: %v", workflowError),
		}
	} else {
		summaryData.Status = string(models.ScanStatusCompleted)
	}
}
