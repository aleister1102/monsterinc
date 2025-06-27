package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	logger "github.com/aleister1102/go-logbook"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/scanner"
	"github.com/rs/zerolog"
)

// External function to track active scan session - this should be defined in main.go
var SetActiveScanSessionID func(string) = func(string) {}                         // Default no-op
var GetAndSetInterruptNotificationSent func() bool = func() bool { return false } // Default no-op

type scanAttemptConfig struct {
	maxRetries          int
	retryDelay          time.Duration
	scanSessionID       string
	initialTargetSource string
}

// executeScanCycleWithRetries executes scan cycle with retry logic
func (s *Scheduler) executeScanCycleWithRetries(ctx context.Context) {
	if s.scanTargetsFile == "" {
		s.logger.Info().Msg("Scheduler: No scan targets (-st) provided. Skipping scan cycle execution.")
		return
	}

	config := s.createScanAttemptConfig()

	for attempt := 0; attempt <= config.maxRetries; attempt++ {
		if s.shouldCancelScan(ctx, attempt) {
			return
		}

		if attempt > 0 {
			if s.shouldCancelDuringRetryDelay(ctx, config) {
				return
			}
		}

		success := s.attemptScanCycle(ctx, config, attempt)
		if success {
			return
		}

		if attempt == config.maxRetries {
			s.handleFinalFailure(config)
			return
		}
	}
}

// createScanAttemptConfig creates configuration for scan attempts
func (s *Scheduler) createScanAttemptConfig() scanAttemptConfig {
	_, initialTargetSource, _ := s.targetManager.LoadAndSelectTargets(s.scanTargetsFile)
	if initialTargetSource == "" {
		initialTargetSource = "ErrorDeterminingSource"
	}

	generator := NewTimeStampGenerator()
	return scanAttemptConfig{
		maxRetries:          s.globalConfig.SchedulerConfig.RetryAttempts,
		retryDelay:          DefaultRetryDelay,
		scanSessionID:       generator.GenerateSessionID(),
		initialTargetSource: initialTargetSource,
	}
}

// shouldCancelScan checks if scan should be cancelled
func (s *Scheduler) shouldCancelScan(ctx context.Context, attempt int) bool {
	if result := CheckCancellationWithLog(ctx, s.logger, fmt.Sprintf("scan attempt %d", attempt+1)); result.Cancelled {
		s.logger.Info().Int("attempt", attempt+1).Msg("Scan cancelled before attempt")
		return true
	}
	return false
}

// shouldCancelDuringRetryDelay checks if scan should be cancelled during retry delay
func (s *Scheduler) shouldCancelDuringRetryDelay(ctx context.Context, config scanAttemptConfig) bool {
	select {
	case <-time.After(config.retryDelay):
		return false
	case <-ctx.Done():
		s.logger.Info().Str("scan_session_id", config.scanSessionID).Msg("Scheduler: Context cancelled during retry delay.")
		return true
	}
}

// attemptScanCycle attempts a single scan cycle
func (s *Scheduler) attemptScanCycle(ctx context.Context, config scanAttemptConfig, attempt int) bool {
	summary, reportFilePaths, err := s.executeScanCycle(ctx, config.scanSessionID, config.initialTargetSource)
	updatedSummary := s.updateSummaryWithAttemptResult(summary, attempt, err)

	// Add cycle minutes for completion notification
	updatedSummary.CycleMinutes = s.globalConfig.SchedulerConfig.CycleMinutes

	if err == nil {
		s.logger.Info().Str("scan_session_id", config.scanSessionID).Msg("Scheduler: Cycle completed successfully.")
		s.notificationHelper.SendScanCompletionNotification(
			context.Background(),
			updatedSummary,
			notifier.ScanServiceNotification,
			reportFilePaths,
		)
		return true
	}

	if s.isContextCancellationError(err) {
		s.handleContextCancellation(updatedSummary)
		return true
	}

	return false
}

