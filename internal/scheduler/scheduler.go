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
	monitorTargetsFile string
	notificationHelper *notifier.NotificationHelper
	targetManager      *urlhandler.TargetManager
	scanner            *scanner.Scanner
	monitoringService  *monitor.MonitoringService
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.Mutex

	// Monitor worker coordination
	monitorWorkerChan chan MonitorJob
	monitorWorkerWG   sync.WaitGroup
	monitorTicker     *time.Ticker
}

// NewScheduler creates a new Scheduler instance
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

	db, err := initializeDatabase(cfg.SchedulerConfig.SQLiteDBPath, schedulerLogger)
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		globalConfig:       cfg,
		db:                 db,
		logger:             schedulerLogger,
		scanTargetsFile:    scanTargetsFile,
		monitorTargetsFile: monitorTargetsFile,
		notificationHelper: notificationHelper,
		targetManager:      urlhandler.NewTargetManager(schedulerLogger),
		scanner:            scanner,
		monitoringService:  monitoringService,
		stopChan:           make(chan struct{}),
	}, nil
}

func initializeDatabase(dbPath string, logger zerolog.Logger) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("sqliteDBPath is required for scheduler")
	}

	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for sqlite database '%s': %w", dbDir, err)
	}

	db, err := NewDB(dbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize scheduler database: %w", err)
	}

	return db, nil
}

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

func (s *Scheduler) setRunningState(running bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if running && s.isRunning {
		return false
	}
	s.isRunning = running
	return true
}

func (s *Scheduler) resetStopChannel() {
	s.stopChan = make(chan struct{})
}

func (s *Scheduler) startConfiguredServices(ctx context.Context) error {
	scanStarted := s.tryStartScanService(ctx)
	monitorStarted := s.tryStartMonitorService(ctx)

	if !scanStarted && !monitorStarted {
		return fmt.Errorf("no services configured to run")
	}

	return nil
}

func (s *Scheduler) tryStartScanService(ctx context.Context) bool {
	if s.scanTargetsFile == "" {
		return false
	}

	s.wg.Add(1)
	go s.runScanner(ctx)
	return true
}

func (s *Scheduler) tryStartMonitorService(ctx context.Context) bool {
	if s.monitorTargetsFile == "" {
		s.logger.Info().Msg("Monitor targets file not configured, skipping monitor service")
		return false
	}

	if s.monitoringService == nil {
		s.logger.Error().Msg("Monitoring service is nil, cannot start monitor service")
		return false
	}

	s.logger.Info().Str("targets_file", s.monitorTargetsFile).Msg("Starting monitor service")
	s.monitoringService.SetParentContext(ctx)
	s.wg.Add(1)
	go s.runMonitorService(ctx)
	return true
}

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

func (s *Scheduler) shouldStopScanning(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		s.handleScanContextCancellation()
		return true
	default:
		return false
	}
}

func (s *Scheduler) handleScanContextCancellation() {
	// Don't send notification here because scan_executor will handle it
	// This prevents double interrupt notifications
	s.logger.Debug().Msg("Scan context cancelled - interrupt notification will be handled by scan executor")
}

func (s *Scheduler) waitForNextScan(ctx context.Context) (interrupted bool, err error) {
	nextScanTime, err := s.calculateNextScanTime()
	if err != nil {
		return false, err
	}

	sleepDuration := time.Until(nextScanTime)
	if sleepDuration <= 0 {
		s.logger.Info().Msg("No sleep needed, starting scan immediately")
		return false, nil
	}

	s.logger.Info().
		Dur("sleep_duration", sleepDuration).
		Time("next_scan_time", nextScanTime).
		Msg("Waiting for next scan cycle")

	select {
	case <-time.After(sleepDuration):
		s.logger.Info().Msg("Wait completed, starting next scan")
		return false, nil
	case <-ctx.Done():
		s.logger.Info().Msg("Context cancelled during wait")
		return true, nil
	case <-s.stopChan:
		s.logger.Info().Msg("Stop signal received during wait")
		return true, nil
	}
}

