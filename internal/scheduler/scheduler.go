package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"monsterinc/internal/common"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
	"monsterinc/internal/secrets"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Scheduler manages periodic scan operations in automated mode
type Scheduler struct {
	*common.ServiceLifecycleManager
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
		return nil, common.NewConfigurationError("scheduler", "SQLiteDBPath", "is required for scheduler")
	}

	// Ensure the directory for SQLiteDBPath exists
	dbDir := filepath.Dir(cfg.SchedulerConfig.SQLiteDBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		moduleLogger.Error().Err(err).Str("path", dbDir).Msg("Failed to create directory for SQLite database")
		return nil, common.WrapError(err, fmt.Sprintf("failed to create directory for SQLite database '%s'", dbDir))
	}

	db, err := NewDB(cfg.SchedulerConfig.SQLiteDBPath, moduleLogger)
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to initialize scheduler database")
		return nil, common.WrapError(err, "failed to initialize scheduler database")
	}

	targetManager := NewTargetManager(moduleLogger)

	// Initialize ParquetReader & Writer (needed for orchestrator)
	parquetReader := datastore.NewParquetReader(&cfg.StorageConfig, logger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&cfg.StorageConfig, logger)
	if parquetErr != nil {
		moduleLogger.Warn().Err(parquetErr).Msg("Failed to initialize ParquetWriter for scheduler's orchestrator. Parquet writing might be disabled or limited.")
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(cfg, logger, parquetReader, parquetWriter, secretDetector)

	// Create scheduler instance
	scheduler := &Scheduler{
		globalConfig:       cfg,
		db:                 db,
		logger:             moduleLogger,
		urlFileOverride:    urlFileOverride,
		notificationHelper: notificationHelper,
		targetManager:      targetManager,
		scanOrchestrator:   scanOrchestrator,
		stopChan:           make(chan struct{}),
	}

	// Initialize service lifecycle manager
	lifecycleConfig := common.ServiceLifecycleConfig{
		ServiceInfo: common.ServiceInfo{
			Name:    "Scheduler",
			Type:    "scan_scheduler",
			Version: "1.0.0",
		},
		Logger:   moduleLogger,
		OnStart:  scheduler.startMainLoop,
		OnStop:   scheduler.stopMainLoop,
		OnHealth: scheduler.healthCheck,
	}

	scheduler.ServiceLifecycleManager = common.NewServiceLifecycleManager(lifecycleConfig)

	// Register database as a resource for cleanup
	dbResource := common.NewDatabaseResource(db, "scheduler_db", moduleLogger)
	scheduler.RegisterResource(dbResource)

	return scheduler, nil
}

// startMainLoop is called by the service lifecycle manager to start the main loop
func (s *Scheduler) startMainLoop(ctx context.Context) error {
	s.logger.Info().Msg("Starting scheduler main loop")

	s.AddWorker()
	go func() {
		defer s.WorkerDone()
		s.runMainLoop(ctx)
	}()

	return nil
}

// stopMainLoop is called by the service lifecycle manager during stop
func (s *Scheduler) stopMainLoop() error {
	s.logger.Info().Msg("Stopping scheduler main loop")
	// Main cleanup is handled by service lifecycle manager
	return nil
}

// healthCheck provides custom health check for the scheduler
func (s *Scheduler) healthCheck() common.HealthStatus {
	healthy := true
	message := "Scheduler is healthy"
	details := make(map[string]interface{})

	// Check database connection by attempting a simple query
	if s.db != nil {
		_, err := s.db.GetLastScanTime()
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			healthy = false
			message = "Database connection failed"
			details["database_error"] = err.Error()
		} else {
			details["database"] = "connected"
		}
	}

	// Check if orchestrator is available
	if s.scanOrchestrator != nil {
		details["orchestrator"] = "available"
	} else {
		details["orchestrator"] = "unavailable"
	}

	return common.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		CheckedAt: time.Now(),
	}
}

