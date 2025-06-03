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

	// Check if the input file for scan targets is provided
	if s.scanTargetsFile == "" {
		s.logger.Info().Msg("Scheduler: No scan targets provided. Running in monitor-only mode for scheduled tasks.")
	} else {
		s.logger.Info().Str("scan_targets_file", s.scanTargetsFile).Msg("Scheduler: Scan targets file provided, will perform scans.")
	}
	if s.monitorTargetsFile != "" {
		s.logger.Info().Str("monitor_targets_file", s.monitorTargetsFile).Msg("Scheduler: Monitor targets file provided, will perform monitoring.")
	} else {
		s.logger.Info().Msg("Scheduler: No monitor targets file provided, monitoring will not be initialized.")
	}

	s.initializeMonitorWorkers() // This initializes workers and starts the periodic ticker for monitoring.

	// If monitoring service is active, start it.
	// Its Start() method will handle adding initial URLs and performing the first cycle.
	if s.monitoringService != nil {
		// URLs should be loaded into monitoringService by initializeMonitorWorkers (if monitorTargetsFile is set)
		// or could be added by other means if the design evolves.
		// We pass the currently known monitored URLs to Start().
		monitoredURLs := s.monitoringService.GetCurrentlyMonitorUrls()

		if err := s.monitoringService.Start(monitoredURLs); err != nil {
			s.logger.Error().Err(err).Msg("Failed to start MonitoringService from Scheduler.")
			// Propagate the error to the caller of Scheduler.Start()
			return fmt.Errorf("scheduler failed to start monitoring service: %w", err)
		}
		s.logger.Info().Msg("MonitoringService started successfully by Scheduler.")
		// The monitoringService.Start() method handles its own initial checks, notifications, and cycle.
		// The previous explicit call to s.executeMonitoringCycle("initial-startup") here is no longer needed.
	}

	// Start the main loop for scheduled scans.
	s.wg.Add(1)
	go s.runMainLoop(ctx)

	s.logger.Info().Msg("Scheduler main loop goroutine started.")
	s.wg.Wait() // Block until the main loop finishes
	s.logger.Info().Msg("Scheduler Start method is returning as the main loop has finished.")

	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	return nil
}

