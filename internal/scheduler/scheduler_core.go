package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/aleister1102/monsterinc/internal/config"
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
	isStopped          bool
	mu                 sync.Mutex
	stopOnce           sync.Once
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

// initializeDatabase initializes the SQLite database for scheduler
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

// setRunningState safely sets the running state
func (s *Scheduler) setRunningState(running bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isStopped {
		return false
	}

	if running && s.isRunning {
		return false
	}
	s.isRunning = running
	return true
}

// resetStopChannel resets the stop channel
func (s *Scheduler) resetStopChannel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Reset stopped state
	s.isStopped = false
	s.stopOnce = sync.Once{} // Reset the once
	
	// Close old channel if it exists and create new one
	select {
	case <-s.stopChan:
		// Channel already closed or empty
	default:
		// Channel is open, close it
		close(s.stopChan)
	}
	
	s.stopChan = make(chan struct{})
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.logger.Info().Msg("Stopping scheduler...")

		s.mu.Lock()
		if s.isStopped {
			s.mu.Unlock()
			return
		}
		s.isStopped = true
		s.mu.Unlock()

		// Signal all goroutines to stop
		select {
		case <-s.stopChan:
			// Channel already closed
		default:
			close(s.stopChan)
		}

		// Wait for all goroutines to finish
		s.wg.Wait()

		// Close database connection
		if s.db != nil {
			if err := s.db.Close(); err != nil {
				s.logger.Error().Err(err).Msg("Error closing scheduler database")
			}
		}

		s.logger.Info().Msg("Scheduler stopped")
	})
}
