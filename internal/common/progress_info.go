package common

import "time"

// ProgressInfo chứa thông tin về tiến trình
type ProgressInfo struct {
	Type           ProgressType   `json:"type"`
	Status         ProgressStatus `json:"status"`
	Current        int64          `json:"current"`
	Total          int64          `json:"total"`
	Stage          string         `json:"stage"`
	Message        string         `json:"message"`
	StartTime      time.Time      `json:"start_time"`
	LastUpdateTime time.Time      `json:"last_update_time"`
	EstimatedETA   time.Duration  `json:"estimated_eta"`
	ProcessingRate float64        `json:"processing_rate"` // items per second

	// Batch processing info
	BatchInfo *BatchProgressInfo `json:"batch_info,omitempty"`

	// Monitor specific info
	MonitorInfo *MonitorProgressInfo `json:"monitor_info,omitempty"`
}

// GetPercentage tính phần trăm hoàn thành
func (pi *ProgressInfo) GetPercentage() float64 {
	if pi.Total == 0 {
		return 0
	}
	return float64(pi.Current) / float64(pi.Total) * 100
}

// GetElapsedTime tính thời gian đã trôi qua
func (pi *ProgressInfo) GetElapsedTime() time.Duration {
	return time.Since(pi.StartTime)
}

// UpdateETA cập nhật thời gian ước tính hoàn thành
func (pi *ProgressInfo) UpdateETA() {
	if pi.Current == 0 || pi.Total == 0 {
		pi.EstimatedETA = 0
		return
	}

	elapsed := pi.GetElapsedTime()
	remaining := pi.Total - pi.Current

	if pi.Current > 0 {
		avgTimePerItem := elapsed / time.Duration(pi.Current)
		pi.EstimatedETA = avgTimePerItem * time.Duration(remaining)
		pi.ProcessingRate = float64(pi.Current) / elapsed.Seconds()
	}
}
