package monitor

import (
	"context"
	"encoding/json"
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
	scheduler          *Scheduler
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
		var errReporter error
		htmlDiffReporterInstance, errReporter = reporter.NewHtmlDiffReporter(mainReporterCfg, instanceLogger, store)
		if errReporter != nil {
			instanceLogger.Error().Err(errReporter).Msg("Failed to initialize HtmlDiffReporter for MonitoringService. Aggregated diff reporting will be impacted.")
		}
	} else {
		instanceLogger.Warn().Msg("HtmlDiffReporter not initialized due to missing mainReporterCfg or historyStore.")
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
	}

	s.scheduler = NewScheduler(monitorCfg, baseLogger, s)

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

// AddTargetURL adds a new URL to the monitoring list.
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

// Start begins the monitoring process by starting its scheduler.
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

	// Start the dedicated monitor scheduler
	if err := s.scheduler.Start(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to start monitor scheduler")
		return err
	}

	// Goroutine to listen on monitorChan and add URLs dynamically
	go func() {
		for {
			select {
			case <-s.serviceCtx.Done(): // Use service-specific context here
				s.logger.Info().Msg("MonitorChan listener stopping due to service context cancellation.")
				return
			case urlToAdd := <-s.monitorChan:
				if urlToAdd == "" { // Allow a way to signal shutdown of this goroutine if needed, though ctx is better
					return
				}
				s.AddTargetURL(urlToAdd)
			}
		}
	}()

	s.logger.Info().Msg("MonitoringService and its scheduler started successfully.")
	return nil
}

// Stop signals the MonitoringService and its scheduler to shut down gracefully.
func (s *MonitoringService) Stop() {
	s.logger.Info().Msg("Attempting to stop MonitoringService...")

	// Get current monitored URLs for notification before cleanup
	currentMonitoredURLs := s.GetCurrentlyMonitoredURLs()

	// Send any remaining aggregated changes and errors before stopping
	s.logger.Info().Msg("Sending final aggregated changes and errors before stopping...")
	s.sendAggregatedChanges()
	s.sendAggregatedErrors()

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
		s.notificationHelper.SendScanCompletionNotification(context.Background(), interruptedSummary, notifier.MonitorServiceNotification)
	}

	// Stop the scheduler first
	if s.scheduler != nil {
		s.scheduler.Stop()
	}

	// Cancel the service-specific context to stop other goroutines like monitorChan listener
	if s.serviceCancelFunc != nil {
		s.serviceCancelFunc()
	}

	if s.aggregationTicker != nil {
		s.aggregationTicker.Stop()
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
	defer s.aggregationWg.Done() // Decrement WaitGroup when worker exits
	defer s.logger.Info().Msg("Aggregation worker stopped.")
	for {
		select {
		case <-s.aggregationTicker.C:
			s.logger.Debug().Msg("Aggregation ticker triggered.")
			s.sendAggregatedChanges()
			s.sendAggregatedErrors() // Also send aggregated errors
		case <-s.doneChan:
			s.logger.Info().Msg("Aggregation worker stopping.")
			return
		}
	}
}

func (s *MonitoringService) sendAggregatedChanges() {
	s.fileChangeEventsMutex.Lock()
	defer s.fileChangeEventsMutex.Unlock()

	if len(s.fileChangeEvents) == 0 {
		s.logger.Debug().Msg("No file changes to aggregate and send.")
		return
	}

	s.logger.Info().Int("count", len(s.fileChangeEvents)).Msg("Sending aggregated file changes notification.")

	// Chỉ gửi thông báo, không tạo báo cáo ở đây
	if s.notificationHelper != nil {
		// Pass "" for reportFilePath as we are not generating an aggregated report here.
		// Individual change reports (if generated and paths stored in FileChangeInfo) might still be used by the formatter.
		s.notificationHelper.SendAggregatedFileChangesNotification(s.serviceCtx, s.fileChangeEvents, "")
	}

	// Clear events after sending
	s.fileChangeEvents = make([]models.FileChangeInfo, 0)
	s.logger.Info().Msg("Aggregated file changes notification sent and event list cleared.")
}

func (s *MonitoringService) sendAggregatedErrors() {
	s.aggregatedFetchErrorsMutex.Lock()
	defer s.aggregatedFetchErrorsMutex.Unlock()

	if len(s.aggregatedFetchErrors) == 0 {
		s.logger.Debug().Msg("No aggregated fetch/process errors to send.")
		return
	}

	// Log the errors, but do not send a Discord notification for them by default.
	// Discord notifications for these types of errors can be very noisy.
	// Critical errors or interruptions will still trigger notifications elsewhere.
	s.logger.Warn().Int("count", len(s.aggregatedFetchErrors)).Msg("Aggregated monitor fetch/process errors occurred.")
	for i, errInfo := range s.aggregatedFetchErrors {
		s.logger.Warn().Int("index", i+1).Str("url", errInfo.URL).Str("source", errInfo.Source).Str("error", errInfo.Error).Time("occurred_at", errInfo.OccurredAt).Msg("Aggregated monitor error detail")
	}

	// Commenting out the notification sending part:
	/*
		s.logger.Info().Int("count", len(s.aggregatedFetchErrors)).Msg("Sending aggregated monitor error notifications.")
		if s.notificationHelper != nil {
			s.notificationHelper.SendAggregatedMonitorErrorsNotification(s.serviceCtx, s.aggregatedFetchErrors)
		}
	*/

	s.aggregatedFetchErrors = []models.MonitorFetchErrorInfo{} // Clear the list
	s.logger.Info().Msg("Aggregated monitor error list cleared after logging.")
}

