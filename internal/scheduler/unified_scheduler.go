package scheduler

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/monitor"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/orchestrator"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/secrets"

	"github.com/rs/zerolog"
)

// UnifiedScheduler manages both scan and monitor operations in automated mode
type UnifiedScheduler struct {
	globalConfig       *config.GlobalConfig
	db                 *DB
	logger             zerolog.Logger
	urlFileOverride    string
	notificationHelper *notifier.NotificationHelper
	targetManager      *TargetManager
	scanOrchestrator   *orchestrator.ScanOrchestrator
	monitoringService  *monitor.MonitoringService
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.Mutex
	httpClient         *http.Client

	// Monitor scheduling fields (integrated from monitor.Scheduler)
	monitorWorkerChan chan monitorJob
	monitorWorkerWG   sync.WaitGroup
	monitorTicker     *time.Ticker
}

// monitorJob wraps a URL and a WaitGroup for a specific monitoring cycle.
type monitorJob struct {
	URL     string
	CycleWG *sync.WaitGroup
}

// NewUnifiedScheduler creates a new UnifiedScheduler instance
func NewUnifiedScheduler(
	cfg *config.GlobalConfig,
	urlFileOverride string,
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	secretDetector *secrets.SecretDetectorService,
	monitoringService *monitor.MonitoringService,
) (*UnifiedScheduler, error) {
	moduleLogger := logger.With().Str("module", "UnifiedScheduler").Logger()

	if cfg.SchedulerConfig.SQLiteDBPath == "" {
		moduleLogger.Error().Msg("SQLiteDBPath is not configured in SchedulerConfig")
		return nil, fmt.Errorf("sqliteDBPath is required for unified scheduler")
	}

	// Ensure the directory for SQLiteDBPath exists
	dbDir := filepath.Dir(cfg.SchedulerConfig.SQLiteDBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		moduleLogger.Error().Err(err).Str("path", dbDir).Msg("Failed to create directory for SQLite database")
		return nil, fmt.Errorf("failed to create directory for sqlite database '%s': %w", dbDir, err)
	}

	db, err := NewDB(cfg.SchedulerConfig.SQLiteDBPath, moduleLogger)
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to initialize unified scheduler database")
		return nil, fmt.Errorf("failed to initialize unified scheduler database: %w", err)
	}

	targetManager := NewTargetManager(moduleLogger)

	// Initialize ParquetReader & Writer for scan orchestrator
	parquetReader := datastore.NewParquetReader(&cfg.StorageConfig, logger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&cfg.StorageConfig, logger)
	if parquetErr != nil {
		moduleLogger.Warn().Err(parquetErr).Msg("Failed to initialize ParquetWriter for unified scheduler's orchestrator. Parquet writing might be disabled or limited.")
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(cfg, logger, parquetReader, parquetWriter, secretDetector)

	// Create HTTP client for content type detection
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	return &UnifiedScheduler{
		globalConfig:       cfg,
		db:                 db,
		logger:             moduleLogger,
		urlFileOverride:    urlFileOverride,
		notificationHelper: notificationHelper,
		targetManager:      targetManager,
		scanOrchestrator:   scanOrchestrator,
		monitoringService:  monitoringService,
		stopChan:           make(chan struct{}),
		httpClient:         httpClient,
	}, nil
}

