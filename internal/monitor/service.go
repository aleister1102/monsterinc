package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	sync "sync"
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

// MonitoringService orchestrates the monitoring of HTML/JS files.
type MonitoringService struct {
	// Fields that are initialized in NewMonitoringService
	gCfg               *config.GlobalConfig
	historyStore       models.FileHistoryStore
	logger             zerolog.Logger
	notificationHelper *notifier.NotificationHelper

	fetcher          *common.Fetcher
	processor        *Processor
	contentDiffer    *differ.ContentDiffer
	pathExtractor    *extractor.PathExtractor
	htmlDiffReporter *reporter.HtmlDiffReporter

	// For managing target URLs dynamically
	monitorChan         chan string         // Channel to receive new URLs to monitor
	monitorUrls         map[string]struct{} // Set of URLs currently being monitored
	monitoredUrlsRMutex sync.RWMutex        // Mutex for monitoredURLs map

	// For aggregating file changes
	fileChangeEvents      []models.FileChangeInfo
	fileChangeEventsMutex sync.Mutex
	// For aggregating fetch/processing errors
	aggregatedFetchErrors      []models.MonitorFetchErrorInfo
	aggregatedFetchErrorsMutex sync.Mutex
	aggregationTicker          *time.Ticker
	aggregationWg              sync.WaitGroup // Added WaitGroup for aggregation worker
	doneChan                   chan struct{}  // To stop the aggregation goroutine

	// For tracking changes within a full monitoring cycle
	changedURLsInCycle      map[string]struct{} // URLs that had changes in the current full cycle
	changedURLsInCycleMutex sync.RWMutex

	// Service specific context for operations like notifications
	serviceCtx        context.Context
	serviceCancelFunc context.CancelFunc
	// Shutdown flag to prevent sending changes during shutdown
	isShuttingDown bool
	shutdownMutex  sync.RWMutex

	// Flag to track if Stop() has already been called
	isStopped    bool
	stoppedMutex sync.Mutex

	// Per-URL mutexes to prevent concurrent processing of the same URL
	urlCheckMutexes      map[string]*sync.Mutex
	urlCheckMutexMapLock sync.RWMutex
}

