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
	cycleTracker    *CycleTracker
	eventAggregator *EventAggregator
	urlChecker      *URLChecker
	mutexManager    *URLMutexManager

	// Communication channel
	monitorChan chan string

	// Service lifecycle management
	serviceCtx        context.Context
	serviceCancelFunc context.CancelFunc
	isStopped         bool
	stoppedMutex      sync.Mutex
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
	htmlDiffReporter := initializeHtmlDiffReporter(historyStore, instanceLogger, notificationHelper)

	// Initialize modular components
	urlManager := NewURLManager(instanceLogger)
	cycleTracker := createInitialCycleTracker(instanceLogger)
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
		cycleTracker:       cycleTracker,
		eventAggregator:    eventAggregator,
		urlChecker:         urlChecker,
		mutexManager:       mutexManager,
		monitorChan:        make(chan string, gCfg.MonitorConfig.MaxConcurrentChecks*2),
		isStopped:          false,
		stoppedMutex:       sync.Mutex{},
	}

	return service, nil
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
}

// LoadAndMonitorFromSources loads and monitors URLs from various sources
func (s *MonitoringService) LoadAndMonitorFromSources(inputFileOption string, inputConfigUrls []string, cfgInputFile string) error {
	return s.urlManager.LoadAndMonitorFromSources(inputFileOption, inputConfigUrls, cfgInputFile)
}

// CheckURL checks a single URL for changes
func (s *MonitoringService) CheckURL(url string) {
	if !s.acquireURLMutex() {
		return
	}
	defer s.releaseURLMutex(url)

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

// Stop gracefully stops the monitoring service
func (s *MonitoringService) Stop() {
	s.stoppedMutex.Lock()
	defer s.stoppedMutex.Unlock()

	if s.isStopped {
		return
	}

	s.logger.Info().Msg("Stopping monitoring service")
	s.performCleanShutdown()
	s.isStopped = true
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
	s.logger.Info().Str("cycle_id", newCycleID).Msg("Generated new cycle ID")
	return newCycleID
}

// SetCurrentCycleID sets the current cycle ID
func (s *MonitoringService) SetCurrentCycleID(cycleID string) {
	s.cycleTracker.SetCurrentCycleID(cycleID)
}

// GetMonitoringStats returns current monitoring statistics
func (s *MonitoringService) GetMonitoringStats() map[string]interface{} {
	return map[string]interface{}{
		"total_monitored_urls": s.urlManager.Count(),
		"changed_urls_count":   s.cycleTracker.GetChangeCount(),
		"current_cycle_id":     s.cycleTracker.GetCurrentCycleID(),
		"mutex_count":          s.mutexManager.GetMutexCount(),
		"has_changes":          s.cycleTracker.HasChanges(),
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
	historyStore models.FileHistoryStore,
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) *reporter.HtmlDiffReporter {
	if historyStore == nil {
		return nil
	}

	htmlDiffReporter, err := reporter.NewHtmlDiffReporter(logger, historyStore)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize HtmlDiffReporter")
		return nil
	}

	if notificationHelper != nil {
		notificationHelper.SetDiffReportCleaner(htmlDiffReporter)
	}

	return htmlDiffReporter
}

func createInitialCycleTracker(logger zerolog.Logger) *CycleTracker {
	initialCycleID := fmt.Sprintf("monitor-init-%s", time.Now().Format("20060102-150405"))
	return NewCycleTracker(logger, initialCycleID)
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

func (s *MonitoringService) performURLCheck(url string) CheckResult {
	urlMutex := s.mutexManager.GetMutex(url)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	cycleID := s.cycleTracker.GetCurrentCycleID()
	return s.urlChecker.CheckURL(url, cycleID)
}

func (s *MonitoringService) handleCheckResult(url string, result CheckResult) {
	if !result.Success {
		s.handleCheckError(result)
		return
	}

	if result.FileChangeInfo != nil {
		s.handleFileChange(url, result)
	}
}

func (s *MonitoringService) handleCheckError(result CheckResult) {
	if result.ErrorInfo != nil && s.eventAggregator != nil {
		s.eventAggregator.AddFetchErrorEvent(*result.ErrorInfo)
	}
}

func (s *MonitoringService) handleFileChange(url string, result CheckResult) {
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

	reportPath, err := s.urlChecker.htmlDiffReporter.GenerateDiffReport(monitoredURLs, cycleID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to generate cycle end diff report")
		s.sendCycleCompleteNotification(cycleID, changedURLs, "", len(monitoredURLs))
		return
	}

	if reportPath == "" {
		s.logger.Info().Msg("No changes detected - sending notification without report")
		s.sendCycleCompleteNotification(cycleID, changedURLs, "", len(monitoredURLs))
		return
	}

	s.logger.Info().Str("report_path", reportPath).Msg("Generated cycle end diff report")
	s.sendCycleCompleteNotification(cycleID, changedURLs, reportPath, len(monitoredURLs))
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
	s.logger.Debug().Msg("Updated service context with new parent")
}

func (s *MonitoringService) createCycleID() string {
	return fmt.Sprintf("monitor-%s", time.Now().Format("20060102-150405"))
}
