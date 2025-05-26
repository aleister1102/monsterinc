package monitor

import (
	"context"
	"errors"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"net/http"
	"sync"

	"github.com/rs/zerolog"
)

// MonitoringService orchestrates the monitoring of HTML/JS files.
type MonitoringService struct {
	cfg             *config.MonitorConfig
	notificationCfg *config.NotificationConfig
	historyStore    models.FileHistoryStore
	logger          zerolog.Logger
	discordNotifier *notifier.DiscordNotifier
	httpClient      *http.Client
	fetcher         *Fetcher
	processor       *Processor
	scheduler       *Scheduler

	// For managing target URLs dynamically
	monitorChan        chan string         // Channel to receive new URLs to monitor
	monitoredURLs      map[string]struct{} // Set of URLs currently being monitored
	monitoredURLsMutex sync.RWMutex        // Mutex for monitoredURLs map

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
	discordNotifier *notifier.DiscordNotifier,
	httpClient *http.Client,
) *MonitoringService {
	serviceSpecificCtx, serviceSpecificCancel := context.WithCancel(context.Background())
	instanceLogger := baseLogger.With().Str("component", "MonitoringService").Logger()

	fetcherInstance := NewFetcher(httpClient, instanceLogger, monitorCfg)
	processorInstance := NewProcessor(instanceLogger)

	s := &MonitoringService{
		cfg:                monitorCfg,
		notificationCfg:    notificationCfg,
		historyStore:       store,
		logger:             instanceLogger,
		discordNotifier:    discordNotifier,
		httpClient:         httpClient,
		fetcher:            fetcherInstance,
		processor:          processorInstance,
		monitorChan:        make(chan string, 100),
		monitoredURLs:      make(map[string]struct{}),
		monitoredURLsMutex: sync.RWMutex{},
		serviceCtx:         serviceSpecificCtx,
		serviceCancelFunc:  serviceSpecificCancel,
	}

	// Initialize the new scheduler, passing the service instance
	s.scheduler = NewScheduler(baseLogger, monitorCfg, s)

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

	// Close monitorChan if not already closed, to unblock any potential senders if necessary,
	// though context cancellation should be the primary mechanism.
	// Be careful with closing channels if multiple goroutines might write to it.
	// For now, assuming context handles shutdown of the monitorChan listener.

	s.logger.Info().Msg("MonitoringService stopped.")
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
		if errors.Is(err, ErrNotModified) {
			s.logger.Info().Str("url", url).Msg("Content not modified (304), skipping further processing.")
			return
		}
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to fetch file content")
		// TODO: FR9 notification for fetch errors (other than 304)
		return
	}
	s.logger.Debug().Str("url", url).Str("contentType", fetchResult.ContentType).Int("content_size", len(fetchResult.Content)).Msg("File fetched successfully by service")

	processedUpdate, err := s.processor.ProcessContent(url, fetchResult.Content, fetchResult.ContentType)
	if err != nil {
		s.logger.Error().Err(err).Str("url", url).Msg("Failed to process file content")
		return
	}
	s.logger.Debug().Str("url", url).Str("new_hash", processedUpdate.NewHash).Msg("Content processed successfully")

	currentTime := processedUpdate.FetchedAt
	var isNewFile = lastRecord == nil
	var contentChanged = !isNewFile && processedUpdate.NewHash != lastRecord.Hash

	if isNewFile || contentChanged {
		s.logger.Info().Str("url", url).Bool("is_new", isNewFile).Bool("content_changed", contentChanged).Msg("File is new or content has changed.")
		newRecord := models.FileHistoryRecord{
			URL:          url,
			Timestamp:    currentTime,
			Hash:         processedUpdate.NewHash,
			ContentType:  processedUpdate.ContentType,
			ETag:         fetchResult.ETag,
			LastModified: fetchResult.LastModified,
		}
		if s.cfg.StoreFullContentOnChange {
			newRecord.Content = processedUpdate.Content
		}

		if errStore := s.historyStore.StoreFileRecord(newRecord); errStore != nil {
			s.logger.Error().Err(errStore).Str("url", url).Msg("Failed to store file record")
		} else {
			s.logger.Info().Str("url", url).Msg("Successfully stored new file record.")
			oldHash := ""
			if lastRecord != nil {
				oldHash = lastRecord.Hash
			}
			notificationPayload := notifier.FormatFileChangeNotification(url, oldHash, newRecord.Hash, newRecord.ContentType, *s.notificationCfg)
			if errNotify := s.discordNotifier.SendNotification(s.serviceCtx, notificationPayload, ""); errNotify != nil {
				s.logger.Error().Err(errNotify).Str("url", url).Msg("Failed to send file change notification")
			}
		}
	} else {
		s.logger.Info().Str("url", url).Msg("File content has not changed.")
	}
}