// NewMonitoringService creates a new instance of MonitoringService.
// Refactored ✅
func NewMonitoringService(
	gCfg *config.GlobalConfig,
	baseLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) *MonitoringService {
	serviceSpecificCtx, serviceSpecificCancel := context.WithCancel(context.Background())
	instanceLogger := baseLogger.With().Str("component", "MonitoringService").Logger()

	if gCfg == nil || !gCfg.MonitorConfig.Enabled {
		instanceLogger.Error().Msg("Global configuration is nil or monitoring is disabled. Monitoring cannot be initialized.")
	}

	// Initialize file history store
	historyStore, hErr := datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, instanceLogger)
	if hErr != nil {
		instanceLogger.Error().Err(hErr).Msg("Failed to initialize ParquetFileHistoryStore for monitoring. Monitoring will be disabled.")
	}

	// Initialize HTTP client for monitor
	monitorHTTPClientTimeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
	if gCfg.MonitorConfig.HTTPTimeoutSeconds <= 0 {
		monitorHTTPClientTimeout = 30 * time.Second
		instanceLogger.Warn().
			Int("configured_timeout", gCfg.MonitorConfig.HTTPTimeoutSeconds).
			Dur("default_timeout", monitorHTTPClientTimeout).
			Msg("Monitor HTTPTimeoutSeconds invalid or not set, using default timeout")
	}
	clientFactory := common.NewHTTPClientFactory(instanceLogger)
	monitorHttpClient, clientErr := clientFactory.CreateMonitorClient(
		monitorHTTPClientTimeout,
		gCfg.MonitorConfig.MonitorInsecureSkipVerify,
	)
	if clientErr != nil {
		instanceLogger.Error().Err(clientErr).Msg("Failed to create HTTP client for monitoring. Monitoring will be disabled.")
	}

	// Initialize fetcher, processor, content differ, and path extractor
	fetcherInstance := common.NewFetcher(
		monitorHttpClient,
		instanceLogger,
		&common.HTTPClientFetcherConfig{MaxContentSize: gCfg.MonitorConfig.MaxContentSize},
	)
	processorInstance := NewProcessor(instanceLogger)
	contentDifferInstance := differ.NewContentDiffer(instanceLogger, &gCfg.DiffReporterConfig)
	pathExtractorInstance, err := extractor.NewPathExtractor(gCfg.ExtractorConfig, instanceLogger)
	if err != nil {
		instanceLogger.Error().Err(err).Msg("Failed to initialize PathExtractor for MonitoringService. Path extraction from JS may not work.")
		pathExtractorInstance = nil
	}

	var htmlDiffReporterInstance *reporter.HtmlDiffReporter
	if historyStore != nil {
		instanceLogger.Info().Msg("Attempting to initialize HtmlDiffReporter for MonitoringService.")
		var errReporter error
		htmlDiffReporterInstance, errReporter = reporter.NewHtmlDiffReporter(instanceLogger, historyStore)
		if errReporter != nil {
			instanceLogger.Error().Err(errReporter).Str("error_type", fmt.Sprintf("%T", errReporter)).Msg("Failed to initialize HtmlDiffReporter for MonitoringService. Aggregated diff reporting will be impacted.")
			htmlDiffReporterInstance = nil // Ensure it's nil on error
		} else {
			instanceLogger.Info().Msg("HtmlDiffReporter initialized successfully for MonitoringService.")
		}
	} else {
		instanceLogger.Warn().Bool("has_store", historyStore != nil).Msg("HtmlDiffReporter not initialized due to missing mainReporterCfg or historyStore.")
	}

	s := &MonitoringService{
		gCfg:                  gCfg,
		historyStore:          historyStore,
		logger:                instanceLogger,
		notificationHelper:    notificationHelper,
		fetcher:               fetcherInstance,
		processor:             processorInstance,
		contentDiffer:         contentDifferInstance,
		htmlDiffReporter:      htmlDiffReporterInstance,
		pathExtractor:         pathExtractorInstance,
		monitorChan:           make(chan string, gCfg.MonitorConfig.MaxConcurrentChecks*2),
		monitorUrls:           make(map[string]struct{}),
		monitoredUrlsRMutex:   sync.RWMutex{},
		fileChangeEvents:      make([]models.FileChangeInfo, 0),
		aggregatedFetchErrors: make([]models.MonitorFetchErrorInfo, 0),
		doneChan:              make(chan struct{}),
		serviceCtx:            serviceSpecificCtx,
		serviceCancelFunc:     serviceSpecificCancel, changedURLsInCycle: make(map[string]struct{}),
		changedURLsInCycleMutex: sync.RWMutex{},
		isShuttingDown:          false,
		shutdownMutex:           sync.RWMutex{},
		isStopped:               false,
		stoppedMutex:            sync.Mutex{},
		urlCheckMutexes:         make(map[string]*sync.Mutex),
		urlCheckMutexMapLock:    sync.RWMutex{},
	}
	// Setup aggregation worker if interval is configured
	if s.gCfg.MonitorConfig.AggregationIntervalSeconds <= 0 {
		s.logger.Warn().Int("interval_seconds", s.gCfg.MonitorConfig.AggregationIntervalSeconds).Msg("Monitor aggregation interval is not configured or invalid. Aggregation worker will not start.")
	} else {
		aggregationDuration := time.Duration(s.gCfg.MonitorConfig.AggregationIntervalSeconds) * time.Second
		s.aggregationTicker = time.NewTicker(aggregationDuration)
		s.aggregationWg.Add(1) // Increment WaitGroup before starting worker
		go s.aggregationWorker()
		s.logger.Info().Dur("interval", aggregationDuration).Int("max_events", s.gCfg.MonitorConfig.MaxAggregatedEvents).Msg("Aggregation worker started for monitor events.")
	}

	return s
}

