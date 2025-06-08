package monitor

import (
	"context"
	"fmt"
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/logger"
	"github.com/aleister1102/monsterinc/internal/notifier"

	"github.com/rs/zerolog"
)

// MonitoringService orchestrates the monitoring of HTML/JS files using modular components
type MonitoringService struct {
	gCfg               *config.GlobalConfig
	logger             zerolog.Logger
	notificationHelper *notifier.NotificationHelper

	// Core components
	urlManager      *URLManager
	batchURLManager *BatchURLManager // Added for batch processing
	cycleTracker    *CycleTracker
	urlChecker      *URLChecker
	mutexManager    *URLMutexManager

	// Memory optimization components
	resourceLimiter *common.ResourceLimiter
	bufferPool      *common.BufferPool
	slicePool       *common.SlicePool
	stringSlicePool *common.StringSlicePool

	// Communication channel
	monitorChan chan string

	// Service lifecycle management
	serviceCtx        context.Context
	serviceCancelFunc context.CancelFunc
	isStopped         bool
	stoppedMutex      sync.Mutex

	// Progress display
	progressDisplay *common.ProgressDisplayManager
}

// NewMonitoringService creates a new refactored monitoring service
func NewMonitoringService(
	gCfg *config.GlobalConfig,
	baseLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) (*MonitoringService, error) {
	instanceLogger := baseLogger.With().Str("component", "MonitoringServiceRefactored").Logger()

	if err := validateMonitoringConfig(gCfg); err != nil {
		return nil, err
	}

	// Initialize memory optimization components
	resourceLimiter := initializeResourceLimiter(gCfg, instanceLogger)
	bufferPool := common.NewBufferPool(64 * 1024)     // 64KB buffers for content
	slicePool := common.NewSlicePool(32 * 1024)       // 32KB slices for processing
	stringSlicePool := common.NewStringSlicePool(500) // 500 URLs per batch

	// Initialize core dependencies
	historyStore, err := initializeHistoryStore(gCfg, instanceLogger)
	if err != nil {
		return nil, err
	}

	fetcher, err := initializeHTTPFetcher(gCfg, instanceLogger)
	if err != nil {
		return nil, err
	}

	processor := NewContentProcessor(instanceLogger)

	// Initialize optional components
	contentDiffer := initializeContentDiffer(gCfg, instanceLogger)
	pathExtractor := initializePathExtractor(gCfg, instanceLogger)
	htmlDiffReporter := initializeHtmlDiffReporter(gCfg, historyStore, instanceLogger, notificationHelper)

	// Initialize modular components
	urlManager := NewURLManager(instanceLogger)

	// Set MaxConcurrentBatch based on monitor worker count if not already set
	monitorBatchConfig := gCfg.MonitorBatchConfig
	monitorBatchConfig.SetMaxConcurrentFromMonitorWorkers(gCfg.MonitorConfig.MaxConcurrentChecks)

	batchURLManager := NewBatchURLManager(monitorBatchConfig, instanceLogger) // Initialize batch URL manager

	instanceLogger.Info().
		Int("monitor_workers", gCfg.MonitorConfig.MaxConcurrentChecks).
		Int("max_concurrent_batch", monitorBatchConfig.GetEffectiveMaxConcurrentBatch()).
		Int("batch_size", monitorBatchConfig.BatchSize).
		Msg("Monitor service batch configuration set based on worker count")

	cycleTracker := createInitialCycleTracker()
	mutexManager := NewURLMutexManager(instanceLogger)

	urlChecker := NewURLChecker(
		instanceLogger,
		gCfg,
		historyStore,
		fetcher,
		processor,
		contentDiffer,
		pathExtractor,
		htmlDiffReporter,
	)

	service := &MonitoringService{
		gCfg:               gCfg,
		logger:             instanceLogger,
		notificationHelper: notificationHelper,
		urlManager:         urlManager,
		batchURLManager:    batchURLManager,
		cycleTracker:       cycleTracker,

		urlChecker:   urlChecker,
		mutexManager: mutexManager,
		// Memory optimization components
		resourceLimiter: resourceLimiter,
		bufferPool:      bufferPool,
		slicePool:       slicePool,
		stringSlicePool: stringSlicePool,
		monitorChan:     make(chan string, gCfg.MonitorConfig.MaxConcurrentChecks*2),
		isStopped:       false,
		stoppedMutex:    sync.Mutex{},
	}

	// Set shutdown callback for resource limiter
	resourceLimiter.SetShutdownCallback(func() {
		instanceLogger.Warn().Msg("Resource limiter triggered graceful shutdown")
		service.Stop()
	})

	// Start resource monitoring
	resourceLimiter.Start()

	return service, nil
}

