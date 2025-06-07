package common

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

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

// ProgressDisplayManager quản lý hiển thị tiến trình
type ProgressDisplayManager struct {
	scanProgress    *ProgressInfo
	monitorProgress *ProgressInfo
	mutex           sync.RWMutex
	logger          zerolog.Logger
	displayTicker   *time.Ticker
	isRunning       bool
	stopChan        chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewProgressDisplayManager tạo progress display manager mới
func NewProgressDisplayManager(logger zerolog.Logger) *ProgressDisplayManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ProgressDisplayManager{
		scanProgress: &ProgressInfo{
			Type:   ProgressTypeScan,
			Status: ProgressStatusIdle,
		},
		monitorProgress: &ProgressInfo{
			Type:   ProgressTypeMonitor,
			Status: ProgressStatusIdle,
		},
		logger:   logger.With().Str("component", "ProgressDisplay").Logger(),
		stopChan: make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start bắt đầu hiển thị progress
func (pdm *ProgressDisplayManager) Start() {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	if pdm.isRunning {
		return
	}

	pdm.isRunning = true
	pdm.displayTicker = time.NewTicker(2 * time.Second) // Cập nhật mỗi 2 giây

	go pdm.displayLoop()
}

// Stop dừng hiển thị progress
func (pdm *ProgressDisplayManager) Stop() {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	if !pdm.isRunning {
		return
	}

	pdm.isRunning = false
	pdm.cancel()

	if pdm.displayTicker != nil {
		pdm.displayTicker.Stop()
	}

	close(pdm.stopChan)
}

// UpdateScanProgress cập nhật tiến trình scan
func (pdm *ProgressDisplayManager) UpdateScanProgress(current, total int64, stage, message string) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	now := time.Now()

	if pdm.scanProgress.Status == ProgressStatusIdle && current > 0 {
		pdm.scanProgress.StartTime = now
		pdm.scanProgress.Status = ProgressStatusRunning
	}

	pdm.scanProgress.Current = current
	pdm.scanProgress.Total = total
	pdm.scanProgress.Stage = stage
	pdm.scanProgress.Message = message
	pdm.scanProgress.LastUpdateTime = now
	pdm.scanProgress.UpdateETA()
}

// UpdateMonitorProgress cập nhật tiến trình monitor
func (pdm *ProgressDisplayManager) UpdateMonitorProgress(current, total int64, stage, message string) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	now := time.Now()

	if pdm.monitorProgress.Status == ProgressStatusIdle && current > 0 {
		pdm.monitorProgress.StartTime = now
		pdm.monitorProgress.Status = ProgressStatusRunning
	}

	pdm.monitorProgress.Current = current
	pdm.monitorProgress.Total = total
	pdm.monitorProgress.Stage = stage
	pdm.monitorProgress.Message = message
	pdm.monitorProgress.LastUpdateTime = now
	pdm.monitorProgress.UpdateETA()
}

// SetScanStatus đặt trạng thái scan
func (pdm *ProgressDisplayManager) SetScanStatus(status ProgressStatus, message string) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	pdm.scanProgress.Status = status
	pdm.scanProgress.Message = message
	pdm.scanProgress.LastUpdateTime = time.Now()
}

// SetMonitorStatus đặt trạng thái monitor
func (pdm *ProgressDisplayManager) SetMonitorStatus(status ProgressStatus, message string) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	pdm.monitorProgress.Status = status
	pdm.monitorProgress.Message = message
	pdm.monitorProgress.LastUpdateTime = time.Now()
}

// UpdateBatchProgress cập nhật thông tin batch processing
func (pdm *ProgressDisplayManager) UpdateBatchProgress(progressType ProgressType, currentBatch, totalBatches int) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	var targetProgress *ProgressInfo
	switch progressType {
	case ProgressTypeScan:
		targetProgress = pdm.scanProgress
	case ProgressTypeMonitor:
		targetProgress = pdm.monitorProgress
	default:
		return
	}

	if targetProgress.BatchInfo == nil {
		targetProgress.BatchInfo = &BatchProgressInfo{}
	}

	targetProgress.BatchInfo.CurrentBatch = currentBatch
	targetProgress.BatchInfo.TotalBatches = totalBatches
	targetProgress.LastUpdateTime = time.Now()
}

// UpdateMonitorEventCounts cập nhật số lượng events cho monitor
func (pdm *ProgressDisplayManager) UpdateMonitorEventCounts(changedEvents, errorEvents int, aggregationInterval time.Duration) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	if pdm.monitorProgress.MonitorInfo == nil {
		pdm.monitorProgress.MonitorInfo = &MonitorProgressInfo{}
	}

	pdm.monitorProgress.MonitorInfo.ChangedEventCount = changedEvents
	pdm.monitorProgress.MonitorInfo.ErrorEventCount = errorEvents
	pdm.monitorProgress.MonitorInfo.AggregationInterval = aggregationInterval
	pdm.monitorProgress.LastUpdateTime = time.Now()
}

// UpdateMonitorStats cập nhật thống kê chi tiết cho monitor
func (pdm *ProgressDisplayManager) UpdateMonitorStats(processed, failed, completed int) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	if pdm.monitorProgress.MonitorInfo == nil {
		pdm.monitorProgress.MonitorInfo = &MonitorProgressInfo{}
	}

	pdm.monitorProgress.MonitorInfo.ProcessedURLs = processed
	pdm.monitorProgress.MonitorInfo.FailedURLs = failed
	pdm.monitorProgress.MonitorInfo.CompletedURLs = completed
	pdm.monitorProgress.LastUpdateTime = time.Now()
}

