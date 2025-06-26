package progress

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
	// Thêm thông tin URL tracking cho scan service
	CurrentBatchURLs int `json:"current_batch_urls"` // Số URL trong batch hiện tại
	TotalURLs        int `json:"total_urls"`         // Tổng số URL toàn bộ quá trình
	ProcessedURLs    int `json:"processed_urls"`     // Số URL đã xử lý toàn bộ quá trình
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