// SetProgressDisplay đặt progress display manager
func (s *MonitoringService) SetProgressDisplay(progressDisplay *common.ProgressDisplayManager) {
	s.progressDisplay = progressDisplay
}

// GetCurrentlyMonitorUrls returns a copy of currently monitored URLs
func (s *MonitoringService) GetCurrentlyMonitorUrls() []string {
	return s.urlManager.GetCurrentURLs()
}

// Preload adds multiple URLs to the monitored list
func (s *MonitoringService) Preload(initialURLs []string) {
	s.urlManager.PreloadURLs(initialURLs)

	// Log batch processing information
	if len(initialURLs) > 0 {
		useBatching, batchCount, _ := s.batchURLManager.GetBatchingInfo(len(initialURLs))
		s.logger.Info().
			Int("url_count", len(initialURLs)).
			Bool("uses_batching", useBatching).
			Int("batch_count", batchCount).
			Msg("URLs preloaded for monitoring with batch processing capability")
	}
}

// LoadAndMonitorFromSources loads and monitors URLs from various sources
func (s *MonitoringService) LoadAndMonitorFromSources(inputFileOption string) error {
	return s.urlManager.LoadAndMonitorFromSources(inputFileOption)
}

// CheckURL checks a single URL for changes with memory optimization
func (s *MonitoringService) CheckURL(url string) {
	// Check if service is stopped or context cancelled before starting
	if s.isStopped {
		return
	}

	// Update progress display
	if s.progressDisplay != nil {
		currentURLs := s.urlManager.GetCurrentURLs()
		s.progressDisplay.UpdateMonitorProgress(1, int64(len(currentURLs)), "Checking", fmt.Sprintf("Checking %s", url))
	}

	if s.serviceCtx != nil {
		select {
		case <-s.serviceCtx.Done():
			s.logger.Debug().Str("url", url).Msg("URL check cancelled due to context cancellation")
			return
		default:
		}
	}

	// Check resource limits before processing
	if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
		s.logger.Warn().Err(err).Str("url", url).Msg("Memory limit exceeded, triggering GC")
		s.resourceLimiter.ForceGC()

		// Recheck after GC
		if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Memory limit still exceeded after GC, skipping URL")
			return
		}
	}

	if err := s.resourceLimiter.CheckGoroutineLimit(); err != nil {
		s.logger.Warn().Err(err).Str("url", url).Msg("Goroutine limit exceeded, skipping URL")
		return
	}

	// Check again after acquiring mutex
	if s.serviceCtx != nil {
		select {
		case <-s.serviceCtx.Done():
			s.logger.Debug().Str("url", url).Msg("URL check cancelled after acquiring mutex")
			return
		default:
		}
	}

	result := s.performURLCheck(url)
	s.handleCheckResult(url, result)
}