// runMainLoop contains the main scheduler loop logic
func (s *Scheduler) runMainLoop(ctx context.Context) {
	s.logger.Info().Msg("Scheduler main loop started")
	defer s.logger.Info().Msg("Scheduler main loop stopped")

	for {
		// Check for context cancellation
		if cancelled := common.CheckCancellationWithLog(ctx, s.logger, "scheduler main loop"); cancelled.Cancelled {
			return
		}

		// Check for shutdown signal
		select {
		case <-s.ShutdownChan():
			s.logger.Info().Msg("Scheduler stopping due to shutdown signal")
			return
		case <-ctx.Done():
			s.logger.Info().Msg("Scheduler stopping due to context cancellation")
			return
		default:
			// Continue with scan cycle
		}

		// Determine next scan time
		nextScanTime, err := s.calculateNextScanTime()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to calculate next scan time. Retrying after 1 minute.")
			if err := common.WaitWithCancellation(ctx, 1*time.Minute); err != nil {
				s.logger.Info().Msg("Context cancelled during error retry wait")
				return
			}
			continue
		}

		now := time.Now()
		if now.Before(nextScanTime) {
			sleepDuration := nextScanTime.Sub(now)
			s.logger.Debug().Time("next_scan_at", nextScanTime).Dur("sleep_duration", sleepDuration).Msg("Scheduler waiting for next scan cycle.")

			if err := common.WaitWithCancellation(ctx, sleepDuration); err != nil {
				s.logger.Info().Msg("Scheduler context cancelled during sleep period")
				return
			}

			// Check shutdown signal after sleep
			select {
			case <-s.ShutdownChan():
				s.logger.Info().Msg("Scheduler stopping due to shutdown signal after sleep")
				return
			default:
				// Continue
			}
		}

		s.logger.Debug().Msg("Scheduler starting new scan cycle")
		s.runScanCycleWithRetries(ctx) // Pass context to the scan cycle
	}
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

	retryConfig := common.RetryConfig{
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
		Operation:  "scan_cycle",
		SessionID:  currentScanSessionID,
	}

	retryFunc := func(attempt int) error {
		// Initialize summary for this attempt
		scanSummary = models.GetDefaultScanSummaryData() // Reset for each attempt, but keep some fields if needed
		scanSummary.ScanSessionID = currentScanSessionID
		scanSummary.TargetSource = initialTargetSource // Use consistently determined target source
		scanSummary.RetriesAttempted = attempt

		var scanErr error
		scanSummary, reportGeneratedPath, scanErr = s.runScanCycle(ctx, currentScanSessionID, initialTargetSource)

		// Store report path in scan summary for notification
		if reportGeneratedPath != "" {
			scanSummary.ReportPath = reportGeneratedPath
		}

		if scanErr != nil {
			// Check if error contains cancellation
			if common.ContainsCancellationError([]string{scanErr.Error()}) {
				s.logger.Info().Str("session_id", currentScanSessionID).Msg("Scan cycle interrupted by context cancellation, no further retries")
				return scanErr // Don't retry on cancellation
			}
			return scanErr
		}

		return nil
	}

	result := common.RetryWithCancellation(ctx, s.logger, retryConfig, retryFunc)

	// Handle final result and send notifications
	if result.Success {
		s.logger.Info().
			Str("session_id", currentScanSessionID).
			Int("attempts", result.Attempt+1).
			Dur("total_duration", result.TotalDuration).
			Msg("Scan cycle completed successfully")
	} else {
		s.logger.Error().
			Err(result.LastError).
			Str("session_id", currentScanSessionID).
			Int("attempts", result.Attempt+1).
			Dur("total_duration", result.TotalDuration).
			Msg("Scan cycle failed after all retries")

		// Update scan summary with failure information
		scanSummary.Status = string(models.ScanStatusFailed)
		if result.LastError != nil {
			scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, result.LastError.Error())
		}
	}

	// Send completion notification
	if s.notificationHelper != nil {
		s.notificationHelper.SendScanCompletionNotification(ctx, scanSummary, notifier.ScanServiceNotification)
	}
}

// runScanCycle executes a complete scan cycle
func (s *Scheduler) runScanCycle(ctx context.Context, scanSessionID string, predeterminedTargetSource string) (models.ScanSummaryData, string, error) {
	startTime := time.Now()

	// Prepare targets for scanning
	targets, seedURLs, summary, determinedTargetSource, err := s.prepareTargets(scanSessionID, predeterminedTargetSource)
	if err != nil {
		return summary, determinedTargetSource, err
	}

	// Handle case where no targets are available
	if len(targets) == 0 {
		return summary, determinedTargetSource, nil
	}

	// Send scan start notification
	startNotificationSummary := summary
	startNotificationSummary.Status = string(models.ScanStatusStarted)
	s.notificationHelper.SendScanStartNotification(ctx, startNotificationSummary)

	// Record scan start in database
	scanDBID, err := s.db.RecordScanStart(summary.ScanSessionID, summary.TargetSource, len(seedURLs), startTime)
	if err != nil {
		msg := fmt.Sprintf("failed to record scan start in DB: %v", err)
		s.logger.Error().Err(err).Str("scan_session_id", summary.ScanSessionID).Msg("Failed to record scan start in DB")
		summary.ErrorMessages = append(summary.ErrorMessages, msg)
	}

	// Execute scan for targets
	probeResults, urlDiffResults, secretFindings, updatedSummary, err := s.executeScanForTargets(ctx, seedURLs, summary, scanDBID, startTime)
	if err != nil {
		return updatedSummary, determinedTargetSource, err
	}

	// Process scan results and generate reports
	finalSummary, reportPath, err := s.processScanResults(probeResults, urlDiffResults, secretFindings, updatedSummary, scanDBID)
	if err != nil {
		return finalSummary, "", err
	}

	return finalSummary, reportPath, nil
}