// Start begins the unified scheduler's main loop
func (us *UnifiedScheduler) Start(ctx context.Context) error {
	us.mu.Lock()
	if us.isRunning {
		us.mu.Unlock()
		us.logger.Warn().Msg("UnifiedScheduler is already running.")
		return fmt.Errorf("unified scheduler is already running")
	}
	us.isRunning = true
	us.stopChan = make(chan struct{})
	us.mu.Unlock()

	us.logger.Info().Msg("UnifiedScheduler starting...")

	// Initialize monitor workers if monitoring service is available
	if us.monitoringService != nil {
		us.logger.Info().Msg("UnifiedScheduler: Monitoring service is available, starting monitor workers.")
		us.startMonitorWorkers()
	} else {
		us.logger.Warn().Msg("UnifiedScheduler: Monitoring service is not available, monitor workers will not be started.")
	}

	us.wg.Add(1)
	go func() {
		defer us.wg.Done()
		defer func() {
			us.mu.Lock()
			us.isRunning = false
			us.mu.Unlock()
			us.logger.Info().Msg("UnifiedScheduler has stopped main loop.")
		}()

		for {
			select {
			case <-ctx.Done():
				us.logger.Info().Msg("UnifiedScheduler stopping due to context cancellation.")
				if us.notificationHelper != nil {
					interruptionSummary := models.GetDefaultScanSummaryData()
					interruptionSummary.ScanSessionID = fmt.Sprintf("unified_scheduler_interrupted_%s", time.Now().Format("20060102-150405"))
					interruptionSummary.Status = string(models.ScanStatusInterrupted)
					interruptionSummary.ScanMode = "automated"
					interruptionSummary.ErrorMessages = []string{"Unified scheduler service was interrupted by context cancellation."}
					interruptionSummary.TargetSource = "UnifiedScheduler"
					us.logger.Info().Msg("Sending unified scheduler interruption notification from Start() due to context cancellation.")
					// Send to both scan and monitor webhooks using unified interrupt notifications
					us.notificationHelper.SendScanInterruptNotification(context.Background(), interruptionSummary)
					us.notificationHelper.SendMonitorInterruptNotification(context.Background(), interruptionSummary)
				}
				return
			case <-us.stopChan:
				us.logger.Info().Msg("UnifiedScheduler stopping due to explicit Stop() call.")
				return
			default:
				// Determine next scan time
				nextScanTime, err := us.calculateNextScanTime()
				if err != nil {
					us.logger.Error().Err(err).Msg("Failed to calculate next scan time. Retrying after 1 minute.")
					time.Sleep(1 * time.Minute)
					continue
				}

				now := time.Now()
				if now.Before(nextScanTime) {
					sleepDuration := nextScanTime.Sub(now)
					us.logger.Info().Time("next_scan_at", nextScanTime).Dur("sleep_duration", sleepDuration).Msg("UnifiedScheduler waiting for next scan cycle.")

					select {
					case <-time.After(sleepDuration):
						// Continue to scan
					case <-us.stopChan:
						us.logger.Info().Msg("UnifiedScheduler stopped during sleep period.")
						return
					case <-ctx.Done():
						us.logger.Info().Msg("UnifiedScheduler context cancelled during sleep period.")
						if us.notificationHelper != nil {
							interruptionSummary := models.GetDefaultScanSummaryData()
							interruptionSummary.ScanSessionID = fmt.Sprintf("unified_scheduler_interrupted_sleep_%s", time.Now().Format("20060102-150405"))
							interruptionSummary.Status = string(models.ScanStatusInterrupted)
							interruptionSummary.ScanMode = "automated"
							interruptionSummary.ErrorMessages = []string{"Unified scheduler service was interrupted during sleep period by context cancellation."}
							interruptionSummary.TargetSource = "UnifiedScheduler"
							us.logger.Info().Msg("Sending unified scheduler interruption notification from Start() due to context cancellation during sleep.")
							// Send to both scan and monitor webhooks using unified interrupt notifications
							us.notificationHelper.SendScanInterruptNotification(context.Background(), interruptionSummary)
							us.notificationHelper.SendMonitorInterruptNotification(context.Background(), interruptionSummary)
						}
						return
					}
				}
				us.logger.Info().Msg("UnifiedScheduler starting new unified cycle.")
				us.runUnifiedCycleWithRetries(ctx)
			}
		}
	}()

	us.logger.Info().Msg("UnifiedScheduler main loop goroutine started.")
	us.wg.Wait()
	us.logger.Info().Msg("UnifiedScheduler Start method is returning as the main loop has finished.")

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// Stop gracefully stops the unified scheduler
func (us *UnifiedScheduler) Stop() {
	us.mu.Lock()
	if !us.isRunning {
		us.mu.Unlock()
		us.logger.Info().Msg("UnifiedScheduler is not running, no action needed for Stop().")
		return
	}

	us.logger.Info().Msg("UnifiedScheduler Stop() called, attempting to stop gracefully...")
	if us.stopChan != nil {
		select {
		case _, ok := <-us.stopChan:
			if !ok {
				us.logger.Info().Msg("stopChan was already closed.")
			}
		default:
			close(us.stopChan)
			us.logger.Info().Msg("stopChan successfully closed.")
		}
	}
	us.mu.Unlock()

	// Stop monitor workers and ticker
	if us.monitoringService != nil {
		us.logger.Info().Msg("Stopping monitor workers and ticker...")
		if us.monitorTicker != nil {
			us.monitorTicker.Stop()
		}
		if us.monitorWorkerChan != nil {
			close(us.monitorWorkerChan)
		}
		us.monitorWorkerWG.Wait()
		us.logger.Info().Msg("Monitor workers and ticker stopped.")
	}

	us.logger.Info().Msg("Waiting for unified scheduler's main goroutine to complete...")
	us.wg.Wait()

	us.mu.Lock()
	us.isRunning = false
	us.logger.Info().Msg("UnifiedScheduler main goroutine confirmed finished.")

	// Close database connection
	if us.db != nil {
		us.logger.Info().Msg("Closing unified scheduler database connection...")
		if err := us.db.Close(); err != nil {
			us.logger.Error().Err(err).Msg("Error closing unified scheduler database")
		} else {
			us.logger.Info().Msg("UnifiedScheduler database closed successfully.")
		}
		us.db = nil
	}
	us.mu.Unlock()

	us.logger.Info().Msg("UnifiedScheduler has been stopped and resources cleaned up.")
}

// calculateNextScanTime determines when the next scan should run
func (us *UnifiedScheduler) calculateNextScanTime() (time.Time, error) {
	lastScanTime, err := us.db.GetLastScanTime()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			us.logger.Info().Msg("No previous completed scan found in history. Scheduling next scan immediately.")
			return time.Now(), nil
		}
		return time.Time{}, err
	}

	intervalDuration := time.Duration(us.globalConfig.SchedulerConfig.CycleMinutes) * time.Minute
	nextScanTime := lastScanTime.Add(intervalDuration)

	if nextScanTime.Before(time.Now()) {
		return time.Now(), nil
	}

	return nextScanTime, nil
}