// executeScanCycle executes a single scan cycle
func (s *Scheduler) executeScanCycle(
	ctx context.Context,
	scanSessionID string,
	predeterminedTargetSource string,
) (models.ScanSummaryData, []string, error) {
	startTime := time.Now()

	// Track active scan session for interrupt handling
	SetActiveScanSessionID(scanSessionID)
	defer SetActiveScanSessionID("") // Clear when done
	defer s.scanner.ResetCrawler()   // Reset crawler state after cycle

	// Create scan logger
	scanLogger, err := logger.NewWithScanID(s.globalConfig.LogConfig, scanSessionID)
	if err != nil {
		s.logger.Warn().Err(err).Str("scan_session_id", scanSessionID).Msg("Failed to create scan logger, using default logger")
		scanLogger = s.logger
	}

	// Build base summary
	baseSummary, err := s.buildBaseScanSummary(scanSessionID, predeterminedTargetSource)
	if err != nil {
		return models.ScanSummaryData{}, nil, err
	}

	// Load targets
	htmlURLs, targetSource, err := s.loadAndPrepareScanTargets(predeterminedTargetSource)
	if err != nil {
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	// Build and send scan start notification
	startSummary := models.GetDefaultScanSummaryData()
	startSummary.ScanSessionID = scanSessionID
	startSummary.TargetSource = targetSource
	startSummary.ScanMode = "automated"
	startSummary.Targets = htmlURLs
	startSummary.TotalTargets = len(htmlURLs)
	startSummary.Status = string(models.ScanStatusStarted)
	startSummary.CycleMinutes = s.globalConfig.SchedulerConfig.CycleMinutes

	s.logger.Info().
		Str("scan_session_id", scanSessionID).
		Str("target_source", targetSource).
		Int("total_targets", len(htmlURLs)).
		Msg("Sending scan start notification for automated scan")

	if s.notificationHelper != nil {
		s.notificationHelper.SendScanStartNotification(ctx, startSummary)
	}

	// Record scan start in DB
	dbScanID, err := s.recordScanStartToDB(scanSessionID, targetSource, htmlURLs, startTime)
	if err != nil {
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	// Execute batch scan using orchestrator
	return s.executeSchedulerBatchScan(ctx, scanSessionID, baseSummary, dbScanID, scanLogger)
}

// executeSchedulerBatchScan executes batch scan with scheduler-specific handling
func (s *Scheduler) executeSchedulerBatchScan(
	ctx context.Context,
	scanSessionID string,
	baseSummary models.ScanSummaryData,
	dbScanID int64,
	scanLogger zerolog.Logger,
) (models.ScanSummaryData, []string, error) {
	// Create batch workflow orchestrator
	batchOrchestrator := scanner.NewBatchWorkflowOrchestrator(s.globalConfig, s.scanner, scanLogger)

	// Execute batch scan workflow
	batchResult, err := batchOrchestrator.ExecuteBatchScan(
		ctx,
		s.globalConfig,
		s.scanTargetsFile,
		scanSessionID,
		baseSummary.TargetSource,
		"automated",
	)

	if err != nil {
		s.updateDBOnFailure(dbScanID, err)
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	if batchResult == nil {
		err = fmt.Errorf("batch scan returned nil result")
		s.updateDBOnFailure(dbScanID, err)
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	// Update database with success
	s.updateDBOnSuccess(dbScanID, batchResult.SummaryData)

	scanLogger.Info().
		Str("scan_session_id", scanSessionID).
		Str("status", batchResult.SummaryData.Status).
		Bool("used_batching", batchResult.UsedBatching).
		Int("total_batches", batchResult.TotalBatches).
		Msg("Scheduler scan cycle completed")

	updatedSummary := s.updateSummaryWithScanResult(baseSummary, batchResult.SummaryData)
	return updatedSummary, batchResult.ReportFilePaths, nil
}

// Helper functions for summary building and error handling
func (s *Scheduler) updateSummaryWithAttemptResult(summary models.ScanSummaryData, attempt int, err error) models.ScanSummaryData {
	builder := models.NewScanSummaryDataBuilder().
		WithScanSessionID(summary.ScanSessionID).
		WithScanMode(summary.ScanMode).
		WithTargetSource(summary.TargetSource).
		WithRetriesAttempted(attempt).
		WithTargets(summary.Targets).
		WithTotalTargets(summary.TotalTargets).
		WithProbeStats(summary.ProbeStats).
		WithDiffStats(summary.DiffStats).
		WithScanDuration(summary.ScanDuration).
		WithReportPath(summary.ReportPath).
		WithErrorMessages(summary.ErrorMessages)

	if err == nil {
		builder.WithStatus(models.ScanStatusCompleted)
	} else {
		builder.WithStatus(models.ScanStatusFailed)
	}

	result, _ := builder.Build()
	return result
}

func (s *Scheduler) isContextCancellationError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *Scheduler) handleContextCancellation(summary models.ScanSummaryData) {
	// Check if notification already sent to avoid duplicates
	if !GetAndSetInterruptNotificationSent() {
		interruptSummary := s.buildInterruptSummary(summary)
		s.notificationHelper.SendScanInterruptNotification(context.Background(), interruptSummary)
		s.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Sent scan interrupt notification from scheduler")
	} else {
		s.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Scan interrupt notification already sent, skipping duplicate from scheduler")
	}
}

func (s *Scheduler) buildInterruptSummary(summary models.ScanSummaryData) models.ScanSummaryData {
	builder := models.NewScanSummaryDataBuilder().
		WithScanSessionID(summary.ScanSessionID).
		WithScanMode(summary.ScanMode).
		WithTargetSource(summary.TargetSource).
		WithRetriesAttempted(summary.RetriesAttempted).
		WithTargets(summary.Targets).
		WithTotalTargets(summary.TotalTargets).
		WithProbeStats(summary.ProbeStats).
		WithDiffStats(summary.DiffStats).
		WithScanDuration(summary.ScanDuration).
		WithReportPath(summary.ReportPath).
		WithStatus(models.ScanStatusInterrupted).
		WithErrorMessages([]string{"Scan was interrupted by user signal (Ctrl+C)"})

	result, _ := builder.Build()
	return result
}

func (s *Scheduler) handleFinalFailure(config scanAttemptConfig) {
	summary := s.buildFailureSummary(config)

	// Add cycle minutes for failure notification
	summary.CycleMinutes = s.globalConfig.SchedulerConfig.CycleMinutes

	s.logger.Error().Str("scan_session_id", config.scanSessionID).Msg("Scheduler: All retry attempts exhausted.")
	s.notificationHelper.SendScanCompletionNotification(
		context.Background(),
		summary,
		notifier.ScanServiceNotification,
		nil,
	)
}

func (s *Scheduler) buildFailureSummary(config scanAttemptConfig) models.ScanSummaryData {
	summary, _ := models.NewScanSummaryDataBuilder().
		WithScanSessionID(config.scanSessionID).
		WithScanMode("automated").
		WithTargetSource(config.initialTargetSource).
		WithRetriesAttempted(config.maxRetries).
		WithStatus(models.ScanStatusFailed).
		WithErrorMessages([]string{"All retry attempts exhausted"}).
		Build()

	return summary
}

func (s *Scheduler) buildBaseScanSummary(scanSessionID, targetSource string) (models.ScanSummaryData, error) {
	return models.NewScanSummaryDataBuilder().
		WithScanSessionID(scanSessionID).
		WithScanMode("automated").
		WithTargetSource(targetSource).
		WithStatus(models.ScanStatusStarted).
		Build()
}

func (s *Scheduler) buildErrorSummary(baseSummary models.ScanSummaryData, err error) models.ScanSummaryData {
	result, _ := models.NewScanSummaryDataBuilder().
		WithScanSessionID(baseSummary.ScanSessionID).
		WithScanMode(baseSummary.ScanMode).
		WithTargetSource(baseSummary.TargetSource).
		WithStatus(models.ScanStatusFailed).
		WithErrorMessages([]string{err.Error()}).
		Build()

	return result
}

// Database operations
func (s *Scheduler) updateDBOnFailure(dbScanID int64, err error) {
	err = s.db.UpdateScanCompletion(
		dbScanID,
		time.Now(),
		"FAILED",
		fmt.Sprintf("Scan failed: %v", err),
		0, 0, 0,
		"",
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Scheduler: Failed to update scan completion in database")
	}
}

func (s *Scheduler) updateDBOnSuccess(dbScanID int64, result models.ScanSummaryData) {
	reportPath := ""
	if len(result.ReportPath) > 0 {
		reportPath = result.ReportPath
	}

	err := s.db.UpdateScanCompletion(
		dbScanID,
		time.Now(),
		result.Status,
		s.buildLogSummary(result),
		result.DiffStats.New,
		result.DiffStats.Old,
		result.DiffStats.Existing,
		reportPath,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Scheduler: Failed to update scan completion in database")
	}
}

func (s *Scheduler) buildLogSummary(result models.ScanSummaryData) string {
	parts := []string{
		fmt.Sprintf("Targets: %d", result.TotalTargets),
		fmt.Sprintf("Probed: %d", result.ProbeStats.TotalProbed),
		fmt.Sprintf("Success: %d", result.ProbeStats.SuccessfulProbes),
	}

	if len(result.ErrorMessages) > 0 {
		parts = append(parts, fmt.Sprintf("Errors: %s", strings.Join(result.ErrorMessages, "; ")))
	}

	return strings.Join(parts, ", ")
}

func (s *Scheduler) updateSummaryWithScanResult(baseSummary models.ScanSummaryData, scanResult models.ScanSummaryData) models.ScanSummaryData {
	result, _ := models.NewScanSummaryDataBuilder().
		WithScanSessionID(baseSummary.ScanSessionID).
		WithScanMode(baseSummary.ScanMode).
		WithTargetSource(baseSummary.TargetSource).
		WithTargets(scanResult.Targets).
		WithTotalTargets(scanResult.TotalTargets).
		WithProbeStats(scanResult.ProbeStats).
		WithDiffStats(scanResult.DiffStats).
		WithScanDuration(scanResult.ScanDuration).
		WithReportPath(scanResult.ReportPath).
		WithStatus(models.ScanStatus(scanResult.Status)).
		WithErrorMessages(scanResult.ErrorMessages).
		Build()

	return result
}

// Target loading and preparation
func (s *Scheduler) loadAndPrepareScanTargets(initialTargetSource string) (htmlURLs []string, determinedSource string, err error) {
	s.logger.Info().Msg("Scheduler: Starting to load and prepare scan targets.")
	targets, detSource, loadErr := s.targetManager.LoadAndSelectTargets(s.scanTargetsFile)
	if loadErr != nil {
		return nil, initialTargetSource, WrapError(loadErr, "failed to load targets")
	}

	determinedSource = detSource
	if determinedSource == "" {
		determinedSource = "UnknownSource"
	}

	if len(targets) == 0 {
		s.logger.Info().Str("source", determinedSource).Msg("Scheduler: No targets loaded to process.")
		return nil, determinedSource, NewError("no targets to process from source: " + determinedSource)
	}

	// Convert targets to string slice
	allTargetURLs := make([]string, len(targets))
	for i, target := range targets {
		allTargetURLs[i] = target.NormalizedURL
	}

	// All loaded URLs are used for scanning
	htmlURLs = make([]string, len(allTargetURLs))
	copy(htmlURLs, allTargetURLs)

	s.logger.Info().
		Int("total_targets_loaded", len(allTargetURLs)).
		Str("determined_source", determinedSource).
		Msg("Scheduler: Target loading completed.")

	return htmlURLs, determinedSource, nil
}

func (s *Scheduler) recordScanStartToDB(
	scanSessionID string,
	targetSource string,
	htmlURLs []string,
	startTime time.Time,
) (int64, error) {
	dbScanID, err := s.db.RecordScanStart(scanSessionID, targetSource, len(htmlURLs), startTime)
	if err != nil {
		return 0, WrapError(err, fmt.Sprintf("scheduler failed to record scan start in DB for session %s", scanSessionID))
	}

	s.logger.Info().
		Int64("db_scan_id", dbScanID).
		Str("scan_session_id", scanSessionID).
		Msg("Scheduler: Recorded scan start in database.")

	return dbScanID, nil
}
