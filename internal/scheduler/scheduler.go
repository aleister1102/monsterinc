package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/orchestrator"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/secrets"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Scheduler manages periodic scan operations in automated mode
type Scheduler struct {
	globalConfig       *config.GlobalConfig
	db                 *DB
	logger             zerolog.Logger
	urlFileOverride    string // From -urlfile command line flag
	notificationHelper *notifier.NotificationHelper
	targetManager      *TargetManager
	scanOrchestrator   *orchestrator.ScanOrchestrator
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.Mutex
}

// NewScheduler creates a new Scheduler instance
func NewScheduler(cfg *config.GlobalConfig, urlFileOverride string, logger zerolog.Logger, notificationHelper *notifier.NotificationHelper, secretDetector *secrets.SecretDetectorService) (*Scheduler, error) {
	moduleLogger := logger.With().Str("module", "Scheduler").Logger()

	if cfg.SchedulerConfig.SQLiteDBPath == "" {
		moduleLogger.Error().Msg("SQLiteDBPath is not configured in SchedulerConfig")
		return nil, fmt.Errorf("SQLiteDBPath is required for scheduler")
	}

	// Ensure the directory for SQLiteDBPath exists
	dbDir := filepath.Dir(cfg.SchedulerConfig.SQLiteDBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		moduleLogger.Error().Err(err).Str("path", dbDir).Msg("Failed to create directory for SQLite database")
		return nil, fmt.Errorf("failed to create directory for SQLite database '%s': %w", dbDir, err)
	}

	db, err := NewDB(cfg.SchedulerConfig.SQLiteDBPath, moduleLogger) // Pass moduleLogger to NewDB
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to initialize scheduler database")
		return nil, fmt.Errorf("failed to initialize scheduler database: %w", err)
	}

	targetManager := NewTargetManager(moduleLogger) // Pass moduleLogger

	// Initialize ParquetReader & Writer (needed for orchestrator)
	// These should use the main application logger or a sub-logger for datastore, not scheduler's logger directly unless specifically for scheduler's own parquet ops.
	// For now, using the passed 'logger' which is the main app logger instance.
	parquetReader := datastore.NewParquetReader(&cfg.StorageConfig, logger) // Use main logger
	// Initialize ParquetWriter without the ParquetReader argument
	parquetWriter, parquetErr := datastore.NewParquetWriter(&cfg.StorageConfig, logger) // Use main logger
	if parquetErr != nil {
		// This error will be logged by NewParquetWriter if it's critical, or handled by orchestrator if writer is nil.
		moduleLogger.Warn().Err(parquetErr).Msg("Failed to initialize ParquetWriter for scheduler's orchestrator. Parquet writing might be disabled or limited.")
		// Continue with parquetWriter as nil, orchestrator should handle this.
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(cfg, logger, parquetReader, parquetWriter, secretDetector)

	return &Scheduler{
		globalConfig:       cfg,
		db:                 db,
		logger:             moduleLogger, // Use the module-specific logger for scheduler's own logging
		urlFileOverride:    urlFileOverride,
		notificationHelper: notificationHelper,
		targetManager:      targetManager,
		scanOrchestrator:   scanOrchestrator,
		stopChan:           make(chan struct{}),
	}, nil
}

