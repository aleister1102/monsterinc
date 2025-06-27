package models

import httpx "github.com/aleister1102/monsterinc/internal/httpxrunner"

// URLStatus represents the status of a URL in a diff comparison.
type URLStatus string

const (
	// StatusNew indicates a URL that is present in the current scan but not in the previous scan.
	StatusNew URLStatus = "new"
	// StatusOld indicates a URL that was present in the previous scan but not in the current scan.
	StatusOld URLStatus = "old"
	// StatusExisting indicates a URL that is present in both the current and previous scans.
	StatusExisting URLStatus = "existing"
)

// DiffedURL represents a URL that has been compared and its status determined.
// It now directly embeds/references ProbeResult which holds all necessary data including its status.
type DiffedURL struct {
	ProbeResult httpx.ProbeResult // Embeds or references ProbeResult, which includes URLStatus and other details
	// Status URLStatus // Removed: Status is now part of ProbeResult
	// OldestScanTimestamp *time.Time // Removed: This information, if needed, should be part of ProbeResult (e.g., LastSeen)
}

// URLDiffResult represents the result of a URL diff operation for a specific root target.
type URLDiffResult struct {
	RootTargetURL string      `json:"root_target_url"`
	Results       []DiffedURL `json:"results,omitempty"` // Keep omitempty if results can be nil/empty
	New           int         `json:"new"`
	Old           int         `json:"old"`
	Existing      int         `json:"existing"`
	Error         string      `json:"error,omitempty"`
}

// CountStatuses counts the number of DiffedURL entries in Results that match the given status.
func (udr *URLDiffResult) CountStatuses(statusToMatch URLStatus) int {
	if udr == nil || udr.Results == nil {
		return 0
	}
	count := 0
	for _, r := range udr.Results {
		// URLStatus in ProbeResult is string, so cast statusToMatch to string for comparison
		if r.ProbeResult.URLStatus == string(statusToMatch) {
			count++
		}
	}
	return count
}
