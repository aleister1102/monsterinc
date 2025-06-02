package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
)

// executeScanCycleWithRetries runs a cycle with retry logic
func (s *Scheduler) executeScanCycleWithRetries(ctx context.Context) {
	if s.urlFileOverride == "" {
		s.logger.Info().Msg("Scheduler: No scan targets (-st) provided. Skipping scan cycle execution. Monitoring (if active) will continue via its own ticker.")
		return
	}

	maxRetries := s.globalConfig.SchedulerConfig.RetryAttempts
	retryDelay := 5 * time.Minute

	var lastErr error
	var scanSummary models.ScanSummaryData
	currentScanSessionID := time.Now().Format("20060102-150405")

	_, initialTargetSource, initialTargetsErr := s.targetManager.LoadAndSelectTargets(
		s.urlFileOverride,
		s.globalConfig.InputConfig.InputURLs,
		s.globalConfig.InputConfig.InputFile,
	)
	if initialTargetsErr != nil {
		s.logger.Error().Err(initialTargetsErr).Msg("Scheduler: Failed to determine initial target source for cycle. Notifications might be affected.")
		initialTargetSource = "ErrorDeterminingSource"
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		summaryBuilderBase := models.NewScanSummaryDataBuilder().
			WithScanSessionID(currentScanSessionID).
			WithScanMode("automated").
			WithTargetSource(initialTargetSource).
			WithRetriesAttempted(attempt)
		scanSummary = summaryBuilderBase.Build()

		select {
		case <-ctx.Done():
			s.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: Context cancelled before retry attempt. Stopping retry loop.")
			if lastErr != nil && attempt > 0 {
				// Update existing scanSummary with interruption status
				updatedSummary := models.NewScanSummaryDataBuilder().
					WithScanSessionID(scanSummary.ScanSessionID).
					WithScanMode(scanSummary.ScanMode).
					WithTargetSource(scanSummary.TargetSource).
					WithRetriesAttempted(scanSummary.RetriesAttempted).
					WithTargets(scanSummary.Targets).
					WithTotalTargets(scanSummary.TotalTargets).
					WithProbeStats(scanSummary.ProbeStats).
					WithDiffStats(scanSummary.DiffStats).
					WithScanDuration(scanSummary.ScanDuration).
					WithReportPath(scanSummary.ReportPath).
					WithStatus(models.ScanStatusInterrupted).
					WithErrorMessages(scanSummary.ErrorMessages) // Keep existing errors
				if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
					updatedSummary.WithErrorMessages([]string{fmt.Sprintf("Scheduler cycle aborted during retries due to context cancellation. Last error: %v", lastErr)})
				}
				scanSummary = updatedSummary.Build()
				s.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
			}
			return
		default:
		}

		if attempt > 0 {
			s.logger.Info().Int("attempt", attempt).Int("max_retries", maxRetries).Dur("delay", retryDelay).Msg("Scheduler: Retrying cycle after delay...")
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				s.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: Context cancelled during retry delay. Stopping retry loop.")
				if lastErr != nil {
					updatedSummary := models.NewScanSummaryDataBuilder().
						WithScanSessionID(scanSummary.ScanSessionID).
						WithScanMode(scanSummary.ScanMode).
						WithTargetSource(scanSummary.TargetSource).
						WithRetriesAttempted(scanSummary.RetriesAttempted).
						WithStatus(models.ScanStatusInterrupted).
						WithErrorMessages(scanSummary.ErrorMessages) // Keep existing errors
					if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
						updatedSummary.WithErrorMessages([]string{fmt.Sprintf("Scheduler cycle aborted during retry delay due to context cancellation. Last error: %v", lastErr)})
					}
					scanSummary = updatedSummary.Build()
					s.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
				}
				return
			}
		}

		var currentAttemptSummary models.ScanSummaryData
		var reportFilePaths []string
		currentAttemptSummary, reportFilePaths, lastErr = s.executeScanCycle(ctx, currentScanSessionID, initialTargetSource)

		// Update scanSummary with results from currentAttemptSummary
		summaryUpdater := models.NewScanSummaryDataBuilder().
			WithScanSessionID(scanSummary.ScanSessionID).
			WithScanMode(scanSummary.ScanMode).
			WithTargetSource(scanSummary.TargetSource).
			WithRetriesAttempted(scanSummary.RetriesAttempted).
			WithTargets(currentAttemptSummary.Targets).
			WithTotalTargets(currentAttemptSummary.TotalTargets).
			WithProbeStats(currentAttemptSummary.ProbeStats).
			WithDiffStats(currentAttemptSummary.DiffStats).
			WithScanDuration(currentAttemptSummary.ScanDuration).
			WithReportPath(currentAttemptSummary.ReportPath).
			WithErrorMessages(scanSummary.ErrorMessages).          // Keep previous errors
			WithErrorMessages(currentAttemptSummary.ErrorMessages) // Add new errors

		if currentAttemptSummary.Status != "" {
			summaryUpdater.WithStatus(models.ScanStatus(currentAttemptSummary.Status))
		} else if lastErr == nil {
			summaryUpdater.WithStatus(models.ScanStatusCompleted)
		} else {
			summaryUpdater.WithStatus(models.ScanStatusFailed) // Default to failed if error and no specific status
		}
		scanSummary = summaryUpdater.Build()

		if lastErr == nil {
			s.logger.Info().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: Cycle completed successfully.")
			// scanSummary already has status Completed from above
			s.notificationHelper.SendScanCompletionNotification(context.Background(), scanSummary, notifier.ScanServiceNotification, reportFilePaths)
			return
		}

		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			s.logger.Info().Str("scan_session_id", currentScanSessionID).Err(lastErr).Msg("Scheduler: Cycle interrupted by context cancellation. No further retries.")

			updatedSummary := models.NewScanSummaryDataBuilder().
				WithScanSessionID(scanSummary.ScanSessionID).
				WithScanMode(scanSummary.ScanMode).
				WithTargetSource(scanSummary.TargetSource).
				WithRetriesAttempted(scanSummary.RetriesAttempted).
				WithTargets(scanSummary.Targets).
				WithTotalTargets(scanSummary.TotalTargets).
				WithProbeStats(scanSummary.ProbeStats).
				WithDiffStats(scanSummary.DiffStats).
				WithScanDuration(scanSummary.ScanDuration).
				WithReportPath(scanSummary.ReportPath).
				WithStatus(models.ScanStatusInterrupted).
				WithErrorMessages(scanSummary.ErrorMessages) // Keep existing errors
			if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
				updatedSummary.WithErrorMessages([]string{fmt.Sprintf("Scheduler cycle interrupted: %v", lastErr)})
			}
			scanSummary = updatedSummary.Build()

			s.logger.Info().Msg("Scheduler: Sending interrupt notification to both scan and monitor services.")
			s.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
			s.notificationHelper.SendMonitorInterruptNotification(context.Background(), scanSummary)
			return
		}

		s.logger.Error().Err(lastErr).Str("scan_session_id", currentScanSessionID).Int("attempt", attempt+1).Int("total_attempts", maxRetries+1).Msg("Scheduler: Cycle failed")

		if attempt == maxRetries {
			s.logger.Error().Str("scan_session_id", currentScanSessionID).Msg("Scheduler: All retry attempts exhausted. Cycle failed permanently.")

			updatedSummary := models.NewScanSummaryDataBuilder().
				WithScanSessionID(scanSummary.ScanSessionID).
				WithScanMode(scanSummary.ScanMode).
				WithTargetSource(scanSummary.TargetSource).
				WithRetriesAttempted(scanSummary.RetriesAttempted).
				WithTargets(scanSummary.Targets).
				WithTotalTargets(scanSummary.TotalTargets).
				WithProbeStats(scanSummary.ProbeStats).
				WithDiffStats(scanSummary.DiffStats).
				WithScanDuration(scanSummary.ScanDuration).
				WithReportPath(scanSummary.ReportPath).
				WithStatus(models.ScanStatusFailed).
				WithErrorMessages(scanSummary.ErrorMessages) // Keep existing errors
			if !common.ContainsCancellationError(scanSummary.ErrorMessages) {
				updatedSummary.WithErrorMessages([]string{fmt.Sprintf("All %d retry attempts failed. Last error: %v", maxRetries+1, lastErr)})
			}
			scanSummary = updatedSummary.Build()
			s.notificationHelper.SendScanInterruptNotification(context.Background(), scanSummary)
		}
	}
}

