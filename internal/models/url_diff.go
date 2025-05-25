package models

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

// DiffedURL represents a URL along with its diff status and potentially its last known data.
type DiffedURL struct {
	NormalizedURL string      `json:"normalized_url"`
	Status        URLStatus   `json:"status"`
	LastSeenData  ProbeResult `json:"last_seen_data,omitempty"` // Used for StatusOld URLs
}

// URLDiffResult represents the result of a URL diff operation for a specific root target.
type URLDiffResult struct {
	RootTargetURL string      `json:"root_target_url"`
	Results       []DiffedURL `json:"results"`
}
 