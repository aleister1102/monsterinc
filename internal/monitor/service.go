package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/logger"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/reporter"

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
	eventAggregator *EventAggregator
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
	batchURLManager := NewBatchURLManager(gCfg.MonitorBatchConfig, instanceLogger) // Initialize batch URL manager
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

	eventAggregator := initializeEventAggregator(gCfg, instanceLogger, notificationHelper)

	service := &MonitoringService{
		gCfg:               gCfg,
		logger:             instanceLogger,
		notificationHelper: notificationHelper,
		urlManager:         urlManager,
		batchURLManager:    batchURLManager,
		cycleTracker:       cycleTracker,
		eventAggregator:    eventAggregator,
		urlChecker:         urlChecker,
		mutexManager:       mutexManager,
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

// AddMonitorUrl adds a URL to the list of monitored URLs
func (s *MonitoringService) AddMonitorUrl(url string) {
	if !s.isValidURL(url) {
		return
	}

	s.urlManager.AddURL(url)
	s.queueURLForMonitoring(url)
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

	if !s.acquireURLMutex() {
		return
	}
	defer s.releaseURLMutex(url)

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

	if !s.hasChangesToReport(changedURLs, len(monitoredURLs)) {
		return
	}

	s.generateAndSendCycleReport(monitoredURLs, changedURLs, cycleID)
	s.finalizeCycle()
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

	// Update progress display
	if s.progressDisplay != nil {
		s.progressDisplay.UpdateMonitorProgress(0, int64(totalURLs), "Batch", fmt.Sprintf("Starting batch monitoring of %d URLs", totalURLs))
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

		// Update progress display
		if s.progressDisplay != nil {
			s.progressDisplay.UpdateMonitorProgress(int64(totalProcessed), int64(totalURLs), "Batch", fmt.Sprintf("Processed %d/%d URLs", totalProcessed, totalURLs))
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

	if s.eventAggregator != nil {
		s.eventAggregator.UpdateLogger(newLogger)
	}

	if s.mutexManager != nil {
		s.mutexManager.UpdateLogger(newLogger)
	}
}

// SetCurrentCycleID sets the current cycle ID
func (s *MonitoringService) SetCurrentCycleID(cycleID string) {
	s.cycleTracker.SetCurrentCycleID(cycleID)
}

// GetMonitoringStats returns current monitoring statistics
func (s *MonitoringService) GetMonitoringStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_monitored_urls": s.urlManager.Count(),
		"changed_urls_count":   s.cycleTracker.GetChangeCount(),
		"current_cycle_id":     s.cycleTracker.GetCurrentCycleID(),
		"mutex_count":          s.mutexManager.GetMutexCount(),
		"has_changes":          s.cycleTracker.HasChanges(),
	}

	// Add resource usage statistics if available
	if s.resourceLimiter != nil {
		resourceUsage := s.resourceLimiter.GetResourceUsage()
		stats["memory_usage_mb"] = resourceUsage.AllocMB
		stats["goroutines"] = resourceUsage.Goroutines
		stats["system_memory_percent"] = resourceUsage.SystemMemUsedPercent
		stats["cpu_usage_percent"] = resourceUsage.CPUUsagePercent
		stats["gc_count"] = resourceUsage.GCCount
	}

	return stats
}

// GetBufferFromPool gets a buffer from the memory pool
func (s *MonitoringService) GetBufferFromPool() *common.BufferPool {
	return s.bufferPool
}

// GetSliceFromPool gets a slice from the memory pool
func (s *MonitoringService) GetSliceFromPool() *common.SlicePool {
	return s.slicePool
}

// GetStringSliceFromPool gets a string slice from the memory pool
func (s *MonitoringService) GetStringSliceFromPool() *common.StringSlicePool {
	return s.stringSlicePool
}

// CheckResourceLimits checks current resource usage and takes action if needed
func (s *MonitoringService) CheckResourceLimits() error {
	if s.resourceLimiter == nil {
		return nil
	}

	// Check memory limit
	if err := s.resourceLimiter.CheckMemoryLimit(); err != nil {
		s.logger.Warn().Err(err).Msg("Memory limit exceeded, triggering GC")
		s.resourceLimiter.ForceGC()
		return err
	}

	// Check goroutine limit
	if err := s.resourceLimiter.CheckGoroutineLimit(); err != nil {
		s.logger.Warn().Err(err).Msg("Goroutine limit exceeded")
		return err
	}

	// Check system resource limits
	if systemMemExceeded, err := s.resourceLimiter.CheckSystemMemoryLimit(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to check system memory limit")
		return err
	} else if systemMemExceeded {
		s.logger.Warn().Msg("System memory threshold exceeded")
		return fmt.Errorf("system memory threshold exceeded")
	}

	if cpuExceeded, err := s.resourceLimiter.CheckCPULimit(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to check CPU limit")
		return err
	} else if cpuExceeded {
		s.logger.Warn().Msg("CPU threshold exceeded")
		return fmt.Errorf("CPU threshold exceeded")
	}

	return nil
}

// ForceGarbageCollection triggers garbage collection manually
func (s *MonitoringService) ForceGarbageCollection() {
	if s.resourceLimiter != nil {
		s.resourceLimiter.ForceGC()
		s.logger.Info().Msg("Forced garbage collection completed")
	}
}

// Private initialization helper methods

func validateMonitoringConfig(gCfg *config.GlobalConfig) error {
	if gCfg == nil || !gCfg.MonitorConfig.Enabled {
		return fmt.Errorf("global configuration is nil or monitoring is disabled")
	}
	return nil
}

func initializeHistoryStore(gCfg *config.GlobalConfig, logger zerolog.Logger) (*datastore.ParquetFileHistory, error) {
	historyStore, err := datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ParquetFileHistoryStore: %w", err)
	}
	return historyStore, nil
}