// Start begins the scheduler's main loop, now accepting a context for cancellation.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		s.logger.Warn().Msg("Scheduler is already running.")
		return fmt.Errorf("scheduler is already running")
	}
	s.isRunning = true
	s.stopChan = make(chan struct{}) // Recreate stopChan in case it was closed by a previous Stop()
	s.mu.Unlock()

	s.logger.Info().Msg("Scheduler starting...")
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			s.isRunning = false
			s.mu.Unlock()
			s.logger.Info().Msg("Scheduler has stopped main loop.")
		}()

		for {
			select {
			case <-ctx.Done(): // Handle context cancellation (e.g., from main application shutdown)
				s.logger.Info().Msg("Scheduler stopping due to context cancellation.")
				return
			case <-s.stopChan: // Handle explicit Stop() call
				s.logger.Info().Msg("Scheduler stopping due to explicit Stop() call.")
				return
			default:
				// Determine next scan time
				nextScanTime, err := s.calculateNextScanTime()
				if err != nil {
					s.logger.Error().Err(err).Msg("Failed to calculate next scan time. Retrying after 1 minute.")
					time.Sleep(1 * time.Minute) // Prevent rapid retries on calculation error
					continue
				}

				now := time.Now()
				if now.Before(nextScanTime) {
					sleepDuration := nextScanTime.Sub(now)
					s.logger.Info().Time("next_scan_at", nextScanTime).Dur("sleep_duration", sleepDuration).Msg("Scheduler waiting for next scan cycle.")

					select {
					case <-time.After(sleepDuration): // Wait for the calculated duration
						// Continue to scan
					case <-s.stopChan: // Or stop if Stop() is called during sleep
						s.logger.Info().Msg("Scheduler stopped during sleep period.")
						return
					case <-ctx.Done(): // Or stop if context is cancelled during sleep
						s.logger.Info().Msg("Scheduler context cancelled during sleep period.")
						return
					}
				}
				s.logger.Info().Msg("Scheduler starting new scan cycle.")
				s.runScanCycleWithRetries(ctx) // Pass context to the scan cycle
			}
		}
	}()

	s.logger.Info().Msg("Scheduler main loop goroutine started.")

	// Block here until the scheduler's main goroutine finishes.
	s.wg.Wait() // This will wait for s.wg.Done() to be called in the goroutine.

	s.logger.Info().Msg("Scheduler Start method is returning as the main loop has finished.")
	// Check context to understand why it stopped, if needed for return value
	if ctx.Err() != nil {
		return ctx.Err() // Return context error if that caused the stop
	}
	return nil
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
	// It's important that isRunning is true here.
	// Signal the main loop to stop.
	// We check if stopChan is nil or already closed to prevent panic.
	if s.stopChan != nil {
		select {
		case _, ok := <-s.stopChan:
			if !ok {
				s.logger.Info().Msg("stopChan was already closed.")
			}
		default:
			// Channel is open and not closed, so close it.
			close(s.stopChan)
			s.logger.Info().Msg("stopChan successfully closed.")
		}
	}
	s.mu.Unlock() // Unlock before s.wg.Wait() to avoid deadlock. The goroutine needs to acquire the lock to set isRunning to false.

	s.logger.Info().Msg("Waiting for scheduler's main goroutine to complete...")
	s.wg.Wait() // Wait for the main loop goroutine to finish.

	s.mu.Lock() // Re-acquire lock for final cleanup.
	// s.isRunning should have been set to false by the deferred function in the goroutine.
	// We can assert this or just ensure it here.
	s.isRunning = false
	s.logger.Info().Msg("Scheduler main goroutine confirmed finished.")

	// Close database connection
	if s.db != nil {
		s.logger.Info().Msg("Closing scheduler database connection...")
		if err := s.db.Close(); err != nil {
			s.logger.Error().Err(err).Msg("Error closing scheduler database")
		} else {
			s.logger.Info().Msg("Scheduler database closed successfully.")
		}
		s.db = nil // Prevent further use
	}
	s.mu.Unlock()

	s.logger.Info().Msg("Scheduler has been stopped and resources cleaned up.")
}

// calculateNextScanTime determines when the next scan should run
func (s *Scheduler) calculateNextScanTime() (time.Time, error) {
	lastScanTime, err := s.db.GetLastScanTime()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No previous completed scan, schedule for now
			s.logger.Info().Msg("No previous completed scan found in history. Scheduling next scan immediately.")
			return time.Now(), nil
		}
		// For other errors, propagate them
		return time.Time{}, err
	}

	// If lastScanTime is nil but no error (e.g. GetLastScanTime returned nil, nil for a valid scenario like an interrupted scan being the latest)
	// This case should ideally be handled by GetLastScanTime returning sql.ErrNoRows or a specific error.
	// Given the current GetLastScanTime logic, it returns sql.ErrNoRows if the time is NULL or no completed scan is found.
	// So the errors.Is(err, sql.ErrNoRows) above should cover this.
	// If, for some reason, lastScanTime could be nil without an error, that would be handled here:
	/*
		if lastScanTime == nil {
			s.logger.Info().Msg("Last scan time is nil (e.g., last scan was interrupted). Scheduling next scan immediately.")
			return time.Now(), nil
		}
	*/

	// Calculate next scan time based on interval
	intervalDuration := time.Duration(s.globalConfig.SchedulerConfig.CycleMinutes) * time.Minute
	nextScanTime := lastScanTime.Add(intervalDuration)

	// If the calculated time is in the past, schedule for now
	if nextScanTime.Before(time.Now()) {
		return time.Now(), nil
	}

	return nextScanTime, nil
}

