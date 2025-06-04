package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/monitor"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/scanner"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/rs/zerolog"
)

// Scheduler manages both scan and monitor operations in automated mode
type Scheduler struct {
	globalConfig       *config.GlobalConfig
	db                 *DB
	logger             zerolog.Logger
	scanTargetsFile    string
	monitorTargetsFile string // This is used for monitoring targets, if provided
	notificationHelper *notifier.NotificationHelper
	targetManager      *urlhandler.TargetManager
	scanner            *scanner.Scanner
	monitoringService  *monitor.MonitoringService
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.Mutex

	// Monitor scheduling fields are now primarily managed in monitor_workers.go
	// but the Scheduler struct still needs to hold references to the channel and WG
	// for coordination, especially during shutdown.
	monitorWorkerChan chan MonitorJob // MonitorJob is defined in monitor_workers.go
	monitorWorkerWG   sync.WaitGroup
	monitorTicker     *time.Ticker
}

// NewScheduler creates a new Scheduler instance
// Refactored ✅
func NewScheduler(
	cfg *config.GlobalConfig,
	scanTargetsFile string,
	scanner *scanner.Scanner,
	monitorTargetsFile string,
	monitoringService *monitor.MonitoringService,
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) (*Scheduler, error) {
	schedulerLogger := logger.With().Str("module", "Scheduler").Logger()

	if cfg.SchedulerConfig.SQLiteDBPath == "" {
		schedulerLogger.Error().Msg("SQLiteDBPath is not configured in SchedulerConfig")
		return nil, fmt.Errorf("sqliteDBPath is required for scheduler")
	}

	dbDir := filepath.Dir(cfg.SchedulerConfig.SQLiteDBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		schedulerLogger.Error().Err(err).Str("path", dbDir).Msg("Failed to create directory for SQLite database")
		return nil, fmt.Errorf("failed to create directory for sqlite database '%s': %w", dbDir, err)
	}

	db, err := NewDB(cfg.SchedulerConfig.SQLiteDBPath, schedulerLogger)
	if err != nil {
		schedulerLogger.Error().Err(err).Msg("Failed to initialize scheduler database")
		return nil, fmt.Errorf("failed to initialize scheduler database: %w", err)
	}

	targetManager := urlhandler.NewTargetManager(schedulerLogger)

	return &Scheduler{
		globalConfig:       cfg,
		db:                 db,
		logger:             schedulerLogger,
		scanTargetsFile:    scanTargetsFile,
		monitorTargetsFile: monitorTargetsFile,
		notificationHelper: notificationHelper,
		targetManager:      targetManager,
		scanner:            scanner,
		monitoringService:  monitoringService,
		stopChan:           make(chan struct{}),
	}, nil
}

// Start begins the scheduler's main loop
func (s *Scheduler) Start(ctx context.Context) error {
	// Ensure the scheduler is not already running
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		s.logger.Warn().Msg("Scheduler is already running.")
		return fmt.Errorf("scheduler is already running")
	}
	s.isRunning = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	// Start scan service if scan targets are configured
	if s.scanTargetsFile != "" {
		s.logger.Info().Str("scan_targets_file", s.scanTargetsFile).Msg("Scheduler: Scan targets file provided, will perform scanning.")

		s.wg.Add(1)
		go s.runScanner(ctx)

		s.logger.Info().Msg("Scheduler scan service started.")
	} else {
		s.logger.Info().Msg("Scheduler: No scan targets file provided, scanning will not be initialized.")
	}
	// Start monitor service independently if configured
	if s.monitorTargetsFile != "" && s.monitoringService != nil {
		s.logger.Info().Str("monitor_targets_file", s.monitorTargetsFile).Msg("Scheduler: Monitor targets file provided, will perform monitoring.")

		// Set parent context for proper interrupt detection
		s.monitoringService.SetParentContext(ctx)
		s.logger.Debug().Msg("Parent context set for MonitoringService")

		// Initialize and start monitoring service independently
		s.wg.Add(1)
		go s.runMonitorService(ctx)

		s.logger.Info().Msg("Monitor service started independently.")
	} else {
		s.logger.Info().Msg("Scheduler: No monitor targets file provided, monitoring will not be initialized.")
	}

	// If neither scan nor monitor is configured, log and return
	if s.scanTargetsFile == "" && (s.monitorTargetsFile == "" || s.monitoringService == nil) {
		s.logger.Warn().Msg("Scheduler: Neither scan targets nor monitor targets are configured. Nothing to do.")
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
		return fmt.Errorf("no services configured to run")
	}

	s.logger.Info().Msg("Scheduler services started, waiting for completion...")
	s.wg.Wait() // Block until all services finish
	s.logger.Info().Msg("Scheduler Start method is returning as all services have finished.")

	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	return nil
}