func (s *Scheduler) calculateNextScanTime() (time.Time, error) {
	lastScanTime, err := s.db.GetLastScanTime()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, fmt.Errorf("failed to get last scan time: %w", err)
	}

	cycleDuration := time.Duration(s.globalConfig.SchedulerConfig.CycleMinutes) * time.Minute

	if lastScanTime == nil {
		return time.Now(), nil
	}

	return lastScanTime.Add(cycleDuration), nil
}

func (s *Scheduler) runMonitorService(ctx context.Context) {
	defer s.wg.Done()

	s.logger.Info().Msg("Monitor service starting")

	if err := s.loadInitialMonitorTargets(); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to load initial monitor targets, monitor will continue without them")
	}

	s.logger.Info().Msg("Calling initializeMonitorWorkers")
	s.initializeMonitorWorkers(ctx)

	s.logger.Info().Msg("Monitor workers initialized, waiting for shutdown")
	interrupted := s.waitForMonitorShutdown(ctx)

	// Send interrupt notification if needed
	if interrupted {
		s.handleMonitorContextCancellation()
	}
}

func (s *Scheduler) loadInitialMonitorTargets() error {
	if err := s.monitoringService.LoadAndMonitorFromSources(
		s.monitorTargetsFile,
		s.globalConfig.MonitorConfig.InputURLs,
		s.globalConfig.MonitorConfig.InputFile,
	); err != nil {
		s.logger.Error().Err(err).Msg("Failed to load initial monitor targets")
		return err
	}
	return nil
}

func (s *Scheduler) waitForMonitorShutdown(ctx context.Context) bool {
	// Wait for either stop signal or context cancellation
	var interrupted bool
	select {
	case <-s.stopChan:
		s.logger.Info().Msg("Monitor shutdown via stop channel")
		interrupted = false
	case <-ctx.Done():
		s.logger.Info().Msg("Monitor shutdown via context cancellation")
		interrupted = true
	}

	// Stop monitoring service first
	if s.monitoringService != nil {
		s.logger.Info().Msg("Stopping monitoring service from scheduler")
		s.monitoringService.Stop()
	}

	if s.monitorTicker != nil {
		s.monitorTicker.Stop()
	}

	// Safe close of worker channel
	s.mu.Lock()
	if s.monitorWorkerChan != nil {
		close(s.monitorWorkerChan)
		s.monitorWorkerChan = nil
	}
	s.mu.Unlock()

	s.monitorWorkerWG.Wait()
	return interrupted
}

// handleMonitorContextCancellation handles monitor service interruption notification
func (s *Scheduler) handleMonitorContextCancellation() {
	s.logger.Info().Msg("Sending monitor interrupt notification due to context cancellation")

	if s.notificationHelper == nil {
		s.logger.Warn().Msg("No notification helper available for monitor interrupt notification")
		return
	}

	// Get current monitored URLs
	var monitoredURLs []string
	if s.monitoringService != nil {
		monitoredURLs = s.monitoringService.GetCurrentlyMonitorUrls()
	}

	// Build summary data for monitor interrupt notification
	interruptSummary := models.GetDefaultScanSummaryData()
	interruptSummary.ScanMode = "monitor"
	interruptSummary.TargetSource = "monitoring_service"
	interruptSummary.Targets = monitoredURLs
	interruptSummary.TotalTargets = len(monitoredURLs)
	interruptSummary.Status = string(models.ScanStatusInterrupted)
	interruptSummary.Component = "MonitoringService"
	interruptSummary.ErrorMessages = []string{"Monitor service interrupted by user signal (Ctrl+C)"}

	// Send monitor interrupt notification
	s.notificationHelper.SendMonitorInterruptNotification(
		context.Background(), // Use background context since main context is cancelled
		interruptSummary,
	)
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.logger.Info().Msg("Stopping scheduler...")

	// Stop monitoring service first if not already stopped
	if s.monitoringService != nil {
		s.logger.Info().Msg("Stopping monitoring service from Stop()")
		s.monitoringService.Stop()
	}

	close(s.stopChan)

	// Force cleanup of monitor workers if they exist
	if s.monitorTicker != nil {
		s.monitorTicker.Stop()
	}
	if s.monitorWorkerChan != nil {
		close(s.monitorWorkerChan)
		s.monitorWorkerChan = nil
	}

	s.logger.Info().Msg("Scheduler stopped")
}