// checkURL performs the actual check for a single URL. Called by the scheduler's workers.
func (s *MonitoringService) checkURL(url string) {
	s.logger.Info().Str("url", url).Msg("Checking URL for changes")

	fetchResult, err := s.fetcher.FetchFileContent(FetchFileContentInput{URL: url})
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
		// Optionally, trigger immediate error notification if count exceeds threshold
		if s.cfg.MaxAggregatedEvents > 0 && len(s.aggregatedFetchErrors) >= s.cfg.MaxAggregatedEvents {
			s.sendAggregatedErrors()
		}
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
		if s.cfg.MaxAggregatedEvents > 0 && len(s.aggregatedFetchErrors) >= s.cfg.MaxAggregatedEvents {
			s.sendAggregatedErrors()
		}
		return
	}

	// Perform secret detection on the new content
	var secretFindings []models.SecretFinding
	if s.secretsConfig.Enabled && s.secretDetector != nil && len(processedUpdate.Content) > 0 {
		findings, err := s.secretDetector.ScanContent(url, processedUpdate.Content, processedUpdate.ContentType)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Error during secret detection for monitored file")
			// Decide if this error should be added to aggregatedFetchErrors or handled differently
		} else if len(findings) > 0 {
			s.logger.Info().Int("count", len(findings)).Str("url", url).Msg("Secrets found in monitored file content")
			secretFindings = findings
			// Notification for high severity secrets is handled by secretDetector itself.
		}
	}

	changeInfo, diffResult, err := s.detectURLChanges(url, processedUpdate, fetchResult, secretFindings)
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Error detecting URL changes")
		// This error is already logged by detectURLChanges, consider if further action is needed.
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
		if s.cfg.MaxAggregatedEvents > 0 && len(s.aggregatedFetchErrors) >= s.cfg.MaxAggregatedEvents {
			s.sendAggregatedErrors()
		}
		return
	}

	if changeInfo != nil {
		s.logger.Info().Str("url", url).Msg("Change detected")
		s.fileChangeEventsMutex.Lock()
		s.fileChangeEvents = append(s.fileChangeEvents, *changeInfo)
		s.fileChangeEventsMutex.Unlock()

		// Add to changed URLs for the current cycle
		s.changedURLsInCycleMutex.Lock()
		s.changedURLsInCycle[url] = struct{}{}
		s.changedURLsInCycleMutex.Unlock()

		// Optionally, trigger immediate change notification if count exceeds threshold
		if s.cfg.MaxAggregatedEvents > 0 && len(s.fileChangeEvents) >= s.cfg.MaxAggregatedEvents {
			s.sendAggregatedChanges()
		}
	} else {
		s.logger.Debug().Str("url", url).Msg("No change detected")
	}
}

// processURLContent hashes the content and prepares an update structure.
func (s *MonitoringService) processURLContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error) {
	s.logger.Debug().Str("url", url).Msg("Processing URL content")

	processedUpdate, err := s.processor.ProcessContent(url, content, contentType)
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
		return nil, err
	}

	return processedUpdate, nil
}

