package scanner

import (
	"context"
	"time"

	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
)

// ScanWorkflowInput holds the input parameters for building a ScanSummaryData object.
// It encapsulates details about the scan session and targets.
type ScanWorkflowInput struct {
	ScanSessionID string    // Unique identifier for the current scan session
	TargetSource  string    // Describes the origin of the scan targets (e.g., file name, configuration key)
	Targets       []string  // The list of initial target URLs or identifiers for the scan
	StartTime     time.Time // The time when the scan workflow started
}

// ScanWorkflowResult holds the output parameters from a scan workflow execution,
// used for building a ScanSummaryData object.
type ScanWorkflowResult struct {
	ProbeResults   []models.ProbeResult            // Results from the probing phase of the scan
	URLDiffResults map[string]models.URLDiffResult // Results from the URL diffing phase, keyed by root target URL
	WorkflowError  error                           // Any error that occurred during the overall workflow execution
}

// HTTPXProbingInput holds the parameters for executeHTTPXProbing.
// This reduces the number of direct parameters to the function and makes
// it easier to add or modify parameters in the future.
type HTTPXProbingInput struct {
	DiscoveredURLs       []string
	SeedURLs             []string
	PrimaryRootTargetURL string
	ScanSessionID        string
	HttpxRunnerConfig    *httpxrunner.Config
}

// DiffTargetInput holds parameters for diffAndPrepareStorageForTarget.
// This encapsulates inputs for clarity and easier future modification.
type DiffTargetInput struct {
	RootTarget            string
	ProbeResultsForTarget []models.ProbeResult
	ScanSessionID         string
	URLDiffer             *differ.UrlDiffer
}

// ProcessDiffingAndStorageInput holds the parameters for processDiffingAndStorage.
// This follows the principle of grouping parameters into a struct.
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
type ProcessDiffingAndStorageOutput struct {
	URLDiffResults          map[string]models.URLDiffResult
	UpdatedScanProbeResults []models.ProbeResult // The original slice with updated URLStatus and OldestScanTimestamp
	AllProbesToStore        []models.ProbeResult // All probes that are candidates for writing, after diffing and status updates
}

// WorkflowResult encapsulates the result of a scan workflow execution.
type WorkflowResult struct {
	SummaryData    models.ScanSummaryData
	ProbeResults   []models.ProbeResult
	URLDiffResults map[string]models.URLDiffResult
}