// runUnifiedCycleWithRetries runs a unified cycle with retry logic
func (us *UnifiedScheduler) runUnifiedCycleWithRetries(ctx context.Context) {
	maxRetries := us.globalConfig.SchedulerConfig.RetryAttempts
	retryDelay := 5 * time.Minute

	var lastErr error
	var scanSummary models.ScanSummaryData
	currentScanSessionID := time.Now().Format("20060102-150405")

	// Determine TargetSource once for this cycle
	_, initialTargetSource, initialTargetsErr := us.targetManager.LoadAndSelectTargets(
		us.urlFileOverride,
		us.globalConfig.InputConfig.InputURLs,
		us.globalConfig.InputConfig.InputFile,
	)
	if initialTargetsErr != nil {
		us.logger.Error().Err(initialTargetsErr).Msg("UnifiedScheduler: Failed to determine initial target source for unified cycle. Notifications might be affected.")
		initialTargetSource = "ErrorDeterminingSource"
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		scanSummary = models.GetDefaultScanSummaryData()
		scanSummary.ScanSessionID = currentScanSessionID
		scanSummary.ScanMode = "automated"
		scanSummary.TargetSource = initialTargetSource
		scanSummary.RetriesAttempted = attempt

		select {
		case <-ctx.Done():
			us.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("UnifiedScheduler: Context cancelled before retry attempt. Stopping retry loop.")
			if lastErr != nil && attempt > 0 {
				scanSummary.Status = string(models.ScanStatusInterrupted)
				if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
					scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Unified cycle aborted during retries due to context cancellation. Last error: %v", lastErr))
				}
				us.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
			}
			return
		default:
		}

		if attempt > 0 {
			us.logger.Info().Int("attempt", attempt).Int("max_retries", maxRetries).Dur("delay", retryDelay).Msg("UnifiedScheduler: Retrying unified cycle after delay...")
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				us.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("UnifiedScheduler: Context cancelled during retry delay. Stopping retry loop.")
				if lastErr != nil {
					scanSummary.Status = string(models.ScanStatusInterrupted)
					if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
						scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Unified cycle aborted during retry delay due to context cancellation. Last error: %v", lastErr))
					}
					us.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
				}
				return
			}
		}

		var currentAttemptSummary models.ScanSummaryData
		var reportFilePaths []string
		currentAttemptSummary, reportFilePaths, lastErr = us.runUnifiedCycle(ctx, currentScanSessionID, initialTargetSource)

		// Merge results from runUnifiedCycle
		scanSummary.Targets = currentAttemptSummary.Targets
		scanSummary.TotalTargets = currentAttemptSummary.TotalTargets
		scanSummary.ProbeStats = currentAttemptSummary.ProbeStats
		scanSummary.DiffStats = currentAttemptSummary.DiffStats
		scanSummary.SecretStats = currentAttemptSummary.SecretStats
		scanSummary.ScanDuration = currentAttemptSummary.ScanDuration
		scanSummary.ReportPath = currentAttemptSummary.ReportPath
		scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, currentAttemptSummary.ErrorMessages...)

		if lastErr == nil {
			us.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("UnifiedScheduler: Unified cycle completed successfully.")
			scanSummary.Status = string(models.ScanStatusCompleted)
			us.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary, notifier.ScanServiceNotification, reportFilePaths)
			return
		}

		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			us.logger.Info().Str("scan_session_id", currentScanSessionID).Err(lastErr).Msg("UnifiedScheduler: Unified cycle interrupted by context cancellation. No further retries.")
			scanSummary.Status = string(models.ScanStatusInterrupted)
			if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
				scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("Unified cycle interrupted: %v", lastErr))
			}

			// Send interrupt notification to both scan and monitor webhooks using unified interrupt notifications
			us.logger.Info().Msg("UnifiedScheduler: Sending interrupt notification to both scan and monitor services.")
			us.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
			us.notificationHelper.SendMonitorInterruptNotification(context.Background(), scanSummary)
			return
		}

		us.logger.Error().Err(lastErr).Str("scan_session_id", currentScanSessionID).Int("attempt", attempt+1).Int("total_attempts", maxRetries+1).Msg("UnifiedScheduler: Unified cycle failed")

		if attempt == maxRetries {
			us.logger.Error().Str("scan_session_id", currentScanSessionID).Msg("UnifiedScheduler: All retry attempts exhausted. Unified cycle failed permanently.")
			scanSummary.Status = string(models.ScanStatusFailed)
			if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
				scanSummary.ErrorMessages = append(scanSummary.ErrorMessages, fmt.Sprintf("All %d retry attempts failed. Last error: %v", maxRetries+1, lastErr))
			}
			us.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
		}
	}
}