// detectURLChanges compares current content with historical data and detects changes
func (s *MonitoringService) detectURLChanges(url string, processedUpdate *models.MonitoredFileUpdate, fetchResult *FetchFileContentResult, secretFindings []models.SecretFinding) (*models.FileChangeInfo, *models.ContentDiffResult, error) {
	// Compare with historical data
	lastKnownRecord, err := s.historyStore.GetLastKnownRecord(url)
	if err != nil {
		s.logger.Warn().Err(err).Str("url", url).Msg("Could not get last known record for comparison. Assuming new or first check.")
		// Continue, treat as if no previous record exists
	}

	var previousContent []byte
	var oldHash string
	if lastKnownRecord != nil {
		previousContent = lastKnownRecord.Content // Assumes StoreFullContentOnChange is true
		oldHash = lastKnownRecord.Hash
	}

	// Initialize diffResult to nil to ensure it's defined
	var diffResult *models.ContentDiffResult
	var changeInfo *models.FileChangeInfo

	if processedUpdate.NewHash != oldHash {
		s.logger.Info().Str("url", url).Str("old_hash", oldHash).Str("new_hash", processedUpdate.NewHash).Msg("Change detected")

		// Extract paths if it's a JS file and pathExtractor is available
		var extractedPaths []models.ExtractedPath
		if s.pathExtractor != nil && (strings.Contains(processedUpdate.ContentType, "javascript") || strings.HasSuffix(url, ".js")) {
			paths, err := s.pathExtractor.ExtractPaths(url, fetchResult.Content, fetchResult.ContentType)
			if err != nil {
				s.logger.Error().Err(err).Str("url", url).Msg("Failed to extract paths from JS content during change detection")
			} else {
				extractedPaths = paths
				s.logger.Debug().Str("url", url).Int("path_count", len(extractedPaths)).Msg("Extracted paths from changed JS content")
			}
		}

		changeInfo = &models.FileChangeInfo{
			URL:            url,
			OldHash:        oldHash,
			NewHash:        processedUpdate.NewHash,
			ContentType:    processedUpdate.ContentType,
			ChangeTime:     processedUpdate.FetchedAt,
			ExtractedPaths: extractedPaths,
			SecretFindings: secretFindings,
		}

		// Generate diff if ContentDiffer is available and content needs to be stored for diffing
		if s.contentDiffer != nil && s.cfg.StoreFullContentOnChange {
			var diffErr error // Declare diffErr here
			diffResult, diffErr = s.contentDiffer.GenerateDiff(previousContent, fetchResult.Content, fetchResult.ContentType, oldHash, processedUpdate.NewHash)
			if diffErr != nil {
				s.logger.Error().Err(diffErr).Str("url", url).Msg("Failed to generate content diff")
			} else if diffResult != nil {
				// Add secret findings to the diff result
				diffResult.SecretFindings = secretFindings

				if !diffResult.IsIdentical {
					s.logger.Info().Str("url", url).Bool("is_identical", diffResult.IsIdentical).Int("diff_count", len(diffResult.Diffs)).Int("secret_count", len(secretFindings)).Msg("Content diff generated with secret detection results.")
					// Generate HTML report for this specific diff
					if s.htmlDiffReporter != nil {
						reportPath, reportErr := s.htmlDiffReporter.GenerateSingleDiffReport(url, diffResult, oldHash, processedUpdate.NewHash, fetchResult.Content)
						if reportErr != nil {
							s.logger.Error().Err(reportErr).Str("url", url).Msg("Failed to generate single HTML diff report")
						} else {
							s.logger.Info().Str("url", url).Str("report_path", reportPath).Msg("Single HTML diff report created")
							changeInfo.DiffReportPath = &reportPath // Store the path
						}
					}
				}
			}
		}
	} else {
		s.logger.Debug().Str("url", url).Msg("No change detected (hash is identical)")
	}

	return changeInfo, diffResult, nil
}

// storeURLRecord saves the history record to the datastore.
func (s *MonitoringService) storeURLRecord(url string, processedUpdate *models.MonitoredFileUpdate, fetchResult *FetchFileContentResult, diffResult *models.ContentDiffResult) error {
	record := models.FileHistoryRecord{
		URL:          url,
		Timestamp:    processedUpdate.FetchedAt.UnixMilli(),
		Hash:         processedUpdate.NewHash,
		ContentType:  processedUpdate.ContentType,
		ETag:         fetchResult.ETag,
		LastModified: fetchResult.LastModified,
	}

	if s.cfg.StoreFullContentOnChange && diffResult != nil && !diffResult.IsIdentical {
		record.Content = processedUpdate.Content // Store full content only if changed and configured
	}

	if diffResult != nil {
		diffJSON, err := json.Marshal(diffResult)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Failed to marshal diff result to JSON for storage")
		} else {
			jsonStr := string(diffJSON)
			record.DiffResultJSON = &jsonStr
		}
	}

	// Store extracted paths if any
	if diffResult != nil && len(diffResult.ExtractedPaths) > 0 {
		pathsJSON, err := json.Marshal(diffResult.ExtractedPaths)
		if err != nil {
			s.logger.Error().Err(err).Str("url", url).Msg("Failed to marshal extracted paths to JSON for storage")
		} else {
			jsonStr := string(pathsJSON)
			record.ExtractedPathsJSON = &jsonStr
		}
	}

	if s.historyStore != nil {
		return s.historyStore.StoreFileRecord(record)
	}
	s.logger.Warn().Str("url", url).Msg("History store is nil, cannot store file record.")
	return nil
}

// TriggerCycleEndReport is called by the scheduler when a full monitoring cycle for all URLs is complete.
// It generates an aggregated diff report for all monitored URLs and sends a notification.
func (s *MonitoringService) TriggerCycleEndReport() {
	s.logger.Info().Msg("Monitoring cycle complete. Triggering end-of-cycle report and notification.")

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
		s.logger.Warn().Msg("HtmlDiffReporter is not initialized. Cannot generate aggregated monitor diff report.")
	}

	// Send notification for the completed cycle
	if s.notificationHelper != nil {
		monitoredCount := len(s.GetCurrentlyMonitoredURLs())
		cycleCompleteData := models.MonitorCycleCompleteData{
			ChangedURLs:    changedURLs,
			ReportPath:     reportPath, // This will be empty if report generation failed or was skipped
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
