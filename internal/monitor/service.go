package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	sync "sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/secrets"

	"github.com/rs/zerolog"
)

// MonitoringService orchestrates the monitoring of HTML/JS files.
type MonitoringService struct {
	cfg                *config.MonitorConfig
	crawlerCfg         *config.CrawlerConfig
	extractorConfig    *config.ExtractorConfig
	notificationCfg    *config.NotificationConfig
	reporterConfig     *config.ReporterConfig
	secretsConfig      *config.SecretsConfig
	historyStore       models.FileHistoryStore
	logger             zerolog.Logger
	notificationHelper *notifier.NotificationHelper
	httpClient         *http.Client
	fetcher            *Fetcher
	processor          *Processor
	contentDiffer      *differ.ContentDiffer
	htmlDiffReporter   *reporter.HtmlDiffReporter
	pathExtractor      *extractor.PathExtractor
	secretDetector     *secrets.SecretDetectorService

	// For managing target URLs dynamically
	monitorChan        chan string         // Channel to receive new URLs to monitor
	monitoredURLs      map[string]struct{} // Set of URLs currently being monitored
	monitoredURLsMutex sync.RWMutex        // Mutex for monitoredURLs map

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

	// Per-URL mutexes to prevent concurrent processing of the same URL
	urlCheckMutexes      map[string]*sync.Mutex
	urlCheckMutexMapLock sync.RWMutex
}

// NewMonitoringService creates a new instance of MonitoringService.
func NewMonitoringService(
	monitorCfg *config.MonitorConfig,
	crawlerCfg *config.CrawlerConfig,
	extractorCfg *config.ExtractorConfig,
	notificationCfg *config.NotificationConfig,
	mainReporterCfg *config.ReporterConfig,
	diffReporterCfg *config.DiffReporterConfig,
	secretsCfg *config.SecretsConfig,
	store models.FileHistoryStore,
	baseLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	httpClient *http.Client,
	secretDetector *secrets.SecretDetectorService,
) *MonitoringService {
	serviceSpecificCtx, serviceSpecificCancel := context.WithCancel(context.Background())
	instanceLogger := baseLogger.With().Str("component", "MonitoringService").Logger()

	fetcherInstance := NewFetcher(httpClient, instanceLogger, monitorCfg)
	processorInstance := NewProcessor(instanceLogger)
	contentDifferInstance := differ.NewContentDiffer(instanceLogger, diffReporterCfg)

	// Initialize PathExtractor using the passed extractorCfg
	pathExtractorInstance, err := extractor.NewPathExtractor(*extractorCfg, instanceLogger)
	if err != nil {
		instanceLogger.Error().Err(err).Msg("Failed to initialize PathExtractor for MonitoringService. Path extraction from JS may not work.")
		pathExtractorInstance = nil
	}

	var htmlDiffReporterInstance *reporter.HtmlDiffReporter
	if mainReporterCfg != nil && store != nil {
		instanceLogger.Info().Msg("Attempting to initialize HtmlDiffReporter for MonitoringService.")
		var errReporter error
		htmlDiffReporterInstance, errReporter = reporter.NewHtmlDiffReporter(mainReporterCfg, instanceLogger, store)
		if errReporter != nil {
			instanceLogger.Error().Err(errReporter).Str("error_type", fmt.Sprintf("%T", errReporter)).Msg("Failed to initialize HtmlDiffReporter for MonitoringService. Aggregated diff reporting will be impacted.")
			htmlDiffReporterInstance = nil // Ensure it's nil on error
		} else {
			instanceLogger.Info().Msg("HtmlDiffReporter initialized successfully for MonitoringService.")
		}
	} else {
		instanceLogger.Warn().Bool("has_reporter_cfg", mainReporterCfg != nil).Bool("has_store", store != nil).Msg("HtmlDiffReporter not initialized due to missing mainReporterCfg or historyStore.")
	}

	s := &MonitoringService{
		cfg:                     monitorCfg,
		crawlerCfg:              crawlerCfg,
		extractorConfig:         extractorCfg,
		notificationCfg:         notificationCfg,
		reporterConfig:          mainReporterCfg,
		secretsConfig:           secretsCfg,
		historyStore:            store,
		logger:                  instanceLogger,
		notificationHelper:      notificationHelper,
		httpClient:              httpClient,
		fetcher:                 fetcherInstance,
		processor:               processorInstance,
		contentDiffer:           contentDifferInstance,
		htmlDiffReporter:        htmlDiffReporterInstance,
		pathExtractor:           pathExtractorInstance,
		secretDetector:          secretDetector,
		monitorChan:             make(chan string, monitorCfg.MaxConcurrentChecks*2),
		monitoredURLs:           make(map[string]struct{}),
		monitoredURLsMutex:      sync.RWMutex{},
		fileChangeEvents:        make([]models.FileChangeInfo, 0),
		aggregatedFetchErrors:   make([]models.MonitorFetchErrorInfo, 0),
		doneChan:                make(chan struct{}),
		serviceCtx:              serviceSpecificCtx,
		serviceCancelFunc:       serviceSpecificCancel,
		changedURLsInCycle:      make(map[string]struct{}),
		changedURLsInCycleMutex: sync.RWMutex{},
		isShuttingDown:          false,
		shutdownMutex:           sync.RWMutex{},
		urlCheckMutexes:         make(map[string]*sync.Mutex),
		urlCheckMutexMapLock:    sync.RWMutex{},
	}

	// Log final state of htmlDiffReporter
	instanceLogger.Info().Bool("htmlDiffReporter_assigned", s.htmlDiffReporter != nil).Msg("MonitoringService created with htmlDiffReporter state.")

	if s.cfg.AggregationIntervalSeconds <= 0 {
		s.logger.Warn().Int("interval_seconds", s.cfg.AggregationIntervalSeconds).Msg("Monitor aggregation interval is not configured or invalid. Aggregation worker will not start.")
	} else {
		aggregationDuration := time.Duration(s.cfg.AggregationIntervalSeconds) * time.Second
		s.aggregationTicker = time.NewTicker(aggregationDuration)
		s.aggregationWg.Add(1) // Increment WaitGroup before starting worker
		go s.aggregationWorker()
		s.logger.Info().Dur("interval", aggregationDuration).Int("max_events", s.cfg.MaxAggregatedEvents).Msg("Aggregation worker started for monitor events.")
	}

	return s
}

