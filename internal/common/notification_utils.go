package common

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// NotificationSummary represents a generic notification summary
type NotificationSummary struct {
	SessionID     string
	Component     string
	Status        string
	TargetSource  string
	Targets       []string
	TotalTargets  int
	Duration      time.Duration
	ReportPath    string
	ErrorMessages []string
	Metadata      map[string]interface{}
	Timestamp     time.Time
}

// NotificationBuilder helps build notification summaries
type NotificationBuilder struct {
	summary NotificationSummary
	logger  zerolog.Logger
}

// NewNotificationBuilder creates a new notification builder
func NewNotificationBuilder(sessionID, component string, logger zerolog.Logger) *NotificationBuilder {
	return &NotificationBuilder{
		summary: NotificationSummary{
			SessionID:     sessionID,
			Component:     component,
			Metadata:      make(map[string]interface{}),
			Timestamp:     time.Now(),
			ErrorMessages: make([]string, 0),
			Targets:       make([]string, 0),
		},
		logger: logger.With().Str("notification_builder", component).Str("session_id", sessionID).Logger(),
	}
}

// SetStatus sets the status of the notification
func (nb *NotificationBuilder) SetStatus(status string) *NotificationBuilder {
	nb.summary.Status = status
	return nb
}

// SetTargetSource sets the target source
func (nb *NotificationBuilder) SetTargetSource(source string) *NotificationBuilder {
	nb.summary.TargetSource = source
	return nb
}

// SetTargets sets the list of targets
func (nb *NotificationBuilder) SetTargets(targets []string) *NotificationBuilder {
	nb.summary.Targets = targets
	nb.summary.TotalTargets = len(targets)
	return nb
}

// SetDuration sets the operation duration
func (nb *NotificationBuilder) SetDuration(duration time.Duration) *NotificationBuilder {
	nb.summary.Duration = duration
	return nb
}

// SetReportPath sets the report file path
func (nb *NotificationBuilder) SetReportPath(path string) *NotificationBuilder {
	nb.summary.ReportPath = path
	return nb
}

// AddError adds an error message
func (nb *NotificationBuilder) AddError(err error) *NotificationBuilder {
	if err != nil {
		nb.summary.ErrorMessages = append(nb.summary.ErrorMessages, err.Error())
	}
	return nb
}

// AddErrorMessage adds an error message string
func (nb *NotificationBuilder) AddErrorMessage(message string) *NotificationBuilder {
	if message != "" {
		nb.summary.ErrorMessages = append(nb.summary.ErrorMessages, message)
	}
	return nb
}

// SetMetadata sets a metadata value
func (nb *NotificationBuilder) SetMetadata(key string, value interface{}) *NotificationBuilder {
	nb.summary.Metadata[key] = value
	return nb
}

// Build returns the built notification summary
func (nb *NotificationBuilder) Build() NotificationSummary {
	nb.logger.Debug().
		Str("status", nb.summary.Status).
		Int("target_count", nb.summary.TotalTargets).
		Int("error_count", len(nb.summary.ErrorMessages)).
		Msg("Built notification summary")

	return nb.summary
}

// NotificationScheduler manages notification scheduling and batching
type NotificationScheduler struct {
	notifications []NotificationSummary
	batchSize     int
	batchTimeout  time.Duration
	logger        zerolog.Logger
	ctx           context.Context
	cancelFunc    context.CancelFunc
	flushFunc     func([]NotificationSummary) error
}

// NotificationSchedulerConfig holds configuration for notification scheduler
type NotificationSchedulerConfig struct {
	BatchSize    int
	BatchTimeout time.Duration
	Logger       zerolog.Logger
	FlushFunc    func([]NotificationSummary) error
}

// NewNotificationScheduler creates a new notification scheduler
func NewNotificationScheduler(config NotificationSchedulerConfig) *NotificationScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &NotificationScheduler{
		notifications: make([]NotificationSummary, 0),
		batchSize:     config.BatchSize,
		batchTimeout:  config.BatchTimeout,
		logger:        config.Logger.With().Str("component", "NotificationScheduler").Logger(),
		ctx:           ctx,
		cancelFunc:    cancel,
		flushFunc:     config.FlushFunc,
	}
}

