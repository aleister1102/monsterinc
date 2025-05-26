package scheduler

import (
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"monsterinc/internal/notification"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Scheduler manages periodic scan operations in automated mode
type Scheduler struct {
	globalConfig       *config.GlobalConfig
	db                 *DB
	logger             *log.Logger
	urlFileOverride    string // From -urlfile command line flag
	notificationHelper *notification.NotificationHelper
	targetManager      *TargetManager
	scanOrchestrator   *orchestrator.ScanOrchestrator
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.Mutex
}

// NewScheduler creates a new Scheduler instance
func NewScheduler(cfg *config.GlobalConfig, urlFileOverride string, logger *log.Logger) (*Scheduler, error) {
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
		logger.Printf("[WARN] Scheduler: Failed to initialize ParquetWriter for orchestrator: %v. Parquet storage will be disabled.", err)
		parquetWriter = nil
	}

	// Initialize NotificationHelper
	notificationHelper := notification.NewNotificationHelper(&cfg.NotificationConfig, logger)

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

	s.logger.Println("[INFO] Scheduler: Starting automated scan scheduler...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run initial scan immediately
	s.logger.Println("[INFO] Scheduler: Running initial scan...")
	s.runScanCycleWithRetries()

	// Main scheduler loop
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			// Calculate next scan time
			nextScanTime, err := s.calculateNextScanTime()
			if err != nil {
				s.logger.Printf("[ERROR] Scheduler: Failed to calculate next scan time: %v", err)
				// Wait a bit before retrying
				time.Sleep(5 * time.Minute)
				continue
			}

			s.logger.Printf("[INFO] Scheduler: Next scan scheduled for: %v", nextScanTime)

			// Wait until next scan time or stop signal
			select {
			case <-time.After(time.Until(nextScanTime)):
				s.logger.Println("[INFO] Scheduler: Starting scheduled scan...")
				s.runScanCycleWithRetries()
			case <-s.stopChan:
				s.logger.Println("[INFO] Scheduler: Received stop signal, exiting scheduler loop...")
				return
			case sig := <-sigChan:
				s.logger.Printf("[INFO] Scheduler: Received signal %v, initiating graceful shutdown...", sig)
				close(s.stopChan)
				return
			}
		}
	}()

	// Wait for scheduler to stop
	s.wg.Wait()
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.logger.Println("[INFO] Scheduler: Stopping scheduler...")
	close(s.stopChan)
	s.wg.Wait()
	s.isRunning = false

	// Close database connection
	if s.db != nil {
		s.db.Close()
	}

	s.logger.Println("[INFO] Scheduler: Scheduler stopped.")
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
func (s *Scheduler) runScanCycleWithRetries() {
	maxRetries := s.globalConfig.SchedulerConfig.RetryAttempts
	retryDelay := 5 * time.Minute // Fixed retry delay

	var lastErr error
	var targetSource string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Printf("[INFO] Scheduler: Retry attempt %d/%d after %v delay...", attempt, maxRetries, retryDelay)
			time.Sleep(retryDelay)
		}

		targetSource, lastErr = s.runScanCycle()
		if lastErr == nil {
			// Success
			s.logger.Println("[INFO] Scheduler: Scan cycle completed successfully.")
			return
		}

		s.logger.Printf("[ERROR] Scheduler: Scan cycle failed (attempt %d/%d): %v", attempt+1, maxRetries+1, lastErr)

		if attempt == maxRetries {
			// All retries exhausted
			s.logger.Printf("[ERROR] Scheduler: All retry attempts exhausted. Scan cycle failed.")
			// Send failure notification
			if err := s.notificationHelper.SendScanFailureNotification(targetSource, lastErr, maxRetries+1); err != nil {
				s.logger.Printf("[ERROR] Scheduler: Failed to send failure notification: %v", err)
			}
		}
	}
}