// displayLoop vòng lặp hiển thị progress
func (pdm *ProgressDisplayManager) displayLoop() {
	for {
		select {
		case <-pdm.ctx.Done():
			return
		case <-pdm.stopChan:
			return
		case <-pdm.displayTicker.C:
			pdm.displayProgress()
		}
	}
}

// displayProgress hiển thị progress lên console
func (pdm *ProgressDisplayManager) displayProgress() {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	// Clear previous lines and move cursor to bottom
	fmt.Print("\033[s")      // Save cursor position
	fmt.Print("\033[999;1H") // Move to bottom of screen

	// Display scan progress
	scanLine := pdm.formatProgressLine(pdm.scanProgress)
	fmt.Printf("📡 SCAN:    %s\r\n", scanLine)

	// Display monitor progress
	monitorLine := pdm.formatProgressLine(pdm.monitorProgress)
	fmt.Printf("🔍 MONITOR: %s\r\n", monitorLine)

	fmt.Print("\033[u") // Restore cursor position
}

// formatProgressLine định dạng một dòng progress
func (pdm *ProgressDisplayManager) formatProgressLine(progress *ProgressInfo) string {
	statusIcon := pdm.getStatusIcon(progress.Status)

	var line strings.Builder

	// Status và stage
	line.WriteString(fmt.Sprintf("%s %s", statusIcon, progress.Status))

	if progress.Stage != "" {
		line.WriteString(fmt.Sprintf(" [%s]", progress.Stage))
	}

	// Batch info
	if progress.BatchInfo != nil && progress.BatchInfo.TotalBatches > 0 {
		line.WriteString(fmt.Sprintf(" [Batch %d/%d]", progress.BatchInfo.CurrentBatch, progress.BatchInfo.TotalBatches))
	}

	// Progress bar và percentage
	if progress.Total > 0 && progress.Status == ProgressStatusRunning {
		percentage := progress.GetPercentage()
		progressBar := pdm.createProgressBar(percentage, 20)
		line.WriteString(fmt.Sprintf(" %s %.1f%% (%d/%d)",
			progressBar, percentage, progress.Current, progress.Total))

		// ETA và rate
		if progress.EstimatedETA > 0 {
			line.WriteString(fmt.Sprintf(" ETA: %s", pdm.formatDuration(progress.EstimatedETA)))
		}

		if progress.ProcessingRate > 0 {
			line.WriteString(fmt.Sprintf(" Rate: %.1f/s", progress.ProcessingRate))
		}
	}

	// Monitor specific stats
	if progress.Type == ProgressTypeMonitor && progress.MonitorInfo != nil {
		mInfo := progress.MonitorInfo
		line.WriteString(fmt.Sprintf(" [P:%d F:%d C:%d]", mInfo.ProcessedURLs, mInfo.FailedURLs, mInfo.CompletedURLs))

		if mInfo.ChangedEventCount > 0 || mInfo.ErrorEventCount > 0 {
			line.WriteString(fmt.Sprintf(" [Events: ±%d ✗%d]", mInfo.ChangedEventCount, mInfo.ErrorEventCount))
		}

		if mInfo.AggregationInterval > 0 {
			line.WriteString(fmt.Sprintf(" [Agg: %s]", pdm.formatDuration(mInfo.AggregationInterval)))
		}
	}

	// Message
	if progress.Message != "" {
		line.WriteString(fmt.Sprintf(" - %s", progress.Message))
	}

	// Elapsed time cho running tasks
	if progress.Status == ProgressStatusRunning && !progress.StartTime.IsZero() {
		elapsed := progress.GetElapsedTime()
		line.WriteString(fmt.Sprintf(" [%s]", pdm.formatDuration(elapsed)))
	}

	return line.String()
}

// getStatusIcon trả về icon cho trạng thái
func (pdm *ProgressDisplayManager) getStatusIcon(status ProgressStatus) string {
	switch status {
	case ProgressStatusIdle:
		return "⏸️"
	case ProgressStatusRunning:
		return "🔄"
	case ProgressStatusComplete:
		return "✅"
	case ProgressStatusError:
		return "❌"
	case ProgressStatusCancelled:
		return "⏹️"
	default:
		return "❓"
	}
}

// createProgressBar tạo thanh progress bar
func (pdm *ProgressDisplayManager) createProgressBar(percentage float64, width int) string {
	filled := int(percentage / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s]", bar)
}

// formatDuration định dạng duration cho hiển thị
func (pdm *ProgressDisplayManager) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm%.0fs", d.Minutes(), d.Seconds()-60*d.Minutes())
	} else {
		return fmt.Sprintf("%.0fh%.0fm", d.Hours(), d.Minutes()-60*d.Hours())
	}
}

// GetCurrentProgress trả về thông tin progress hiện tại
func (pdm *ProgressDisplayManager) GetCurrentProgress() (scanProgress, monitorProgress ProgressInfo) {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	return *pdm.scanProgress, *pdm.monitorProgress
}

// IsAnyServiceRunning kiểm tra có service nào đang chạy không
func (pdm *ProgressDisplayManager) IsAnyServiceRunning() bool {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	return pdm.scanProgress.Status == ProgressStatusRunning ||
		pdm.monitorProgress.Status == ProgressStatusRunning
}