// prepareTargets loads and prepares targets for scanning
func (s *Scheduler) prepareTargets(scanSessionID, predeterminedTargetSource string) ([]models.Target, []string, models.ScanSummaryData, string, error) {
	summary := models.GetDefaultScanSummaryData()
	summary.ScanSessionID = scanSessionID
	summary.TargetSource = predeterminedTargetSource

	// Load targets using TargetManager
	targets, determinedTargetSource, err := s.targetManager.LoadAndSelectTargets(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Scheduler: Failed to load targets for scan cycle.")
		return nil, nil, summary, "ErrorDeterminingSource", common.WrapError(err, "failed to load targets")
	}
	if determinedTargetSource == "" {
		determinedTargetSource = "UnknownSource"
	}
	summary.TargetSource = determinedTargetSource

	if len(targets) == 0 {
		s.logger.Info().Str("source", determinedTargetSource).Msg("Scheduler: No targets to scan in this cycle.")
		summary.Status = string(models.ScanStatusNoTargets)
		summary.ErrorMessages = []string{"No targets loaded for this scan cycle."}
		return targets, nil, summary, determinedTargetSource, nil
	}

	// Get target strings for summary
	targetStringsForSummary, _ := s.targetManager.GetTargetStrings(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	summary.Targets = targetStringsForSummary
	summary.TotalTargets = len(targets)

	// Convert targets to seed URLs
	seedURLs := make([]string, 0, len(targets))
	for _, target := range targets {
		seedURLs = append(seedURLs, target.OriginalURL)
	}

	if len(seedURLs) == 0 {
		msg := fmt.Sprintf("No valid seed URLs to scan from source: %s", summary.TargetSource)
		s.logger.Warn().Str("target_source", summary.TargetSource).Msg(msg)
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = []string{msg}
		return targets, seedURLs, summary, determinedTargetSource, common.NewValidationError("targets", len(seedURLs), msg)
	}

	return targets, seedURLs, summary, determinedTargetSource, nil
}

// executeScanForTargets executes the scan workflow for prepared targets
func (s *Scheduler) executeScanForTargets(ctx context.Context, seedURLs []string, summary models.ScanSummaryData, scanDBID int64, startTime time.Time) ([]models.ProbeResult, map[string]models.URLDiffResult, []models.SecretFinding, models.ScanSummaryData, error) {
	// Create log scan ID for detailed logging
	logScanID := summary.ScanSessionID
	if scanDBID > 0 {
		logScanID = fmt.Sprintf("%s (DB ID: %d)", summary.ScanSessionID, scanDBID)
	}

	s.logger.Info().Str("scan_id_log", logScanID).Int("num_targets", len(seedURLs)).Str("target_source", summary.TargetSource).Msg("Scheduler: Starting scan cycle execution.")

	// Execute the scan workflow
	probeResults, urlDiffResults, secretFindings, workflowErr := s.scanOrchestrator.ExecuteScanWorkflow(ctx, seedURLs, summary.ScanSessionID)
	scanDuration := time.Since(startTime)
	summary.ScanDuration = scanDuration

	if workflowErr != nil {
		s.logger.Error().Err(workflowErr).Str("scan_id_log", logScanID).Msg("Scheduler: Scan workflow execution failed.")
		if scanDBID > 0 {
			s.db.UpdateScanCompletion(scanDBID, time.Now(), "FAILED", workflowErr.Error(), 0, 0, 0, "")
		}
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Scan workflow failed: %v", workflowErr))
		return probeResults, urlDiffResults, secretFindings, summary, workflowErr
	}

	s.logger.Info().Str("scan_id_log", logScanID).Msg("Scheduler: Scan workflow completed successfully.")

	// Log secret findings summary
	if len(secretFindings) > 0 {
		s.logger.Info().Int("secret_findings_count", len(secretFindings)).Str("scan_id_log", logScanID).Msg("Secret detection found findings during scheduled scan")
	}

	return probeResults, urlDiffResults, secretFindings, summary, nil
}

// processScanResults processes scan results and generates reports
func (s *Scheduler) processScanResults(probeResults []models.ProbeResult, urlDiffResults map[string]models.URLDiffResult, secretFindings []models.SecretFinding, summary models.ScanSummaryData, scanDBID int64) (models.ScanSummaryData, string, error) {
	// Create log scan ID for detailed logging
	logScanID := summary.ScanSessionID
	if scanDBID > 0 {
		logScanID = fmt.Sprintf("%s (DB ID: %d)", summary.ScanSessionID, scanDBID)
	}

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
		for _, diffResult := range urlDiffResults {
			summary.DiffStats.New += diffResult.New
			summary.DiffStats.Old += diffResult.Old
			summary.DiffStats.Existing += diffResult.Existing
		}
	}

	// Generate report
	reportFilename := fmt.Sprintf("%s_automated_report.html", summary.ScanSessionID)
	reportPath := filepath.Join(s.globalConfig.ReporterConfig.OutputDir, reportFilename)

	// Convert probeResults to []*models.ProbeResult for reporter
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	err := s.generateReport(probeResultsPtr, urlDiffResults, secretFindings, reportPath)
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
		summary.ReportPath = ""
		reportPath = ""
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
		return common.WrapError(err, "failed to generate HTML report")
	}

	return nil
}