// AddMonitorUrl adds a URL to the list of monitored URLs.
// This method will be called by the unified scheduler.
// No need refactoring ✅
func (s *MonitoringService) AddMonitorUrl(url string) {
	if url == "" {
		return
	}
	s.monitoredUrlsRMutex.Lock()
	defer s.monitoredUrlsRMutex.Unlock()

	if _, exists := s.monitorUrls[url]; !exists {
		s.monitorUrls[url] = struct{}{}
		s.logger.Info().Str("url", url).Msg("Added new target URL for monitoring.")
	} else {
		s.logger.Debug().Str("url", url).Msg("Target URL already in monitoring list.")
	}
}

// GetCurrentlyMonitorUrls returns a copy of the current list of monitored URLs.
// This method will be called by the scheduler.
// No need refactoring ✅
func (s *MonitoringService) GetCurrentlyMonitorUrls() []string {
	s.monitoredUrlsRMutex.RLock()
	defer s.monitoredUrlsRMutex.RUnlock()

	urls := make([]string, 0, len(s.monitorUrls))
	for url := range s.monitorUrls {
		urls = append(urls, url)
	}
	return urls
}

func (s *MonitoringService) Preload(initialURLs []string) {
	for _, u := range initialURLs {
		s.AddMonitorUrl(u)
	}
}

// Stop signals the MonitoringService to shut down gracefully.
func (s *MonitoringService) Stop() {
	s.stoppedMutex.Lock()
	if s.isStopped {
		s.stoppedMutex.Unlock()
		s.logger.Info().Msg("MonitoringService.Stop() called, but service is already stopped. Ignoring.")
		return
	}
	s.isStopped = true
	s.stoppedMutex.Unlock()

	s.logger.Info().Msg("Attempting to stop MonitoringService...")

	// Set shutdown flag to prevent sending aggregated changes
	s.shutdownMutex.Lock()
	s.isShuttingDown = true
	s.shutdownMutex.Unlock()

	// Get current monitored URLs for notification before cleanup
	currentMonitoredURLs := s.GetCurrentlyMonitorUrls()

	// Skip sending aggregated changes during shutdown to avoid confusion
	s.logger.Info().Msg("Skipping aggregated changes during shutdown...")

	// Stop aggregation ticker
	if s.aggregationTicker != nil {
		s.aggregationTicker.Stop()
	} // Determine if this is an interrupt (context cancelled) or normal shutdown
	isInterrupt := s.serviceCtx.Err() != nil

	// Send interrupted notification for monitor service only if interrupted
	if s.notificationHelper != nil && len(currentMonitoredURLs) > 0 && isInterrupt {
		s.logger.Info().Int("monitored_url_count", len(currentMonitoredURLs)).Msg("Sending monitor service interrupted notification...")

		// Create ScanSummaryData for interrupted monitor service
		interruptedSummary := models.ScanSummaryData{
			ScanSessionID: time.Now().Format("20060102-150405-monitor"),
			TargetSource:  "monitor_service",
			Targets:       currentMonitoredURLs,
			TotalTargets:  len(currentMonitoredURLs),
			Status:        string(models.ScanStatusInterrupted),
			ErrorMessages: []string{"Monitor service was interrupted by signal or context cancellation"},
			Component:     "MonitorService",
		}

		// Use a background context since serviceCtx might be cancelled
		s.notificationHelper.SendMonitorInterruptNotification(context.Background(), interruptedSummary)
	} else if len(currentMonitoredURLs) > 0 {
		s.logger.Info().Int("monitored_url_count", len(currentMonitoredURLs)).Msg("Monitor service stopping normally, no interrupt notification needed.")
	}

	// Cancel the service-specific context to stop other goroutines like monitorChan listener
	if s.serviceCancelFunc != nil {
		s.serviceCancelFunc()
	}

	// Safely close doneChan only once
	s.logger.Debug().Msg("Attempting to close doneChan for aggregation worker...")
	select {
	case <-s.doneChan:
		// Channel is already closed
		s.logger.Debug().Msg("doneChan was already closed")
	default:
		// Channel is open, safe to close
		close(s.doneChan)
		s.logger.Debug().Msg("doneChan closed successfully")
	}

	// Wait for the aggregation worker to finish its current tasks, including final sends
	s.logger.Info().Msg("Waiting for aggregation worker to complete...")
	s.aggregationWg.Wait()
	s.logger.Info().Msg("Aggregation worker completed.")

	s.logger.Info().Msg("MonitoringService stopped.")
}