// runUnifiedCycle executes a complete unified cycle
func (us *UnifiedScheduler) runUnifiedCycle(ctx context.Context, scanSessionID string, predeterminedTargetSource string) (models.ScanSummaryData, []string, error) {
	startTime := time.Now()
	summary := models.GetDefaultScanSummaryData()
	summary.ScanSessionID = scanSessionID
	summary.ScanMode = "automated"
	summary.TargetSource = predeterminedTargetSource
	var generatedReportPaths []string // To store paths of generated reports

	// Load targets
	targets, determinedTargetSource, err := us.targetManager.LoadAndSelectTargets(
		us.urlFileOverride,
		us.globalConfig.InputConfig.InputURLs,
		us.globalConfig.InputConfig.InputFile,
	)
	if err != nil {
		us.logger.Error().Err(err).Msg("UnifiedScheduler: Failed to load targets for unified cycle.")
		return summary, nil, fmt.Errorf("failed to load targets: %w", err)
	}
	if determinedTargetSource == "" {
		determinedTargetSource = "UnknownSource"
	}
	summary.TargetSource = determinedTargetSource

	if len(targets) == 0 {
		us.logger.Info().Str("source", determinedTargetSource).Msg("UnifiedScheduler: No targets to process in this unified cycle.")
		summary.Status = string(models.ScanStatusNoTargets)
		summary.ErrorMessages = []string{"No targets loaded for this unified cycle."}
		return summary, nil, nil
	}

	// Convert targets to string slice for compatibility
	targetURLs := make([]string, len(targets))
	for i, target := range targets {
		targetURLs[i] = target.NormalizedURL
	}

	// Classify URLs by content type BEFORE setting summary.Targets
	htmlURLs, monitorURLs, err := us.classifyURLsByContentType(ctx, targetURLs)
	if err != nil {
		us.logger.Error().Err(err).Msg("UnifiedScheduler: Failed to classify URLs by content type.")
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Failed to classify URLs: %v", err))
		// Continue with empty classifications to avoid complete failure
	}

	us.logger.Info().Int("html_urls", len(htmlURLs)).Int("monitor_urls", len(monitorURLs)).Int("total_targets", len(targets)).Msg("UnifiedScheduler: URL classification completed.")

	// Set summary targets to only HTML URLs that will be scanned
	summary.Targets = htmlURLs
	summary.TotalTargets = len(htmlURLs)

	// Send scan start notification BEFORE starting any work (only for HTML URLs)
	if us.notificationHelper != nil && len(htmlURLs) > 0 {
		startNotificationSummary := summary
		startNotificationSummary.Status = string(models.ScanStatusStarted)
		us.logger.Info().Str("session_id", scanSessionID).Int("html_target_count", len(htmlURLs)).Msg("UnifiedScheduler: Sending scan start notification for HTML URLs.")
		us.notificationHelper.SendScanStartNotification(ctx, startNotificationSummary)
	}

	// Record scan start in database
	dbScanID, err := us.db.RecordScanStart(scanSessionID, determinedTargetSource, len(htmlURLs), startTime)
	if err != nil {
		us.logger.Error().Err(err).Msg("UnifiedScheduler: Failed to record scan start in database.")
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Database recording error: %v", err))
	}

	// Start monitor service immediately with URLs (run in parallel)
	var monitorWG sync.WaitGroup
	if len(monitorURLs) > 0 && us.monitoringService != nil {
		// Send monitor start notification
		if us.notificationHelper != nil {
			monitorStartSummary := models.GetDefaultScanSummaryData()
			monitorStartSummary.ScanSessionID = scanSessionID
			monitorStartSummary.Component = "MonitoringService"
			monitorStartSummary.ScanMode = "automated"
			monitorStartSummary.TargetSource = determinedTargetSource
			monitorStartSummary.Targets = monitorURLs
			monitorStartSummary.TotalTargets = len(monitorURLs)
			monitorStartSummary.Status = string(models.ScanStatusStarted)
			us.logger.Info().Str("session_id", scanSessionID).Int("monitor_target_count", len(monitorURLs)).Msg("UnifiedScheduler: Sending monitor start notification.")
			us.notificationHelper.SendMonitorStartNotification(ctx, monitorStartSummary)
		}

		monitorWG.Add(1)
		go func() {
			defer monitorWG.Done()

			// Check if context is cancelled before starting
			select {
			case <-ctx.Done():
				us.logger.Info().Msg("UnifiedScheduler: Monitor setup cancelled due to context cancellation.")
				return
			default:
			}

			us.logger.Info().Int("monitor_count", len(monitorURLs)).Msg("UnifiedScheduler: Adding URLs to monitoring service (parallel).")
			for _, url := range monitorURLs {
				select {
				case <-ctx.Done():
					us.logger.Info().Str("url", url).Msg("UnifiedScheduler: Monitor URL addition cancelled due to context cancellation.")
					return
				default:
					us.monitoringService.AddTargetURL(url)
				}
			}
			us.logger.Info().Msg("UnifiedScheduler: URLs added to monitoring service successfully.")

			// Check for cancellation before triggering monitor cycle
			select {
			case <-ctx.Done():
				us.logger.Info().Msg("UnifiedScheduler: Monitor cycle trigger cancelled due to context cancellation.")
				return
			default:
			}

			// Trigger an immediate monitor cycle after adding URLs
			us.logger.Info().Msg("UnifiedScheduler: Triggering immediate monitor cycle after adding URLs.")
			us.performMonitorCycle("post-scan")
		}()
	} else if us.monitoringService == nil {
		us.logger.Warn().Msg("UnifiedScheduler: Monitoring service is not available, skipping monitor workflow.")
	} else if len(monitorURLs) == 0 {
		us.logger.Info().Msg("UnifiedScheduler: No monitor URLs to add to monitoring service.")
	}

	// Execute scan workflow for HTML URLs (run in parallel with monitor)
	var scanWorkflowSummary models.ScanSummaryData // Renamed to avoid conflict with outer summary
	var probeResults []models.ProbeResult
	var urlDiffResults map[string]models.URLDiffResult
	var secretFindings []models.SecretFinding

	if len(htmlURLs) > 0 {
		us.logger.Info().Int("html_count", len(htmlURLs)).Msg("UnifiedScheduler: Executing scan workflow for HTML URLs (parallel).")
		var workflowErr error

		scanWorkflowSummary, probeResults, urlDiffResults, secretFindings, workflowErr = us.scanOrchestrator.ExecuteCompleteScanWorkflow(ctx, htmlURLs, scanSessionID, determinedTargetSource)
		if workflowErr != nil {
			us.logger.Error().Err(workflowErr).Msg("UnifiedScheduler: Scan workflow failed for HTML URLs.")
			summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Scan workflow error: %v", workflowErr))
			if errors.Is(workflowErr, context.Canceled) {
				// Send interrupt notification to both webhooks when scan workflow is cancelled
				us.logger.Info().Msg("UnifiedScheduler: Scan workflow cancelled, sending interrupt notification to both services.")
				summary.Status = string(models.ScanStatusInterrupted)
				us.notificationHelper.SendScanInterruptNotification(context.Background(), summary)
				us.notificationHelper.SendMonitorInterruptNotification(context.Background(), summary)
				return summary, nil, workflowErr // Return nil for report paths
			}
		} else {
			us.logger.Info().Int("probe_results", len(probeResults)).Int("diff_results", len(urlDiffResults)).Int("secret_findings", len(secretFindings)).Msg("UnifiedScheduler: Scan workflow completed successfully.")
		}

		// Merge scan results into summary
		summary.ProbeStats = scanWorkflowSummary.ProbeStats
		summary.DiffStats = scanWorkflowSummary.DiffStats
		summary.SecretStats = scanWorkflowSummary.SecretStats
		summary.ErrorMessages = append(summary.ErrorMessages, scanWorkflowSummary.ErrorMessages...)

		// Generate HTML report (always generate, even if no probe results)
		generatedReportPaths = us.generateAndSetReport(ctx, scanSessionID, probeResults, secretFindings, &summary)
	}

	// Wait for monitor service to complete adding URLs and initial cycle
	us.logger.Info().Msg("UnifiedScheduler: Waiting for monitor service to complete initial setup.")

	// Use a channel to wait for monitor completion with context cancellation support
	monitorDone := make(chan struct{})
	go func() {
		monitorWG.Wait()
		close(monitorDone)
	}()

	select {
	case <-monitorDone:
		us.logger.Info().Msg("UnifiedScheduler: Monitor service initial setup completed.")
	case <-ctx.Done():
		us.logger.Info().Msg("UnifiedScheduler: Context cancelled while waiting for monitor service setup.")
		// Send interrupt notification to both services
		summary.Status = string(models.ScanStatusInterrupted)
		if !common.ContainsCancellationError(summary.ErrorMessages) {
			summary.ErrorMessages = append(summary.ErrorMessages, "Unified cycle interrupted during monitor setup")
		}
		us.notificationHelper.SendScanInterruptNotification(context.Background(), summary)
		us.notificationHelper.SendMonitorInterruptNotification(context.Background(), summary)
		return summary, nil, ctx.Err() // Return nil for report paths
	}

	// Update scan completion in database
	endTime := time.Now()
	status := "COMPLETED"
	if len(summary.ErrorMessages) > 0 {
		status = "PARTIAL_COMPLETE"
	}

	logSummary := fmt.Sprintf("Total Targets: %d, HTML URLs (scanned): %d, Monitor URLs: %d, Errors: %d",
		len(targetURLs), len(htmlURLs), len(monitorURLs), len(summary.ErrorMessages))

	if dbScanID > 0 {
		if err := us.db.UpdateScanCompletion(dbScanID, endTime, status, logSummary,
			summary.DiffStats.New, summary.DiffStats.Old, summary.DiffStats.Existing, summary.ReportPath); err != nil {
			us.logger.Error().Err(err).Msg("UnifiedScheduler: Failed to update scan completion in database.")
			summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Database update error: %v", err))
		}
	}

	summary.ScanDuration = time.Since(startTime)
	if len(summary.ErrorMessages) == 0 {
		summary.Status = string(models.ScanStatusCompleted)
	} else {
		summary.Status = string(models.ScanStatusPartialComplete)
	}

	us.logger.Info().Str("session_id", scanSessionID).Dur("duration", summary.ScanDuration).Msg("UnifiedScheduler: Unified cycle completed.")
	return summary, generatedReportPaths, nil
}

