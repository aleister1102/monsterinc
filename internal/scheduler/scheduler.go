package scheduler

import (
	"context"
	"errors"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
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
func NewScheduler(cfg *config.GlobalConfig, urlFileOverride string, logger zerolog.Logger, notificationHelper *notifier.NotificationHelper) (*Scheduler, error) {
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
	parquetReader := datastore.NewParquetReader(&cfg.StorageConfig, logger)             // Use main logger
	parquetWriter, parquetErr := datastore.NewParquetWriter(&cfg.StorageConfig, logger) // Use main logger
	if parquetErr != nil {
		// This error will be logged by NewParquetWriter if it's critical, or handled by orchestrator if writer is nil.
		moduleLogger.Warn().Err(parquetErr).Msg("Failed to initialize ParquetWriter for scheduler's orchestrator. Parquet writing might be disabled or limited.")
		// Continue with parquetWriter as nil, orchestrator should handle this.
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(cfg, logger, parquetReader, parquetWriter) // Use main logger for orchestrator

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

	s.logger.Info().Msg("Scheduler main loop started.")
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		s.logger.Info().Msg("Scheduler is not running, no action needed for Stop().")
		return
	}

	s.logger.Info().Msg("Scheduler Stop() called, attempting to stop...")
	if s.stopChan != nil {
		close(s.stopChan) // Signal the main loop to stop
	}
	// isRunning will be set to false by the main loop goroutine upon exiting.

	// Optional: Wait for the scheduler's main goroutine to finish
	// This might be useful if you need to ensure cleanup or that no more scans are initiated.
	// However, this could block if the goroutine is stuck. Consider a timeout if using this.
	// s.wg.Wait() // This might block if runScanCycleWithRetries is long-running and doesn't respect stopChan quickly.
	s.logger.Info().Msg("Scheduler: Stopping scheduler...")
	close(s.stopChan)
	s.wg.Wait()
	s.isRunning = false

	// Close database connection
	if s.db != nil {
		s.db.Close()
	}

	s.logger.Info().Msg("Scheduler: Scheduler stopped.")
}

// calculateNextScanTime determines when the next scan should run
func (s *Scheduler) calculateNextScanTime() (time.Time, error) {
	lastScanTime, err := s.db.GetLastScanTime()
	if err != nil {
		return time.Time{}, err
	}

	// If no previous scan, schedule for now
	if lastScanTime == nil {
		return time.Now(), nil
	}

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
	scanID := time.Now().Format("20060102-150405")

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check for context cancellation at the beginning of each attempt
		select {
		case <-ctx.Done():
			s.logger.Info().Str("scan_id", scanID).Msg("Scheduler: Context cancelled before retry attempt. Stopping retry loop.")
			// If there was a previous error, ensure a failure notification for that attempt is sent.
			if lastErr != nil && attempt > 0 { // Only if this isn't the very first try and there was an error
				// Use the summary from the last failed attempt
				scanSummary.Status = string(models.ScanStatusFailed)
				if !containsCancellationError(scanSummary.ErrorMessages) {
					scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Scan cycle aborted during retries due to context cancellation. Last error: %v", lastErr))
				}
				s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary) // Use a new context for this final notification
			}
			return
		default:
		}

		if attempt > 0 {
			s.logger.Info().Int("attempt", attempt).Int("max_retries", maxRetries).Dur("delay", retryDelay).Msg("Scheduler: Retrying scan cycle after delay...")
			// Make the delay interruptible by context cancellation
			select {
			case <-time.After(retryDelay):
				// Delay completed
			case <-ctx.Done():
				s.logger.Info().Str("scan_id", scanID).Msg("Scheduler: Context cancelled during retry delay. Stopping retry loop.")
				// Similar notification logic as above if an error had occurred
				if lastErr != nil {
					scanSummary.Status = string(models.ScanStatusFailed)
					if !containsCancellationError(scanSummary.ErrorMessages) {
						scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Scan cycle aborted during retry delay due to context cancellation. Last error: %v", lastErr))
					}
					s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary)
				}
				return
			}
		}

		scanSummary, reportGeneratedPath, lastErr = s.runScanCycle(ctx, scanID)
		scanSummary.RetriesAttempted = attempt

		if lastErr == nil {
			s.logger.Info().Str("scan_id", scanID).Msg("Scheduler: Scan cycle completed successfully.")
			scanSummary.Status = string(models.ScanStatusCompleted)
			scanSummary.ReportPath = reportGeneratedPath
			s.notificationHelper.SendScanCompletionNotification(ctx, scanSummary)
			return
		}

		// If the error is due to context cancellation, stop retrying immediately.
		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			s.logger.Info().Str("scan_id", scanID).Err(lastErr).Msg("Scheduler: Scan cycle interrupted by context cancellation. No further retries.")
			scanSummary.Status = string(models.ScanStatusFailed) // Or a specific "Interrupted" status
			if !containsCancellationError(scanSummary.ErrorMessages) {
				scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Scan cycle interrupted: %v", lastErr))
			}
			scanSummary.ReportPath = reportGeneratedPath                                           // Report might have been partially generated
			s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary) // Use new context for this notification
			return
		}

		s.logger.Error().Err(lastErr).Str("scan_id", scanID).Int("attempt", attempt+1).Int("total_attempts", maxRetries+1).Msg("Scheduler: Scan cycle failed")

		if attempt == maxRetries {
			s.logger.Error().Str("scan_id", scanID).Msg("Scheduler: All retry attempts exhausted. Scan cycle failed permanently.")
			scanSummary.Status = string(models.ScanStatusFailed)
			if !containsCancellationError(scanSummary.ErrorMessages) {
				scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("All %d retry attempts failed. Last error: %v", maxRetries+1, lastErr))
			}
			scanSummary.ReportPath = reportGeneratedPath
			s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary)
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
func (s *Scheduler) runScanCycle(ctx context.Context, scanSessionID string) (models.ScanSummaryData, string, error) { // Added context, scanSessionID, return summary, report path and error
	startTime := time.Now()
	summary := models.GetDefaultScanSummaryData()
	summary.ScanID = scanSessionID

	// Load targets using TargetManager
	// In scheduler mode, target source is usually determined by config or persisted state, not direct file override each time.
	// For now, let's assume LoadAndSelectTargets correctly handles this based on initial config and potential future state management.
	targets, targetSource, err := s.targetManager.LoadAndSelectTargets(
		s.urlFileOverride, // This might be empty if not provided at startup
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if err != nil {
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = []string{fmt.Sprintf("Failed to load targets: %v", err)}
		return summary, "", fmt.Errorf("failed to load targets: %w", err) // Return error and empty summary
	}

	// Get target strings for summary
	targetStringsForSummary, err := s.targetManager.GetTargetStrings(
		s.urlFileOverride, // This might be empty if not provided at startup
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if err != nil {
		// Log the error, but proceed with an empty list for summary if critical targets themselves loaded fine.
		// The main `targets` variable from LoadAndSelectTargets is used for actual scanning logic.
		s.logger.Error().Err(err).Msg("Scheduler: Failed to get target strings for summary, summary.Targets will be empty.")
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Could not retrieve target strings for summary: %v", err))
		targetStringsForSummary = []string{}
	}
	summary.Targets = targetStringsForSummary
	summary.TotalTargets = len(targets) // TotalTargets should be based on the successfully loaded `targets` for scanning

	// Extract original URLs for crawler and other parts that expect []string
	var seedURLs []string
	for _, target := range targets {
		seedURLs = append(seedURLs, target.OriginalURL)
	}

	if len(seedURLs) == 0 {
		msg := fmt.Sprintf("No valid seed URLs to scan from source: %s", targetSource)
		s.logger.Warn().Str("target_source", targetSource).Msg(msg)
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = []string{msg}
		return summary, "", fmt.Errorf(msg)
	}

	// Send scan start notification
	s.notificationHelper.SendScanStartNotification(ctx, scanSessionID, summary.Targets, summary.TotalTargets)

	// Record scan start in DB
	scanDBID, err := s.db.RecordScanStart(scanSessionID, targetSource, len(seedURLs), startTime)
	if err != nil {
		msg := fmt.Sprintf("failed to record scan start in DB: %v", err)
		s.logger.Error().Err(err).Str("scan_id", scanSessionID).Msg("Failed to record scan start in DB")
		// Continue scan, but this is a notable issue
		summary.ErrorMessages = append(summary.ErrorMessages, msg)
	}
	summary.ScanID = fmt.Sprintf("%s (DB ID: %d)", scanSessionID, scanDBID) // Update ScanID with DB ID for clarity

	s.logger.Info().Str("scan_id", scanSessionID).Int("num_targets", len(seedURLs)).Str("target_source", targetSource).Msg("Scheduler: Starting scan cycle execution.")

	// Execute Scan Workflow via Orchestrator
	probeResults, urlDiffResults, workflowErr := s.scanOrchestrator.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
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

	var reportPath string
	if workflowErr != nil {
		s.logger.Error().Err(workflowErr).Str("scan_id", scanSessionID).Msg("Scheduler: Scan workflow execution failed.")
		if scanDBID > 0 {
			s.db.UpdateScanCompletion(scanDBID, time.Now(), "FAILED", workflowErr.Error(), 0, 0, 0, "")
		}
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Scan workflow failed: %v", workflowErr))
		return summary, "", workflowErr // Return error and current summary
	}

	s.logger.Info().Str("scan_id", scanSessionID).Msg("Scheduler: Scan workflow completed successfully.")

	// Generate HTML report
	reportFilename := fmt.Sprintf("%s_automated_report.html", scanSessionID)
	reportPath = filepath.Join(s.globalConfig.ReporterConfig.OutputDir, reportFilename)

	// Convert probeResults to []*models.ProbeResult for reporter
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	err = s.generateReport(probeResultsPtr, urlDiffResults, reportPath)
	if err != nil {
		s.logger.Error().Err(err).Str("scan_id", scanSessionID).Msg("Scheduler: Failed to generate HTML report.")
		if scanDBID > 0 {
			s.db.UpdateScanCompletion(scanDBID, time.Now(), "PARTIAL_COMPLETE", fmt.Sprintf("Workflow OK, but report generation failed: %v", err), summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing, "")
		}
		summary.Status = string(models.ScanStatusPartialComplete) // Or Failed, depending on severity
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Report generation failed: %v", err))
		// Even if report fails, the scan itself might have been a success in terms of data gathering
		// Return the error, but the caller (runScanCycleWithRetries) will decide if this is a full failure for retry purposes.
		return summary, "", err // Return with report error
	}
	s.logger.Info().Str("scan_id", scanSessionID).Str("report_path", reportPath).Msg("Scheduler: HTML report generated successfully.")
	summary.ReportPath = reportPath

	// Record scan completion in DB
	if scanDBID > 0 {
		s.db.UpdateScanCompletion(scanDBID, time.Now(), "COMPLETED", "", summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing, reportPath)
	}

	summary.Status = string(models.ScanStatusCompleted)
	return summary, reportPath, nil
}

// generateReport generates a report from scan results
func (s *Scheduler) generateReport(probeResults []*models.ProbeResult, urlDiffResults map[string]models.URLDiffResult, reportPath string) error {
	// Initialize HTML reporter
	htmlReporter, err := reporter.NewHtmlReporter(&s.globalConfig.ReporterConfig, s.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize HTML reporter: %w", err)
	}

	// Generate the report
	// The htmlReporter.GenerateReport method now expects []*models.ProbeResult
	if err := htmlReporter.GenerateReport(probeResults, urlDiffResults, reportPath); err != nil {
		return fmt.Errorf("failed to generate HTML report: %w", err)
	}

	return nil
}