// executeScanCycle executes a complete cycle
func (s *Scheduler) executeScanCycle(
	ctx context.Context,
	scanSessionID string,
	predeterminedTargetSource string,
) (models.ScanSummaryData, []string, error) {
	startTime := time.Now()
	summaryBuilder := models.NewScanSummaryDataBuilder().
		WithScanSessionID(scanSessionID).
		WithScanMode("automated").
		WithTargetSource(predeterminedTargetSource)
	// summary is built later after more fields are potentially set

	var finalReportFilePaths []string
	var overallCycleError error

	htmlURLs, determinedTargetSourceFromLoad, err := s.loadAndPrepareScanTargets(predeterminedTargetSource)
	if err != nil {
		s.logger.Error().Err(err).Str("scan_session_id", scanSessionID).Msg("Scheduler: Failed to load and prepare scan targets for cycle.")
		summaryBuilder.WithErrorMessages([]string{fmt.Sprintf("Failed to load/prepare targets: %v", err)})
		if strings.Contains(err.Error(), "failed to load targets") || strings.Contains(err.Error(), "no valid URLs found") {
			summaryBuilder.WithStatus(models.ScanStatusFailed)
			return summaryBuilder.Build(), nil, err
		}
		overallCycleError = err
	}
	summaryBuilder.WithTargetSource(determinedTargetSourceFromLoad)

	if len(htmlURLs) == 0 {
		s.logger.Info().Str("source", determinedTargetSourceFromLoad).Msg("Scheduler: No targets (HTML or monitor) to process in this cycle after preparation.")
		summaryBuilder.WithStatus(models.ScanStatusNoTargets)
		if overallCycleError == nil {
			summaryBuilder.WithErrorMessages([]string{"No targets available for scanning or monitoring after preparation."})
		}
		return summaryBuilder.Build(), nil, overallCycleError
	}

	summaryBuilder.WithTargets(htmlURLs).WithTotalTargets(len(htmlURLs))

	// Build summary for start notification
	tempSummaryForStart := summaryBuilder.Build() // Build a temporary summary for sending start notification
	s.sendScanStartNotification(ctx, tempSummaryForStart, scanSessionID, htmlURLs)

	dbScanID, dbErr := s.recordScanStartToDB(scanSessionID, determinedTargetSourceFromLoad, htmlURLs, startTime)
	if dbErr != nil {
		s.logger.Error().Err(dbErr).Msg("Scheduler: Failed to record scan start in database for cycle.")
		summaryBuilder.WithErrorMessages([]string{fmt.Sprintf("DB record start error: %v", dbErr)})
	}

	var monitorWG sync.WaitGroup
	s.manageMonitorServiceTasks(ctx, &monitorWG, scanSessionID, determinedTargetSourceFromLoad)

	var scanWorkflowSummary models.ScanSummaryData

	if len(htmlURLs) > 0 {
		s.logger.Info().Int("html_count", len(htmlURLs)).Msg("Scheduler: Executing scan workflow for HTML URLs using shared function.")
		var workflowErr error

		scanWorkflowSummary, _, finalReportFilePaths, workflowErr = s.orchestrator.ExecuteSingleScanWorkflowWithReporting(
			ctx,
			s.globalConfig,
			s.logger.With().Str("component", "ScanWorkflowRunner").Logger(),
			htmlURLs,
			scanSessionID,
			determinedTargetSourceFromLoad,
			"automated",
		)

		summaryBuilder.WithProbeStats(scanWorkflowSummary.ProbeStats).
			WithDiffStats(scanWorkflowSummary.DiffStats).
			WithScanDuration(scanWorkflowSummary.ScanDuration). // This might be overwritten later
			WithReportPath(scanWorkflowSummary.ReportPath).
			WithErrorMessages(scanWorkflowSummary.ErrorMessages) // Appends

		if workflowErr != nil {
			s.logger.Error().Err(workflowErr).Msg("Scheduler: Scan workflow (via shared function) failed for HTML URLs.")
			overallCycleError = workflowErr
			summaryBuilder.WithStatus(models.ScanStatus(scanWorkflowSummary.Status)) // Status from workflow

			if errors.Is(workflowErr, context.Canceled) || errors.Is(workflowErr, context.DeadlineExceeded) {
				s.logger.Info().Msg("Scheduler: Scan workflow cancelled, propagating error up.")
				return summaryBuilder.Build(), finalReportFilePaths, workflowErr
			}
		} else {
			s.logger.Info().Int("probe_results_count", scanWorkflowSummary.ProbeStats.TotalProbed).Msg("Scheduler: Scan workflow (via shared function) completed.")
			summaryBuilder.WithStatus(models.ScanStatus(scanWorkflowSummary.Status)) // Status from workflow
		}

	} else {
		s.logger.Info().Msg("Scheduler: No HTML URLs to scan. Skipping scan workflow execution.")
		if overallCycleError == nil {
			summaryBuilder.WithStatus(models.ScanStatusCompleted)
		} else { // if overallCycleError is not nil but we had no HTML URLs, it implies error from load/prepare
			summaryBuilder.WithStatus(models.ScanStatusNoTargets) // Or Failed based on error type
		}
	}

	s.logger.Info().Msg("Scheduler: Waiting for monitor service initial setup (if any).")
	monitorDone := make(chan struct{})
	go func() {
		monitorWG.Wait()
		close(monitorDone)
	}()

	currentSummaryBuilt := summaryBuilder.Build() // Build before waiting for monitor to have a current state

	select {
	case <-monitorDone:
		s.logger.Info().Msg("Scheduler: Monitor service initial setup completed.")
	case <-ctx.Done():
		s.logger.Info().Msg("Scheduler: Context cancelled while waiting for monitor service setup.")
		// Rebuild summary with interruption details
		summaryBuilderInternal := models.NewScanSummaryDataBuilder().
			WithScanSessionID(currentSummaryBuilt.ScanSessionID).
			WithTargetSource(currentSummaryBuilt.TargetSource).
			WithScanMode(currentSummaryBuilt.ScanMode).
			WithTargets(currentSummaryBuilt.Targets).
			WithTotalTargets(currentSummaryBuilt.TotalTargets).
			WithProbeStats(currentSummaryBuilt.ProbeStats).
			WithDiffStats(currentSummaryBuilt.DiffStats).
			WithScanDuration(currentSummaryBuilt.ScanDuration).
			WithReportPath(currentSummaryBuilt.ReportPath).
			WithErrorMessages(currentSummaryBuilt.ErrorMessages) // Keep existing
		if !common.ContainsCancellationError(currentSummaryBuilt.ErrorMessages) {
			summaryBuilderInternal.WithErrorMessages([]string{"Scheduler cycle interrupted during monitor setup: " + ctx.Err().Error()})
		}
		summaryBuilderInternal.WithStatus(models.ScanStatusInterrupted)
		if overallCycleError == nil {
			overallCycleError = ctx.Err()
		}
		return summaryBuilderInternal.Build(), finalReportFilePaths, overallCycleError
	case <-s.stopChan:
		s.logger.Info().Msg("Scheduler: Stop signal received while waiting for monitor service setup.")
		summaryBuilderInternal := models.NewScanSummaryDataBuilder().
			WithScanSessionID(currentSummaryBuilt.ScanSessionID).
			WithTargetSource(currentSummaryBuilt.TargetSource).
			WithScanMode(currentSummaryBuilt.ScanMode).
			WithTargets(currentSummaryBuilt.Targets).
			WithTotalTargets(currentSummaryBuilt.TotalTargets).
			WithProbeStats(currentSummaryBuilt.ProbeStats).
			WithDiffStats(currentSummaryBuilt.DiffStats).
			WithScanDuration(currentSummaryBuilt.ScanDuration).
			WithReportPath(currentSummaryBuilt.ReportPath).
			WithErrorMessages(currentSummaryBuilt.ErrorMessages) // Keep existing
		if !common.ContainsCancellationError(currentSummaryBuilt.ErrorMessages) {
			summaryBuilderInternal.WithErrorMessages([]string{"Scheduler cycle stopped during monitor setup."})
		}
		summaryBuilderInternal.WithStatus(models.ScanStatusInterrupted)
		if overallCycleError == nil {
			overallCycleError = errors.New("scheduler stop signal received")
		}
		return summaryBuilderInternal.Build(), finalReportFilePaths, overallCycleError
	}

	endTime := time.Now()

	// Re-initialize a builder from currentSummaryBuilt to finalize it
	finalSummaryBuilder := models.NewScanSummaryDataBuilder().
		WithScanSessionID(currentSummaryBuilt.ScanSessionID).
		WithTargetSource(currentSummaryBuilt.TargetSource).
		WithScanMode(currentSummaryBuilt.ScanMode).
		WithTargets(currentSummaryBuilt.Targets).
		WithTotalTargets(currentSummaryBuilt.TotalTargets).
		WithProbeStats(currentSummaryBuilt.ProbeStats).
		WithDiffStats(currentSummaryBuilt.DiffStats).
		WithReportPath(currentSummaryBuilt.ReportPath).
		WithErrorMessages(currentSummaryBuilt.ErrorMessages).     // These are all accumulated errors
		WithStatus(models.ScanStatus(currentSummaryBuilt.Status)) // This is status from workflow or earlier stages

	if currentSummaryBuilt.ScanDuration == 0 { // If not set by workflow, set it now
		finalSummaryBuilder.WithScanDuration(endTime.Sub(startTime))
	} else {
		finalSummaryBuilder.WithScanDuration(currentSummaryBuilt.ScanDuration) // respect workflow duration
	}

	dbStatus := "COMPLETED_WITH_ISSUES"
	builtSummaryForDB := finalSummaryBuilder.Build() // Build to check status for DB

	if overallCycleError == nil && len(builtSummaryForDB.ErrorMessages) == 0 {
		if builtSummaryForDB.Status == string(models.ScanStatusCompleted) || builtSummaryForDB.Status == string(models.ScanStatusNoTargets) || (builtSummaryForDB.Status == "" && len(htmlURLs) == 0) {
			dbStatus = "COMPLETED"
		}
	} else if builtSummaryForDB.Status == string(models.ScanStatusFailed) || builtSummaryForDB.Status == string(models.ScanStatusInterrupted) {
		dbStatus = builtSummaryForDB.Status
	}

	logSummary := fmt.Sprintf("HTML URLs (scanned): %d, Errors: %d, Status: %s",
		len(htmlURLs), len(builtSummaryForDB.ErrorMessages), builtSummaryForDB.Status)

	if dbScanID > 0 {
		if err := s.db.UpdateScanCompletion(dbScanID, endTime, dbStatus, logSummary,
			builtSummaryForDB.DiffStats.New, builtSummaryForDB.DiffStats.Old, builtSummaryForDB.DiffStats.Existing, builtSummaryForDB.ReportPath); err != nil {
			s.logger.Error().Err(err).Msg("Scheduler: Failed to update scan completion in database.")
			finalSummaryBuilder.WithErrorMessages([]string{fmt.Sprintf("Database update error: %v", err)})
			if overallCycleError == nil {
				overallCycleError = err
			}
		}
	}

	// Final status determination
	// Re-build summary with potentially new DB error, and re-evaluate status
	summaryForFinalStatus := finalSummaryBuilder.Build()

	if overallCycleError != nil {
		if errors.Is(overallCycleError, context.Canceled) || errors.Is(overallCycleError, context.DeadlineExceeded) {
			finalSummaryBuilder.WithStatus(models.ScanStatusInterrupted)
		} else if summaryForFinalStatus.Status != string(models.ScanStatusInterrupted) { // Avoid overriding a more specific interrupt
			finalSummaryBuilder.WithStatus(models.ScanStatusFailed)
		}
	} else if len(summaryForFinalStatus.ErrorMessages) > 0 {
		if summaryForFinalStatus.Status != string(models.ScanStatusCompleted) && summaryForFinalStatus.Status != string(models.ScanStatusNoTargets) {
			finalSummaryBuilder.WithStatus(models.ScanStatusPartialComplete)
		}
	} else if summaryForFinalStatus.Status == "" || summaryForFinalStatus.Status == string(models.ScanStatusUnknown) { // If status wasn't set by workflow and no errors
		finalSummaryBuilder.WithStatus(models.ScanStatusCompleted)
	}

	finalSummary := finalSummaryBuilder.Build()
	s.logger.Info().Str("session_id", scanSessionID).Dur("duration", finalSummary.ScanDuration).Str("final_status", finalSummary.Status).Msg("Scheduler: Cycle processing finished.")
	return finalSummary, finalReportFilePaths, overallCycleError
}