func initializeHTTPFetcher(gCfg *config.GlobalConfig, logger zerolog.Logger) (*common.Fetcher, error) {
	timeout := determineHTTPTimeout(gCfg, logger)

	clientFactory := common.NewHTTPClientFactory(logger)
	monitorHttpClient, err := clientFactory.CreateMonitorClient(
		timeout,
		gCfg.MonitorConfig.MonitorInsecureSkipVerify,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	fetcher := common.NewFetcher(
		monitorHttpClient,
		logger,
		&common.HTTPClientFetcherConfig{MaxContentSize: gCfg.MonitorConfig.MaxContentSize},
	)
	return fetcher, nil
}

func determineHTTPTimeout(gCfg *config.GlobalConfig, logger zerolog.Logger) time.Duration {
	timeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
	if gCfg.MonitorConfig.HTTPTimeoutSeconds <= 0 {
		timeout = 30 * time.Second
		logger.Warn().
			Int("configured_timeout", gCfg.MonitorConfig.HTTPTimeoutSeconds).
			Dur("default_timeout", timeout).
			Msg("Monitor HTTPTimeoutSeconds invalid, using default")
	}
	return timeout
}

func initializeContentDiffer(gCfg *config.GlobalConfig, logger zerolog.Logger) *differ.ContentDiffer {
	contentDiffer, err := differ.NewContentDiffer(logger, &gCfg.DiffReporterConfig)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize ContentDiffer")
		return nil
	}
	return contentDiffer
}

func initializePathExtractor(gCfg *config.GlobalConfig, logger zerolog.Logger) *extractor.PathExtractor {
	pathExtractor, err := extractor.NewPathExtractor(gCfg.ExtractorConfig, logger)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize PathExtractor")
		return nil
	}
	return pathExtractor
}

func initializeHtmlDiffReporter(
	gCfg *config.GlobalConfig,
	historyStore models.FileHistoryStore,
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) *reporter.HtmlDiffReporter {
	if historyStore == nil {
		return nil
	}

	htmlDiffReporter, err := reporter.NewHtmlDiffReporter(logger, historyStore, &gCfg.MonitorConfig)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize HtmlDiffReporter")
		return nil
	}

	if notificationHelper != nil {
		notificationHelper.SetDiffReportCleaner(htmlDiffReporter)
	}

	return htmlDiffReporter
}

func createInitialCycleTracker() *CycleTracker {
	initialCycleID := fmt.Sprintf("monitor-init-%s", time.Now().Format("20060102-150405"))
	return NewCycleTracker(initialCycleID)
}

func initializeResourceLimiter(gCfg *config.GlobalConfig, logger zerolog.Logger) *common.ResourceLimiter {
	resourceConfig := common.ResourceLimiterConfig{
		MaxMemoryMB:        gCfg.ResourceLimiterConfig.MaxMemoryMB,
		MaxGoroutines:      gCfg.ResourceLimiterConfig.MaxGoroutines,
		CheckInterval:      time.Duration(gCfg.ResourceLimiterConfig.CheckIntervalSecs) * time.Second,
		MemoryThreshold:    gCfg.ResourceLimiterConfig.MemoryThreshold,
		GoroutineWarning:   gCfg.ResourceLimiterConfig.GoroutineWarning,
		SystemMemThreshold: gCfg.ResourceLimiterConfig.SystemMemThreshold,
		CPUThreshold:       gCfg.ResourceLimiterConfig.CPUThreshold,
		EnableAutoShutdown: gCfg.ResourceLimiterConfig.EnableAutoShutdown,
	}

	return common.NewResourceLimiter(resourceConfig, logger)
}

func initializeEventAggregator(
	gCfg *config.GlobalConfig,
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) *EventAggregator {
	if gCfg.MonitorConfig.AggregationIntervalSeconds <= 0 {
		logger.Warn().Msg("Aggregation interval not configured, events will not be aggregated")
		return nil
	}

	aggregationInterval := time.Duration(gCfg.MonitorConfig.AggregationIntervalSeconds) * time.Second
	return NewEventAggregator(
		logger,
		notificationHelper,
		aggregationInterval,
		gCfg.MonitorConfig.MaxAggregatedEvents,
	)
}

// Private service operation helper methods

