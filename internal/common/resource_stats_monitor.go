package common

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ResourceStatsMonitor hiển thị liên tục resource stats trong quá trình scan
type ResourceStatsMonitor struct {
	resourceLimiter *ResourceLimiter
	logger          zerolog.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	isRunning       bool
	mu              sync.RWMutex

	// Config
	displayInterval time.Duration
	componentName   string
	moduleName      string

	// Counters
	totalURLsProcessed int64
	totalAssetsFound   int64
	totalErrors        int64
	startTime          time.Time
}

// ResourceStatsMonitorConfig cấu hình cho ResourceStatsMonitor
type ResourceStatsMonitorConfig struct {
	DisplayInterval time.Duration // Tần suất hiển thị stats
	ComponentName   string        // Tên component (CrawlerManager, MonitorService, etc.)
	ModuleName      string        // Tên module (Crawler, Monitor, etc.)
}

// NewResourceStatsMonitor tạo ResourceStatsMonitor mới
func NewResourceStatsMonitor(
	resourceLimiter *ResourceLimiter,
	config ResourceStatsMonitorConfig,
	logger zerolog.Logger,
) *ResourceStatsMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	// Default values
	if config.DisplayInterval <= 0 {
		config.DisplayInterval = 10 * time.Second
	}
	if config.ComponentName == "" {
		config.ComponentName = "ResourceMonitor"
	}
	if config.ModuleName == "" {
		config.ModuleName = "System"
	}

	return &ResourceStatsMonitor{
		resourceLimiter: resourceLimiter,
		logger:          logger.With().Str("component", config.ComponentName).Str("module", config.ModuleName).Logger(),
		ctx:             ctx,
		cancel:          cancel,
		displayInterval: config.DisplayInterval,
		componentName:   config.ComponentName,
		moduleName:      config.ModuleName,
		startTime:       time.Now(),
	}
}

// Start bắt đầu hiển thị resource stats
func (rsm *ResourceStatsMonitor) Start() {
	rsm.mu.Lock()
	if rsm.isRunning {
		rsm.mu.Unlock()
		return
	}
	rsm.isRunning = true
	rsm.startTime = time.Now()
	rsm.mu.Unlock()

	rsm.wg.Add(1)
	go rsm.monitorLoop()

	rsm.logger.Info().
		Dur("display_interval", rsm.displayInterval).
		Str("component", rsm.componentName).
		Str("module", rsm.moduleName).
		Msg("Resource stats monitor started")
}

// Stop dừng hiển thị resource stats
func (rsm *ResourceStatsMonitor) Stop() {
	rsm.mu.Lock()
	if !rsm.isRunning {
		rsm.mu.Unlock()
		return
	}
	rsm.isRunning = false
	rsm.mu.Unlock()

	rsm.cancel()
	rsm.wg.Wait()

	// Log final stats
	rsm.logResourceStats(true)

	rsm.logger.Info().Msg("Resource stats monitor stopped")
}

// IncrementURLsProcessed tăng counter URLs đã xử lý
func (rsm *ResourceStatsMonitor) IncrementURLsProcessed(count int64) {
	rsm.mu.Lock()
	rsm.totalURLsProcessed += count
	rsm.mu.Unlock()
}

// IncrementAssetsFound tăng counter assets đã tìm thấy
func (rsm *ResourceStatsMonitor) IncrementAssetsFound(count int64) {
	rsm.mu.Lock()
	rsm.totalAssetsFound += count
	rsm.mu.Unlock()
}

// IncrementErrors tăng counter lỗi
func (rsm *ResourceStatsMonitor) IncrementErrors(count int64) {
	rsm.mu.Lock()
	rsm.totalErrors += count
	rsm.mu.Unlock()
}

// GetStats trả về thống kê hiện tại
func (rsm *ResourceStatsMonitor) GetStats() (urlsProcessed, assetsFound, errors int64, uptime time.Duration) {
	rsm.mu.RLock()
	defer rsm.mu.RUnlock()
	return rsm.totalURLsProcessed, rsm.totalAssetsFound, rsm.totalErrors, time.Since(rsm.startTime)
}

// monitorLoop vòng lặp hiển thị resource stats
func (rsm *ResourceStatsMonitor) monitorLoop() {
	defer rsm.wg.Done()

	ticker := time.NewTicker(rsm.displayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rsm.ctx.Done():
			return
		case <-ticker.C:
			rsm.logResourceStats(false)
		}
	}
}

// logResourceStats ghi log resource stats với format giống log mẫu
func (rsm *ResourceStatsMonitor) logResourceStats(isFinal bool) {
	if rsm.resourceLimiter == nil {
		return
	}

	usage := rsm.resourceLimiter.GetResourceUsage()
	urlsProcessed, assetsFound, errors, uptime := rsm.GetStats()

	logType := "Resource stats"
	if isFinal {
		logType = "Final resource stats"
	}

	// Calculate rates per minute
	uptimeMinutes := uptime.Minutes()
	var urlsPerMin, assetsPerMin, errorsPerMin float64
	if uptimeMinutes > 0 {
		urlsPerMin = float64(urlsProcessed) / uptimeMinutes
		assetsPerMin = float64(assetsFound) / uptimeMinutes
		errorsPerMin = float64(errors) / uptimeMinutes
	}

	rsm.logger.Info().
		Str("type", logType).
		Int64("memory_mb", usage.AllocMB).
		Int64("system_memory_mb", usage.SystemMemUsedMB).
		Float64("system_memory_percent", usage.SystemMemUsedPercent).
		Float64("cpu_percent", usage.CPUUsagePercent).
		Int("goroutines", usage.Goroutines).
		Int64("gc_count", usage.GCCount).
		Int64("urls_processed", urlsProcessed).
		Int64("assets_found", assetsFound).
		Int64("errors", errors).
		Dur("uptime", uptime).
		Float64("urls_per_min", urlsPerMin).
		Float64("assets_per_min", assetsPerMin).
		Float64("errors_per_min", errorsPerMin).
		Str("component", rsm.componentName).
		Str("module", rsm.moduleName).
		Msg("Resource statistics")
}

// SetParentContext đặt parent context
func (rsm *ResourceStatsMonitor) SetParentContext(parentCtx context.Context) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	if rsm.isRunning {
		// Cancel current context and create new one with parent
		rsm.cancel()
		rsm.ctx, rsm.cancel = context.WithCancel(parentCtx)
	} else {
		rsm.ctx, rsm.cancel = context.WithCancel(parentCtx)
	}
}