func (s *MonitoringService) aggregationWorker() {
	defer s.aggregationWg.Done() // Ensure WaitGroup is decremented when worker exits
	defer s.logger.Info().Msg("Aggregation worker stopped.")

	for {
		select {
		case <-s.aggregationTicker.C:
			s.sendAggregatedChanges()
			s.sendAggregatedErrors()
		case <-s.doneChan:
			s.logger.Info().Msg("Aggregation worker stopping.")
			// Perform final send before exiting if there are pending events
			s.sendAggregatedChanges()
			s.sendAggregatedErrors()
			return
		}
	}
}

func (s *MonitoringService) sendAggregatedChanges() {
	s.fileChangeEventsMutex.Lock()
	defer s.fileChangeEventsMutex.Unlock()

	s.shutdownMutex.RLock()
	isShuttingDown := s.isShuttingDown
	s.shutdownMutex.RUnlock()

	if isShuttingDown {
		return
	}

	if len(s.fileChangeEvents) == 0 {
		return
	}

	s.logger.Info().Int("count", len(s.fileChangeEvents)).Msg("Aggregated file changes detected, sending notification.")

	// Send notification for aggregated changes
	if s.notificationHelper != nil {
		s.notificationHelper.SendAggregatedFileChangesNotification(s.serviceCtx, s.fileChangeEvents, "")
	}

	s.logger.Info().Msg("Aggregated file changes notification sent and event list cleared.")
	s.fileChangeEvents = nil
}

func (s *MonitoringService) sendAggregatedErrors() {
	s.aggregatedFetchErrorsMutex.Lock()
	defer s.aggregatedFetchErrorsMutex.Unlock()

	if len(s.aggregatedFetchErrors) == 0 {
		return
	}

	s.logger.Info().Int("count", len(s.aggregatedFetchErrors)).Msg("Aggregated monitor fetch/process errors occurred.")

	for i, errInfo := range s.aggregatedFetchErrors {
		s.logger.Info().Int("index", i+1).Str("url", errInfo.URL).Str("source", errInfo.Source).Str("error", errInfo.Error).Time("occurred_at", errInfo.OccurredAt).Msg("Aggregated monitor error detail")
	}

	s.logger.Info().Int("count", len(s.aggregatedFetchErrors)).Msg("Sending aggregated monitor error notifications.")

	if s.notificationHelper != nil {
		s.notificationHelper.SendAggregatedMonitorErrorsNotification(s.serviceCtx, s.aggregatedFetchErrors)
	}

	s.aggregatedFetchErrors = nil
	s.logger.Info().Msg("Aggregated monitor error list cleared after logging.")
}

// CheckURL performs the actual check for a single URL. This is an exported wrapper for checkURL.
func (s *MonitoringService) CheckURL(url string) {
	s.checkURL(url)
}