// sendScanStartNotification sends a notification when a scan cycle starts.
func (s *Scheduler) sendScanStartNotification(ctx context.Context, baseSummary models.ScanSummaryData, scanSessionID string, htmlURLs []string) {
	if s.notificationHelper != nil && len(htmlURLs) > 0 {
		startNotificationSummary := models.NewScanSummaryDataBuilder().
			WithScanSessionID(scanSessionID). // Use the direct scanSessionID for this specific notification
			WithTargetSource(baseSummary.TargetSource).
			WithScanMode(baseSummary.ScanMode).
			WithStatus(models.ScanStatusStarted).
			WithTargets(htmlURLs).
			WithTotalTargets(len(htmlURLs)).
			Build()
		s.logger.Info().Str("session_id", scanSessionID).Int("html_target_count", len(htmlURLs)).Msg("Scheduler: Sending scan start notification for HTML URLs.")
		s.notificationHelper.SendScanStartNotification(ctx, startNotificationSummary)
	}
}

// recordScanStartToDB records the start of a scan cycle to the database.
func (s *Scheduler) recordScanStartToDB(
	scanSessionID string,
	targetSource string,
	htmlURLs []string,
	startTime time.Time,
) (int64, error) {
	dbScanID, err := s.db.RecordScanStart(scanSessionID, targetSource, len(htmlURLs), startTime)
	if err != nil {
		return 0, fmt.Errorf("scheduler failed to record scan start in DB for session %s: %w", scanSessionID, err)
	}
	s.logger.Info().Int64("db_scan_id", dbScanID).Str("scan_session_id", scanSessionID).Msg("Scheduler: Recorded scan start in database.")
	return dbScanID, nil
}
