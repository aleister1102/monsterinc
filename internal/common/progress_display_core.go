package common

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
	config          *ProgressDisplayConfig
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
		config:   config,
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
