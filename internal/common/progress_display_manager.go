package common

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

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
	lastDisplayed   string // Track last displayed content to avoid duplicates
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
	pdm.displayTicker = time.NewTicker(3 * time.Second) // Tăng thời gian update lên 3 giây để giảm spam

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

	// Clear the progress line and move cursor to new line
	fmt.Print("\r\033[K\n")

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

// UpdateBatchProgress cập nhật thông tin batch
func (pdm *ProgressDisplayManager) UpdateBatchProgress(progressType ProgressType, currentBatch, totalBatches int) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	batchInfo := &BatchProgressInfo{
		CurrentBatch: currentBatch,
		TotalBatches: totalBatches,
	}

	if progressType == ProgressTypeScan {
		pdm.scanProgress.BatchInfo = batchInfo
	} else {
		pdm.monitorProgress.BatchInfo = batchInfo
	}
}

// UpdateMonitorStats cập nhật stats monitor
func (pdm *ProgressDisplayManager) UpdateMonitorStats(processed, failed, completed int) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	if pdm.monitorProgress.MonitorInfo == nil {
		pdm.monitorProgress.MonitorInfo = &MonitorProgressInfo{}
	}

	pdm.monitorProgress.MonitorInfo.ProcessedURLs = processed
	pdm.monitorProgress.MonitorInfo.FailedURLs = failed
	pdm.monitorProgress.MonitorInfo.CompletedURLs = completed
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

// displayProgress hiển thị progress hiện tại
func (pdm *ProgressDisplayManager) displayProgress() {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	var output strings.Builder

	// Scan progress
	if pdm.scanProgress.Status != ProgressStatusIdle {
		percentage := pdm.scanProgress.GetPercentage()
		icon := pdm.getStatusIcon(pdm.scanProgress.Status)
		progressBar := pdm.createProgressBar(percentage, 20)

		output.WriteString(fmt.Sprintf("%s SCAN: %s %.1f%% (%d/%d) %s",
			icon, progressBar, percentage,
			pdm.scanProgress.Current, pdm.scanProgress.Total,
			pdm.scanProgress.Stage))

		if pdm.scanProgress.BatchInfo != nil {
			output.WriteString(fmt.Sprintf(" [Batch %d/%d]",
				pdm.scanProgress.BatchInfo.CurrentBatch+1,
				pdm.scanProgress.BatchInfo.TotalBatches))
		}

		if pdm.scanProgress.EstimatedETA > 0 {
			output.WriteString(fmt.Sprintf(" ETA: %s", pdm.formatDuration(pdm.scanProgress.EstimatedETA)))
		}
		output.WriteString("\n")
	}

	// Monitor progress
	if pdm.monitorProgress.Status != ProgressStatusIdle {
		percentage := pdm.monitorProgress.GetPercentage()
		icon := pdm.getStatusIcon(pdm.monitorProgress.Status)
		progressBar := pdm.createProgressBar(percentage, 20)

		output.WriteString(fmt.Sprintf("%s MONITOR: %s %.1f%% (%d/%d) %s",
			icon, progressBar, percentage,
			pdm.monitorProgress.Current, pdm.monitorProgress.Total,
			pdm.monitorProgress.Stage))

		if pdm.monitorProgress.BatchInfo != nil {
			output.WriteString(fmt.Sprintf(" [Batch %d/%d]",
				pdm.monitorProgress.BatchInfo.CurrentBatch+1,
				pdm.monitorProgress.BatchInfo.TotalBatches))
		}

		if pdm.monitorProgress.MonitorInfo != nil {
			output.WriteString(fmt.Sprintf(" P:%d F:%d C:%d",
				pdm.monitorProgress.MonitorInfo.ProcessedURLs,
				pdm.monitorProgress.MonitorInfo.FailedURLs,
				pdm.monitorProgress.MonitorInfo.CompletedURLs))
		}
		output.WriteString("\n")
	}

	// Only display if content changed
	content := output.String()
	if content != "" && content != pdm.lastDisplayed {
		fmt.Print("\r\033[K" + strings.TrimSuffix(content, "\n"))
		pdm.lastDisplayed = content
	}
}

// getStatusIcon trả về icon cho status
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

// createProgressBar tạo thanh progress
func (pdm *ProgressDisplayManager) createProgressBar(percentage float64, width int) string {
	filled := int(percentage * float64(width) / 100)
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return "[" + bar + "]"
}

// formatDuration format duration thành string dễ đọc
func (pdm *ProgressDisplayManager) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
