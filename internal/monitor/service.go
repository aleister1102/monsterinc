package monitor

import (
	"context"
	"encoding/json"
	"monsterinc/internal/config"
	"monsterinc/internal/differ"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"monsterinc/internal/reporter"
	"net/http"
	sync "sync"
	"time"

	"github.com/rs/zerolog"
)

// MonitoringService orchestrates the monitoring of HTML/JS files.
type MonitoringService struct {
	cfg                *config.MonitorConfig
	notificationCfg    *config.NotificationConfig
	reporterConfig     *config.ReporterConfig
	historyStore       models.FileHistoryStore
	logger             zerolog.Logger
	notificationHelper *notifier.NotificationHelper
	httpClient         *http.Client
	fetcher            *Fetcher
	processor          *Processor
	scheduler          *Scheduler
	contentDiffer      *differ.ContentDiffer
	htmlDiffReporter   *reporter.HtmlDiffReporter

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
	doneChan                   chan struct{} // To stop the aggregation goroutine

	// Service specific context for operations like notifications
	serviceCtx        context.Context
	serviceCancelFunc context.CancelFunc
}

// NewMonitoringService creates a new instance of MonitoringService.
func NewMonitoringService(
	monitorCfg *config.MonitorConfig,
	notificationCfg *config.NotificationConfig,
	mainReporterCfg *config.ReporterConfig,
	diffReporterCfg *config.DiffReporterConfig,
	store models.FileHistoryStore,
	baseLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	httpClient *http.Client,
) *MonitoringService {
	serviceSpecificCtx, serviceSpecificCancel := context.WithCancel(context.Background())
	instanceLogger := baseLogger.With().Str("component", "MonitoringService").Logger()

	fetcherInstance := NewFetcher(httpClient, instanceLogger, monitorCfg)
	processorInstance := NewProcessor(instanceLogger)
	contentDifferInstance := differ.NewContentDiffer(instanceLogger, diffReporterCfg)
	// urlHandlerInstance := urlhandler.NewURLHandler(instanceLogger) // Removed

	var htmlDiffReporterInstance *reporter.HtmlDiffReporter // Declare var for clarity
	if mainReporterCfg != nil && store != nil {             // Ensure necessary configs are present
		var errReporter error                                                                                        // Declare errReporter to avoid shadowing
		htmlDiffReporterInstance, errReporter = reporter.NewHtmlDiffReporter(mainReporterCfg, instanceLogger, store) // Removed urlHandlerInstance
		if errReporter != nil {
			instanceLogger.Error().Err(errReporter).Msg("Failed to initialize HtmlDiffReporter for MonitoringService. Aggregated diff reporting will be impacted.")
			// Depending on policy, could return nil or a service with a disabled reporter
		}
	} else {
		instanceLogger.Warn().Msg("HtmlDiffReporter not initialized due to missing mainReporterCfg or historyStore.")
	}

	s := &MonitoringService{
		cfg:                   monitorCfg,
		notificationCfg:       notificationCfg,
		reporterConfig:        mainReporterCfg,
		historyStore:          store,
		logger:                instanceLogger,
		notificationHelper:    notificationHelper,
		httpClient:            httpClient,
		fetcher:               fetcherInstance,
		processor:             processorInstance,
		contentDiffer:         contentDifferInstance,
		htmlDiffReporter:      htmlDiffReporterInstance,
		monitorChan:           make(chan string, monitorCfg.MaxConcurrentChecks*2),
		monitoredURLs:         make(map[string]struct{}),
		monitoredURLsMutex:    sync.RWMutex{},
		fileChangeEvents:      make([]models.FileChangeInfo, 0),
		aggregatedFetchErrors: make([]models.MonitorFetchErrorInfo, 0),
		doneChan:              make(chan struct{}),
		serviceCtx:            serviceSpecificCtx,
		serviceCancelFunc:     serviceSpecificCancel,
	}

	s.scheduler = NewScheduler(baseLogger, monitorCfg, s)

	if s.cfg.AggregationIntervalSeconds <= 0 {
		s.logger.Warn().Int("interval_seconds", s.cfg.AggregationIntervalSeconds).Msg("Monitor aggregation interval is not configured or invalid. Aggregation worker will not start.")
	} else {
		aggregationDuration := time.Duration(s.cfg.AggregationIntervalSeconds) * time.Second
		s.aggregationTicker = time.NewTicker(aggregationDuration)
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
	close(s.doneChan) // Signal aggregation worker to stop

	// Close monitorChan if not already closed, to unblock any potential senders if necessary,
	// though context cancellation should be the primary mechanism.
	// Be careful with closing channels if multiple goroutines might write to it.
	// For now, assuming context handles shutdown of the monitorChan listener.

	s.logger.Info().Msg("MonitoringService stopped.")
}

func (s *MonitoringService) aggregationWorker() {
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
	if len(s.fileChangeEvents) == 0 {
		s.fileChangeEventsMutex.Unlock()
		return
	}

	eventsToSend := make([]models.FileChangeInfo, len(s.fileChangeEvents))
	copy(eventsToSend, s.fileChangeEvents)
	s.fileChangeEvents = make([]models.FileChangeInfo, 0) // Clear the buffer
	s.fileChangeEventsMutex.Unlock()

	var aggregatedReportPath string
	// Generate an aggregated diff report if there are changes and the reporter is configured
	if len(eventsToSend) > 0 && s.htmlDiffReporter != nil {
		currentMonitoredURLs := s.GetCurrentlyMonitoredURLs()
		reportPath, err := s.htmlDiffReporter.GenerateDiffReport(currentMonitoredURLs)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to generate aggregated HTML diff report for monitor changes")
			// Proceed to send notification without the report path
		} else if reportPath != "" {
			s.logger.Info().Str("path", reportPath).Msg("Aggregated HTML diff report generated for monitor changes")
			aggregatedReportPath = reportPath
		} else {
			s.logger.Info().Msg("Aggregated HTML diff report was not generated (no diffs found or other issue), notification will be sent without it.")
		}
	}

	s.logger.Info().Int("count", len(eventsToSend)).Msg("Sending aggregated file change notifications.")
	s.notificationHelper.SendAggregatedFileChangesNotification(s.serviceCtx, eventsToSend, aggregatedReportPath)
}

