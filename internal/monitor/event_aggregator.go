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
	logger                zerolog.Logger
	notificationHelper    *notifier.NotificationHelper
	fileChangeEvents      []models.FileChangeInfo
	fileChangeEventsMutex sync.Mutex
	fetchErrors           []models.MonitorFetchErrorInfo
	fetchErrorsMutex      sync.Mutex
	aggregationTicker     *time.Ticker
	aggregationWg         sync.WaitGroup
	doneChan              chan struct{}
	maxAggregatedEvents   int
	isShuttingDown        bool
	shutdownMutex         sync.RWMutex
	parentCtx             context.Context // Thêm context để có thể cancel
}

// NewEventAggregator creates a new EventAggregator
func NewEventAggregator(
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	aggregationInterval time.Duration,
	maxAggregatedEvents int,
) *EventAggregator {
	ea := &EventAggregator{
		logger:              logger.With().Str("component", "EventAggregator").Logger(),
		notificationHelper:  notificationHelper,
		fileChangeEvents:    make([]models.FileChangeInfo, 0),
		fetchErrors:         make([]models.MonitorFetchErrorInfo, 0),
		doneChan:            make(chan struct{}),
		maxAggregatedEvents: maxAggregatedEvents,
		isShuttingDown:      false,
		shutdownMutex:       sync.RWMutex{},
		parentCtx:           context.Background(), // Default context
	}

	ea.startAggregationWorker(aggregationInterval)
	return ea
}

// AddFileChangeEvent adds a file change event to the aggregated list
func (ea *EventAggregator) AddFileChangeEvent(event models.FileChangeInfo) {
	if !ea.shouldAcceptEvent() {
		ea.logEventRejected("file change", event.URL)
		return
	}

	ea.addFileChangeEventToBuffer(event)
	ea.checkMaxEventsReached()
}

// AddFetchErrorEvent adds a fetch error event to the aggregated list
func (ea *EventAggregator) AddFetchErrorEvent(errorInfo models.MonitorFetchErrorInfo) {
	if !ea.shouldAcceptEvent() {
		ea.logEventRejected("fetch error", errorInfo.URL)
		return
	}

	ea.addFetchErrorToBuffer(errorInfo)
}

// Stop gracefully stops the event aggregator
func (ea *EventAggregator) Stop() {
	ea.logger.Info().Msg("Stopping event aggregator...")
	ea.initiateShutdown()
	ea.waitForWorkerCompletion()
	// Don't send remaining events during forced shutdown to avoid delays
	ea.logger.Info().Msg("Event aggregator stopped")
}

// SetParentContext updates the parent context for notifications
func (ea *EventAggregator) SetParentContext(ctx context.Context) {
	ea.parentCtx = ctx
}

// GetEventCounts returns current event counts for monitoring
func (ea *EventAggregator) GetEventCounts() map[string]int {
	return map[string]int{
		"file_changes": ea.getFileChangeEventCount(),
		"fetch_errors": ea.getFetchErrorCount(),
	}
}

// Private helper methods for initialization

func (ea *EventAggregator) startAggregationWorker(aggregationInterval time.Duration) {
	if aggregationInterval <= 0 {
		ea.logger.Warn().Msg("Invalid aggregation interval, worker not started")
		return
	}

	ea.aggregationTicker = time.NewTicker(aggregationInterval)
	ea.aggregationWg.Add(1)
	go ea.aggregationWorker()

	ea.logger.Info().
		Dur("interval", aggregationInterval).
		Int("max_events", ea.maxAggregatedEvents).
		Msg("Event aggregation worker started")
}

// Private helper methods for event acceptance

func (ea *EventAggregator) shouldAcceptEvent() bool {
	ea.shutdownMutex.RLock()
	defer ea.shutdownMutex.RUnlock()
	return !ea.isShuttingDown
}

func (ea *EventAggregator) logEventRejected(eventType, url string) {
	ea.logger.Debug().
		Str("event_type", eventType).
		Str("url", url).
		Msg("Ignoring event due to shutdown")
}

// Private helper methods for event buffer management

func (ea *EventAggregator) addFileChangeEventToBuffer(event models.FileChangeInfo) {
	ea.fileChangeEventsMutex.Lock()
	defer ea.fileChangeEventsMutex.Unlock()

	ea.fileChangeEvents = append(ea.fileChangeEvents, event)
	ea.logFileChangeEventAdded(event)
}

func (ea *EventAggregator) addFetchErrorToBuffer(errorInfo models.MonitorFetchErrorInfo) {
	ea.fetchErrorsMutex.Lock()
	defer ea.fetchErrorsMutex.Unlock()

	ea.fetchErrors = append(ea.fetchErrors, errorInfo)
	ea.logFetchErrorAdded(errorInfo)
}

func (ea *EventAggregator) logFileChangeEventAdded(event models.FileChangeInfo) {
	ea.logger.Debug().
		Str("url", event.URL).
		Str("cycle_id", event.CycleID).
		Msg("File change event added to aggregation")
}

func (ea *EventAggregator) logFetchErrorAdded(errorInfo models.MonitorFetchErrorInfo) {
	ea.logger.Debug().
		Str("url", errorInfo.URL).
		Str("source", errorInfo.Source).
		Msg("Fetch error event added to aggregation")
}

