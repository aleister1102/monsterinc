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

		// Send interrupt notification before shutdown
		cycleID := service.cycleTracker.GetCurrentCycleID()
		if cycleID != "" && service.notificationHelper != nil {
			currentURLs := service.urlManager.GetCurrentURLs()
			service.sendMonitorInterruptNotification(context.Background(), cycleID, len(currentURLs), 0, "resource_limit", "Monitor service interrupted due to resource limits exceeded")
		}

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

// GetCurrentProgress returns current monitoring progress
func (s *MonitoringService) GetCurrentProgress() (processedTargets, totalTargets int) {
	if s.progressDisplay == nil {
		return 0, len(s.urlManager.GetCurrentURLs())
	}

	// Get monitor progress from progress display
	monitorProgress := s.progressDisplay.GetMonitorProgress()
	if monitorProgress == nil {
		return 0, len(s.urlManager.GetCurrentURLs())
	}

	// If monitor info is available, use processed URLs count
	if monitorProgress.MonitorInfo != nil {
		processedTargets = monitorProgress.MonitorInfo.ProcessedURLs + monitorProgress.MonitorInfo.FailedURLs
	} else {
		// Fallback to progress current count
		processedTargets = int(monitorProgress.Current)
	}

	totalTargets = int(monitorProgress.Total)
	if totalTargets == 0 {
		totalTargets = len(s.urlManager.GetCurrentURLs())
	}

	return processedTargets, totalTargets
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

		// Check again after GC
		if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Memory limit still exceeded after GC, skipping URL")
			return
		}
	}

	// Process URL using the URLChecker with current cycle ID
	cycleID := s.cycleTracker.GetCurrentCycleID()
	urlCheckResult := s.urlChecker.CheckURLWithContext(s.serviceCtx, url, cycleID)

	// Handle result - reduced logging for performance
	if !urlCheckResult.Success {
		if urlCheckResult.ErrorInfo != nil {
			s.logger.Debug().Str("url", url).Str("error", urlCheckResult.ErrorInfo.Error).Msg("URL check failed")
		}
	}
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

