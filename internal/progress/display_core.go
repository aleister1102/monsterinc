package progress

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ProgressDisplayConfig chứa cấu hình cho progress display
type ProgressDisplayConfig struct {
	DisplayInterval   time.Duration
	EnableProgress    bool
	ShowETAEstimation bool
}

// ProgressDisplayManager quản lý hiển thị tiến trình
type ProgressDisplayManager struct {
	scanProgress    *Progress
	monitorProgress *Progress
	mutex           sync.RWMutex
	logger          zerolog.Logger
	displayTicker   *time.Ticker
	isRunning       bool
	stopChan        chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	lastDisplayed   string // Track last displayed content to avoid duplicates
	config          *ProgressDisplayConfig
	triggerDisplay  chan struct{}
}

// NewProgressDisplayManager tạo progress display manager mới
func NewProgressDisplayManager(logger zerolog.Logger, config *ProgressDisplayConfig) *ProgressDisplayManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Use default config if nil
	if config == nil {
		config = &ProgressDisplayConfig{
			DisplayInterval:   3 * time.Second,
			EnableProgress:    true,
			ShowETAEstimation: true,
		}
	}

	return &ProgressDisplayManager{
		scanProgress:    NewProgress(ProgressTypeScan),
		monitorProgress: NewProgress(ProgressTypeMonitor),
		logger:          logger.With().Str("component", "ProgressDisplay").Logger(),
		stopChan:        make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
		triggerDisplay:  make(chan struct{}, 1), // Buffered channel
	}
}

// Start bắt đầu hiển thị progress
func (pdm *ProgressDisplayManager) Start() {
	pdm.mutex.Lock()
	defer pdm.mutex.Unlock()

	if pdm.isRunning {
		return
	}

	// Check if progress is enabled
	if !pdm.config.EnableProgress {
		pdm.logger.Debug().Msg("Progress display disabled in configuration")
		return
	}

	pdm.isRunning = true
	pdm.displayTicker = time.NewTicker(pdm.config.DisplayInterval)

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

// GetMonitorProgress returns a copy of current monitor progress info
func (pdm *ProgressDisplayManager) GetMonitorProgress() ProgressInfo {
	return pdm.monitorProgress.Info()
}

// UpdateScanProgress cập nhật tiến trình scan
func (pdm *ProgressDisplayManager) UpdateScanProgress(current, total int64, stage, message string) {
	pdm.scanProgress.Update(current, total, stage, message)
	pdm.triggerImmediateDisplay()
}

// UpdateMonitorProgress cập nhật tiến trình monitor
func (pdm *ProgressDisplayManager) UpdateMonitorProgress(current, total int64, stage, message string) {
	pdm.monitorProgress.Update(current, total, stage, message)
	pdm.triggerImmediateDisplay()
}

// SetScanStatus đặt trạng thái scan
func (pdm *ProgressDisplayManager) SetScanStatus(status ProgressStatus, message string) {
	pdm.scanProgress.SetStatus(status, message)
	pdm.triggerImmediateDisplay()
}

// SetMonitorStatus đặt trạng thái monitor
func (pdm *ProgressDisplayManager) SetMonitorStatus(status ProgressStatus, message string) {
	pdm.monitorProgress.SetStatus(status, message)
	pdm.triggerImmediateDisplay()
}

// UpdateBatchProgress cập nhật thông tin batch
func (pdm *ProgressDisplayManager) UpdateBatchProgress(progressType ProgressType, currentBatch, totalBatches int) {
	if progressType == ProgressTypeScan {
		pdm.scanProgress.UpdateBatch(currentBatch, totalBatches)
	} else {
		pdm.monitorProgress.UpdateBatch(currentBatch, totalBatches)
	}
	pdm.triggerImmediateDisplay()
}

// UpdateBatchProgressWithURLs cập nhật thông tin batch với URL tracking
func (pdm *ProgressDisplayManager) UpdateBatchProgressWithURLs(progressType ProgressType, currentBatch, totalBatches, currentBatchURLs, totalURLs, processedURLs int) {
	if progressType == ProgressTypeScan {
		pdm.scanProgress.UpdateBatchWithURLs(currentBatch, totalBatches, currentBatchURLs, totalURLs, processedURLs)
	} else {
		pdm.monitorProgress.UpdateBatchWithURLs(currentBatch, totalBatches, currentBatchURLs, totalURLs, processedURLs)
	}
	pdm.triggerImmediateDisplay()
}

// UpdateMonitorStats cập nhật stats monitor
func (pdm *ProgressDisplayManager) UpdateMonitorStats(processed, failed, completed int) {
	pdm.monitorProgress.UpdateMonitorStats(processed, failed, completed)
	pdm.triggerImmediateDisplay()
}

// ResetBatchProgress resets progress for a new batch with fresh timing
func (pdm *ProgressDisplayManager) ResetBatchProgress(progressType ProgressType, currentBatch, totalBatches int, stage, message string) {
	if progressType == ProgressTypeScan {
		pdm.scanProgress.ResetBatch(currentBatch, totalBatches, stage, message)
	} else {
		pdm.monitorProgress.ResetBatch(currentBatch, totalBatches, stage, message)
	}
	pdm.triggerImmediateDisplay()
}

// UpdateWorkflowProgress cập nhật tiến trình workflow bên trong batch (không ảnh hưởng đến batch progress)
func (pdm *ProgressDisplayManager) UpdateWorkflowProgress(current, total int64, stage, message string) {
	pdm.scanProgress.UpdateWorkflow(current, total, stage, message)
	pdm.triggerImmediateDisplay()
}

func (pdm *ProgressDisplayManager) triggerImmediateDisplay() {
	// Non-blocking send
	select {
	case pdm.triggerDisplay <- struct{}{}:
	default:
	}
}