func (ea *EventAggregator) checkMaxEventsReached() {
	currentCount := ea.getFileChangeEventCount()
	if currentCount >= ea.maxAggregatedEvents {
		ea.logger.Info().
			Int("event_count", currentCount).
			Msg("Max aggregated events reached, triggering immediate send")

		// Check if we should still send notifications
		if ea.shouldAcceptEvent() {
			go ea.sendAggregatedChanges()
		} else {
			ea.logger.Debug().Msg("Skipping immediate send due to shutdown")
		}
	}
}

func (ea *EventAggregator) getFileChangeEventCount() int {
	ea.fileChangeEventsMutex.Lock()
	defer ea.fileChangeEventsMutex.Unlock()
	return len(ea.fileChangeEvents)
}

func (ea *EventAggregator) getFetchErrorCount() int {
	ea.fetchErrorsMutex.Lock()
	defer ea.fetchErrorsMutex.Unlock()
	return len(ea.fetchErrors)
}

// Private helper methods for shutdown

func (ea *EventAggregator) initiateShutdown() {
	ea.shutdownMutex.Lock()
	defer ea.shutdownMutex.Unlock()

	ea.isShuttingDown = true
	ea.stopAggregationTicker()
	close(ea.doneChan)
}

func (ea *EventAggregator) stopAggregationTicker() {
	if ea.aggregationTicker != nil {
		ea.aggregationTicker.Stop()
	}
}

func (ea *EventAggregator) waitForWorkerCompletion() {
	ea.aggregationWg.Wait()
}

// Private worker methods

func (ea *EventAggregator) aggregationWorker() {
	defer ea.aggregationWg.Done()

	for {
		select {
		case <-ea.doneChan:
			ea.logger.Debug().Msg("Aggregation worker stopped via done channel")
			return
		case <-ea.parentCtx.Done():
			ea.logger.Debug().Msg("Aggregation worker stopped via parent context cancellation")
			return
		case <-ea.aggregationTicker.C:
			// Check if we should still process ticks
			if ea.shouldAcceptEvent() {
				ea.handleAggregationTick()
			}
		}
	}
}

func (ea *EventAggregator) handleAggregationTick() {
	ea.logger.Debug().Msg("Aggregation timer tick - sending aggregated events")
	ea.sendAggregatedChanges()
	ea.sendAggregatedErrors()
}

// Private notification methods

func (ea *EventAggregator) sendAggregatedChanges() {
	changes := ea.extractFileChangeEvents()
	if len(changes) == 0 {
		return
	}

	ea.sendFileChangesNotification(changes)
}

func (ea *EventAggregator) sendAggregatedErrors() {
	errors := ea.extractFetchErrors()
	if len(errors) == 0 {
		return
	}

	ea.sendFetchErrorsNotification(errors)
}

func (ea *EventAggregator) extractFileChangeEvents() []models.FileChangeInfo {
	ea.fileChangeEventsMutex.Lock()
	defer ea.fileChangeEventsMutex.Unlock()

	if len(ea.fileChangeEvents) == 0 {
		return nil
	}

	// Create a new slice with exact capacity to avoid over-allocation
	changes := make([]models.FileChangeInfo, len(ea.fileChangeEvents))
	copy(changes, ea.fileChangeEvents)

	// Reset slice efficiently while keeping capacity
	ea.fileChangeEvents = ea.fileChangeEvents[:0]
	return changes
}

func (ea *EventAggregator) extractFetchErrors() []models.MonitorFetchErrorInfo {
	ea.fetchErrorsMutex.Lock()
	defer ea.fetchErrorsMutex.Unlock()

	if len(ea.fetchErrors) == 0 {
		return nil
	}

	// Create a new slice with exact capacity to avoid over-allocation
	errors := make([]models.MonitorFetchErrorInfo, len(ea.fetchErrors))
	copy(errors, ea.fetchErrors)

	// Reset slice efficiently while keeping capacity
	ea.fetchErrors = ea.fetchErrors[:0]
	return errors
}

func (ea *EventAggregator) sendFileChangesNotification(changes []models.FileChangeInfo) {
	if ea.notificationHelper == nil {
		return
	}

	// Don't send notifications if shutting down
	if !ea.shouldAcceptEvent() {
		ea.logger.Debug().Msg("Skipping file changes notification due to shutdown")
		return
	}

	// Check if context is cancelled
	ctx := ea.getOperationContext()
	select {
	case <-ctx.Done():
		ea.logger.Debug().Msg("Skipping file changes notification due to context cancellation")
		return
	default:
	}

	ea.logger.Info().
		Int("change_count", len(changes)).
		Msg("Sending aggregated file changes notification")

	ea.notificationHelper.SendAggregatedFileChangesNotification(
		ctx,
		changes,
		"",
	)
}

func (ea *EventAggregator) sendFetchErrorsNotification(errors []models.MonitorFetchErrorInfo) {
	if ea.notificationHelper == nil {
		return
	}

	// Don't send notifications if shutting down
	if !ea.shouldAcceptEvent() {
		ea.logger.Debug().Msg("Skipping fetch errors notification due to shutdown")
		return
	}

	// Check if context is cancelled
	ctx := ea.getOperationContext()
	select {
	case <-ctx.Done():
		ea.logger.Debug().Msg("Skipping fetch errors notification due to context cancellation")
		return
	default:
	}

	ea.logger.Info().
		Int("error_count", len(errors)).
		Msg("Sending aggregated fetch errors notification")

	ea.notificationHelper.SendAggregatedMonitorErrorsNotification(
		ctx,
		errors,
	)
}

func (ea *EventAggregator) getOperationContext() context.Context {
	// Use parent context if available, otherwise use background
	if ea.parentCtx != nil {
		return ea.parentCtx
	}
	return context.Background()
}