// ExecuteBatchMonitoring executes batch monitoring from file input
func (s *MonitoringService) ExecuteBatchMonitoring(ctx context.Context, inputFile string) error {
	s.updateServiceContext(ctx)

	// Optimized memory check - no logging for successful checks
	if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
		s.logger.Warn().Err(err).Msg("Memory limit near threshold, triggering GC before batch processing")
		s.resourceLimiter.ForceGC()
	}

	// Load URLs using batch URL manager
	batchTracker, err := s.batchURLManager.LoadURLsInBatches(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to load URLs in batches: %w", err)
	}

	cycleID := s.GenerateNewCycleID()
	totalProcessed := 0
	totalFailed := 0
	totalURLs := len(batchTracker.AllURLs)
	var allChangedURLs []string // Track all changed URLs across batches

	// Start progress display for monitoring
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateMonitorProgress(0, int64(totalURLs), "Starting", "Initializing batch monitoring")
		s.progressDisplay.UpdateBatchProgressWithURLs(common.ProgressTypeMonitor, 0, batchTracker.TotalBatches, 0, totalURLs, 0)
	}

	// Process URLs in batches with memory optimization
	for {
		batch, hasMore := s.batchURLManager.GetNextBatch(batchTracker)
		if len(batch) == 0 {
			break
		}

		// Update progress display for current batch
		if s.progressDisplay != nil {
			currentCompleted := totalProcessed + totalFailed
			s.progressDisplay.UpdateMonitorProgress(int64(currentCompleted), int64(totalURLs), "Processing", fmt.Sprintf("Batch %d/%d", batchTracker.CurrentBatch, batchTracker.TotalBatches))
			s.progressDisplay.UpdateBatchProgressWithURLs(common.ProgressTypeMonitor, batchTracker.CurrentBatch, batchTracker.TotalBatches, len(batch), totalURLs, currentCompleted)
			s.progressDisplay.UpdateMonitorStats(totalProcessed, totalFailed, len(allChangedURLs))
		}

		// Use optimized URL slice from pool
		urlSlice := s.stringSlicePool.Get()
		urlSlice = append(urlSlice, batch...)

		// Execute batch monitoring with progress callback for display updates
		progressCallback := func(batchProcessed, batchFailed int) {
			// Update progress display every 10 items
			if batchProcessed%10 == 0 || batchProcessed == len(batch) {
				cumulativeProcessed := totalProcessed + batchProcessed
				cumulativeFailed := totalFailed + batchFailed
				currentCompleted := cumulativeProcessed + cumulativeFailed

				if s.progressDisplay != nil {
					s.progressDisplay.UpdateMonitorProgress(int64(currentCompleted), int64(totalURLs), "Processing", fmt.Sprintf("Batch %d/%d", batchTracker.CurrentBatch, batchTracker.TotalBatches))
					s.progressDisplay.UpdateMonitorStats(cumulativeProcessed, cumulativeFailed, len(allChangedURLs))
				}
			}
		}

		batchResult, err := s.batchURLManager.ExecuteBatchMonitoring(ctx, urlSlice, cycleID, s.urlChecker, progressCallback)

		// Return slice to pool
		s.stringSlicePool.Put(urlSlice)

		// Update stats based on batch result
		if err != nil {
			s.logger.Error().Err(err).Msg("Batch monitoring failed")
			totalFailed += len(batch)

			// Check if context was cancelled (interrupt)
			if ctx.Err() != nil {
				s.logger.Warn().Err(ctx.Err()).Msg("Batch monitoring interrupted by context cancellation")

				// Send interrupt notification
				s.sendMonitorInterruptNotification(ctx, cycleID, totalURLs, totalProcessed+totalFailed, "context_canceled", "Batch monitoring interrupted by context cancellation")

				break // Exit the batch processing loop
			}
		} else if batchResult != nil {
			totalProcessed += len(batchResult.ProcessedURLs)
			totalFailed += len(batch) - len(batchResult.ProcessedURLs)

			// Accumulate changed URLs
			allChangedURLs = append(allChangedURLs, batchResult.ChangedURLs...)
		}

		s.batchURLManager.CompleteCurrentBatch(batchTracker)

		if !hasMore {
			break
		}

		// Optimized memory check - no logging for successful checks
		if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
			s.resourceLimiter.ForceGC()
		}
	}

	// Add all changed URLs to cycle tracker
	for _, changedURL := range allChangedURLs {
		s.cycleTracker.AddChangedURL(changedURL)
	}

	// Final completion with progress display update
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateMonitorProgress(int64(totalURLs), int64(totalURLs), "Complete", "Batch monitoring completed")
		s.progressDisplay.UpdateMonitorStats(totalProcessed, totalFailed, len(allChangedURLs))
		s.progressDisplay.SetMonitorStatus(common.ProgressStatusComplete, fmt.Sprintf("Completed %d/%d URLs | P:%d F:%d C:%d", totalURLs, totalURLs, totalProcessed, totalFailed, len(allChangedURLs)))
	}

	s.logger.Info().
		Str("cycle_id", cycleID).
		Int("total_processed", totalProcessed).
		Int("total_failed", totalFailed).
		Int("total_changed", len(allChangedURLs)).
		Int("total_batches", batchTracker.ProcessedBatches).
		Msgf("👁 Monitor: ✅ 100.0%% (%d/%d) | P:%d F:%d C:%d | Completed",
			totalURLs, totalURLs, totalProcessed, totalFailed, len(allChangedURLs))

	// Trigger cycle end report and notifications
	s.logger.Info().
		Str("cycle_id", cycleID).
		Int("total_changed", len(allChangedURLs)).
		Int("total_processed", totalProcessed).
		Msg("🚀 TRIGGERING CYCLE END REPORT AND DISCORD NOTIFICATION")

	s.TriggerCycleEndReport()

	s.logger.Info().
		Str("cycle_id", cycleID).
		Msg("✅ CYCLE END REPORT AND NOTIFICATION COMPLETED")

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

	// Send interrupt notification if there's an active cycle
	cycleID := s.cycleTracker.GetCurrentCycleID()
	if cycleID != "" && s.notificationHelper != nil {
		currentURLs := s.urlManager.GetCurrentURLs()
		s.sendMonitorInterruptNotification(context.Background(), cycleID, len(currentURLs), 0, "service_stopped", "Monitor service stopped gracefully")
	}

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

// GetCurrentCycleID gets the current cycle ID
func (s *MonitoringService) GetCurrentCycleID() string {
	return s.cycleTracker.GetCurrentCycleID()
}
