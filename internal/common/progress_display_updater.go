package common

import (
	"time"
)

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
