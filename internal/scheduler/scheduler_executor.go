package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// Start begins the scheduler's main loop
func (s *Scheduler) Start(ctx context.Context) error {
	if !s.setRunningState(true) {
		return fmt.Errorf("scheduler is already running")
	}
	defer s.setRunningState(false)

	s.resetStopChannel()

	if err := s.startConfiguredServices(ctx); err != nil {
		return err
	}

	s.wg.Wait()
	return s.checkContextError(ctx)
}

// startConfiguredServices starts the configured services
func (s *Scheduler) startConfiguredServices(ctx context.Context) error {
	scanStarted := s.tryStartScanService(ctx)
	monitorStarted := s.tryStartMonitorService(ctx)

	if !scanStarted && !monitorStarted {
		return fmt.Errorf("no services configured to run")
	}

	return nil
}

// tryStartScanService attempts to start the scan service
func (s *Scheduler) tryStartScanService(ctx context.Context) bool {
	if s.scanTargetsFile == "" {
		return false
	}

	s.wg.Add(1)
	go s.runScanner(ctx)
	return true
}

// tryStartMonitorService attempts to start the monitor service
func (s *Scheduler) tryStartMonitorService(ctx context.Context) bool {
	if s.monitorTargetsFile == "" {
		s.logger.Info().Msg("Monitor targets file not configured, skipping monitor service")
		return false
	}

	if s.monitoringService == nil {
		s.logger.Error().Msg("Monitoring service is nil, cannot start monitor service")
		return false
	}

	// s.logger.Info().Str("targets_file", s.monitorTargetsFile).Msg("Starting monitor service")
	s.monitoringService.SetParentContext(ctx)
	s.wg.Add(1)
	go s.runMonitorService(ctx)
	return true
}

// checkContextError checks for context errors
func (s *Scheduler) checkContextError(ctx context.Context) error {
	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	return nil
}

// runScanner executes scan operations loop
func (s *Scheduler) runScanner(ctx context.Context) {
	defer s.wg.Done()

	// Execute first scan immediately on startup
	s.logger.Info().Msg("Executing initial scan immediately on startup")
	s.executeScanCycleWithRetries(ctx)

	// Continue with regular scheduled cycles
	for {
		if s.shouldStopScanning(ctx) {
			return
		}

		if interrupted, err := s.waitForNextScan(ctx); interrupted || err != nil {
			if interrupted {
				return
			}
			continue
		}

		if s.shouldStopScanning(ctx) {
			return
		}

		s.executeScanCycleWithRetries(ctx)
	}
}

// shouldStopScanning checks if scanning should stop
func (s *Scheduler) shouldStopScanning(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		s.handleScanContextCancellation()
		return true
	case <-s.stopChan:
		s.logger.Info().Msg("Stop signal received, stopping scan service")
		return true
	default:
		return false
	}
}

// handleScanContextCancellation handles scan context cancellation
func (s *Scheduler) handleScanContextCancellation() {
	s.logger.Info().Msg("Context cancelled, stopping scan service")
}

// waitForNextScan waits for the next scan cycle
func (s *Scheduler) waitForNextScan(ctx context.Context) (interrupted bool, err error) {
	nextScanTime, err := s.calculateNextScanTime()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to calculate next scan time")
		return false, err
	}

	waitDuration := time.Until(nextScanTime)
	s.logger.Info().
		Time("next_scan", nextScanTime).
		Dur("wait_duration", waitDuration).
		Msg("Waiting for next scan cycle")

	select {
	case <-time.After(waitDuration):
		return false, nil
	case <-ctx.Done():
		return true, nil
	case <-s.stopChan:
		return true, nil
	}
}

// calculateNextScanTime calculates when the next scan should occur
func (s *Scheduler) calculateNextScanTime() (time.Time, error) {
	cycleMinutes := s.globalConfig.SchedulerConfig.CycleMinutes
	if cycleMinutes <= 0 {
		return time.Time{}, fmt.Errorf("invalid cycle minutes: %d", cycleMinutes)
	}

	cycleDuration := time.Duration(cycleMinutes) * time.Minute
	nextScanTime := time.Now().Add(cycleDuration)

	return nextScanTime, nil
}

