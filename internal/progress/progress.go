package progress

import (
	"sync"
	"time"
)

// Progress encapsulates a single progress indicator.
type Progress struct {
	mu   sync.RWMutex
	info ProgressInfo
}

// NewProgress creates a new Progress indicator.
func NewProgress(progressType ProgressType) *Progress {
	return &Progress{
		info: ProgressInfo{
			Type:   progressType,
			Status: ProgressStatusIdle,
		},
	}
}

// Info returns a copy of the ProgressInfo.
func (p *Progress) Info() ProgressInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.info
}

// Update updates the progress.
func (p *Progress) Update(current, total int64, stage, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	if p.info.Status == ProgressStatusIdle || current == 0 {
		p.info.StartTime = now
		p.info.Status = ProgressStatusRunning
	}

	p.info.Current = current
	p.info.Total = total
	p.info.Stage = stage
	p.info.Message = message
	p.info.LastUpdateTime = now
	p.info.UpdateETA()
}

// SetStatus sets the progress status.
func (p *Progress) SetStatus(status ProgressStatus, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.info.Status = status
	p.info.Message = message
	p.info.LastUpdateTime = time.Now()
}

// UpdateBatch updates the batch information.
func (p *Progress) UpdateBatch(currentBatch, totalBatches int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.info.BatchInfo == nil {
		p.info.BatchInfo = &BatchProgressInfo{}
	}
	p.info.BatchInfo.CurrentBatch = currentBatch
	p.info.BatchInfo.TotalBatches = totalBatches
}

// UpdateBatchWithURLs updates batch information with URL tracking.
func (p *Progress) UpdateBatchWithURLs(currentBatch, totalBatches, currentBatchURLs, totalURLs, processedURLs int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.info.BatchInfo == nil {
		p.info.BatchInfo = &BatchProgressInfo{}
	}
	p.info.BatchInfo.CurrentBatch = currentBatch
	p.info.BatchInfo.TotalBatches = totalBatches
	p.info.BatchInfo.CurrentBatchURLs = currentBatchURLs
	p.info.BatchInfo.TotalURLs = totalURLs
	p.info.BatchInfo.ProcessedURLs = processedURLs
}

// UpdateMonitorStats updates the monitor statistics.
func (p *Progress) UpdateMonitorStats(processed, failed, completed int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.info.MonitorInfo == nil {
		p.info.MonitorInfo = &MonitorProgressInfo{}
	}
	p.info.MonitorInfo.ProcessedURLs = processed
	p.info.MonitorInfo.FailedURLs = failed
	p.info.MonitorInfo.CompletedURLs = completed
}

// ResetBatch resets progress for a new batch.
func (p *Progress) ResetBatch(currentBatch, totalBatches int, stage, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	p.info.Current = int64(currentBatch - 1)
	p.info.Total = int64(totalBatches)
	p.info.Stage = stage
	p.info.Message = message
	p.info.StartTime = now
	p.info.LastUpdateTime = now
	p.info.EstimatedETA = 0
	p.info.Status = ProgressStatusRunning

	if p.info.BatchInfo == nil {
		p.info.BatchInfo = &BatchProgressInfo{}
	}
	p.info.BatchInfo.CurrentBatch = currentBatch
	p.info.BatchInfo.TotalBatches = totalBatches
}

// UpdateWorkflow updates the workflow progress inside a batch.
func (p *Progress) UpdateWorkflow(current, total int64, stage, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	if p.info.BatchInfo != nil {
		p.info.Stage = stage
		p.info.Message = message
		p.info.LastUpdateTime = now
	} else {
		if p.info.Status == ProgressStatusIdle || current == 0 {
			p.info.StartTime = now
			p.info.Status = ProgressStatusRunning
		}
		p.info.Current = current
		p.info.Total = total
		p.info.Stage = stage
		p.info.Message = message
		p.info.LastUpdateTime = now
		p.info.UpdateETA()
	}
}
