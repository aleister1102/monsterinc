package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"

	"github.com/rs/zerolog"
)

// EventAggregator manages aggregation of monitoring events
type EventAggregator struct {
	logger                     zerolog.Logger
	notificationHelper         *notifier.NotificationHelper
	fileChangeEvents           []models.FileChangeInfo
	fileChangeEventsMutex      sync.Mutex
	aggregatedFetchErrors      []models.MonitorFetchErrorInfo
	aggregatedFetchErrorsMutex sync.Mutex
	aggregationTicker          *time.Ticker
	aggregationWg              sync.WaitGroup
	doneChan                   chan struct{}
	maxAggregatedEvents        int
	isShuttingDown             bool
	shutdownMutex              sync.RWMutex
}

// NewEventAggregator creates a new EventAggregator
func NewEventAggregator(
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	aggregationInterval time.Duration,
	maxAggregatedEvents int,
) *EventAggregator {
	ea := &EventAggregator{
		logger:                logger.With().Str("component", "EventAggregator").Logger(),
		notificationHelper:    notificationHelper,
		fileChangeEvents:      make([]models.FileChangeInfo, 0),
		aggregatedFetchErrors: make([]models.MonitorFetchErrorInfo, 0),
		doneChan:              make(chan struct{}),
		maxAggregatedEvents:   maxAggregatedEvents,
		isShuttingDown:        false,
		shutdownMutex:         sync.RWMutex{},
	}

	if aggregationInterval > 0 {
		ea.aggregationTicker = time.NewTicker(aggregationInterval)
		ea.aggregationWg.Add(1)
		go ea.aggregationWorker()
		ea.logger.Info().Dur("interval", aggregationInterval).Int("max_events", maxAggregatedEvents).Msg("Event aggregation worker started")
	}

	return ea
}

// AddFileChangeEvent adds a file change event to the aggregated list
func (ea *EventAggregator) AddFileChangeEvent(event models.FileChangeInfo) {
	ea.shutdownMutex.RLock()
	if ea.isShuttingDown {
		ea.shutdownMutex.RUnlock()
		ea.logger.Debug().Str("url", event.URL).Msg("Ignoring file change event due to shutdown")
		return
	}
	ea.shutdownMutex.RUnlock()

	ea.fileChangeEventsMutex.Lock()
	defer ea.fileChangeEventsMutex.Unlock()

	ea.fileChangeEvents = append(ea.fileChangeEvents, event)
	ea.logger.Debug().Str("url", event.URL).Str("cycle_id", event.CycleID).Msg("File change event added to aggregation")

	// Check if we've reached the max aggregated events
	if len(ea.fileChangeEvents) >= ea.maxAggregatedEvents {
		ea.logger.Info().Int("event_count", len(ea.fileChangeEvents)).Msg("Max aggregated events reached, triggering immediate send")
		go ea.sendAggregatedChanges()
	}
}

// AddFetchErrorEvent adds a fetch error event to the aggregated list
func (ea *EventAggregator) AddFetchErrorEvent(errorInfo models.MonitorFetchErrorInfo) {
	ea.shutdownMutex.RLock()
	if ea.isShuttingDown {
		ea.shutdownMutex.RUnlock()
		ea.logger.Debug().Str("url", errorInfo.URL).Msg("Ignoring fetch error event due to shutdown")
		return
	}
	ea.shutdownMutex.RUnlock()

	ea.aggregatedFetchErrorsMutex.Lock()
	defer ea.aggregatedFetchErrorsMutex.Unlock()

	ea.aggregatedFetchErrors = append(ea.aggregatedFetchErrors, errorInfo)
	ea.logger.Debug().Str("url", errorInfo.URL).Str("source", errorInfo.Source).Msg("Fetch error event added to aggregation")
}

// Stop gracefully stops the event aggregator
func (ea *EventAggregator) Stop() {
	ea.shutdownMutex.Lock()
	ea.isShuttingDown = true
	ea.shutdownMutex.Unlock()

	if ea.aggregationTicker != nil {
		ea.aggregationTicker.Stop()
	}

	close(ea.doneChan)
	ea.aggregationWg.Wait()

	// Send any remaining events before shutdown
	ea.sendAggregatedChanges()
	ea.sendAggregatedErrors()

	ea.logger.Info().Msg("Event aggregator stopped")
}

// aggregationWorker runs the aggregation ticker
func (ea *EventAggregator) aggregationWorker() {
	defer ea.aggregationWg.Done()

	for {
		select {
		case <-ea.doneChan:
			ea.logger.Debug().Msg("Aggregation worker stopped via done channel")
			return
		case <-ea.aggregationTicker.C:
			ea.logger.Debug().Msg("Aggregation timer tick - sending aggregated events")
			ea.sendAggregatedChanges()
			ea.sendAggregatedErrors()
		}
	}
}

// sendAggregatedChanges sends aggregated file change events via notifications
func (ea *EventAggregator) sendAggregatedChanges() {
	ea.fileChangeEventsMutex.Lock()
	if len(ea.fileChangeEvents) == 0 {
		ea.fileChangeEventsMutex.Unlock()
		return
	}

	changes := make([]models.FileChangeInfo, len(ea.fileChangeEvents))
	copy(changes, ea.fileChangeEvents)
	ea.fileChangeEvents = ea.fileChangeEvents[:0]
	ea.fileChangeEventsMutex.Unlock()

	if ea.notificationHelper != nil {
		ea.logger.Info().Int("change_count", len(changes)).Msg("Sending aggregated file changes notification")
		ea.notificationHelper.SendAggregatedFileChangesNotification(ea.getContext(), changes, "")
	}
}

// sendAggregatedErrors sends aggregated fetch error events via notifications
func (ea *EventAggregator) sendAggregatedErrors() {
	ea.aggregatedFetchErrorsMutex.Lock()
	if len(ea.aggregatedFetchErrors) == 0 {
		ea.aggregatedFetchErrorsMutex.Unlock()
		return
	}

	errors := make([]models.MonitorFetchErrorInfo, len(ea.aggregatedFetchErrors))
	copy(errors, ea.aggregatedFetchErrors)
	ea.aggregatedFetchErrors = ea.aggregatedFetchErrors[:0]
	ea.aggregatedFetchErrorsMutex.Unlock()

	if ea.notificationHelper != nil {
		ea.logger.Info().Int("error_count", len(errors)).Msg("Sending aggregated fetch errors notification")
		ea.notificationHelper.SendAggregatedMonitorErrorsNotification(ea.getContext(), errors)
	}
}

// getContext provides a context for operations (simplified for now)
func (ea *EventAggregator) getContext() context.Context {
	// In a real implementation, this should be passed from the service
	return context.Background()
}
