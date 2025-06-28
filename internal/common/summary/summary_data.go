package summary

import "time"

// Discord color constants for different types of notifications
const (
	DiscordColorSuccess   = 0x00ff00 // Green
	DiscordColorError     = 0xff0000 // Red
	DiscordColorWarning   = 0xffa500 // Orange
	DiscordColorInfo      = 0x0099ff // Blue
	DiscordColorDefault   = 0x36393f // Discord default gray
	DiscordColorCritical  = 0x8b0000 // Dark red
	DiscordColorCompleted = 0x228b22 // Forest green
)

// ScanSummaryData holds all relevant information about a scan to be used in notifications.
type ScanSummaryData struct {
	ScanSessionID    string        // Unique identifier for the scan session (e.g., YYYYMMDD-HHMMSS timestamp)
	TargetSource     string        // The source of the targets (e.g., file path, "config_input_urls")
	ScanMode         string        // Mode of the scan (e.g., "onetime", "automated")
	Targets          []string      // List of original target URLs/identifiers
	TotalTargets     int           // Total number of targets processed or attempted
	ProbeStats       ProbeStats    // Statistics from the probing phase
	DiffStats        DiffStats     // Statistics from the diffing phase (New, Old, Existing)
	ScanDuration     time.Duration // Total duration of the scan
	ReportPath       string        // Filesystem path to the generated report (used by notifier to attach)
	Status           string        // Overall status: "COMPLETED", "FAILED", "STARTED", "INTERRUPTED", "PARTIAL_COMPLETE"
	ErrorMessages    []string      // Any critical errors encountered during the scan
	Component        string        // Component where an error might have occurred (for critical errors)
	RetriesAttempted int           // Number of retries, if applicable
	CycleMinutes     int           // Cycle interval in minutes (only for automated mode)
}

// GetDefaultScanSummaryData initializes a ScanSummaryData with default/empty values.
func GetDefaultScanSummaryData() ScanSummaryData {
	return ScanSummaryData{
		ScanSessionID: "",
		TargetSource:  "Unknown",
		ScanMode:      "Unknown",
		Targets:       []string{},
		TotalTargets:  0,
		ProbeStats:    ProbeStats{},
		DiffStats:     DiffStats{},
		Status:        string(ScanStatusUnknown), // Default to unknown status
	}
}

// ProbeStats holds statistics related to the probing phase of a scan.
type ProbeStats struct {
	TotalProbed       int // Total URLs sent to the prober
	SuccessfulProbes  int // Number of probes that returned a successful response (e.g., 2xx)
	FailedProbes      int // Number of probes that failed or returned error codes
	DiscoverableItems int // e.g. number of items from httpx
}

// DiffStats holds statistics related to the diffing phase of a scan.
type DiffStats struct {
	New      int
	Old      int
	Existing int
	Changed  int // (If StatusChanged is implemented)
}

// FileChangeInfo holds information about a single file change for aggregation.
type FileChangeInfo struct {
	URL            string
	OldHash        string
	NewHash        string
	ContentType    string
	ChangeTime     time.Time  // Time the change was detected
	DiffReportPath *string    // Path to the generated HTML diff report for this specific change
	CycleID        string     // Unique identifier for the monitoring cycle
	BatchInfo      *BatchInfo `json:"batch_info,omitempty"` // Information about the batch this change belongs to
}

// BatchInfo holds information about a monitoring batch
type BatchInfo struct {
	BatchNumber      int `json:"batch_number"`       // Current batch number (1-based)
	TotalBatches     int `json:"total_batches"`      // Total number of batches in the cycle
	BatchSize        int `json:"batch_size"`         // Number of URLs in this batch
	ProcessedInBatch int `json:"processed_in_batch"` // Number of URLs processed in this batch so far
}

// NewBatchInfo creates a new BatchInfo instance
func NewBatchInfo(batchNumber, totalBatches, batchSize, processedInBatch int) *BatchInfo {
	return &BatchInfo{
		BatchNumber:      batchNumber,
		TotalBatches:     totalBatches,
		BatchSize:        batchSize,
		ProcessedInBatch: processedInBatch,
	}
}