// AddTargetURL adds a URL to the list of monitored URLs.
// This method will be called by the unified scheduler.
func (s *MonitoringService) AddTargetURL(url string) {
	if url == "" {
		return
	}
	s.monitoredURLsMutex.Lock()
	defer s.monitoredURLsMutex.Unlock()

	if _, exists := s.monitoredURLs[url]; !exists {
		s.monitoredURLs[url] = struct{}{}
		s.logger.Info().Str("url", url).Msg("Added new target URL for monitoring.")
	} else {
		s.logger.Debug().Str("url", url).Msg("Target URL already in monitoring list.")
	}
}

// RemoveTargetURL removes a URL from the monitoring list.
func (s *MonitoringService) RemoveTargetURL(url string) {
	s.monitoredURLsMutex.Lock()
	defer s.monitoredURLsMutex.Unlock()

	if _, exists := s.monitoredURLs[url]; exists {
		delete(s.monitoredURLs, url)
		s.logger.Info().Str("url", url).Msg("Removed target URL from monitoring.")
	}
}

// GetCurrentlyMonitoredURLs returns a copy of the current list of monitored URLs.
// This method will be called by the scheduler.
func (s *MonitoringService) GetCurrentlyMonitoredURLs() []string {
	s.monitoredURLsMutex.RLock()
	defer s.monitoredURLsMutex.RUnlock()

	urls := make([]string, 0, len(s.monitoredURLs))
	for url := range s.monitoredURLs {
		urls = append(urls, url)
	}
	return urls
}

// Start begins the monitoring process.
func (s *MonitoringService) Start(initialURLs []string) error {
	s.logger.Info().Msg("Starting MonitoringService...")

	for _, u := range initialURLs {
		s.AddTargetURL(u)
	}

	// Send initial list of monitored URLs
	monitoredNow := s.GetCurrentlyMonitoredURLs()
	if len(monitoredNow) > 0 && s.notificationHelper != nil {
		s.notificationHelper.SendInitialMonitoredURLsNotification(s.serviceCtx, monitoredNow)
	}

	// Perform initial check of all monitored URLs immediately
	if len(monitoredNow) > 0 {
		s.logger.Info().Int("url_count", len(monitoredNow)).Msg("Performing initial check of all monitored URLs...")
		for _, url := range monitoredNow {
			s.CheckURL(url)
		}
		s.logger.Info().Msg("Initial check of all monitored URLs completed.")

		// Trigger cycle end report after initial checks to send any changes found
		s.TriggerCycleEndReport()
	}

	s.logger.Info().Msg("MonitoringService started successfully.")
	return nil
}