// TriggerCycleEndReport triggers the end-of-cycle report generation
func (s *MonitoringService) TriggerCycleEndReport() {
	cycleID := s.cycleTracker.GetCurrentCycleID()
	s.logger.Info().Str("cycle_id", cycleID).Msg("Triggering cycle end report")

	monitoredURLs := s.urlManager.GetCurrentURLs()
	changedURLs := s.cycleTracker.GetChangedURLs()

	// Always send cycle complete notification, regardless of whether there are changes
	s.generateAndSendCycleReport(monitoredURLs, changedURLs, cycleID)
	s.cycleTracker.ClearChangedURLs()
}

// ExecuteBatchMonitoring executes monitoring for URLs using batch processing with memory optimization
func (s *MonitoringService) ExecuteBatchMonitoring(ctx context.Context, inputFile string) error {
	s.logger.Info().Str("input_file", inputFile).Msg("Starting batch monitoring execution")

	// Check resource limits before starting batch processing
	if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
		s.logger.Warn().Err(err).Msg("Memory limit near threshold, triggering GC before batch processing")
		s.resourceLimiter.ForceGC()
	}

	// Get resource usage before processing
	resourceUsageBefore := s.resourceLimiter.GetResourceUsage()
	s.logger.Info().
		Int64("memory_mb", resourceUsageBefore.AllocMB).
		Int("goroutines", resourceUsageBefore.Goroutines).
		Float64("system_mem_percent", resourceUsageBefore.SystemMemUsedPercent).
		Msg("Resource usage before batch monitoring")

	// Load URLs using batch URL manager
	batchTracker, err := s.batchURLManager.LoadURLsInBatches(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs in batches: %w", err)
	}

	cycleID := s.GenerateNewCycleID()
	totalProcessed := 0
	totalURLs := len(batchTracker.AllURLs)

	// Update progress display with batch info
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateMonitorProgress(0, int64(totalURLs), "Batch", fmt.Sprintf("Starting batch monitoring of %d URLs", totalURLs))
		s.progressDisplay.UpdateBatchProgress(common.ProgressTypeMonitor, 0, batchTracker.TotalBatches)

	}

	// Process URLs in batches with memory optimization
	for {
		batch, hasMore := s.batchURLManager.GetNextBatch(batchTracker)
		if len(batch) == 0 {
			break
		}

		// Use optimized URL slice from pool
		urlSlice := s.stringSlicePool.Get()
		urlSlice = append(urlSlice, batch...)

		// Execute batch monitoring with memory monitoring
		batchResult, err := s.batchURLManager.ExecuteBatchMonitoring(ctx, urlSlice, cycleID, s.urlChecker)

		// Return slice to pool
		s.stringSlicePool.Put(urlSlice)

		if err != nil {
			s.logger.Error().Err(err).Msg("Batch monitoring failed")
			// Continue with next batch on error
		} else {
			totalProcessed += len(batchResult.ProcessedURLs)
		}

		s.batchURLManager.CompleteCurrentBatch(batchTracker)

		// Update progress display with detailed stats
		if s.progressDisplay != nil {
			s.progressDisplay.UpdateMonitorProgress(int64(totalProcessed), int64(totalURLs), "Batch", fmt.Sprintf("Processed %d/%d URLs", totalProcessed, totalURLs))

			// Update batch progress
			s.progressDisplay.UpdateBatchProgress(common.ProgressTypeMonitor, batchTracker.ProcessedBatches, batchTracker.TotalBatches)

			// Update monitor stats (processed, failed, completed)
			// Calculate cumulative stats
			cumulativeSuccessful := totalProcessed
			cumulativeFailed := (batchTracker.ProcessedBatches * len(batch)) - totalProcessed
			if cumulativeFailed < 0 {
				cumulativeFailed = 0
			}
			s.progressDisplay.UpdateMonitorStats(totalProcessed, cumulativeFailed, cumulativeSuccessful)

			// Update event counts
			// Event aggregation removed - notifications handled in batch completion
		}

		s.logger.Info().
			Int("batch_processed", len(batch)).
			Int("total_processed", totalProcessed).
			Bool("has_more", hasMore).
			Msg("Batch monitoring completed")

		if !hasMore {
			break
		}

		// Check resource usage and trigger GC if needed
		if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
			s.logger.Warn().Err(err).Msg("Memory limit approaching, triggering GC between batches")
			s.resourceLimiter.ForceGC()
		}
	}

	// Log final resource usage
	resourceUsageAfter := s.resourceLimiter.GetResourceUsage()
	// Update progress display - completed
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateMonitorProgress(int64(totalProcessed), int64(totalURLs), "Complete", "Batch monitoring completed")
		s.progressDisplay.SetMonitorStatus(common.ProgressStatusComplete, fmt.Sprintf("Processed %d URLs", totalProcessed))
	}

	s.logger.Info().
		Str("cycle_id", cycleID).
		Int("total_processed", totalProcessed).
		Int("total_batches", batchTracker.ProcessedBatches).
		Int64("memory_before_mb", resourceUsageBefore.AllocMB).
		Int64("memory_after_mb", resourceUsageAfter.AllocMB).
		Int("goroutines_before", resourceUsageBefore.Goroutines).
		Int("goroutines_after", resourceUsageAfter.Goroutines).
		Msg("Batch monitoring cycle completed")

	return nil
}

