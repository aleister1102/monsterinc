package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/scanner"
)

type scanAttemptConfig struct {
	maxRetries          int
	retryDelay          time.Duration
	scanSessionID       string
	initialTargetSource string
}

func (s *Scheduler) executeScanCycleWithRetries(ctx context.Context) {
	if s.scanTargetsFile == "" {
		s.logger.Info().Msg("Scheduler: No scan targets (-st) provided. Skipping scan cycle execution. Monitoring (if active) will continue via its own ticker.")
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

func (s *Scheduler) shouldCancelScan(ctx context.Context, attempt int) bool {
	if result := common.CheckCancellationWithLog(ctx, s.logger, fmt.Sprintf("scan attempt %d", attempt+1)); result.Cancelled {
		s.logger.Info().Int("attempt", attempt+1).Msg("Scan cancelled before attempt")
		return true
	}
	return false
}

func (s *Scheduler) shouldCancelDuringRetryDelay(ctx context.Context, config scanAttemptConfig) bool {
	select {
	case <-time.After(config.retryDelay):
		return false
	case <-ctx.Done():
		s.logger.Info().Str("scan_session_id", config.scanSessionID).Msg("Scheduler: Context cancelled during retry delay. Stopping retry loop.")
		return true
	}
}

func (s *Scheduler) attemptScanCycle(ctx context.Context, config scanAttemptConfig, attempt int) bool {
	summary, reportFilePaths, err := s.executeScanCycle(ctx, config.scanSessionID, config.initialTargetSource)

	updatedSummary := s.updateSummaryWithAttemptResult(summary, attempt, err)

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
	interruptSummary := s.buildInterruptSummary(summary)
	s.notificationHelper.SendScanInterruptNotification(context.Background(), interruptSummary)
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
	s.logger.Error().Str("scan_session_id", config.scanSessionID).Msg("Scheduler: All retry attempts exhausted. Cycle failed permanently.")
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

func (s *Scheduler) executeScanCycle(
	ctx context.Context,
	scanSessionID string,
	predeterminedTargetSource string,
) (models.ScanSummaryData, []string, error) {
	startTime := time.Now()

	baseSummary, err := s.buildBaseScanSummary(scanSessionID, predeterminedTargetSource)
	if err != nil {
		return models.ScanSummaryData{}, nil, err
	}

	htmlURLs, targetSource, err := s.loadAndPrepareScanTargets(predeterminedTargetSource)
	if err != nil {
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	dbScanID, err := s.recordScanStartToDB(scanSessionID, targetSource, htmlURLs, startTime)
	if err != nil {
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	s.sendScanStartNotification(ctx, baseSummary, scanSessionID, htmlURLs)

	return s.performScanAndGenerateReport(ctx, scanSessionID, htmlURLs, baseSummary, dbScanID)
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

func (s *Scheduler) performScanAndGenerateReport(
	ctx context.Context,
	scanSessionID string,
	htmlURLs []string,
	baseSummary models.ScanSummaryData,
	dbScanID int64,
) (models.ScanSummaryData, []string, error) {
	// Create batch workflow orchestrator for scheduler scans
	batchOrchestrator := scanner.NewBatchWorkflowOrchestrator(s.globalConfig, s.scanner, s.logger)

	// Execute batch scan workflow
	batchResult, err := batchOrchestrator.ExecuteBatchScan(
		ctx,
		s.globalConfig,
		s.scanTargetsFile, // Use the original file for batch processing
		scanSessionID,
		baseSummary.TargetSource,
		"automated",
	)

	if err != nil {
		s.updateDBOnFailure(dbScanID, err)
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	var scanResult models.ScanSummaryData
	var reportPaths []string

	if batchResult != nil {
		scanResult = batchResult.SummaryData
		reportPaths = batchResult.ReportFilePaths

		// Log batch processing information
		if batchResult.UsedBatching {
			s.logger.Info().
				Str("scan_session_id", scanSessionID).
				Int("total_batches", batchResult.TotalBatches).
				Int("processed_batches", batchResult.ProcessedBatches).
				Bool("interrupted", batchResult.InterruptedAt > 0).
				Msg("Scheduler batch scan completed")
		}
	} else {
		err = fmt.Errorf("batch scan returned nil result")
		s.updateDBOnFailure(dbScanID, err)
		return s.buildErrorSummary(baseSummary, err), nil, err
	}

	s.updateDBOnSuccess(dbScanID, scanResult)

	s.logger.Info().
		Str("scan_session_id", scanSessionID).
		Int("targets_processed", len(htmlURLs)).
		Str("status", scanResult.Status).
		Msg("Scan cycle completed successfully")

	updatedSummary := s.updateSummaryWithScanResult(baseSummary, scanResult)
	return updatedSummary, reportPaths, nil
}

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

func (s *Scheduler) sendScanStartNotification(ctx context.Context, baseSummary models.ScanSummaryData, scanSessionID string, htmlURLs []string) {
	if s.notificationHelper != nil && len(htmlURLs) > 0 {
		startNotificationSummary, err := models.NewScanSummaryDataBuilder().
			WithScanSessionID(scanSessionID). // Use the direct scanSessionID for this specific notification
			WithTargetSource(baseSummary.TargetSource).
			WithScanMode(baseSummary.ScanMode).
			WithStatus(models.ScanStatusStarted).
			WithTargets(htmlURLs).
			WithTotalTargets(len(htmlURLs)).
			Build()
		if err != nil {
			s.logger.Error().Err(err).Msg("Scheduler: Failed to build scan summary for cycle attempt.")
			return
		}
		s.logger.Info().Str("session_id", scanSessionID).Int("html_target_count", len(htmlURLs)).Msg("Scheduler: Sending scan start notification for HTML URLs.")
		s.notificationHelper.SendScanStartNotification(ctx, startNotificationSummary)
	}
}

func (s *Scheduler) loadAndPrepareScanTargets(initialTargetSource string) (htmlURLs []string, determinedSource string, err error) {
	s.logger.Info().Msg("Scheduler: Starting to load and prepare scan targets.")
	targets, detSource, loadErr := s.targetManager.LoadAndSelectTargets(s.scanTargetsFile)
	if loadErr != nil {
		return nil, initialTargetSource, common.WrapError(loadErr, "failed to load targets")
	}
	determinedSource = detSource
	if determinedSource == "" {
		determinedSource = "UnknownSource" // Default if not determined
	}

	if len(targets) == 0 {
		s.logger.Info().Str("source", determinedSource).Msg("Scheduler: No targets loaded to process.")
		return nil, determinedSource, common.NewError("no targets to process from source: %s", determinedSource)
	}

	// Convert targets to string slice
	allTargetURLs := make([]string, len(targets))
	for i, target := range targets {
		allTargetURLs[i] = target.NormalizedURL
	}

	// Without content type grouping, all loaded URLs are considered for both scanning (as HTML) and monitoring.
	htmlURLs = make([]string, len(allTargetURLs))
	copy(htmlURLs, allTargetURLs)

	s.logger.Info().Int("total_targets_loaded", len(allTargetURLs)).Str("determined_source", determinedSource).Msg("Scheduler: Target loading completed. All targets will be used for both scanning and monitoring.")
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
		return 0, common.WrapError(err, fmt.Sprintf("scheduler failed to record scan start in DB for session %s", scanSessionID))
	}
	s.logger.Info().Int64("db_scan_id", dbScanID).Str("scan_session_id", scanSessionID).Msg("Scheduler: Recorded scan start in database.")
	return dbScanID, nil
}