// runScanner is the core execution loop for scan operations only
func (s *Scheduler) runScanner(ctx context.Context) {
	defer s.wg.Done()
	defer func() {
		s.logger.Info().Msg("Scheduler scan service has stopped.")
	}()

	s.logger.Info().Msg("Starting scan service main loop...")

	for {
		if s.handleShutdownSignals(ctx, "scanLoop_start") {
			return
		}

		interrupted, err := s.waitForNextScan(ctx)
		if interrupted {
			return
		}
		if err != nil {
			continue
		}

		if s.handleShutdownSignals(ctx, "scanLoop_postWait") {
			return
		}

		s.logger.Info().Msg("Scheduler starting new scan cycle.")

		s.executeScanCycleWithRetries(ctx)
	}
}

// handleShutdownSignals checks for context cancellation or stop signals and handles them appropriately.
func (s *Scheduler) handleShutdownSignals(ctx context.Context, from string) bool {
	select {
	case <-ctx.Done():
		s.logger.Info().Str("source", from).Msg("Scheduler stopping due to context cancellation.")
		if s.notificationHelper != nil {
			interruptionSummary := models.NewScanSummaryDataBuilder().
				WithScanSessionID(fmt.Sprintf("scheduler_interrupted_ctx_%s_%s", from, time.Now().Format("20060102-150405"))).
				WithStatus(models.ScanStatusInterrupted).
				WithScanMode("automated").
				WithErrorMessages([]string{fmt.Sprintf("Scheduler service was interrupted by context cancellation (from %s).", from)}).
				WithTargetSource("Scheduler").
				Build()
			s.logger.Info().Msg("Sending scheduler interruption notification due to context cancellation.")
			s.notificationHelper.SendScanInterruptNotification(context.Background(), interruptionSummary)
		}
		return true
	case <-s.stopChan:
		s.logger.Info().Str("source", from).Msg("Scheduler stopping due to explicit Stop() call.")
		return true
	default:
		return false
	}
}

// waitForNextScan calculates the next scan time and sleeps until then (scan service only)
func (s *Scheduler) waitForNextScan(ctx context.Context) (interrupted bool, err error) {
	// This method now only handles scan scheduling, not monitor-only mode
	if s.scanTargetsFile == "" {
		s.logger.Error().Msg("waitForNextScan called but no scan targets configured - this should not happen")
		return true, fmt.Errorf("no scan targets configured")
	}

	nextScanTime, errCalc := s.calculateNextScanTime()
	if errCalc != nil {
		s.logger.Error().Err(errCalc).Msg("Failed to calculate next scan time. Retrying after 1 minute.")
		select {
		case <-time.After(1 * time.Minute):
			return false, errCalc
		case <-s.stopChan:
			s.logger.Info().Msg("Scan service stopped during error-induced sleep period.")
			return true, nil
		case <-ctx.Done():
			s.logger.Info().Msg("Scan service context cancelled during error-induced sleep period.")
			s.handleShutdownDuringSleep()
			return true, nil
		}
	}

	now := time.Now()
	if now.Before(nextScanTime) {
		sleepDuration := nextScanTime.Sub(now)
		s.logger.Info().Time("next_scan_at", nextScanTime).Dur("sleep_duration", sleepDuration).Msg("Scan service waiting for next scan cycle.")

		select {
		case <-time.After(sleepDuration):
			return false, nil // return false to indicate no interruption and back to the main loop
		case <-s.stopChan:
			s.logger.Info().Msg("Scan service stopped during sleep period.")
			return true, nil
		case <-ctx.Done():
			_ = s.handleShutdownDuringSleep()
			return true, nil
		}
	}
	return false, nil
}

// calculateNextScanTime determines when the next scan should run
func (s *Scheduler) calculateNextScanTime() (time.Time, error) {
	lastScanTime, err := s.db.GetLastScanTime()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Info().Msg("No previous completed scan found in history. Scheduling next scan immediately.")
			return time.Now(), nil
		}
		return time.Time{}, err
	}

	intervalDuration := time.Duration(s.globalConfig.SchedulerConfig.CycleMinutes) * time.Minute
	nextScanTime := lastScanTime.Add(intervalDuration)

	if nextScanTime.Before(time.Now()) {
		return time.Now(), nil
	}

	return nextScanTime, nil
}