// Add adds a notification to the scheduler
func (ns *NotificationScheduler) Add(notification NotificationSummary) {
	ns.notifications = append(ns.notifications, notification)

	ns.logger.Debug().
		Str("session_id", notification.SessionID).
		Str("component", notification.Component).
		Int("queue_size", len(ns.notifications)).
		Msg("Added notification to scheduler")

	// Check if we should flush immediately
	if len(ns.notifications) >= ns.batchSize {
		ns.flush()
	}
}

// Start starts the notification scheduler with periodic flushing
func (ns *NotificationScheduler) Start() {
	if ns.batchTimeout <= 0 {
		ns.logger.Info().Msg("Batch timeout is 0, scheduler will only flush on batch size")
		return
	}

	go func() {
		ticker := time.NewTicker(ns.batchTimeout)
		defer ticker.Stop()

		for {
			select {
			case <-ns.ctx.Done():
				ns.logger.Info().Msg("Notification scheduler stopping")
				ns.flush() // Final flush
				return
			case <-ticker.C:
				if len(ns.notifications) > 0 {
					ns.logger.Debug().Int("queue_size", len(ns.notifications)).Msg("Periodic flush triggered")
					ns.flush()
				}
			}
		}
	}()
}

// Stop stops the notification scheduler
func (ns *NotificationScheduler) Stop() {
	ns.logger.Info().Msg("Stopping notification scheduler")
	ns.cancelFunc()
	ns.flush() // Final flush
}

// flush sends all queued notifications
func (ns *NotificationScheduler) flush() {
	if len(ns.notifications) == 0 {
		return
	}

	toFlush := make([]NotificationSummary, len(ns.notifications))
	copy(toFlush, ns.notifications)
	ns.notifications = ns.notifications[:0] // Clear the slice

	ns.logger.Info().Int("notification_count", len(toFlush)).Msg("Flushing notifications")

	if ns.flushFunc != nil {
		if err := ns.flushFunc(toFlush); err != nil {
			ns.logger.Error().Err(err).Int("notification_count", len(toFlush)).Msg("Failed to flush notifications")
		} else {
			ns.logger.Debug().Int("notification_count", len(toFlush)).Msg("Successfully flushed notifications")
		}
	}
}

// NotificationAggregator aggregates similar notifications
type NotificationAggregator struct {
	aggregations map[string]*AggregatedNotification
	logger       zerolog.Logger
}

// AggregatedNotification represents multiple notifications aggregated together
type AggregatedNotification struct {
	Component     string
	Status        string
	Count         int
	FirstSeen     time.Time
	LastSeen      time.Time
	SessionIDs    []string
	ErrorMessages []string
	Metadata      map[string]interface{}
}

// NewNotificationAggregator creates a new notification aggregator
func NewNotificationAggregator(logger zerolog.Logger) *NotificationAggregator {
	return &NotificationAggregator{
		aggregations: make(map[string]*AggregatedNotification),
		logger:       logger.With().Str("component", "NotificationAggregator").Logger(),
	}
}

// Add adds a notification to the aggregator
func (na *NotificationAggregator) Add(notification NotificationSummary) {
	key := fmt.Sprintf("%s:%s", notification.Component, notification.Status)

	if agg, exists := na.aggregations[key]; exists {
		// Update existing aggregation
		agg.Count++
		agg.LastSeen = notification.Timestamp
		agg.SessionIDs = append(agg.SessionIDs, notification.SessionID)
		agg.ErrorMessages = append(agg.ErrorMessages, notification.ErrorMessages...)

		// Merge metadata
		for k, v := range notification.Metadata {
			agg.Metadata[k] = v
		}
	} else {
		// Create new aggregation
		na.aggregations[key] = &AggregatedNotification{
			Component:     notification.Component,
			Status:        notification.Status,
			Count:         1,
			FirstSeen:     notification.Timestamp,
			LastSeen:      notification.Timestamp,
			SessionIDs:    []string{notification.SessionID},
			ErrorMessages: append([]string(nil), notification.ErrorMessages...),
			Metadata:      make(map[string]interface{}),
		}

		// Copy metadata
		for k, v := range notification.Metadata {
			na.aggregations[key].Metadata[k] = v
		}
	}

	na.logger.Debug().
		Str("key", key).
		Int("count", na.aggregations[key].Count).
		Msg("Added notification to aggregation")
}

// GetAggregations returns all current aggregations
func (na *NotificationAggregator) GetAggregations() map[string]*AggregatedNotification {
	result := make(map[string]*AggregatedNotification)
	for k, v := range na.aggregations {
		result[k] = v
	}
	return result
}