// checkURL performs the actual check for a single URL. Called by the unified scheduler's workers.
func (s *MonitoringService) checkURL(url string) {
	s.logger.Debug().Str("url", url).Msg("Starting to check URL")
	// Get URL-specific mutex to prevent concurrent processing of the same URL
	urlCheckMutex := s.getURLCheckMutex(url)
	urlCheckMutex.Lock()
	defer urlCheckMutex.Unlock()

	// Fetch current state from history
	previousRecord, err := s.historyStore.GetLastKnownRecord(url)                     // Corrected method
	if err != nil && err.Error() != "level=error msg=\"Record not found\" url="+url { // A bit brittle check
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to get latest record from history store")
		// Continue, as this might be the first time seeing the URL
	}

	var etag, lastModified string
	if previousRecord != nil {
		etag = previousRecord.ETag
		lastModified = previousRecord.LastModified
	}

	fetchInput := common.FetchFileContentInput{ // Will be common.FetchFileContentInput
		URL:                  url,
		PreviousETag:         etag,
		PreviousLastModified: lastModified,
	}

	fetchResult, fetchErr := s.fetcher.FetchFileContent(fetchInput)

	if fetchErr != nil {
		if fetchErr == common.ErrNotModified { // Will be common.ErrNotModified
			s.logger.Info().Str("url", url).Msg("Content not modified (304), skipping further processing.")
			// Potentially update last checked timestamp for this URL if it exists and content is not modified
			if previousRecord != nil {
				// No, we don't store a new record for 304. We just log and return.
				// If we wanted to update a "last_polled_at" field without content change,
				// the data model and store logic would need to support that.
			}
			return // Important: return if not modified
		}
		s.logger.Error().Err(fetchErr).Str("url", url).Msg("Failed to fetch file content")
		s.aggregatedFetchErrorsMutex.Lock()
		s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      fetchErr.Error(),
			Source:     "fetch",
			OccurredAt: time.Now(),
		})
		s.aggregatedFetchErrorsMutex.Unlock()
		return
	}

	processedUpdate, err := s.processURLContent(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to process file content")
		s.aggregatedFetchErrorsMutex.Lock()
		s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
			Source:     "process",
			OccurredAt: time.Now(),
		})
		s.aggregatedFetchErrorsMutex.Unlock()
		return
	}

	changeInfo, diffResult, err := s.detectURLChanges(url, processedUpdate, fetchResult)
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Error detecting URL changes")
		return
	}

	if err := s.storeURLRecord(url, processedUpdate, fetchResult, diffResult); err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to store URL record")
		s.aggregatedFetchErrorsMutex.Lock()
		s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
			Source:     "store_history",
			OccurredAt: time.Now(),
		})
		s.aggregatedFetchErrorsMutex.Unlock()
		return
	}

	if changeInfo != nil {
		s.logger.Info().Str("url", url).Msg("Change detected")

		s.fileChangeEventsMutex.Lock()
		s.fileChangeEvents = append(s.fileChangeEvents, *changeInfo)
		s.fileChangeEventsMutex.Unlock()

		s.changedURLsInCycleMutex.Lock()
		s.changedURLsInCycle[url] = struct{}{}
		s.changedURLsInCycleMutex.Unlock()
	}
}

// processURLContent hashes the content and prepares an update structure.
func (s *MonitoringService) processURLContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error) {
	return s.processor.ProcessContent(url, content, contentType)
}