// runScanCycleWithRetries runs a scan cycle with retry logic
func (s *Scheduler) runScanCycleWithRetries(ctx context.Context) {
	maxRetries := s.globalConfig.SchedulerConfig.RetryAttempts
	retryDelay := 5 * time.Minute // Fixed retry delay

	var lastErr error
	var scanSummary models.ScanSummaryData // To store summary for notification
	var reportGeneratedPath string
	currentScanSessionID := time.Now().Format("20060102-150405") // Use a consistent session ID for all retries of this cycle

	// Determine TargetSource once for this cycle based on current config/override
	// This assumes target source doesn't change between retries of the same cycle.
	_, initialTargetSource, initialTargetsErr := s.targetManager.LoadAndSelectTargets(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if initialTargetsErr != nil {
		s.logger.Error().Err(initialTargetsErr).Msg("Scheduler: Failed to determine initial target source for scan cycle. Notifications might be affected.")
		// If target source cannot be determined, use a placeholder or log and continue, notifications will be impacted.
		initialTargetSource = "ErrorDeterminingSource"
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Initialize summary for this attempt
		scanSummary = models.GetDefaultScanSummaryData() // Reset for each attempt, but keep some fields if needed
		scanSummary.ScanSessionID = currentScanSessionID
		scanSummary.TargetSource = initialTargetSource // Use consistently determined target source
		scanSummary.RetriesAttempted = attempt

		// Check for context cancellation at the beginning of each attempt
		select {
		case <-ctx.Done():
			s.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: Context cancelled before retry attempt. Stopping retry loop.")
			if lastErr != nil && attempt > 0 { // Only if this isn't the very first try and there was an error
				scanSummary.Status = string(models.ScanStatusInterrupted)
				if !containsCancellationError(scanSummary.ErrorMessages) {
					scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Scan cycle aborted during retries due to context cancellation. Last error: %v", lastErr))
				}
				s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary, notifier.ScanServiceNotification)
			}
			return
		default:
		}

		if attempt > 0 {
			s.logger.Info().Int("attempt", attempt).Int("max_retries", maxRetries).Dur("delay", retryDelay).Msg("Scheduler: Retrying scan cycle after delay...")
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				s.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: Context cancelled during retry delay. Stopping retry loop.")
				if lastErr != nil {
					scanSummary.Status = string(models.ScanStatusInterrupted)
					if !containsCancellationError(scanSummary.ErrorMessages) {
						scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Scan cycle aborted during retry delay due to context cancellation. Last error: %v", lastErr))
					}
					s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary, notifier.ScanServiceNotification)
				}
				return
			}
		}

		// Pass currentScanSessionID and consistent initialTargetSource to runScanCycle
		var currentAttemptSummary models.ScanSummaryData
		currentAttemptSummary, reportGeneratedPath, lastErr = s.runScanCycle(ctx, currentScanSessionID, initialTargetSource)

		// Merge results from runScanCycle into the main scanSummary for this retry loop iteration
		scanSummary.Targets = currentAttemptSummary.Targets
		scanSummary.TotalTargets = currentAttemptSummary.TotalTargets
		scanSummary.ProbeStats = currentAttemptSummary.ProbeStats
		scanSummary.DiffStats = currentAttemptSummary.DiffStats
		scanSummary.ScanDuration = currentAttemptSummary.ScanDuration
		scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, currentAttemptSummary.ErrorMessages...) // Append new errors
		scanSummary.ReportPath = reportGeneratedPath                                                          // This might change if report is generated on later attempt

		if lastErr == nil {
			s.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: Scan cycle completed successfully.")
			scanSummary.Status = string(models.ScanStatusCompleted)
			s.notificationHelper.SendScanCompletionNotification(ctx, scanSummary, notifier.ScanServiceNotification)
			return
		}

		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			s.logger.Info().Str("scan_session_id", currentScanSessionID).Err(lastErr).Msg("Scheduler: Scan cycle interrupted by context cancellation. No further retries.")
			scanSummary.Status = string(models.ScanStatusInterrupted)
			if !containsCancellationError(scanSummary.ErrorMessages) {
				scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Scan cycle interrupted: %v", lastErr))
			}
			s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary, notifier.ScanServiceNotification)
			return
		}

		s.logger.Error().Err(lastErr).Str("scan_session_id", currentScanSessionID).Int("attempt", attempt+1).Int("total_attempts", maxRetries+1).Msg("Scheduler: Scan cycle failed")

		if attempt == maxRetries {
			s.logger.Error().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: All retry attempts exhausted. Scan cycle failed permanently.")
			scanSummary.Status = string(models.ScanStatusFailed)
			if !containsCancellationError(scanSummary.ErrorMessages) {
				scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("All %d retry attempts failed. Last error: %v", maxRetries+1, lastErr))
			}
			s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary, notifier.ScanServiceNotification)
		}
	}
}