// runMainLoop is the core execution loop of the scheduler.
func (s *Scheduler) runMainLoop(ctx context.Context) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
		s.logger.Info().Msg("Scheduler has stopped main loop.")
	}()

	for {
		if s.handleShutdownSignals(ctx, "mainLoop_start") {
			return
		}

		interrupted, err := s.waitForNextScan(ctx)
		if interrupted {
			return
		}
		if err != nil {
			continue
		}

		if s.handleShutdownSignals(ctx, "mainLoop_postWait") {
			return
		}

		s.logger.Info().Msg("Scheduler starting new cycle.")

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

// waitForNextScan calculates the next scan time and sleeps until then.
func (s *Scheduler) waitForNextScan(ctx context.Context) (interrupted bool, err error) {
	if s.scanTargetsFile == "" && s.monitoringService != nil && len(s.monitoringService.GetCurrentlyMonitorUrls()) > 0 {
		s.logger.Info().Msg("Scheduler: No scan targets (-st) provided. Running in monitor-only mode for scheduled tasks. Waiting indefinitely for stop signal or context cancellation while monitor ticker runs.")
		select {
		case <-s.stopChan:
			s.logger.Info().Msg("Scheduler (monitor-only mode) stopped by Stop() call.")
			return true, nil
		case <-ctx.Done():
			s.logger.Info().Msg("Scheduler (monitor-only mode) context cancelled.")
			// No specific sleep interruption notification needed here as it wasn't in a scan-specific sleep.
			return true, nil
		}
	} else if s.scanTargetsFile == "" {
		s.logger.Info().Msg("Scheduler: No scan targets (-st) and no active periodic monitoring configured through scheduler. Waiting indefinitely for stop/cancellation.")
		select {
		case <-s.stopChan:
			s.logger.Info().Msg("Scheduler (no active tasks) stopped by Stop() call.")
			return true, nil
		case <-ctx.Done():
			s.logger.Info().Msg("Scheduler (no active tasks) context cancelled.")
			return true, nil
		}
	}

	nextScanTime, errCalc := s.calculateNextScanTime()
	if errCalc != nil {
		s.logger.Error().Err(errCalc).Msg("Failed to calculate next scan time. Retrying after 1 minute.")
		select {
		case <-time.After(1 * time.Minute):
			return false, errCalc
		case <-s.stopChan:
			s.logger.Info().Msg("Scheduler stopped during error-induced sleep period.")
			return true, nil
		case <-ctx.Done():
			s.logger.Info().Msg("Scheduler context cancelled during error-induced sleep period.")
			s.handleShutdownDuringSleep()
			return true, nil
		}
	}

	now := time.Now()
	if now.Before(nextScanTime) {
		sleepDuration := nextScanTime.Sub(now)
		s.logger.Info().Time("next_scan_at", nextScanTime).Dur("sleep_duration", sleepDuration).Msg("Scheduler waiting for next scan cycle.")

		select {
		case <-time.After(sleepDuration):
			return false, nil
		case <-s.stopChan:
			s.logger.Info().Msg("Scheduler stopped during sleep period.")
			return true, nil
		case <-ctx.Done():
			_ = s.handleShutdownDuringSleep()
			return true, nil
		}
	}
	return false, nil
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

// loadAndPrepareScanTargets loads targets, classifies them, and returns HTML URLs, monitor URLs, and the determined target source.
func (s *Scheduler) loadAndPrepareScanTargets(initialTargetSource string) (htmlURLs []string, determinedSource string, err error) {
	s.logger.Info().Msg("Scheduler: Starting to load and prepare scan targets.")
	targets, detSource, loadErr := s.targetManager.LoadAndSelectTargets(
		s.scanTargetsFile,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if loadErr != nil {
		return nil, initialTargetSource, fmt.Errorf("failed to load targets: %w", loadErr)
	}
	determinedSource = detSource
	if determinedSource == "" {
		determinedSource = "UnknownSource" // Default if not determined
	}

	if len(targets) == 0 {
		s.logger.Info().Str("source", determinedSource).Msg("Scheduler: No targets loaded to process.")
		return nil, determinedSource, fmt.Errorf("no targets to process from source: %s", determinedSource)
	}

	// Convert targets to string slice
	allTargetURLs := make([]string, len(targets))
	for i, target := range targets {
		allTargetURLs[i] = target.NormalizedURL
	}

	// Without content type grouping, all loaded URLs are considered for both scanning (as HTML) and monitoring.
	htmlURLs = make([]string, len(allTargetURLs))
	copy(htmlURLs, allTargetURLs)

	s.logger.Info().Int("total_targets_loaded", len(allTargetURLs)).Str("determined_source", determinedSource).Msg("Scheduler: Target loading completed. All targets will be used for both scanning and monitoring.")
	return htmlURLs, determinedSource, nil
}

// manageMonitorServiceTasks handles adding URLs to the monitoring service,
// sending start notifications, and triggering an initial monitoring cycle.
// It uses a WaitGroup to allow the caller to wait for these initial tasks.
func (s *Scheduler) manageMonitorServiceTasks(ctx context.Context, monitorWG *sync.WaitGroup, scanSessionID string, determinedSource string) {
	if s.monitoringService == nil {
		s.logger.Warn().Msg("Scheduler: Monitoring service is not available in manageMonitorServiceTasks, skipping monitor workflow.")
		return
	}
	monitorURLs := s.monitoringService.GetCurrentlyMonitorUrls()

	if len(monitorURLs) > 0 && s.monitoringService != nil {
		monitorWG.Add(1)
		go func() {
			defer monitorWG.Done()

			select {
			case <-ctx.Done():
				s.logger.Info().Msg("Scheduler: Monitor setup cancelled due to context cancellation.")
				return
			default:
			}

			s.logger.Info().Int("monitor_count", len(monitorURLs)).Msg("Scheduler: Adding URLs to monitoring service (parallel).")
			for _, url := range monitorURLs {
				select {
				case <-ctx.Done():
					s.logger.Info().Str("url", url).Msg("Scheduler: Monitor URL addition cancelled due to context cancellation.")
					return
				default:
					s.monitoringService.AddMonitorUrl(url)
				}
			}
			s.logger.Info().Msg("Scheduler: URLs added to monitoring service successfully.")

			select {
			case <-ctx.Done():
				s.logger.Info().Msg("Scheduler: Monitor cycle trigger cancelled due to context cancellation.")
				return
			default:
			}

			// The monitoringService.Start() method, called earlier in Scheduler.Start(),
			// now handles its own initial checks, notifications, and cycle based on the URLs it has.
			// Therefore, an explicit call to s.executeMonitoringCycle("post-scan") here is no longer needed
			// and could lead to redundant initial checks if Start() already performed them.
			s.logger.Info().Msg("Scheduler: Monitoring service's Start() method handles initial cycle. No explicit post-scan monitor cycle trigger needed here.")
		}()
	} else if s.monitoringService == nil {
		s.logger.Warn().Msg("Scheduler: Monitoring service is not available, skipping monitor workflow.")
	} else if len(monitorURLs) == 0 {
		s.logger.Info().Msg("Scheduler: No monitor URLs to add to monitoring service.")
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

	s.logger.Info().Msg("Scheduler Stop() called, attempting to stop gracefully...")
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
	}

	s.logger.Info().Msg("Waiting for scheduler's main goroutine to complete...")
	s.wg.Wait()

	s.mu.Lock()
	s.isRunning = false
	s.logger.Info().Msg("Scheduler main goroutine confirmed finished.")

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

	s.logger.Info().Msg("Scheduler has been stopped and resources cleaned up.")
}
