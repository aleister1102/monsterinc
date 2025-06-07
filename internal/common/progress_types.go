package common

import "time"

// ProgressType định nghĩa loại progress
type ProgressType string

const (
	ProgressTypeScan    ProgressType = "SCAN"
	ProgressTypeMonitor ProgressType = "MONITOR"
)

// ProgressStatus định nghĩa trạng thái của progress
type ProgressStatus string

const (
	ProgressStatusIdle      ProgressStatus = "IDLE"
	ProgressStatusRunning   ProgressStatus = "RUNNING"
	ProgressStatusComplete  ProgressStatus = "COMPLETE"
	ProgressStatusError     ProgressStatus = "ERROR"
	ProgressStatusCancelled ProgressStatus = "CANCELLED"
)

// BatchProgressInfo chứa thông tin về batch processing
type BatchProgressInfo struct {
	CurrentBatch int `json:"current_batch"`
	TotalBatches int `json:"total_batches"`
}

// MonitorProgressInfo chứa thông tin chi tiết về monitoring
type MonitorProgressInfo struct {
	ProcessedURLs       int           `json:"processed_urls"`
	FailedURLs          int           `json:"failed_urls"`
	CompletedURLs       int           `json:"completed_urls"`
	ChangedEventCount   int           `json:"changed_event_count"`
	ErrorEventCount     int           `json:"error_event_count"`
	AggregationInterval time.Duration `json:"aggregation_interval"`
}