// classifyURLsByContentType classifies URLs based on their content type
func (us *UnifiedScheduler) classifyURLsByContentType(ctx context.Context, urls []string) (htmlURLs []string, monitorURLs []string, err error) {
	us.logger.Info().Int("total_urls", len(urls)).Msg("UnifiedScheduler: Starting URL classification by content type.")

	// Monitor all URLs regardless of content type
	monitorURLs = make([]string, len(urls))
	copy(monitorURLs, urls)

	// For scan service, detect content type and only include HTML URLs
	for _, url := range urls {
		select {
		case <-ctx.Done():
			return htmlURLs, monitorURLs, ctx.Err()
		default:
		}

		contentType, err := us.detectContentType(ctx, url)
		if err != nil {
			us.logger.Warn().Err(err).Str("url", url).Msg("UnifiedScheduler: Failed to detect content type, defaulting to HTML for scan.")
			htmlURLs = append(htmlURLs, url)
			continue
		}

		if us.isHTMLContent(contentType) {
			htmlURLs = append(htmlURLs, url)
		} else {
			monitorURLs = append(monitorURLs, url)
		}
	}

	us.logger.Info().Int("html_urls", len(htmlURLs)).Int("monitor_urls", len(monitorURLs)).Msg("UnifiedScheduler: URL classification completed.")
	return htmlURLs, monitorURLs, nil
}