// runScanCycle executes a complete scan cycle
func (s *Scheduler) runScanCycle() (string, error) {
	startTime := time.Now() // Ensure startTime is declared and used

	// Load targets using TargetManager
	targets, targetSource, err := s.targetManager.LoadAndSelectTargets(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if err != nil {
		return "", fmt.Errorf("failed to load targets: %w", err)
	}

	// Extract original URLs for crawler and other parts that expect []string
	var seedURLs []string
	for _, target := range targets {
		seedURLs = append(seedURLs, target.OriginalURL)
	}

	if len(seedURLs) == 0 {
		return targetSource, fmt.Errorf("no valid seed URLs to scan from source: %s", targetSource)
	}

	// Send scan start notification
	if err := s.notificationHelper.SendScanStartNotification(targetSource); err != nil {
		s.logger.Printf("[ERROR] Scheduler: Failed to send start notification: %v", err)
	}

	// Record scan start in database
	scanID, err := s.db.RecordScanStart(time.Now(), targetSource) // Added time.Now()
	if err != nil {
		// If we can't even record the scan start, it's a critical failure for this cycle.
		// Log it, but don't necessarily stop the scheduler from trying future cycles.
		s.logger.Printf("[ERROR] Scheduler: Failed to record scan start in DB for source '%s': %v. This cycle will be aborted.", targetSource, err)
		return targetSource, fmt.Errorf("failed to record scan start in DB: %w", err)
	}
	s.logger.Printf("[INFO] Scheduler: Scan cycle initiated with ID: %d for target source: %s", scanID, targetSource)

	// Execute the main scan workflow (crawl, probe, diff)
	// probeResults, urlDiffResults, err := s.scanOrchestrator.ExecuteScanWorkflow(targets, scanSessionID) // Original line with error
	probeResults, urlDiffResults, err := s.scanOrchestrator.ExecuteScanWorkflow(seedURLs, fmt.Sprintf("scan-%d", scanID)) // Use seedURLs and a session ID based on scanID
	if err != nil {
		s.logger.Printf("[ERROR] Scheduler: Scan workflow failed for source '%s' (scanID %d): %v", targetSource, scanID, err)
		// s.db.RecordScanEnd(scanID, time.Now(), models.ScanStatusFailed, "", fmt.Sprintf("Workflow error: %v", err)) // Original line with error
		s.db.UpdateScanCompletion(scanID, time.Now(), "FAILED", "", fmt.Sprintf("Workflow error: %v", err)) // Corrected
		return targetSource, fmt.Errorf("scan workflow execution failed: %w", err)
	}

	s.logger.Printf("[INFO] Scheduler: Scan workflow completed for source '%s' (scanID %d). Probe results: %d, Diff sets: %d", targetSource, scanID, len(probeResults), len(urlDiffResults))

	// Generate report (if any results)
	reportPath := ""
	if len(probeResults) > 0 || len(urlDiffResults) > 0 {
		// Construct a meaningful report path
		// Example: reports/scan-YYYYMMDD-HHMMSS-targetSource.json (or .html, .pdf)
		// For now, using a simple name related to the scanID
		ts := time.Now().Format("20060102-150405")
		// safeTargetSource := SanitizeFilename(targetSource) // Assuming SanitizeFilename exists or will be added
		reportFilename := fmt.Sprintf("%s_automated_report.html", ts)

		// Ensure reports directory exists (using global config)
		reportsDir := s.globalConfig.ReporterConfig.OutputDir // Corrected to OutputDir
		if reportsDir == "" {
			reportsDir = "reports" // Default if not configured
		}
		if err := os.MkdirAll(reportsDir, 0755); err != nil {
			s.logger.Printf("[ERROR] Scheduler: Failed to create reports directory '%s': %v", reportsDir, err)
			// Non-fatal for scan record, but report won't be saved
		} else {
			reportPath = filepath.Join(reportsDir, reportFilename)
			s.logger.Printf("[INFO] Scheduler: Generating report for scanID %d at: %s", scanID, reportPath)

			// Create a slice of pointers for generateReport
			probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
			for i := range probeResults {
				probeResultsPtr[i] = &probeResults[i]
			}

			if err := s.generateReport(probeResultsPtr, urlDiffResults, reportPath); err != nil { // Corrected to use probeResultsPtr
				s.logger.Printf("[ERROR] Scheduler: Failed to generate report for scanID %d: %v", scanID, err)
				// s.db.RecordScanEnd(scanID, time.Now(), models.ScanStatusFailed, "", fmt.Sprintf("Report generation error: %v", err)) // Original line with error
				s.db.UpdateScanCompletion(scanID, time.Now(), "FAILED", "", fmt.Sprintf("Report generation error: %v", err)) // Corrected
				return targetSource, fmt.Errorf("failed to generate report: %w", err)
			}
		}
	}

	// Record scan completion
	// s.db.RecordScanEnd(scanID, time.Now(), models.ScanStatusCompleted, reportPath, "Scan completed successfully") // Original line with error
	err = s.db.UpdateScanCompletion(scanID, time.Now(), "COMPLETED", reportPath, "Scan completed successfully") // Corrected
	if err != nil {
		s.logger.Printf("[ERROR] Scheduler: Failed to update scan completion status for scanID %d: %v", scanID, err)
		// This is an issue, but the scan itself was successful. Log and continue.
	}

	s.logger.Printf("[INFO] Scheduler: Successfully completed and recorded scan cycle for ID: %d, Target Source: %s", scanID, targetSource)

	// Calculate scan duration
	scanDuration := time.Since(startTime)

	// Calculate URL statistics for notification
	urlStats := make(map[string]int)
	urlStats["new"] = 0
	urlStats["existing"] = 0
	urlStats["old"] = 0

	for _, diffResult := range urlDiffResults {
		urlStats["new"] += diffResult.New
		urlStats["existing"] += diffResult.Existing
		urlStats["old"] += diffResult.Old
	}

	// Send success notification
	if err := s.notificationHelper.SendScanSuccessNotification(targetSource, reportPath, scanDuration, urlStats); err != nil {
		s.logger.Printf("[ERROR] Scheduler: Failed to send success notification: %v", err)
	}

	return targetSource, nil // Success
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
