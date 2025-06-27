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

// ProgressInfo chứa thông tin toàn diện về một progress
type ProgressInfo struct {
	Type           ProgressType         `json:"type"`
	Status         ProgressStatus       `json:"status"`
	Current        int64                `json:"current"`
	Total          int64                `json:"total"`
	Stage          string               `json:"stage"`
	Message        string               `json:"message"`
	StartTime      time.Time            `json:"start_time"`
	LastUpdateTime time.Time            `json:"last_update_time"`
	EstimatedETA   time.Duration        `json:"estimated_eta"`
	BatchInfo      *BatchProgressInfo   `json:"batch_info,omitempty"`
	MonitorInfo    *MonitorProgressInfo `json:"monitor_info,omitempty"`
}

// UpdateETA tính toán thời gian dự kiến hoàn thành (ETA)
func (pi *ProgressInfo) UpdateETA() {
	// Không tính ETA nếu không có tổng số hoặc chưa bắt đầu
	if pi.Total <= 0 || pi.Current <= 0 || pi.Status != ProgressStatusRunning {
		pi.EstimatedETA = 0
		return
	}

	// Thời gian đã trôi qua
	elapsed := time.Since(pi.StartTime)
	if elapsed <= 0 {
		pi.EstimatedETA = 0
		return
	}

	// Tốc độ xử lý (items/second)
	rate := float64(pi.Current) / elapsed.Seconds()
	if rate <= 0 {
		pi.EstimatedETA = 0
		return
	}

	// Số lượng còn lại
	remaining := float64(pi.Total - pi.Current)
	if remaining <= 0 {
		pi.EstimatedETA = 0
		return
	}

	// Thời gian dự kiến còn lại (giây)
	etaSeconds := remaining / rate
	pi.EstimatedETA = time.Duration(etaSeconds * float64(time.Second))
}

// GetPercentage tính toán phần trăm hoàn thành
func (pi *ProgressInfo) GetPercentage() float64 {
	if pi.Total <= 0 {
		return 0.0
	}
	percentage := float64(pi.Current) * 100 / float64(pi.Total)
	if percentage > 100 {
		return 100.0
	}
	return percentage
}
