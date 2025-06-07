package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"
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

	if err := s.loadInitialMonitorTargets(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to load initial monitor targets")
		return
	}

	// Start monitoring service
	if err := s.monitoringService.LoadAndMonitorFromSources(s.monitorTargetsFile); err != nil {
		s.logger.Error().Err(err).Msg("Failed to start monitoring service")
		return
	}

	// Wait for shutdown signal
	if s.waitForMonitorShutdown(ctx) {
		s.handleMonitorContextCancellation()
	}
}

// loadInitialMonitorTargets loads initial monitor targets
func (s *Scheduler) loadInitialMonitorTargets() error {
	return nil // Implementation depends on specific requirements
}

// waitForMonitorShutdown waits for monitor shutdown signal
func (s *Scheduler) waitForMonitorShutdown(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-s.stopChan:
		s.logger.Info().Msg("Stop signal received, stopping monitor service")
		return false
	}
}

// handleMonitorContextCancellation handles monitor context cancellation
func (s *Scheduler) handleMonitorContextCancellation() {
	s.logger.Info().Msg("Context cancelled, stopping monitor service")
}
