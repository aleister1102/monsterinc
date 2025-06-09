package common

import (
	"time"
)

// UpdateScanProgress cập nhật tiến trình scan
func (pdm *ProgressDisplayManager) UpdateScanProgress(current, total int64, stage, message string) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	now := time.Now()

	// Initialize or reset for new progress tracking
	if pdm.scanProgress.Status == ProgressStatusIdle || current == 0 {
		pdm.scanProgress.StartTime = now
		pdm.scanProgress.Status = ProgressStatusRunning
	}

	// Reset start time for batch progress to get accurate ETA per batch
	if pdm.scanProgress.BatchInfo != nil && pdm.scanProgress.Current == 0 && current > 0 {
		pdm.scanProgress.StartTime = now
	}

	pdm.scanProgress.Current = current
	pdm.scanProgress.Total = total
	pdm.scanProgress.Stage = stage
	pdm.scanProgress.Message = message
	pdm.scanProgress.LastUpdateTime = now
	pdm.scanProgress.UpdateETA()

	// Only trigger immediate display for significant updates to avoid spam
	if current == 0 || current == total || current%1 == 0 {
		// Use the existing display loop frequency to avoid spam
		select {
		case <-pdm.stopChan:
			return
		default:
			// Trigger display without creating new goroutine
		}
	}
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

	// Immediate display on status change
	go func() {
		time.Sleep(100 * time.Millisecond) // Small delay to ensure lock is released
		pdm.displayProgress()
	}()
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

// UpdateBatchProgressWithURLs cập nhật thông tin batch với URL tracking
func (pdm *ProgressDisplayManager) UpdateBatchProgressWithURLs(progressType ProgressType, currentBatch, totalBatches, currentBatchURLs, totalURLs, processedURLs int) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	batchInfo := &BatchProgressInfo{
		CurrentBatch:     currentBatch,
		TotalBatches:     totalBatches,
		CurrentBatchURLs: currentBatchURLs,
		TotalURLs:        totalURLs,
		ProcessedURLs:    processedURLs,
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

// ResetBatchProgress resets progress for a new batch with fresh timing
func (pdm *ProgressDisplayManager) ResetBatchProgress(progressType ProgressType, currentBatch, totalBatches int, stage, message string) {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	now := time.Now()

	if progressType == ProgressTypeScan {
		pdm.scanProgress.Current = 0
		pdm.scanProgress.Total = 5 // Standard workflow steps
		pdm.scanProgress.Stage = stage
		pdm.scanProgress.Message = message
		pdm.scanProgress.StartTime = now // Reset timer for accurate ETA
		pdm.scanProgress.LastUpdateTime = now
		pdm.scanProgress.EstimatedETA = 0
		pdm.scanProgress.Status = ProgressStatusRunning

		// Update batch info
		pdm.scanProgress.BatchInfo = &BatchProgressInfo{
			CurrentBatch: currentBatch,
			TotalBatches: totalBatches,
		}
	} else {
		pdm.monitorProgress.Current = 0
		pdm.monitorProgress.StartTime = now
		pdm.monitorProgress.LastUpdateTime = now
		pdm.monitorProgress.EstimatedETA = 0
		pdm.monitorProgress.Status = ProgressStatusRunning

		if pdm.monitorProgress.BatchInfo == nil {
			pdm.monitorProgress.BatchInfo = &BatchProgressInfo{}
		}
		pdm.monitorProgress.BatchInfo.CurrentBatch = currentBatch
		pdm.monitorProgress.BatchInfo.TotalBatches = totalBatches
	}
}