func (s *MonitoringService) sendAggregatedErrors() {
	s.aggregatedFetchErrorsMutex.Lock()
	if len(s.aggregatedFetchErrors) == 0 {
		s.aggregatedFetchErrorsMutex.Unlock()
		return
	}

	errorsToSend := make([]models.MonitorFetchErrorInfo, len(s.aggregatedFetchErrors))
	copy(errorsToSend, s.aggregatedFetchErrors)
	s.aggregatedFetchErrors = make([]models.MonitorFetchErrorInfo, 0) // Clear the slice
	s.aggregatedFetchErrorsMutex.Unlock()

	if len(errorsToSend) > 0 {
		s.logger.Info().Int("count", len(errorsToSend)).Msg("Sending aggregated monitor error notifications.")
		s.notificationHelper.SendAggregatedMonitorErrorsNotification(s.serviceCtx, errorsToSend)
	}
}

// checkURL performs the actual check for a single URL. Called by the scheduler's workers.
func (s *MonitoringService) checkURL(url string) {
	s.logger.Debug().Str("url", url).Msg("Checking URL for changes")

	// 1. Fetch current content
	fetchInput := FetchFileContentInput{URL: url}
	// If we have ETag/LastModified from previous successful fetch (not necessarily from history store directly), use it.
	// For now, let's assume historyStore might provide it if needed, or Fetcher handles it.
	// lastRecord, _ := s.historyStore.GetLastKnownRecord(url)
	// if lastRecord != nil {
	// fetchInput.PreviousETag = lastRecord.ETag
	// fetchInput.PreviousLastModified = lastRecord.LastModified
	// }

	fetchResult, err := s.fetcher.FetchFileContent(fetchInput)
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to fetch file content for monitoring")
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

	// 2. Process content (e.g., get hash)
	processedUpdate, err := s.processor.ProcessContent(url, fetchResult.Content, fetchResult.ContentType)
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

	// 3. Compare with historical data
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

	if processedUpdate.NewHash != oldHash {
		s.logger.Info().Str("url", url).Str("old_hash", oldHash).Str("new_hash", processedUpdate.NewHash).Msg("Change detected")

		changeInfo := models.FileChangeInfo{
			URL:         url,
			OldHash:     oldHash,
			NewHash:     processedUpdate.NewHash,
			ContentType: processedUpdate.ContentType,
			ChangeTime:  processedUpdate.FetchedAt,
			// Status and StatusMessage are not part of FileChangeInfo by default.
			// These would need to be added to the struct if intended for use.
			// For now, this information will be logged or handled locally if needed.
		}
		// fileChangeEventsMutex is locked later when actually adding to the slice.

		// Generate diff if ContentDiffer is available and content needs to be stored for diffing
		if s.contentDiffer != nil && s.cfg.StoreFullContentOnChange {
			var diffErr error // Declare diffErr here
			diffResult, diffErr = s.contentDiffer.GenerateDiff(previousContent, fetchResult.Content, fetchResult.ContentType, oldHash, processedUpdate.NewHash)
			if diffErr != nil {
				s.logger.Error().Err(diffErr).Str("url", url).Msg("Failed to generate content diff")
			} else if diffResult != nil && !diffResult.IsIdentical {
				s.logger.Info().Str("url", url).Bool("is_identical", diffResult.IsIdentical).Int("diff_count", len(diffResult.Diffs)).Msg("Content diff generated.")
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

		// 4. Store new record if changed (content or status)
		var diffJSON *string
		if diffResult != nil && !diffResult.IsIdentical { // Serialize diff result if available and not identical
			diffBytes, jsonMarshalErr := json.Marshal(diffResult) // Renamed err to jsonMarshalErr
			if jsonMarshalErr != nil {
				s.logger.Error().Err(jsonMarshalErr).Str("url", url).Msg("Failed to marshal diff result to JSON")
			} else {
				jsonStr := string(diffBytes)
				diffJSON = &jsonStr
			}
		}

		// Determine if a record should be stored in history.
		// This happens if StoreFullContentOnChange is true AND it's a new URL OR content has actually changed.
		shouldStoreRecord := s.cfg.StoreFullContentOnChange &&
			(lastKnownRecord == nil || (diffResult != nil && !diffResult.IsIdentical))

		if shouldStoreRecord {
			storeRecord := models.FileHistoryRecord{
				URL:            url,
				Timestamp:      processedUpdate.FetchedAt.UnixMilli(),
				Hash:           processedUpdate.NewHash,
				ContentType:    processedUpdate.ContentType,
				ETag:           fetchResult.ETag,
				LastModified:   fetchResult.LastModified,
				DiffResultJSON: diffJSON, // Store the marshalled diff
			}
			if s.cfg.StoreFullContentOnChange { // Ensure content is stored if configured
				storeRecord.Content = fetchResult.Content
			}

			if errStore := s.historyStore.StoreFileRecord(storeRecord); errStore != nil {
				s.logger.Error().Err(errStore).Str("url", url).Msg("Failed to store updated file record in history")
				s.aggregatedFetchErrorsMutex.Lock()
				s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
					URL:        url,
					Error:      errStore.Error(),
					Source:     "store_history",
					OccurredAt: time.Now(),
				})
				s.aggregatedFetchErrorsMutex.Unlock()
			}
		}

		// Determine if a notification-worthy event occurred.
		// This happens if the content is actually different.
		// Other status changes (like URL becoming reachable/unreachable) are handled by general error aggregation or not at all by this specific logic.
		triggerNotificationForContentChange := diffResult != nil && !diffResult.IsIdentical

		if triggerNotificationForContentChange {
			// Add to fileChangeEvents for notification.
			// FileChangeInfo currently doesn't have 'DiffStored' or complex 'Status' fields.
			// The notification formatter will interpret based on OldHash vs NewHash for "change".
			s.fileChangeEventsMutex.Lock()
			s.fileChangeEvents = append(s.fileChangeEvents, changeInfo) // changeInfo contains OldHash, NewHash etc.
			s.fileChangeEventsMutex.Unlock()

			// Limit the size of aggregated events to prevent memory exhaustion
			if len(s.fileChangeEvents) > s.cfg.MaxAggregatedEvents*2 { // A bit of buffer
				s.logger.Warn().Int("current_size", len(s.fileChangeEvents)).Int("max_size", s.cfg.MaxAggregatedEvents).Msg("File change event buffer is very large, forcing flush.")
				s.sendAggregatedChanges() // Force send if buffer grows too large
			}
		}

	} else {
		s.logger.Debug().Str("url", url).Msg("No change detected (hash is identical)")
		// If no hash change, we generally don't store a new record unless it's the very first time
		// or if we need to update 'last_checked' for URLs not changing (currently not implemented this way).
		// The current logic correctly skips storing if hash is same and lastKnownRecord existed.
	}

	s.logger.Debug().Str("url", url).Msg("Finished checking URL.")
}

// SanitizeFilenameForMonitoring is removed as it's causing a linter error and its usage isn't apparent here.
// If needed, it should be correctly defined or imported from a utility package like urlhandler.
// func SanitizeFilenameForMonitoring(rawURL string) string {
//	// ... (implementation from urlhandler or similar, for brevity assuming it exists)
//	// Example: replace http://, https://, slashes, colons, etc.
//	sanitized := strings.ReplaceAll(rawURL, "http://", "")
//	sanitized = strings.ReplaceAll(sanitized, "https://", "")
//	sanitized = strings.ReplaceAll(sanitized, "/", "_")
//	sanitized = strings.ReplaceAll(sanitized, ":", "_")
//	sanitized = strings.ReplaceAll(sanitized, "?", "_")
//	sanitized = strings.ReplaceAll(sanitized, "&", "_")
//	sanitized = strings.ReplaceAll(sanitized, "=", "_")
//	// Limit length
//	if len(sanitized) > 100 {
//		sanitized = sanitized[:100]
//	}
//	return sanitized
// }
