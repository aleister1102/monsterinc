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
	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.SchedulerConfig.SQLiteDBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Initialize database
	db, err := NewDB(cfg.SchedulerConfig.SQLiteDBPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	// Initialize ParquetReader (needed for orchestrator)
	parquetReader := datastore.NewParquetReader(&cfg.StorageConfig, logger)

	// Initialize ParquetWriter (needed for orchestrator)
	parquetWriter, err := datastore.NewParquetWriter(&cfg.StorageConfig, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("Scheduler: Failed to initialize ParquetWriter for orchestrator. Parquet storage will be disabled.")
		parquetWriter = nil
	}

	// Initialize TargetManager
	targetManager := NewTargetManager(logger)

	// Initialize ScanOrchestrator
	scanOrchestrator := orchestrator.NewScanOrchestrator(cfg, logger, parquetReader, parquetWriter)

	return &Scheduler{
		globalConfig:       cfg,
		db:                 db,
		logger:             logger,
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
		return fmt.Errorf("scheduler is already running")
	}
	s.isRunning = true
	s.mu.Unlock()

	s.logger.Info().Msg("Scheduler: Starting automated scan scheduler...")

	// Run initial scan immediately, respecting the context
	s.logger.Info().Msg("Scheduler: Running initial scan...")
	s.runScanCycleWithRetries(ctx) // Pass context

	// Main scheduler loop
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			// Check for context cancellation before calculating next scan or waiting
			select {
			case <-ctx.Done():
				s.logger.Info().Msg("Scheduler: Context cancelled, exiting scheduler loop...")
				return
			default:
			}

			nextScanTime, err := s.calculateNextScanTime()
			if err != nil {
				s.logger.Error().Err(err).Msg("Scheduler: Failed to calculate next scan time")
				// Wait a bit before retrying, but also check context
				select {
				case <-time.After(5 * time.Minute):
					continue
				case <-ctx.Done():
					s.logger.Info().Msg("Scheduler: Context cancelled during error backoff, exiting scheduler loop...")
					return
				case <-s.stopChan: // Also respect internal stopChan if Stop() is called directly
					s.logger.Info().Msg("Scheduler: Stop signal received during error backoff, exiting scheduler loop...")
					return
				}
			}

			s.logger.Info().Time("next_scan_time", nextScanTime).Msg("Scheduler: Next scan scheduled for")

			timer := time.NewTimer(time.Until(nextScanTime))
			select {
			case <-timer.C:
				// Ensure timer is stopped and drained if it fired, to prevent race conditions with Stop() or context cancellation
				if !timer.Stop() {
					select {
					case <-timer.C: // Drain the channel if Stop() returned false
					default:
					}
				}
				s.logger.Info().Msg("Scheduler: Starting scheduled scan...")
				s.runScanCycleWithRetries(ctx) // Pass context
			case <-s.stopChan:
				timer.Stop() // Stop the timer if it hasn't fired
				s.logger.Info().Msg("Scheduler: Received internal stop signal, exiting scheduler loop...")
				return
			case <-ctx.Done(): // Listen for context cancellation from main
				timer.Stop()
				s.logger.Info().Msg("Scheduler: Context cancelled by main, exiting scheduler loop...")
				return
				// Removed direct sigChan handling here as it's handled in main and propagated via context and s.stopChan
			}
		}
	}()

	// Wait for scheduler to stop or context to be cancelled
	select {
	case <-s.stopChan:
		s.logger.Info().Msg("Scheduler: Internal stop acknowledged.")
	case <-ctx.Done():
		s.logger.Info().Msg("Scheduler: Main context cancellation acknowledged, ensuring shutdown.")
		s.Stop() // Trigger internal stop to ensure wg is handled correctly
	}

	s.wg.Wait() // Wait for the goroutine to finish
	if s.db != nil {
		s.logger.Info().Msg("Closing scheduler database connection.")
		s.db.Close()
	}

	s.logger.Info().Msg("Scheduler: Scheduler fully stopped.")
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

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
