package monitor

import (
	"context"
	"errors"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"net/http"
	sync "sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	aggregationInterval = 30 * time.Second
	maxAggregatedEvents = 20
)

// MonitoringService orchestrates the monitoring of HTML/JS files.
type MonitoringService struct {
	cfg                *config.MonitorConfig
	notificationCfg    *config.NotificationConfig
	historyStore       models.FileHistoryStore
	logger             zerolog.Logger
	notificationHelper *notifier.NotificationHelper
	httpClient         *http.Client
	fetcher            *Fetcher
	processor          *Processor
	scheduler          *Scheduler

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
	store models.FileHistoryStore,
	baseLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	httpClient *http.Client,
) *MonitoringService {
	serviceSpecificCtx, serviceSpecificCancel := context.WithCancel(context.Background())
	instanceLogger := baseLogger.With().Str("component", "MonitoringService").Logger()

	fetcherInstance := NewFetcher(httpClient, instanceLogger, monitorCfg)
	processorInstance := NewProcessor(instanceLogger)

	s := &MonitoringService{
		cfg:                   monitorCfg,
		notificationCfg:       notificationCfg,
		historyStore:          store,
		logger:                instanceLogger,
		notificationHelper:    notificationHelper,
		httpClient:            httpClient,
		fetcher:               fetcherInstance,
		processor:             processorInstance,
		monitorChan:           make(chan string, monitorCfg.MaxConcurrentChecks*2), // Buffer for incoming URLs
		monitoredURLs:         make(map[string]struct{}),
		monitoredURLsMutex:    sync.RWMutex{},
		fileChangeEvents:      make([]models.FileChangeInfo, 0),
		aggregatedFetchErrors: make([]models.MonitorFetchErrorInfo, 0),
		doneChan:              make(chan struct{}),
		serviceCtx:            serviceSpecificCtx,
		serviceCancelFunc:     serviceSpecificCancel,
	}

	// Initialize the new scheduler, passing the service instance
	s.scheduler = NewScheduler(baseLogger, monitorCfg, s)

	if s.cfg.AggregationIntervalSeconds <= 0 {
		s.logger.Warn().Int("interval_seconds", s.cfg.AggregationIntervalSeconds).Msg("Monitor aggregation interval is not configured or invalid. Aggregation worker will not start.")
		// return nil // Do not return here, let the service start, just no aggregation worker
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

	s.logger.Info().Int("count", len(eventsToSend)).Msg("Sending aggregated file change notifications.")
	s.notificationHelper.SendAggregatedFileChangesNotification(s.serviceCtx, eventsToSend)
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
	s.logger.Info().Str("url", url).Msg("Checking URL")

	lastRecord, err := s.historyStore.GetLastKnownRecord(url)
	if err != nil && !errors.Is(err, models.ErrRecordNotFound) { // Important: Check for ErrRecordNotFound
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to get last known record from history store")
		// Potentially notify about storage errors if critical
		lastRecord = nil // Proceed as if no record found, or handle error more strictly
	} else if errors.Is(err, models.ErrRecordNotFound) {
		s.logger.Debug().Str("url", url).Msg("No previous record found for this URL.")
		lastRecord = nil
	}

	fetchInput := FetchFileContentInput{URL: url}
	if lastRecord != nil {
		fetchInput.PreviousETag = lastRecord.ETag
		fetchInput.PreviousLastModified = lastRecord.LastModified
	}

	fetchResult, err := s.fetcher.FetchFileContent(fetchInput)
	if err != nil {
		if errors.Is(err, ErrNotModified) { // Check if the error is ErrNotModified
			s.logger.Warn().Str("url", url).Msg("Content not modified (304), skipping further processing.")
			return // Do not treat as an error for notification aggregation
		}
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to fetch file content")
		// Aggregate fetch error
		s.aggregatedFetchErrorsMutex.Lock()
		s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      fmt.Sprintf("fetching: %s", err.Error()),
			Source:     "fetch",
			OccurredAt: time.Now(),
		})
		// Check if max aggregated errors reached
		if s.cfg.MaxAggregatedEvents > 0 && len(s.aggregatedFetchErrors) >= s.cfg.MaxAggregatedEvents {
			s.aggregatedFetchErrorsMutex.Unlock() // Unlock before calling send to avoid deadlock
			s.logger.Info().Int("count", len(s.aggregatedFetchErrors)).Msg("Max aggregated monitor errors reached, sending immediately.")
			s.sendAggregatedErrors()
		} else {
			s.aggregatedFetchErrorsMutex.Unlock()
		}
		return
	}
	s.logger.Debug().Str("url", url).Str("contentType", fetchResult.ContentType).Int("content_size", len(fetchResult.Content)).Msg("File fetched successfully by service")

	updatedRecord, err := s.processor.ProcessContent(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to process file content")
		// Aggregate process error
		s.aggregatedFetchErrorsMutex.Lock()
		s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
			URL:        url,
			Error:      fmt.Sprintf("processing: %s", err.Error()),
			Source:     "process",
			OccurredAt: time.Now(),
		})
		if s.cfg.MaxAggregatedEvents > 0 && len(s.aggregatedFetchErrors) >= s.cfg.MaxAggregatedEvents {
			s.aggregatedFetchErrorsMutex.Unlock()
			s.logger.Info().Int("count", len(s.aggregatedFetchErrors)).Msg("Max aggregated monitor errors reached, sending immediately.")
			s.sendAggregatedErrors()
		} else {
			s.aggregatedFetchErrorsMutex.Unlock()
		}
		return
	}
	s.logger.Debug().Str("url", url).Str("new_hash", updatedRecord.NewHash).Msg("Content processed successfully")

	currentTime := updatedRecord.FetchedAt
	var isNewFile = lastRecord == nil
	var contentChanged = !isNewFile && updatedRecord.NewHash != lastRecord.Hash

	if isNewFile || contentChanged {
		s.logger.Info().Str("url", url).Bool("is_new", isNewFile).Bool("content_changed", contentChanged).Msg("File is new or content has changed.")
		newRecord := models.FileHistoryRecord{
			URL:          url,
			Timestamp:    currentTime,
			Hash:         updatedRecord.NewHash,
			ContentType:  updatedRecord.ContentType,
			ETag:         fetchResult.ETag,
			LastModified: fetchResult.LastModified,
		}
		if s.cfg.StoreFullContentOnChange {
			newRecord.Content = updatedRecord.Content
		}

		if errStore := s.historyStore.StoreFileRecord(newRecord); errStore != nil {
			s.logger.Error().Err(errStore).Str("url", url).Msg("Failed to store file record")
			// Aggregate store error
			s.aggregatedFetchErrorsMutex.Lock()
			s.aggregatedFetchErrors = append(s.aggregatedFetchErrors, models.MonitorFetchErrorInfo{
				URL:        url,
				Error:      fmt.Sprintf("storing history: %s", errStore.Error()),
				Source:     "store_history",
				OccurredAt: time.Now(),
			})
			if s.cfg.MaxAggregatedEvents > 0 && len(s.aggregatedFetchErrors) >= s.cfg.MaxAggregatedEvents {
				s.aggregatedFetchErrorsMutex.Unlock()
				s.logger.Info().Int("count", len(s.aggregatedFetchErrors)).Msg("Max aggregated monitor errors reached, sending immediately.")
				s.sendAggregatedErrors()
			} else {
				s.aggregatedFetchErrorsMutex.Unlock()
			}
			return
		}
		s.logger.Info().Str("url", url).Msg("Successfully stored new file record.")

		// Add to aggregation list instead of sending notification directly
		s.fileChangeEventsMutex.Lock()
		var oldHashValue string
		if lastRecord != nil {
			oldHashValue = lastRecord.Hash
		} else {
			oldHashValue = "N/A (new file)" // Or an empty string, depending on desired representation
		}

		event := models.FileChangeInfo{
			URL:         url,
			OldHash:     oldHashValue,
			NewHash:     updatedRecord.NewHash,
			ContentType: fetchResult.ContentType,
			ChangeTime:  updatedRecord.FetchedAt,
		}
		s.fileChangeEvents = append(s.fileChangeEvents, event)
		// Check if max aggregated changes reached
		if s.cfg.MaxAggregatedEvents > 0 && len(s.fileChangeEvents) >= s.cfg.MaxAggregatedEvents {
			s.fileChangeEventsMutex.Unlock() // Unlock before calling send
			s.logger.Info().Int("count", len(s.fileChangeEvents)).Msg("Max aggregated file changes reached, sending immediately.")
			s.sendAggregatedChanges()
		} else {
			s.fileChangeEventsMutex.Unlock()
		}
	} else {
		s.logger.Debug().Str("url", url).Msg("File content has not changed.")
	}
}
