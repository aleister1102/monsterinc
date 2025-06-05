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

	if gCfg == nil || !gCfg.MonitorConfig.Enabled {
		return nil, fmt.Errorf("global configuration is nil or monitoring is disabled")
	}

	// Initialize history store
	historyStore, err := datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, instanceLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ParquetFileHistoryStore: %w", err)
	}

	// Initialize HTTP client
	monitorHTTPClientTimeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
	if gCfg.MonitorConfig.HTTPTimeoutSeconds <= 0 {
		monitorHTTPClientTimeout = 30 * time.Second
		instanceLogger.Warn().
			Int("configured_timeout", gCfg.MonitorConfig.HTTPTimeoutSeconds).
			Dur("default_timeout", monitorHTTPClientTimeout).
			Msg("Monitor HTTPTimeoutSeconds invalid, using default")
	}

	clientFactory := common.NewHTTPClientFactory(instanceLogger)
	monitorHttpClient, err := clientFactory.CreateMonitorClient(
		monitorHTTPClientTimeout,
		gCfg.MonitorConfig.MonitorInsecureSkipVerify,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Initialize core dependencies
	fetcher := common.NewFetcher(
		monitorHttpClient,
		instanceLogger,
		&common.HTTPClientFetcherConfig{MaxContentSize: gCfg.MonitorConfig.MaxContentSize},
	)

	processor := NewContentProcessor(instanceLogger)

	contentDiffer, err := differ.NewContentDiffer(instanceLogger, &gCfg.DiffReporterConfig)
	if err != nil {
		instanceLogger.Error().Err(err).Msg("Failed to initialize ContentDiffer")
		contentDiffer = nil
	}

	pathExtractor, err := extractor.NewPathExtractor(gCfg.ExtractorConfig, instanceLogger)
	if err != nil {
		instanceLogger.Error().Err(err).Msg("Failed to initialize PathExtractor")
		pathExtractor = nil
	}

	var htmlDiffReporter *reporter.HtmlDiffReporter
	if historyStore != nil {
		htmlDiffReporter, err = reporter.NewHtmlDiffReporter(instanceLogger, historyStore)
		if err != nil {
			instanceLogger.Error().Err(err).Msg("Failed to initialize HtmlDiffReporter")
			htmlDiffReporter = nil
		} else if notificationHelper != nil {
			notificationHelper.SetDiffReportCleaner(htmlDiffReporter)
		}
	}

	// Initialize modular components
	urlManager := NewURLManager(instanceLogger)
	cycleTracker := NewCycleTracker(instanceLogger, fmt.Sprintf("monitor-init-%s", time.Now().Format("20060102-150405")))
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

	// Initialize event aggregator
	var eventAggregator *EventAggregator
	if gCfg.MonitorConfig.AggregationIntervalSeconds > 0 {
		aggregationInterval := time.Duration(gCfg.MonitorConfig.AggregationIntervalSeconds) * time.Second
		eventAggregator = NewEventAggregator(
			instanceLogger,
			notificationHelper,
			aggregationInterval,
			gCfg.MonitorConfig.MaxAggregatedEvents,
		)
	} else {
		instanceLogger.Warn().Msg("Aggregation interval not configured, events will not be aggregated")
	}

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
	if url == "" {
		return
	}

	s.urlManager.AddURL(url)
	select {
	case s.monitorChan <- url:
		s.logger.Debug().Str("url", url).Msg("URL queued for monitoring")
	default:
		s.logger.Warn().Str("url", url).Msg("Monitor channel full, URL not queued")
	}
}

// GetCurrentlyMonitorUrls returns a copy of currently monitored URLs
func (s *MonitoringService) GetCurrentlyMonitorUrls() []string {
	return s.urlManager.GetCurrentURLs()
}

// Preload adds multiple URLs to the monitored list
func (s *MonitoringService) Preload(initialURLs []string) {
	s.urlManager.PreloadURLs(initialURLs)
}

