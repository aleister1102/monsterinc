package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common/contextutils"
	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/aleister1102/monsterinc/internal/common/summary"
	"github.com/aleister1102/monsterinc/internal/logger"
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
	if result := contextutils.CheckCancellationWithLog(ctx, s.logger, fmt.Sprintf("scan attempt %d", attempt+1)); result.Cancelled {
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
) (summary.ScanSummaryData, []string, error) {
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
		return summary.ScanSummaryData{}, nil, err
	}

	// Load targets
	htmlURLs, targetSource, err := s.loadAndPrepareScanTargets(predeterminedTargetSource)
	if err != nil {
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	// Build and send scan start notification
	startSummary := summary.GetDefaultScanSummaryData()
	startSummary.ScanSessionID = scanSessionID
	startSummary.TargetSource = targetSource
	startSummary.ScanMode = "automated"
	startSummary.Targets = htmlURLs
	startSummary.TotalTargets = len(htmlURLs)
	startSummary.Status = string(summary.ScanStatusStarted)
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
	return s.executeSchedulerBatchScan(ctx, scanSessionID, baseSummary, dbScanID, scanLogger, startTime)
}

// executeSchedulerBatchScan executes batch scan with scheduler-specific handling
func (s *Scheduler) executeSchedulerBatchScan(
	ctx context.Context,
	scanSessionID string,
	baseSummary summary.ScanSummaryData,
	dbScanID int64,
	scanLogger zerolog.Logger,
	startTime time.Time,
) (summary.ScanSummaryData, []string, error) {
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

	// Check if batch result indicates actual scanning failure (all batches failed due to preprocessing)
	if batchResult.SummaryData.Status == string(summary.ScanStatusFailed) &&
		batchResult.SummaryData.ProbeStats.TotalProbed == 0 &&
		batchResult.ProcessedBatches == 0 {
		// All batches failed due to preprocessing, treat as scan failure
		s.updateDBOnFailure(dbScanID, fmt.Errorf("all batches failed during preprocessing"))
		return s.buildErrorSummary(baseSummary, fmt.Errorf("all batches failed: no URLs processed")), nil, fmt.Errorf("all batches failed during preprocessing")
	}

	// Calculate actual scan duration from start
	actualScanDuration := time.Since(startTime)

	// Update batch result with correct duration
	batchResult.SummaryData.ScanDuration = actualScanDuration

	// Update database with success
	s.updateDBOnSuccess(dbScanID, batchResult.SummaryData)

	scanLogger.Info().
		Str("scan_session_id", scanSessionID).
		Str("status", batchResult.SummaryData.Status).
		Bool("used_batching", batchResult.UsedBatching).
		Int("total_batches", batchResult.TotalBatches).
		Dur("scan_duration", actualScanDuration).
		Msg("Scheduler scan cycle completed")

	updatedSummary := s.updateSummaryWithScanResult(baseSummary, batchResult.SummaryData)
	return updatedSummary, batchResult.ReportFilePaths, nil
}

// Helper functions for summary building and error handling
func (s *Scheduler) updateSummaryWithAttemptResult(summaryData summary.ScanSummaryData, attempt int, err error) summary.ScanSummaryData {
	builder := summary.NewScanSummaryDataBuilder().
		WithScanSessionID(summaryData.ScanSessionID).
		WithScanMode(summaryData.ScanMode).
		WithTargetSource(summaryData.TargetSource).
		WithTargets(summaryData.Targets).
		WithTotalTargets(summaryData.TotalTargets).
		WithProbeStats(summaryData.ProbeStats).
		WithDiffStats(summaryData.DiffStats).
		WithScanDuration(summaryData.ScanDuration)

	if err == nil {
		builder.WithStatus(summary.ScanStatus(summary.ScanStatusCompleted))
	} else {
		builder.WithStatus(summary.ScanStatus(summary.ScanStatusFailed))
	}

	result, _ := builder.Build()
	return result
}

func (s *Scheduler) isContextCancellationError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *Scheduler) handleContextCancellation(summary summary.ScanSummaryData) {
	// Check if notification already sent to avoid duplicates
	if !GetAndSetInterruptNotificationSent() {
		interruptSummary := s.buildInterruptSummary(summary)
		s.notificationHelper.SendScanInterruptNotification(context.Background(), interruptSummary)
		s.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Sent scan interrupt notification from scheduler")
	} else {
		s.logger.Info().Str("scan_session_id", summary.ScanSessionID).Msg("Scan interrupt notification already sent, skipping duplicate from scheduler")
	}
}