// runMonitorService executes monitor operations
func (s *Scheduler) runMonitorService(ctx context.Context) {
	defer s.wg.Done()

	s.logger.Info().Str("targets_file", s.monitorTargetsFile).Msg("🚀 Monitor service starting...")

	// Generate initial cycle ID
	cycleID := fmt.Sprintf("monitor-%s", time.Now().Format("20060102-150405"))

	if err := s.loadInitialMonitorTargets(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to load initial monitor targets")

		// Send error notification for target loading failure
		s.sendMonitorErrorNotification(ctx, cycleID, "initialization", "TargetLoader", err.Error(), false, 0)
		return
	}

	// Start monitoring service
	if err := s.monitoringService.LoadAndMonitorFromSources(s.monitorTargetsFile); err != nil {
		s.logger.Error().Err(err).Msg("Failed to load monitoring targets")

		// Send error notification for service initialization failure
		s.sendMonitorErrorNotification(ctx, cycleID, "initialization", "MonitoringService", err.Error(), false, 0)
		return
	}

	// Get total targets for notifications
	totalTargets := len(s.monitoringService.GetCurrentlyMonitorUrls())

	// Send start notification
	s.sendMonitorStartNotification(ctx, cycleID, totalTargets)

	s.logger.Info().Msg("🎯 Monitor targets loaded, starting monitoring loop...")

	// Monitor loop
	for {
		if s.shouldStopMonitoring(ctx) {
			// Send interrupt notification when stopping normally
			s.sendMonitorInterruptNotification(ctx, cycleID, totalTargets, 0, "service_stopped", "Monitor service stopped gracefully")
			return
		}

		s.logger.Info().Msg("⚡ Starting monitoring cycle...")

		// Execute batch monitoring
		if err := s.monitoringService.ExecuteBatchMonitoring(ctx, s.monitorTargetsFile); err != nil {
			s.logger.Error().Err(err).Msg("Monitor cycle failed")

			// Send error notification for cycle failure
			s.sendMonitorErrorNotification(ctx, cycleID, "batch_processing", "BatchMonitoring", err.Error(), true, totalTargets)
		} else {
			s.logger.Info().Msg("✅ Monitor cycle completed successfully")
		}

		// Wait for next cycle or stop signal
		if interrupted := s.waitForNextMonitorCycle(ctx); interrupted {
			// Send interrupt notification when interrupted
			reason := "context_canceled"
			if ctx.Err() == context.Canceled {
				reason = "user_signal"
			}
			s.sendMonitorInterruptNotification(ctx, cycleID, totalTargets, 0, reason, "Monitor cycle interrupted")
			return
		}
	}
}

// shouldStopMonitoring checks if monitoring should stop
func (s *Scheduler) shouldStopMonitoring(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		s.handleMonitorContextCancellation()
		return true
	case <-s.stopChan:
		s.logger.Info().Msg("Stop signal received, stopping monitor service")
		return true
	default:
		return false
	}
}

// waitForNextMonitorCycle waits for the next monitor cycle
func (s *Scheduler) waitForNextMonitorCycle(ctx context.Context) bool {
	checkIntervalSeconds := s.globalConfig.MonitorConfig.CheckIntervalSeconds
	if checkIntervalSeconds <= 0 {
		checkIntervalSeconds = 1440 // Default 60*24 minutes (1 day) if not configured
	}

	cycleDuration := time.Duration(checkIntervalSeconds) * time.Second
	nextMonitorTime := time.Now().Add(cycleDuration)

	s.logger.Info().
		Time("next_monitor", nextMonitorTime).
		Msg("⏳ Waiting for next monitor cycle...")

	select {
	case <-time.After(cycleDuration):
		return false
	case <-ctx.Done():
		return true
	case <-s.stopChan:
		return true
	}
}

// loadInitialMonitorTargets loads initial monitor targets
func (s *Scheduler) loadInitialMonitorTargets() error {
	return nil // Implementation depends on specific requirements
}

// handleMonitorContextCancellation handles monitor context cancellation
func (s *Scheduler) handleMonitorContextCancellation() {
	s.logger.Info().Msg("Context cancelled, stopping monitor service")
}

// sendMonitorStartNotification sends a notification when monitor service starts
func (s *Scheduler) sendMonitorStartNotification(ctx context.Context, cycleID string, totalTargets int) {
	if s.notificationHelper == nil {
		return
	}

	checkIntervalSeconds := s.globalConfig.MonitorConfig.CheckIntervalSeconds
	if checkIntervalSeconds <= 0 {
		checkIntervalSeconds = 900 // Default to 15 minutes
	}
	cycleIntervalMinutes := checkIntervalSeconds / 60

	startData := models.MonitorStartData{
		CycleID:       cycleID,
		TotalTargets:  totalTargets,
		TargetSource:  s.monitorTargetsFile,
		Timestamp:     time.Now(),
		Mode:          s.globalConfig.Mode,
		CycleInterval: cycleIntervalMinutes,
	}

	s.notificationHelper.SendMonitorStartNotification(ctx, startData)
}

// sendMonitorInterruptNotification sends a notification when monitor service is interrupted
func (s *Scheduler) sendMonitorInterruptNotification(ctx context.Context, cycleID string, totalTargets, processedTargets int, reason, lastActivity string) {
	if s.notificationHelper == nil {
		return
	}

	interruptData := models.MonitorInterruptData{
		CycleID:          cycleID,
		TotalTargets:     totalTargets,
		ProcessedTargets: processedTargets,
		Timestamp:        time.Now(),
		Reason:           reason,
		LastActivity:     lastActivity,
	}

	s.notificationHelper.SendMonitorInterruptNotification(ctx, interruptData)
}

// sendMonitorErrorNotification sends a notification when monitor service encounters an error
func (s *Scheduler) sendMonitorErrorNotification(ctx context.Context, cycleID, errorType, component, errorMessage string, recoverable bool, totalTargets int) {
	if s.notificationHelper == nil {
		return
	}

	errorData := models.MonitorErrorData{
		CycleID:      cycleID,
		TotalTargets: totalTargets,
		Timestamp:    time.Now(),
		ErrorType:    errorType,
		ErrorMessage: errorMessage,
		Component:    component,
		Recoverable:  recoverable,
	}

	s.notificationHelper.SendMonitorErrorNotification(ctx, errorData)
}