// Stop gracefully stops the monitoring service
func (s *MonitoringService) Stop() {
	s.stoppedMutex.Lock()
	defer s.stoppedMutex.Unlock()

	if s.isStopped {
		return
	}

	s.logger.Info().Msg("Stopping monitoring service")

	// Stop resource limiter first
	if s.resourceLimiter != nil {
		s.resourceLimiter.Stop()
	}

	s.performCleanShutdown()
	s.isStopped = true

	// Log final resource usage
	if s.resourceLimiter != nil {
		finalUsage := s.resourceLimiter.GetResourceUsage()
		s.logger.Info().
			Int64("final_memory_mb", finalUsage.AllocMB).
			Int("final_goroutines", finalUsage.Goroutines).
			Float64("final_system_mem_percent", finalUsage.SystemMemUsedPercent).
			Msg("Final resource usage at service stop")
	}

	s.logger.Info().Msg("Monitoring service stopped")
}

// SetParentContext sets the parent context for the service
func (s *MonitoringService) SetParentContext(parentCtx context.Context) {
	s.updateServiceContext(parentCtx)
}

// GenerateNewCycleID generates a new cycle ID
func (s *MonitoringService) GenerateNewCycleID() string {
	newCycleID := s.createCycleID()
	s.cycleTracker.SetCurrentCycleID(newCycleID)

	// Tạo logger riêng cho cycle này
	if cycleLogger, err := logger.NewWithCycleID(s.gCfg.LogConfig, newCycleID); err == nil {
		s.logger = cycleLogger

		// Cập nhật logger cho tất cả các component
		s.updateAllComponentLoggers(cycleLogger)
	} else {
		s.logger.Warn().Err(err).Str("cycle_id", newCycleID).Msg("Failed to create cycle logger, using default logger")
	}

	return newCycleID
}

// updateAllComponentLoggers cập nhật logger cho tất cả các component
func (s *MonitoringService) updateAllComponentLoggers(newLogger zerolog.Logger) {
	// Cập nhật logger cho các component có method UpdateLogger
	if s.urlManager != nil {
		s.urlManager.UpdateLogger(newLogger)
	}

	if s.batchURLManager != nil {
		s.batchURLManager.UpdateLogger(newLogger)
	}

	if s.urlChecker != nil {
		s.urlChecker.UpdateLogger(newLogger)
	}

	if s.mutexManager != nil {
		s.mutexManager.UpdateLogger(newLogger)
	}
}

// SetCurrentCycleID sets the current cycle ID for tracking
func (s *MonitoringService) SetCurrentCycleID(cycleID string) {
	s.cycleTracker.SetCurrentCycleID(cycleID)
}