// CheckURL checks a single URL for changes
func (s *MonitoringService) CheckURL(url string) {
	// Get mutex for this URL to prevent concurrent processing
	urlMutex := s.mutexManager.GetMutex(url)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	cycleID := s.cycleTracker.GetCurrentCycleID()
	result := s.urlChecker.CheckURL(url, cycleID)

	if !result.Success {
		if result.ErrorInfo != nil && s.eventAggregator != nil {
			s.eventAggregator.AddFetchErrorEvent(*result.ErrorInfo)
		}
		return
	}

	if result.FileChangeInfo != nil {
		s.cycleTracker.AddChangedURL(url)
		if s.eventAggregator != nil {
			s.eventAggregator.AddFileChangeEvent(*result.FileChangeInfo)
		}
	}
}

// TriggerCycleEndReport triggers the end-of-cycle report generation
func (s *MonitoringService) TriggerCycleEndReport() {
	s.logger.Info().Str("cycle_id", s.cycleTracker.GetCurrentCycleID()).Msg("Triggering cycle end report")

	monitoredURLs := s.urlManager.GetCurrentURLs()
	changedURLs := s.cycleTracker.GetChangedURLs()

	if len(changedURLs) == 0 {
		s.logger.Info().Int("monitored_count", len(monitoredURLs)).Msg("No changes detected in this cycle")
		return
	}

	// Generate aggregated diff report if HTML diff reporter is available
	if s.urlChecker.htmlDiffReporter != nil {
		reportPath, err := s.urlChecker.htmlDiffReporter.GenerateDiffReport(monitoredURLs, s.cycleTracker.GetCurrentCycleID())
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to generate cycle end diff report")
		} else {
			s.logger.Info().Str("report_path", reportPath).Msg("Generated cycle end diff report")

			if s.notificationHelper != nil {
				data := models.MonitorCycleCompleteData{
					CycleID:        s.cycleTracker.GetCurrentCycleID(),
					ChangedURLs:    changedURLs,
					ReportPath:     reportPath,
					TotalMonitored: len(monitoredURLs),
					Timestamp:      time.Now(),
				}
				s.notificationHelper.SendMonitorCycleCompleteNotification(s.serviceCtx, data)
			}
		}
	}

	// Clear changed URLs for the next cycle
	s.cycleTracker.ClearChangedURLs()
}

// Stop gracefully stops the monitoring service
func (s *MonitoringService) Stop() {
	s.stoppedMutex.Lock()
	defer s.stoppedMutex.Unlock()

	if s.isStopped {
		return
	}

	s.logger.Info().Msg("Stopping monitoring service")

	// Stop event aggregator
	if s.eventAggregator != nil {
		s.eventAggregator.Stop()
	}

	// Cancel service context
	s.serviceCancelFunc()

	// Cleanup
	activeURLs := s.urlManager.GetCurrentURLs()
	s.mutexManager.CleanupUnusedMutexes(activeURLs)

	s.isStopped = true
	s.logger.Info().Msg("Monitoring service stopped")
}

// SetParentContext sets the parent context for the service
func (s *MonitoringService) SetParentContext(parentCtx context.Context) {
	// Cancel current context and create new one from parent
	s.serviceCancelFunc()
	s.serviceCtx, s.serviceCancelFunc = context.WithCancel(parentCtx)
	s.logger.Debug().Msg("Updated service context with new parent")
}

// GenerateNewCycleID generates a new cycle ID
func (s *MonitoringService) GenerateNewCycleID() string {
	newCycleID := fmt.Sprintf("monitor-%s", time.Now().Format("20060102-150405"))
	s.cycleTracker.SetCurrentCycleID(newCycleID)
	s.logger.Info().Str("cycle_id", newCycleID).Msg("Generated new cycle ID")
	return newCycleID
}

// SetCurrentCycleID sets the current cycle ID
func (s *MonitoringService) SetCurrentCycleID(cycleID string) {
	s.cycleTracker.SetCurrentCycleID(cycleID)
}
