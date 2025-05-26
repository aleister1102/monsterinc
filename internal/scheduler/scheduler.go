package scheduler

import (
	"context"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
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

// Start begins the scheduler's main loop
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}
	s.isRunning = true
	s.mu.Unlock()

	s.logger.Info().Msg("Scheduler: Starting automated scan scheduler...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run initial scan immediately
	s.logger.Info().Msg("Scheduler: Running initial scan...")
	s.runScanCycleWithRetries(context.Background())

	// Main scheduler loop
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			// Calculate next scan time
			nextScanTime, err := s.calculateNextScanTime()
			if err != nil {
				s.logger.Error().Err(err).Msg("Scheduler: Failed to calculate next scan time")
				// Wait a bit before retrying
				time.Sleep(5 * time.Minute)
				continue
			}

			s.logger.Info().Time("next_scan_time", nextScanTime).Msg("Scheduler: Next scan scheduled for")

			// Wait until next scan time or stop signal
			select {
			case <-time.After(time.Until(nextScanTime)):
				s.logger.Info().Msg("Scheduler: Starting scheduled scan...")
				s.runScanCycleWithRetries(context.Background())
			case <-s.stopChan:
				s.logger.Info().Msg("Scheduler: Received stop signal, exiting scheduler loop...")
				return
			case sig := <-sigChan:
				s.logger.Info().Str("signal", sig.String()).Msg("Scheduler: Received signal, initiating graceful shutdown")
				close(s.stopChan)
				return
			}
		}
	}()

	// Wait for scheduler to stop
	s.wg.Wait()
	if s.db != nil {
		s.logger.Info().Msg("Closing scheduler database connection.")
		s.db.Close()
	}

	s.logger.Info().Msg("Scheduler: Scheduler stopped.")
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
	scanID := fmt.Sprintf("scheduled_scan_%s", time.Now().Format("20060102-150405"))

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Info().Int("attempt", attempt).Int("max_retries", maxRetries).Dur("delay", retryDelay).Msg("Scheduler: Retrying scan cycle after delay...")
			time.Sleep(retryDelay)
		}
		// Each attempt should have its own summary, or we update a common one.
		// For now, runScanCycle will populate parts of a summary for its specific attempt.
		scanSummary, reportGeneratedPath, lastErr = s.runScanCycle(ctx, scanID) // Pass context and scanID, get summary and report path back
		scanSummary.RetriesAttempted = attempt                                  // Update retries

		if lastErr == nil {
			s.logger.Info().Str("scan_id", scanID).Msg("Scheduler: Scan cycle completed successfully.")
			scanSummary.Status = string(models.ScanStatusCompleted)
			scanSummary.ReportPath = reportGeneratedPath
			s.notificationHelper.SendScanCompletionNotification(ctx, scanSummary)
			return
		}

		s.logger.Error().Err(lastErr).Str("scan_id", scanID).Int("attempt", attempt+1).Int("total_attempts", maxRetries+1).Msg("Scheduler: Scan cycle failed")

		if attempt == maxRetries {
			s.logger.Error().Str("scan_id", scanID).Msg("Scheduler: All retry attempts exhausted. Scan cycle failed permanently.")
			scanSummary.Status = string(models.ScanStatusFailed)
			scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("All %d retry attempts failed. Last error: %v", maxRetries+1, lastErr))
			// reportGeneratedPath might be empty if error was before report generation
			scanSummary.ReportPath = reportGeneratedPath
			s.notificationHelper.SendScanCompletionNotification(ctx, scanSummary) // Send final failure notification
		}
	}
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
	summary.Targets = s.targetManager.GetTargetStrings(targets)
	summary.TotalTargets = len(targets)

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
	probeResults, urlDiffResults, workflowErr := s.scanOrchestrator.ExecuteScanWorkflow(seedURLs, scanSessionID)
	scanDuration := time.Since(startTime)
	summary.ScanDuration = scanDuration

	// Populate stats for summary
	if probeResults != nil {
		// TODO: Accurately count successful/failed probes from probeResults
		summary.ProbeStats.DiscoverableItems = len(probeResults)
		summary.ProbeStats.SuccessfulProbes = len(probeResults) // Placeholder
	}
	if urlDiffResults != nil {
		for _, diff := range urlDiffResults {
			summary.DiffStats.New += diff.New
			summary.DiffStats.Existing += diff.Existing
			summary.DiffStats.Old += diff.Old
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