// detectURLChanges compares current content with historical data and detects changes
// fetchResult will be common.FetchFileContentResult
func (s *MonitoringService) detectURLChanges(url string, processedUpdate *models.MonitoredFileUpdate, fetchResult *common.FetchFileContentResult) (*models.FileChangeInfo, *models.ContentDiffResult, error) {
	s.logger.Debug().Str("url", url).Msg("Detecting changes for URL.")
	// Compare with historical data
	lastRecord, err := s.historyStore.GetLastKnownRecord(url)
	if err != nil || lastRecord == nil {
		s.logger.Debug().Err(err).Str("url", url).Msg("No previous record found for comparison. Treating as new file.")

		// Extract paths for new JS files
		var extractedPaths []models.ExtractedPath
		if s.pathExtractor != nil && (fetchResult.ContentType == "application/javascript" || strings.Contains(fetchResult.ContentType, "javascript")) {
			paths, err := s.pathExtractor.ExtractPaths(url, fetchResult.Content, fetchResult.ContentType)
			if err != nil {
				s.logger.Error().Err(err).Str("url", url).Msg("Failed to extract paths from new JS content")
			} else {
				extractedPaths = paths
			}
		}

		// Create change info for new file
		changeInfo := &models.FileChangeInfo{
			URL:            url,
			OldHash:        "", // No previous hash for new files
			NewHash:        processedUpdate.NewHash,
			ContentType:    fetchResult.ContentType,
			ChangeTime:     processedUpdate.FetchedAt,
			ExtractedPaths: extractedPaths,
		}

		// Generate diff result for new file (treat entire content as new insertions)
		var diffResult *models.ContentDiffResult
		if s.contentDiffer != nil && s.gCfg.MonitorConfig.StoreFullContentOnChange {
			var diffErr error
			// Pass empty previous content to treat entire current content as new
			diffResult, diffErr = s.contentDiffer.GenerateDiff([]byte{}, fetchResult.Content, fetchResult.ContentType, "", processedUpdate.NewHash)
			if diffErr != nil {
				s.logger.Error().Err(diffErr).Str("url", url).Msg("Failed to generate content diff for new file")
			} else {
				diffResult.ExtractedPaths = extractedPaths
				s.logger.Info().Str("url", url).Bool("is_identical", diffResult.IsIdentical).Int("diff_count", len(diffResult.Diffs)).Msg("Content diff generated for new file")

				// Generate HTML diff report for new file
				if s.htmlDiffReporter != nil {
					reportPath, reportErr := s.htmlDiffReporter.GenerateSingleDiffReport(url, diffResult, "", processedUpdate.NewHash, fetchResult.Content)
					if reportErr != nil {
						s.logger.Error().Err(reportErr).Str("url", url).Msg("Failed to generate single HTML diff report for new file")
					} else {
						s.logger.Info().Str("url", url).Str("report_path", reportPath).Msg("Single HTML diff report created for new file")
						changeInfo.DiffReportPath = &reportPath
					}
				}
			}
		}

		s.logger.Info().Str("url", url).Str("new_hash", processedUpdate.NewHash).Msg("New file detected")
		return changeInfo, diffResult, nil
	}

	oldHash := lastRecord.Hash
	if oldHash == processedUpdate.NewHash {
		return nil, nil, nil
	}

	s.logger.Info().Str("url", url).Str("old_hash", oldHash).Str("new_hash", processedUpdate.NewHash).Msg("Change detected")

	var extractedPaths []models.ExtractedPath
	if s.pathExtractor != nil && (fetchResult.ContentType == "application/javascript" || strings.Contains(fetchResult.ContentType, "javascript")) {
		paths, err := s.pathExtractor.ExtractPaths(url, fetchResult.Content, fetchResult.ContentType)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Failed to extract paths from JS content during change detection")
		} else {
			extractedPaths = paths
		}
	}

	changeInfo := &models.FileChangeInfo{
		URL:            url,
		OldHash:        oldHash,
		NewHash:        processedUpdate.NewHash,
		ContentType:    fetchResult.ContentType,
		ChangeTime:     processedUpdate.FetchedAt,
		ExtractedPaths: extractedPaths,
	}

	var diffResult *models.ContentDiffResult
	if s.contentDiffer != nil && s.gCfg.MonitorConfig.StoreFullContentOnChange {
		var diffErr error
		diffResult, diffErr = s.contentDiffer.GenerateDiff(lastRecord.Content, fetchResult.Content, fetchResult.ContentType, oldHash, processedUpdate.NewHash)
		if diffErr != nil {
			s.logger.Error().Err(diffErr).Str("url", url).Msg("Failed to generate content diff")
		} else {
			diffResult.ExtractedPaths = extractedPaths
			s.logger.Info().Str("url", url).Bool("is_identical", diffResult.IsIdentical).Int("diff_count", len(diffResult.Diffs)).Msg("Content diff generated")

			if s.htmlDiffReporter != nil {
				reportPath, reportErr := s.htmlDiffReporter.GenerateSingleDiffReport(url, diffResult, oldHash, processedUpdate.NewHash, fetchResult.Content)
				if reportErr != nil {
					s.logger.Error().Err(reportErr).Str("url", url).Msg("Failed to generate single HTML diff report")
				} else {
					s.logger.Info().Str("url", url).Str("report_path", reportPath).Msg("Single HTML diff report created")
					changeInfo.DiffReportPath = &reportPath
				}
			}
		}
	}

	return changeInfo, diffResult, nil
}