func (s *Scheduler) buildInterruptSummary(summaryData summary.ScanSummaryData) summary.ScanSummaryData {
	builder := summary.NewScanSummaryDataBuilder().
		WithScanSessionID(summaryData.ScanSessionID).
		WithScanMode(summaryData.ScanMode).
		WithTargetSource(summaryData.TargetSource).
		WithRetriesAttempted(summaryData.RetriesAttempted).
		WithTargets(summaryData.Targets).
		WithTotalTargets(summaryData.TotalTargets).
		WithProbeStats(summaryData.ProbeStats).
		WithDiffStats(summaryData.DiffStats).
		WithScanDuration(summaryData.ScanDuration).
		WithReportPath(summaryData.ReportPath).
		WithStatus(summary.ScanStatusInterrupted).
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
		[]string{},
	)
}

func (s *Scheduler) buildFailureSummary(config scanAttemptConfig) summary.ScanSummaryData {
	summary, _ := summary.NewScanSummaryDataBuilder().
		WithScanSessionID(config.scanSessionID).
		WithScanMode("automated").
		WithTargetSource(config.initialTargetSource).
		WithRetriesAttempted(config.maxRetries).
		WithStatus(summary.ScanStatusFailed).
		WithErrorMessages([]string{"All retry attempts exhausted"}).
		Build()

	return summary
}

func (s *Scheduler) buildBaseScanSummary(scanSessionID, targetSource string) (summary.ScanSummaryData, error) {
	return summary.NewScanSummaryDataBuilder().
		WithScanSessionID(scanSessionID).
		WithScanMode("automated").
		WithTargetSource(targetSource).
		WithStatus(summary.ScanStatusStarted).
		Build()
}

func (s *Scheduler) buildErrorSummary(baseSummary summary.ScanSummaryData, err error) summary.ScanSummaryData {
	result, _ := summary.NewScanSummaryDataBuilder().
		WithScanSessionID(baseSummary.ScanSessionID).
		WithScanMode(baseSummary.ScanMode).
		WithTargetSource(baseSummary.TargetSource).
		WithStatus(summary.ScanStatusFailed).
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

func (s *Scheduler) updateDBOnSuccess(dbScanID int64, result summary.ScanSummaryData) {
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

func (s *Scheduler) buildLogSummary(result summary.ScanSummaryData) string {
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

func (s *Scheduler) updateSummaryWithScanResult(baseSummary summary.ScanSummaryData, scanResult summary.ScanSummaryData) summary.ScanSummaryData {
	result, _ := summary.NewScanSummaryDataBuilder().
		WithScanSessionID(baseSummary.ScanSessionID).
		WithScanMode(baseSummary.ScanMode).
		WithTargetSource(baseSummary.TargetSource).
		WithTargets(scanResult.Targets).
		WithTotalTargets(scanResult.TotalTargets).
		WithProbeStats(scanResult.ProbeStats).
		WithDiffStats(scanResult.DiffStats).
		WithScanDuration(scanResult.ScanDuration).
		WithReportPath(scanResult.ReportPath).
		WithStatus(summary.ScanStatus(scanResult.Status)).
		WithErrorMessages(scanResult.ErrorMessages).
		Build()

	return result
}

// Target loading and preparation
func (s *Scheduler) loadAndPrepareScanTargets(initialTargetSource string) (htmlURLs []string, determinedSource string, err error) {
	s.logger.Info().Msg("Scheduler: Starting to load and prepare scan targets.")
	targets, detSource, loadErr := s.targetManager.LoadAndSelectTargets(s.scanTargetsFile)
	if loadErr != nil {
		return nil, initialTargetSource, errorwrapper.WrapError(loadErr, "failed to load targets")
	}

	determinedSource = detSource
	if determinedSource == "" {
		determinedSource = "UnknownSource"
	}

	if len(targets) == 0 {
		s.logger.Info().Str("source", determinedSource).Msg("Scheduler: No targets loaded to process.")
		return nil, determinedSource, errorwrapper.NewError("no targets to process from source: %s", determinedSource)
	}

	// Convert targets to string slice
	allTargetURLs := make([]string, len(targets))
	for i, target := range targets {
		allTargetURLs[i] = target.URL
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
		return 0, errorwrapper.WrapError(err, fmt.Sprintf("scheduler failed to record scan start in DB for session %s", scanSessionID))
	}

	s.logger.Info().
		Int64("db_scan_id", dbScanID).
		Str("scan_session_id", scanSessionID).
		Msg("Scheduler: Recorded scan start in database.")

	return dbScanID, nil
}