// Stop signals the MonitoringService to shut down gracefully.
func (s *MonitoringService) Stop() {
	s.logger.Info().Msg("Attempting to stop MonitoringService...")

	// Set shutdown flag to prevent sending aggregated changes
	s.shutdownMutex.Lock()
	s.isShuttingDown = true
	s.shutdownMutex.Unlock()

	// Get current monitored URLs for notification before cleanup
	currentMonitoredURLs := s.GetCurrentlyMonitoredURLs()

	// Skip sending aggregated changes during shutdown to avoid confusion
	s.logger.Info().Msg("Skipping aggregated changes during shutdown...")

	// Stop aggregation ticker
	if s.aggregationTicker != nil {
		s.aggregationTicker.Stop()
	}

	// Send interrupted notification using scan completion format
	if s.notificationHelper != nil && len(currentMonitoredURLs) > 0 {
		s.logger.Info().Int("monitored_url_count", len(currentMonitoredURLs)).Msg("Sending monitor service interrupted notification...")

		// Create ScanSummaryData for interrupted monitor service
		interruptedSummary := models.ScanSummaryData{
			ScanSessionID: time.Now().Format("20060102-150405-monitor"),
			TargetSource:  "monitor_service",
			Targets:       currentMonitoredURLs,
			TotalTargets:  len(currentMonitoredURLs),
			Status:        string(models.ScanStatusInterrupted),
			ErrorMessages: []string{"Monitor service was stopped/interrupted"},
			Component:     "MonitorService",
		}

		// Use a background context since serviceCtx might be cancelled
		s.notificationHelper.SendScanCompletionNotification(context.Background(), interruptedSummary, notifier.MonitorServiceNotification, nil)
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
	defer s.logger.Info().Msg("Aggregation worker stopped.")

	for {
		select {
		case <-s.aggregationTicker.C:
			s.sendAggregatedChanges()
			s.sendAggregatedErrors()
		case <-s.doneChan:
			s.logger.Info().Msg("Aggregation worker stopping.")
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

	s.logger.Info().Int("count", len(s.fileChangeEvents)).Msg("Aggregated file changes detected (will be reported in cycle complete).")

	// Note: We no longer send notification here, only log the changes
	// The notification will be sent in TriggerCycleEndReport with Monitor Cycle Complete

	s.logger.Info().Msg("Aggregated file changes logged and event list cleared.")
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
	// Get URL-specific mutex to prevent concurrent processing of the same URL
	urlCheckMutex := s.getURLCheckMutex(url)
	urlCheckMutex.Lock()
	defer urlCheckMutex.Unlock()

	fetchResult, err := s.fetcher.FetchFileContent(FetchFileContentInput{
		URL: url,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to fetch file content")
		s.aggregatedFetchErrorsMutex.Lock()
		s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      err.Error(),
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

	var secretFindings []models.SecretFinding
	if s.secretDetector != nil {
		findings, err := s.secretDetector.ScanContent(url, fetchResult.Content, fetchResult.ContentType)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Error during secret detection for monitored file")
		} else if len(findings) > 0 {
			s.logger.Info().Int("count", len(findings)).Str("url", url).Msg("Secrets found in monitored file content")
			secretFindings = findings
		}
	}

	changeInfo, diffResult, err := s.detectURLChanges(url, processedUpdate, fetchResult, secretFindings)
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
func (s *MonitoringService) detectURLChanges(url string, processedUpdate *models.MonitoredFileUpdate, fetchResult *FetchFileContentResult, secretFindings []models.SecretFinding) (*models.FileChangeInfo, *models.ContentDiffResult, error) {
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
			SecretFindings: secretFindings,
		}

		s.logger.Info().Str("url", url).Str("new_hash", processedUpdate.NewHash).Msg("New file detected")
		return changeInfo, nil, nil
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
		SecretFindings: secretFindings,
	}

	var diffResult *models.ContentDiffResult
	if s.contentDiffer != nil && s.cfg.StoreFullContentOnChange {
		var diffErr error
		diffResult, diffErr = s.contentDiffer.GenerateDiff(lastRecord.Content, fetchResult.Content, fetchResult.ContentType, oldHash, processedUpdate.NewHash)
		if diffErr != nil {
			s.logger.Error().Err(diffErr).Str("url", url).Msg("Failed to generate content diff")
		} else {
			diffResult.ExtractedPaths = extractedPaths
			diffResult.SecretFindings = secretFindings
			s.logger.Info().Str("url", url).Bool("is_identical", diffResult.IsIdentical).Int("diff_count", len(diffResult.Diffs)).Int("secret_count", len(secretFindings)).Msg("Content diff generated with secret detection")

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
func (s *MonitoringService) storeURLRecord(url string, processedUpdate *models.MonitoredFileUpdate, fetchResult *FetchFileContentResult, diffResult *models.ContentDiffResult) error {
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
		Content:            fetchResult.Content,
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

	// Get file changes before clearing them
	s.fileChangeEventsMutex.Lock()
	fileChanges := make([]models.FileChangeInfo, len(s.fileChangeEvents))
	copy(fileChanges, s.fileChangeEvents)
	s.fileChangeEvents = nil // Clear the events
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
		monitoredNow := s.GetCurrentlyMonitoredURLs()
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
		monitoredCount := len(s.GetCurrentlyMonitoredURLs())
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
	s.monitoredURLsMutex.RLock()
	monitoredURLs := make(map[string]struct{})
	for url := range s.monitoredURLs {
		monitoredURLs[url] = struct{}{}
	}
	s.monitoredURLsMutex.RUnlock()

	s.urlCheckMutexMapLock.Lock()
	defer s.urlCheckMutexMapLock.Unlock()

	for url := range s.urlCheckMutexes {
		if _, isMonitored := monitoredURLs[url]; !isMonitored {
			delete(s.urlCheckMutexes, url)
		}
	}
}