func (s *MonitoringService) isValidURL(url string) bool {
	return url != ""
}

func (s *MonitoringService) queueURLForMonitoring(url string) {
	select {
	case s.monitorChan <- url:
		s.logger.Debug().Str("url", url).Msg("URL queued for monitoring")
	default:
		s.logger.Warn().Str("url", url).Msg("Monitor channel full, URL not queued")
	}
}

func (s *MonitoringService) acquireURLMutex() bool {
	// In a more complex implementation, we might want to add timeout here
	return true
}

func (s *MonitoringService) releaseURLMutex(url string) {
	// URL mutex release logic if needed
}

func (s *MonitoringService) performURLCheck(url string) LegacyCheckResult {
	urlMutex := s.mutexManager.GetMutex(url)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	cycleID := s.cycleTracker.GetCurrentCycleID()
	return s.urlChecker.CheckURLWithContext(s.serviceCtx, url, cycleID)
}

func (s *MonitoringService) handleCheckResult(url string, result LegacyCheckResult) {
	if !result.Success {
		s.handleCheckError(result)
		return
	}

	if result.FileChangeInfo != nil {
		s.handleFileChange(url, result)
	}
}

func (s *MonitoringService) handleCheckError(result LegacyCheckResult) {
	if result.ErrorInfo != nil && s.eventAggregator != nil {
		s.eventAggregator.AddFetchErrorEvent(*result.ErrorInfo)
	}
}

func (s *MonitoringService) handleFileChange(url string, result LegacyCheckResult) {
	s.cycleTracker.AddChangedURL(url)
	if s.eventAggregator != nil {
		s.eventAggregator.AddFileChangeEvent(*result.FileChangeInfo)
	}
}

func (s *MonitoringService) hasChangesToReport(changedURLs []string, monitoredCount int) bool {
	if len(changedURLs) == 0 {
		s.logger.Info().Int("monitored_count", monitoredCount).Msg("No changes detected in this cycle")
		return false
	}
	return true
}

func (s *MonitoringService) generateAndSendCycleReport(monitoredURLs, changedURLs []string, cycleID string) {
	if s.urlChecker.htmlDiffReporter == nil {
		s.logger.Warn().Msg("HtmlDiffReporter is not available, sending notification without report")
		s.sendCycleCompleteNotification(cycleID, changedURLs, "", len(monitoredURLs))
		return
	}

	reportPaths, err := s.urlChecker.htmlDiffReporter.GenerateDiffReport(monitoredURLs, cycleID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to generate cycle end diff report")
		s.sendCycleCompleteNotification(cycleID, changedURLs, "", len(monitoredURLs))
		return
	}

	if len(reportPaths) == 0 {
		s.logger.Info().Msg("No changes detected - sending notification without report")
		s.sendCycleCompleteNotification(cycleID, changedURLs, "", len(monitoredURLs))
		return
	}

	// Use the first report path for notification (main report)
	mainReportPath := reportPaths[0]
	s.logger.Info().
		Str("main_report_path", mainReportPath).
		Int("total_reports", len(reportPaths)).
		Msg("Generated cycle end diff report(s)")
	s.sendCycleCompleteNotification(cycleID, changedURLs, mainReportPath, len(monitoredURLs))
}

func (s *MonitoringService) sendCycleCompleteNotification(cycleID string, changedURLs []string, reportPath string, totalMonitored int) {
	if s.notificationHelper == nil {
		return
	}

	data := models.MonitorCycleCompleteData{
		CycleID:        cycleID,
		ChangedURLs:    changedURLs,
		ReportPath:     reportPath,
		TotalMonitored: totalMonitored,
		Timestamp:      time.Now(),
	}
	s.notificationHelper.SendMonitorCycleCompleteNotification(s.serviceCtx, data)
}

func (s *MonitoringService) finalizeCycle() {
	s.cycleTracker.ClearChangedURLs()
}

func (s *MonitoringService) performCleanShutdown() {
	s.stopEventAggregator()
	s.cancelServiceContext()
	s.cleanupResources()
}

func (s *MonitoringService) stopEventAggregator() {
	if s.eventAggregator != nil {
		s.eventAggregator.Stop()
	}
}

func (s *MonitoringService) cancelServiceContext() {
	if s.serviceCancelFunc != nil {
		s.serviceCancelFunc()
	}
}

func (s *MonitoringService) cleanupResources() {
	activeURLs := s.urlManager.GetCurrentURLs()
	s.mutexManager.CleanupUnusedMutexes(activeURLs)
}

func (s *MonitoringService) updateServiceContext(parentCtx context.Context) {
	s.cancelServiceContext()
	s.serviceCtx, s.serviceCancelFunc = context.WithCancel(parentCtx)

	// Update event aggregator context as well
	if s.eventAggregator != nil {
		s.eventAggregator.SetParentContext(s.serviceCtx)
	}

	s.logger.Debug().Msg("Updated service context with new parent")
}

func (s *MonitoringService) createCycleID() string {
	return fmt.Sprintf("monitor-%s", time.Now().Format("20060102-150405"))
}