// Helper function to check if cancellation error is already in messages
func containsCancellationError(messages []string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "Scan cycle interrupted") {
			return true
		}
	}
	return false
}

// runScanCycle executes a complete scan cycle
func (s *Scheduler) runScanCycle(ctx context.Context, scanSessionID string, predeterminedTargetSource string) (models.ScanSummaryData, string, error) {
	startTime := time.Now()
	summary := models.GetDefaultScanSummaryData()
	summary.ScanSessionID = scanSessionID
	summary.TargetSource = predeterminedTargetSource // Use the source determined at the start of the retry cycle

	// Load targets using TargetManager. We use predeterminedTargetSource for consistency in reporting,
	// but LoadAndSelectTargets will re-evaluate based on current files/config for the actual scan.
	targets, determinedTargetSource, err := s.targetManager.LoadAndSelectTargets(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Scheduler: Failed to load targets for scan cycle.")
		// summary.ErrorMessages should be populated by caller (runScanCycleWithRetries) if this error is returned
		return summary, "ErrorDeterminingSource", fmt.Errorf("failed to load targets: %w", err)
	}
	if determinedTargetSource == "" { // Should ideally be set by TargetManager
		determinedTargetSource = "UnknownSource"
	}
	summary.TargetSource = determinedTargetSource

	if len(targets) == 0 {
		s.logger.Info().Str("source", determinedTargetSource).Msg("Scheduler: No targets to scan in this cycle.")
		summary.Status = string(models.ScanStatusNoTargets)
		summary.ErrorMessages = []string{"No targets loaded for this scan cycle."}
		// Do not return an error here, as it's a valid state (no targets)
		// The notification will indicate no targets. We will return summary and nil error.
		// The caller (runScanCycleWithRetries) should handle this summary and not treat it as a retryable error.
		return summary, determinedTargetSource, nil
	}

	// Get target strings for summary (these are just for display in notification)
	targetStringsForSummary, _ := s.targetManager.GetTargetStrings(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	summary.Targets = targetStringsForSummary
	summary.TotalTargets = len(targets)

	seedURLs := make([]string, len(targets))
	for _, target := range targets {
		seedURLs = append(seedURLs, target.OriginalURL)
	}

	if len(seedURLs) == 0 {
		msg := fmt.Sprintf("No valid seed URLs to scan from source: %s", summary.TargetSource)
		s.logger.Warn().Str("target_source", summary.TargetSource).Msg(msg)
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = []string{msg}
		return summary, determinedTargetSource, fmt.Errorf(msg)
	}

	// Send scan start notification with the consistent summary data
	// Create a temporary summary for start notification with status STARTED
	startNotificationSummary := summary // Copy current summary details
	startNotificationSummary.Status = string(models.ScanStatusStarted)
	s.notificationHelper.SendScanStartNotification(ctx, startNotificationSummary)

	scanDBID, err := s.db.RecordScanStart(summary.ScanSessionID, summary.TargetSource, len(seedURLs), startTime)
	if err != nil {
		msg := fmt.Sprintf("failed to record scan start in DB: %v", err)
		s.logger.Error().Err(err).Str("scan_session_id", summary.ScanSessionID).Msg("Failed to record scan start in DB")
		summary.ErrorMessages = append(summary.ErrorMessages, msg)
	}
	// For logging, we might want to use a more detailed scan ID if db id is available
	logScanID := summary.ScanSessionID
	if scanDBID > 0 {
		logScanID = fmt.Sprintf("%s (DB ID: %d)", summary.ScanSessionID, scanDBID)
	}

	s.logger.Info().Str("scan_id_log", logScanID).Int("num_targets", len(seedURLs)).Str("target_source", summary.TargetSource).Msg("Scheduler: Starting scan cycle execution.")

	probeResults, urlDiffResults, secretFindings, workflowErr := s.scanOrchestrator.ExecuteScanWorkflow(ctx, seedURLs, summary.ScanSessionID)
	scanDuration := time.Since(startTime)
	summary.ScanDuration = scanDuration

	// Populate stats for summary
	if probeResults != nil {
		summary.ProbeStats.DiscoverableItems = len(probeResults)
		for _, pr := range probeResults {
			// Consider a probe successful if it has no error and status code is not a client/server error (>=400)
			// Or if it has a redirect status code (3xx)
			if pr.Error == "" && (pr.StatusCode < 400 || (pr.StatusCode >= 300 && pr.StatusCode < 400)) {
				summary.ProbeStats.SuccessfulProbes++
			} else {
				summary.ProbeStats.FailedProbes++
			}
		}
	}

	// Populate DiffStats from urlDiffResults
	if urlDiffResults != nil {
		for _, diffResult := range urlDiffResults { // urlDiffResults is a map[string]models.URLDiffResult
			summary.DiffStats.New += diffResult.New
			summary.DiffStats.Old += diffResult.Old
			summary.DiffStats.Existing += diffResult.Existing
			// Note: diffResult.Changed is not currently populated by the differ
		}
	}

	// Log secret findings summary
	if len(secretFindings) > 0 {
		s.logger.Info().Int("secret_findings_count", len(secretFindings)).Str("scan_id_log", logScanID).Msg("Secret detection found findings during scheduled scan")
	}

	var reportPath string
	if workflowErr != nil {
		s.logger.Error().Err(workflowErr).Str("scan_id_log", logScanID).Msg("Scheduler: Scan workflow execution failed.")
		if scanDBID > 0 {
			s.db.UpdateScanCompletion(scanDBID, time.Now(), "FAILED", workflowErr.Error(), 0, 0, 0, "")
		}
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Scan workflow failed: %v", workflowErr))
		return summary, determinedTargetSource, workflowErr
	}

	s.logger.Info().Str("scan_id_log", logScanID).Msg("Scheduler: Scan workflow completed successfully.")

	reportFilename := fmt.Sprintf("%s_automated_report.html", summary.ScanSessionID)
	reportPath = filepath.Join(s.globalConfig.ReporterConfig.OutputDir, reportFilename)

	// Convert probeResults to []*models.ProbeResult for reporter
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	err = s.generateReport(probeResultsPtr, urlDiffResults, secretFindings, reportPath)
	if err != nil {
		s.logger.Error().Err(err).Str("scan_id_log", logScanID).Msg("Scheduler: Failed to generate HTML report.")
		if scanDBID > 0 {
			s.db.UpdateScanCompletion(scanDBID, time.Now(), "PARTIAL_COMPLETE", fmt.Sprintf("Workflow OK, but report generation failed: %v", err), summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing, "")
		}
		summary.Status = string(models.ScanStatusPartialComplete)
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Report generation failed: %v", err))
		return summary, "", err
	}

	// Check if report file was actually created
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		s.logger.Info().Str("scan_id_log", logScanID).Str("report_path", reportPath).Msg("Scheduler: HTML report was skipped (no data to report).")
		summary.ReportPath = "" // Clear report path since no file was created
		reportPath = ""         // Also clear the return value
	} else {
		s.logger.Info().Str("scan_id_log", logScanID).Str("report_path", reportPath).Msg("Scheduler: HTML report generated successfully.")
		summary.ReportPath = reportPath
	}

	// Record scan completion in DB
	if scanDBID > 0 {
		s.db.UpdateScanCompletion(scanDBID, time.Now(), "COMPLETED", "", summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing, reportPath)
	}

	summary.Status = string(models.ScanStatusCompleted)
	return summary, reportPath, nil
}

// generateReport generates a report from scan results
func (s *Scheduler) generateReport(probeResults []*models.ProbeResult, urlDiffResults map[string]models.URLDiffResult, secretFindings []models.SecretFinding, reportPath string) error {
	// Initialize HTML reporter
	htmlReporter, err := reporter.NewHtmlReporter(&s.globalConfig.ReporterConfig, s.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize HTML reporter: %w", err)
	}

	// Generate the report
	// The htmlReporter.GenerateReport method now expects []*models.ProbeResult
	if err := htmlReporter.GenerateReport(probeResults, urlDiffResults, secretFindings, reportPath); err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	return nil
}
