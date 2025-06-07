package common

import (
	"context"
	"fmt"
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