// handleShutdownDuringSleep specifically handles shutdown signals received while waiting for the next scan.
func (s *Scheduler) handleShutdownDuringSleep() bool {
	s.logger.Info().Msg("Scheduler context cancelled during sleep period.")
	if s.notificationHelper != nil {
		interruptionSummary := models.NewScanSummaryDataBuilder().
			WithScanSessionID(fmt.Sprintf("scheduler_interrupted_sleep_%s", time.Now().Format("20060102-150405"))).
			WithStatus(models.ScanStatusInterrupted).
			WithScanMode("automated").
			WithErrorMessages([]string{"Scheduler service's scan cycle was interrupted during sleep period by context cancellation."}).
			WithTargetSource("Scheduler").
			Build()
		s.logger.Info().Msg("Sending scheduler scan interruption notification due to context cancellation during sleep.")

		// Send ScanInterruptNotification because this function is called when the scheduler's sleep FOR A SCAN is interrupted.
		s.notificationHelper.SendScanInterruptNotification(context.Background(), interruptionSummary)
	}
	return true
}

// runMonitorService runs the monitoring service independently
func (s *Scheduler) runMonitorService(ctx context.Context) {
	defer s.wg.Done()
	defer func() {
		s.logger.Info().Msg("Monitor service has stopped.")
		// Ensure MonitoringService.Stop() is called when monitor service exits
		if s.monitoringService != nil {
			s.logger.Info().Msg("Calling MonitoringService.Stop() from runMonitorService defer...")
			s.monitoringService.Stop()
		}
	}()

	s.logger.Info().Msg("Starting monitor service...")

	// Initialize monitor workers and ticker
	s.initializeMonitorWorkers(ctx)

	// Start the initial monitoring cycle
	s.logger.Info().Msg("Starting the initial monitoring cycle.")
	s.executeMonitoringCycle(ctx, "initial")

	// Keep monitor service running until shutdown
	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("Monitor service stopping due to context cancellation.")
			return
		case <-s.stopChan:
			s.logger.Info().Msg("Monitor service stopping due to explicit Stop() call.")
			return
		case <-time.After(1 * time.Minute):
			// Periodic health check or maintenance can be added here
			// For now, just continue the loop to keep the service alive
			continue
		}
	}
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		s.logger.Info().Msg("Scheduler is not running, no action needed for Stop().")
		return
	}

	s.logger.Info().Msg("Scheduler Stop() called, attempting to stop all services gracefully...")

	// Signal all services to stop
	if s.stopChan != nil {
		select {
		case _, ok := <-s.stopChan:
			if !ok {
				s.logger.Info().Msg("stopChan was already closed.")
			}
		default:
			close(s.stopChan)
			s.logger.Info().Msg("stopChan successfully closed.")
		}
	}
	s.mu.Unlock()
	// Stop monitor service components
	if s.monitoringService != nil {
		s.logger.Info().Msg("Stopping monitor workers and ticker...")
		if s.monitorTicker != nil {
			s.monitorTicker.Stop()
			s.logger.Info().Msg("Monitor ticker stopped.")
		}
		if s.monitorWorkerChan != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Warn().Interface("panic_info", r).Msg("Recovered from panic while trying to close monitorWorkerChan. It might have been closed already.")
					}
				}()

				close(s.monitorWorkerChan)
				s.logger.Info().Msg("Monitor worker channel closed.")
			}()
		}
		s.monitorWorkerWG.Wait()
		s.logger.Info().Msg("All monitor workers and ticker goroutine stopped.")

		// Call MonitoringService.Stop() to send interruption notification and cleanup
		s.logger.Info().Msg("Calling MonitoringService.Stop() to send interruption notification...")
		s.monitoringService.Stop()
		s.logger.Info().Msg("MonitoringService.Stop() completed.")
	}

	// Wait for all services (both scan and monitor) to complete
	s.logger.Info().Msg("Waiting for all scheduler services to complete...")
	s.wg.Wait()

	s.mu.Lock()
	s.isRunning = false
	s.logger.Info().Msg("All scheduler services confirmed finished.")

	if s.db != nil {
		s.logger.Info().Msg("Closing scheduler database connection...")
		if err := s.db.Close(); err != nil {
			s.logger.Error().Err(err).Msg("Error closing scheduler database")
		} else {
			s.logger.Info().Msg("Scheduler database closed successfully.")
		}
		s.db = nil
	}
	s.mu.Unlock()

	s.logger.Info().Msg("Scheduler has been stopped and all resources cleaned up.")
}