// detectContentType detects the content type of a URL
func (us *UnifiedScheduler) detectContentType(ctx context.Context, url string) (string, error) {
	// First, try to infer from URL extension
	if contentType := us.inferContentTypeFromURL(url); contentType != "" {
		return contentType, nil
	}

	// If URL-based detection fails, make a HEAD request
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HEAD request: %w", err)
	}

	resp, err := us.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute HEAD request: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return "text/html", nil // Default to HTML if no content type header
	}

	// Parse media type to remove parameters
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType, nil // Return raw content type if parsing fails
	}

	return mediaType, nil
}

// inferContentTypeFromURL infers content type from URL path/extension
func (us *UnifiedScheduler) inferContentTypeFromURL(url string) string {
	lowerURL := strings.ToLower(url)

	// Check for common file extensions
	if strings.HasSuffix(lowerURL, ".js") {
		return "application/javascript"
	}
	if strings.HasSuffix(lowerURL, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(lowerURL, ".css") {
		return "text/css"
	}
	if strings.HasSuffix(lowerURL, ".html") || strings.HasSuffix(lowerURL, ".htm") {
		return "text/html"
	}
	if strings.HasSuffix(lowerURL, ".xml") {
		return "application/xml"
	}
	if strings.HasSuffix(lowerURL, ".txt") {
		return "text/plain"
	}

	// Check for API endpoints that typically return JSON
	if strings.Contains(lowerURL, "/api/") || strings.Contains(lowerURL, "/v1/") || strings.Contains(lowerURL, "/v2/") {
		return "application/json"
	}

	return "" // Unknown, will require HTTP request
}

// isHTMLContent checks if content type is HTML
func (us *UnifiedScheduler) isHTMLContent(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/html")
}

// startMonitorWorkers initializes and starts monitor workers
func (us *UnifiedScheduler) startMonitorWorkers() {
	us.logger.Info().Msg("UnifiedScheduler: startMonitorWorkers called.")

	if us.globalConfig.MonitorConfig.CheckIntervalSeconds <= 0 {
		us.logger.Warn().Int("configured_interval", us.globalConfig.MonitorConfig.CheckIntervalSeconds).Msg("Monitor CheckIntervalSeconds is not configured or invalid, monitor scheduling disabled.")
		return
	}

	// Initialize monitor worker channel
	numWorkers := us.globalConfig.MonitorConfig.MaxConcurrentChecks
	if numWorkers <= 0 {
		numWorkers = 1
		us.logger.Warn().Int("configured_workers", numWorkers).Msg("MaxConcurrentChecks is not configured or invalid, defaulting to 1 worker.")
	}
	us.monitorWorkerChan = make(chan monitorJob, numWorkers)

	// Start monitor workers
	us.logger.Info().Int("num_workers", numWorkers).Msg("Starting monitor workers")
	for i := 0; i < numWorkers; i++ {
		us.monitorWorkerWG.Add(1)
		go us.monitorWorker(i)
	}

	// Start monitor ticker
	intervalDuration := time.Duration(us.globalConfig.MonitorConfig.CheckIntervalSeconds) * time.Second
	us.logger.Info().Dur("interval", intervalDuration).Msg("Starting monitor ticker.")
	us.monitorTicker = time.NewTicker(intervalDuration)
	us.monitorWorkerWG.Add(1)
	go func() {
		defer us.monitorWorkerWG.Done()
		defer us.monitorTicker.Stop()

		// Perform initial check
		us.logger.Info().Msg("Performing initial monitor cycle.")
		us.performMonitorCycle("initial")

		for {
			select {
			case <-us.monitorTicker.C:
				us.performMonitorCycle("periodic")
			case <-us.stopChan:
				us.logger.Info().Msg("Monitor ticker stopping due to stop signal.")
				return
			}
		}
	}()

	us.logger.Info().Msg("Monitor workers and ticker started successfully.")
}

// monitorWorker processes monitor jobs
func (us *UnifiedScheduler) monitorWorker(id int) {
	defer us.monitorWorkerWG.Done()
	us.logger.Info().Int("worker_id", id).Msg("Monitor worker started")

	for {
		select {
		case job, ok := <-us.monitorWorkerChan:
			if !ok {
				us.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping as channel closed.")
				return
			}
			us.monitoringService.CheckURL(job.URL)
			job.CycleWG.Done()
		case <-us.stopChan:
			us.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping due to stop signal.")
			return
		}
	}
}

// performMonitorCycle performs a monitoring cycle for all monitored URLs
func (us *UnifiedScheduler) performMonitorCycle(cycleType string) {
	targetsToCheck := us.monitoringService.GetCurrentlyMonitoredURLs()
	us.logger.Info().Str("cycle_type", cycleType).Int("count", len(targetsToCheck)).Msg("UnifiedScheduler: Performing monitor cycle")

	if len(targetsToCheck) == 0 {
		us.logger.Info().Str("cycle_type", cycleType).Msg("UnifiedScheduler: No targets to check in this monitor cycle. Skipping cycle end report.")
		return
	}

	us.logger.Info().Str("cycle_type", cycleType).Int("targets", len(targetsToCheck)).Msg("UnifiedScheduler: Starting monitor cycle with targets.")

	var cycleWG sync.WaitGroup
	cycleWG.Add(len(targetsToCheck))

	for _, url := range targetsToCheck {
		select {
		case us.monitorWorkerChan <- monitorJob{URL: url, CycleWG: &cycleWG}:
		case <-us.stopChan:
			us.logger.Info().Str("url", url).Msg("Stop signal received during job submission for monitor cycle.")
			return
		}
	}

	us.logger.Info().Str("cycle_type", cycleType).Msg("UnifiedScheduler: Waiting for all monitor jobs to complete.")
	cycleWG.Wait() // Wait for all checkURL calls in this cycle to complete

	select {
	case <-us.stopChan:
		us.logger.Info().Msg("Stop signal received during monitor cycle processing, report not triggered.")
	default:
		us.logger.Info().Str("cycle_type", cycleType).Int("targets_processed", len(targetsToCheck)).Msg("All checks for the monitor cycle completed. Triggering report.")
		us.monitoringService.TriggerCycleEndReport()
	}
}

// generateAndSetReport generates and sets the HTML report for a scan
func (us *UnifiedScheduler) generateAndSetReport(ctx context.Context, scanSessionID string, probeResults []models.ProbeResult, secretFindings []models.SecretFinding, summary *models.ScanSummaryData) []string {
	us.logger.Info().Int("probe_results", len(probeResults)).Int("secret_findings", len(secretFindings)).Msg("UnifiedScheduler: Generating HTML report...")

	// Ensure output directory exists
	if us.globalConfig.ReporterConfig.OutputDir == "" {
		us.logger.Warn().Msg("UnifiedScheduler: ReporterConfig.OutputDir is empty, using default 'reports' directory")
		us.globalConfig.ReporterConfig.OutputDir = "reports"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(us.globalConfig.ReporterConfig.OutputDir, 0755); err != nil {
		us.logger.Error().Err(err).Str("output_dir", us.globalConfig.ReporterConfig.OutputDir).Msg("UnifiedScheduler: Failed to create output directory")
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Failed to create output directory: %v", err))
		return nil // Return nil for report paths on error
	}

	htmlReporter, err := reporter.NewHtmlReporter(&us.globalConfig.ReporterConfig, us.logger)
	if err != nil {
		us.logger.Error().Err(err).Msg("UnifiedScheduler: Failed to initialize HTML reporter")
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		return nil // Return nil for report paths on error
	}

	baseReportFilename := fmt.Sprintf("scheduled_scan_%s_report.html", scanSessionID)
	baseReportPath := filepath.Join(us.globalConfig.ReporterConfig.OutputDir, baseReportFilename)
	us.logger.Info().Str("base_report_path", baseReportPath).Msg("UnifiedScheduler: Base report path set.")

	// Convert []models.ProbeResult to []*models.ProbeResult
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	// Generate report
	reportFilePaths, reportGenErr := htmlReporter.GenerateReport(probeResultsPtr, secretFindings, baseReportPath)
	if reportGenErr != nil {
		us.logger.Error().Err(reportGenErr).Msg("Failed to generate HTML report(s) in unified scheduler cycle")
		summary.Status = string(models.ScanStatusFailed)
		summary.ErrorMessages = append(summary.ErrorMessages, fmt.Sprintf("Failed to generate HTML report(s): %v", reportGenErr))
		// Even if report generation fails, reportFilePaths might contain paths to partially generated files (or be nil)
		// We still return it so the caller can decide how to handle notification.
		return nil // Return nil for report paths on error
	}

	// Check if report file was actually created
	if len(reportFilePaths) == 0 {
		us.logger.Info().Msg("UnifiedScheduler: HTML report generation resulted in no files (e.g., no data and generate_empty_report is false). Scheduled scan report will be empty.")
		summary.ReportPath = "" // Clear report path since no file was created
	} else {
		us.logger.Info().Strs("report_paths", reportFilePaths).Msg("UnifiedScheduler: HTML report(s) generated successfully for scheduled scan.")
		// Set summary.ReportPath to the first part, or a general message if multiple parts
		if len(reportFilePaths) == 1 {
			summary.ReportPath = reportFilePaths[0]
		} else {
			summary.ReportPath = fmt.Sprintf("Multiple report files generated (%d parts), see notifications.", len(reportFilePaths))
		}
	}
	return reportFilePaths // Return all generated paths
}