// Clear clears all aggregations
func (na *NotificationAggregator) Clear() {
	na.aggregations = make(map[string]*AggregatedNotification)
	na.logger.Debug().Msg("Cleared all aggregations")
}

// NotificationFilter filters notifications based on criteria
type NotificationFilter struct {
	allowedComponents []string
	allowedStatuses   []string
	minSeverity       string
	logger            zerolog.Logger
}

// NotificationFilterConfig holds configuration for notification filter
type NotificationFilterConfig struct {
	AllowedComponents []string
	AllowedStatuses   []string
	MinSeverity       string
	Logger            zerolog.Logger
}

// NewNotificationFilter creates a new notification filter
func NewNotificationFilter(config NotificationFilterConfig) *NotificationFilter {
	return &NotificationFilter{
		allowedComponents: config.AllowedComponents,
		allowedStatuses:   config.AllowedStatuses,
		minSeverity:       config.MinSeverity,
		logger:            config.Logger.With().Str("component", "NotificationFilter").Logger(),
	}
}

// ShouldSend determines if a notification should be sent based on filter criteria
func (nf *NotificationFilter) ShouldSend(notification NotificationSummary) bool {
	// Check component filter
	if len(nf.allowedComponents) > 0 {
		allowed := false
		for _, component := range nf.allowedComponents {
			if component == notification.Component {
				allowed = true
				break
			}
		}
		if !allowed {
			nf.logger.Debug().
				Str("component", notification.Component).
				Strs("allowed_components", nf.allowedComponents).
				Msg("Notification filtered out by component")
			return false
		}
	}

	// Check status filter
	if len(nf.allowedStatuses) > 0 {
		allowed := false
		for _, status := range nf.allowedStatuses {
			if status == notification.Status {
				allowed = true
				break
			}
		}
		if !allowed {
			nf.logger.Debug().
				Str("status", notification.Status).
				Strs("allowed_statuses", nf.allowedStatuses).
				Msg("Notification filtered out by status")
			return false
		}
	}

	nf.logger.Debug().
		Str("session_id", notification.SessionID).
		Str("component", notification.Component).
		Str("status", notification.Status).
		Msg("Notification passed filter")

	return true
}

// NotificationFormatter provides common formatting utilities
type NotificationFormatter struct {
	logger zerolog.Logger
}

// NewNotificationFormatter creates a new notification formatter
func NewNotificationFormatter(logger zerolog.Logger) *NotificationFormatter {
	return &NotificationFormatter{
		logger: logger.With().Str("component", "NotificationFormatter").Logger(),
	}
}

// FormatDuration formats a duration for display
func (nf *NotificationFormatter) FormatDuration(duration time.Duration) string {
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
}

// FormatErrorList formats a list of errors for display
func (nf *NotificationFormatter) FormatErrorList(errors []string, maxErrors int) string {
	if len(errors) == 0 {
		return "No errors"
	}

	if maxErrors <= 0 || len(errors) <= maxErrors {
		result := ""
		for i, err := range errors {
			if i > 0 {
				result += "\n"
			}
			result += fmt.Sprintf("• %s", err)
		}
		return result
	}

	// Show first maxErrors and indicate there are more
	result := ""
	for i := 0; i < maxErrors; i++ {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("• %s", errors[i])
	}
	result += fmt.Sprintf("\n... and %d more errors", len(errors)-maxErrors)

	return result
}

// FormatTargetList formats a list of targets for display
func (nf *NotificationFormatter) FormatTargetList(targets []string, maxTargets int) string {
	if len(targets) == 0 {
		return "No targets"
	}

	if maxTargets <= 0 || len(targets) <= maxTargets {
		result := ""
		for i, target := range targets {
			if i > 0 {
				result += "\n"
			}
			result += fmt.Sprintf("• %s", target)
		}
		return result
	}

	// Show first maxTargets and indicate there are more
	result := ""
	for i := 0; i < maxTargets; i++ {
		if i > 0 {
			result += "\n"
		}
		result += fmt.Sprintf("• %s", targets[i])
	}
	result += fmt.Sprintf("\n... and %d more targets", len(targets)-maxTargets)

	return result
}

// TruncateString truncates a string to a maximum length with ellipsis
func (nf *NotificationFormatter) TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}

	if maxLength <= 3 {
		return s[:maxLength]
	}

	return s[:maxLength-3] + "..."
}