// storeURLRecord saves the history record to the datastore.
// fetchResult will be common.FetchFileContentResult
func (s *MonitoringService) storeURLRecord(url string, processedUpdate *models.MonitoredFileUpdate, fetchResult *common.FetchFileContentResult, diffResult *models.ContentDiffResult) error {
	var diffResultJSON *string
	if diffResult != nil {
		jsonBytes, err := json.Marshal(diffResult)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Failed to marshal diff result to JSON for storage")
		} else {
			jsonStr := string(jsonBytes)
			diffResultJSON = &jsonStr
		}
	}

	var extractedPathsJSON *string
	if diffResult != nil && len(diffResult.ExtractedPaths) > 0 {
		jsonBytes, err := json.Marshal(diffResult.ExtractedPaths)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Failed to marshal extracted paths to JSON for storage")
		} else {
			jsonStr := string(jsonBytes)
			extractedPathsJSON = &jsonStr
		}
	}

	if s.historyStore == nil {
		s.logger.Warn().Str("url", url).Msg("History store is nil, cannot store file record.")
		return nil
	}

	return s.historyStore.StoreFileRecord(models.FileHistoryRecord{
		URL:                url,
		Timestamp:          processedUpdate.FetchedAt.Unix(),
		Hash:               processedUpdate.NewHash,
		ContentType:        fetchResult.ContentType,
		Content:            fetchResult.Content, // Store full content if configured
		ETag:               fetchResult.ETag,
		LastModified:       fetchResult.LastModified,
		DiffResultJSON:     diffResultJSON,
		ExtractedPathsJSON: extractedPathsJSON,
	})
}

// TriggerCycleEndReport is called by the unified scheduler when a full monitoring cycle for all URLs is complete.
func (s *MonitoringService) TriggerCycleEndReport() {
	s.logger.Info().Msg("Monitoring cycle complete. Triggering end-of-cycle report and notification.")

	// Cleanup unused mutexes to prevent memory leaks
	s.cleanupUnusedMutexes()
	// Get file changes for cycle report (but don't clear them if aggregation is enabled)
	s.fileChangeEventsMutex.Lock()
	fileChanges := make([]models.FileChangeInfo, len(s.fileChangeEvents))
	copy(fileChanges, s.fileChangeEvents)
	// Only clear events if aggregation is disabled (let aggregation worker handle them otherwise)
	if s.gCfg.MonitorConfig.AggregationIntervalSeconds <= 0 {
		s.fileChangeEvents = nil // Clear the events only if aggregation is disabled
	}
	s.fileChangeEventsMutex.Unlock()

	// Send aggregated errors (but not file changes - those will be in cycle complete)
	s.sendAggregatedErrors()

	s.changedURLsInCycleMutex.RLock()
	changedURLs := make([]string, 0, len(s.changedURLsInCycle))
	for url := range s.changedURLsInCycle {
		changedURLs = append(changedURLs, url)
	}
	s.changedURLsInCycleMutex.RUnlock()

	var reportPath string
	var err error

	if s.htmlDiffReporter != nil {
		// Generate an aggregated report for ALL currently monitored URLs,
		// not just the ones that changed in this specific interval.
		// The report itself will highlight which ones had diffs.
		monitoredNow := s.GetCurrentlyMonitorUrls()
		if len(monitoredNow) > 0 {
			reportPath, err = s.htmlDiffReporter.GenerateDiffReport(monitoredNow)
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to generate aggregated monitor diff report")
				// Proceed to send notification without report path if generation failed
			} else {
				s.logger.Info().Str("report_path", reportPath).Msg("Aggregated monitor diff report generated successfully.")
			}
		} else {
			s.logger.Info().Msg("No URLs currently monitored, skipping aggregated diff report generation.")
		}
	} else {
		s.logger.Warn().Bool("htmlDiffReporter_is_nil", s.htmlDiffReporter == nil).Msg("HtmlDiffReporter is not initialized. Cannot generate aggregated monitor diff report.")
	}

	// Send notification for the completed cycle with file changes included
	if s.notificationHelper != nil {
		monitoredCount := len(s.GetCurrentlyMonitorUrls())
		cycleCompleteData := models.MonitorCycleCompleteData{
			ChangedURLs:    changedURLs,
			FileChanges:    fileChanges, // Include detailed file changes
			ReportPath:     reportPath,  // This will be empty if report generation failed or was skipped
			TotalMonitored: monitoredCount,
			Timestamp:      time.Now(),
		}
		s.notificationHelper.SendMonitorCycleCompleteNotification(s.serviceCtx, cycleCompleteData)
	}

	// Clear the list of changed URLs for the next cycle
	s.changedURLsInCycleMutex.Lock()
	s.changedURLsInCycle = make(map[string]struct{})
	s.changedURLsInCycleMutex.Unlock()
	s.logger.Info().Msg("Cleared changed URLs list for the next monitoring cycle.")
}

// setParentContext allows setting a parent context for proper interrupt detection
func (s *MonitoringService) SetParentContext(parentCtx context.Context) {
	s.logger.Debug().Msg("Setting parent context for MonitoringService")
	// Cancel the current service context and create a new one derived from the parent
	if s.serviceCancelFunc != nil {
		s.serviceCancelFunc()
	}
	s.serviceCtx, s.serviceCancelFunc = context.WithCancel(parentCtx)
}

// getURLCheckMutex returns a mutex for the specific URL to ensure thread-safety
func (s *MonitoringService) getURLCheckMutex(url string) *sync.Mutex {
	s.urlCheckMutexMapLock.RLock()
	mutex, exists := s.urlCheckMutexes[url]
	s.urlCheckMutexMapLock.RUnlock()

	if exists {
		return mutex
	}

	s.urlCheckMutexMapLock.Lock()
	defer s.urlCheckMutexMapLock.Unlock()

	// Double-check after acquiring write lock
	if mutex, exists := s.urlCheckMutexes[url]; exists {
		return mutex
	}

	mutex = &sync.Mutex{}
	s.urlCheckMutexes[url] = mutex
	return mutex
}

// cleanupUnusedMutexes removes mutexes for URLs that are no longer monitored
func (s *MonitoringService) cleanupUnusedMutexes() {
	s.monitoredUrlsRMutex.RLock()
	monitoredURLs := make(map[string]struct{})
	for url := range s.monitorUrls {
		monitoredURLs[url] = struct{}{}
	}
	s.monitoredUrlsRMutex.RUnlock()

	s.urlCheckMutexMapLock.Lock()
	defer s.urlCheckMutexMapLock.Unlock()

	for url := range s.urlCheckMutexes {
		if _, isMonitored := monitoredURLs[url]; !isMonitored {
			delete(s.urlCheckMutexes, url)
		}
	}
}
