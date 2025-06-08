package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// performURLCheck performs a URL check and returns the result
func (s *MonitoringService) performURLCheck(url string) LegacyCheckResult {
	urlMutex := s.mutexManager.GetMutex(url)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	cycleID := s.cycleTracker.GetCurrentCycleID()
	return s.urlChecker.CheckURLWithContext(s.serviceCtx, url, cycleID)
}

// handleCheckResult processes the result of a URL check
func (s *MonitoringService) handleCheckResult(url string, result LegacyCheckResult) {
	if result.FileChangeInfo != nil {
		s.cycleTracker.AddChangedURL(url)
	}
}

// generateAndSendCycleReport generates and sends a cycle completion report
func (s *MonitoringService) generateAndSendCycleReport(monitoredURLs, changedURLs []string, cycleID string) {
	var reportPaths []string

	// Only generate report if there are changes or if we need to track new URLs
	if s.urlChecker.htmlDiffReporter != nil && len(changedURLs) > 0 {
		s.logger.Info().
			Int("monitored_urls", len(monitoredURLs)).
			Int("changed_urls", len(changedURLs)).
			Str("cycle_id", cycleID).
			Msg("Generating HTML diff report for changed URLs only")

		// Generate report only for URLs that have changes - this will significantly reduce file size
		generatedReportPaths, err := s.urlChecker.htmlDiffReporter.GenerateDiffReport(changedURLs, cycleID)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to generate cycle end diff report")
		} else if len(generatedReportPaths) > 0 {
			// Use all generated report paths
			reportPaths = generatedReportPaths
			s.logger.Info().
				Str("main_report_path", reportPaths[0]).
				Int("total_reports", len(reportPaths)).
				Int("changed_urls_reported", len(changedURLs)).
				Msg("Successfully generated HTML diff report for changed URLs")
		}
	} else if s.urlChecker.htmlDiffReporter != nil {
		s.logger.Info().
			Int("monitored_count", len(monitoredURLs)).
			Msg("No changes detected - skipping report generation to save resources")
	} else {
		s.logger.Warn().Msg("HtmlDiffReporter is not available, sending notification without report")
	}

	// Always send cycle complete notification
	s.sendCycleCompleteNotification(cycleID, changedURLs, reportPaths, len(monitoredURLs))
}

// sendCycleCompleteNotification sends a notification when a monitoring cycle completes
func (s *MonitoringService) sendCycleCompleteNotification(cycleID string, changedURLs []string, reportPaths []string, totalMonitored int) {
	if s.notificationHelper == nil {
		return
	}

	// Get batch statistics if batch processing was used
	var batchStats *models.BatchStats
	if s.batchURLManager != nil {
		// Check if batch processing was used by looking at the current monitoring stats
		// Since we don't have direct access to the batch result here, we'll construct minimal stats
		useBatching, batchCount, _ := s.batchURLManager.GetBatchingInfo(totalMonitored)
		if useBatching {
			batchStats = models.NewBatchStats(
				true,                                // usedBatching
				batchCount,                          // totalBatches
				batchCount,                          // completedBatches (assume all completed if we're here)
				totalMonitored/batchCount,           // avgBatchSize (rough estimate)
				s.gCfg.MonitorBatchConfig.BatchSize, // maxBatchSize from config
				totalMonitored,                      // totalURLsProcessed
			)
		}
	}

	data := models.MonitorCycleCompleteData{
		CycleID:        cycleID,
		ChangedURLs:    changedURLs,
		ReportPaths:    reportPaths,
		TotalMonitored: totalMonitored,
		Timestamp:      time.Now(),
		BatchStats:     batchStats,
	}
	s.notificationHelper.SendMonitorCycleCompleteNotification(s.serviceCtx, data)
}

// sendMonitorInterruptNotification sends a notification when monitor service is interrupted
func (s *MonitoringService) sendMonitorInterruptNotification(ctx context.Context, cycleID string, totalTargets, processedTargets int, reason, lastActivity string) {
	if s.notificationHelper == nil {
		return
	}

	interruptData := models.MonitorInterruptData{
		CycleID:          cycleID,
		TotalTargets:     totalTargets,
		ProcessedTargets: processedTargets,
		Timestamp:        time.Now(),
		Reason:           reason,
		LastActivity:     lastActivity,
	}

	s.notificationHelper.SendMonitorInterruptNotification(ctx, interruptData)
}

// performCleanShutdown performs a clean shutdown of the service
func (s *MonitoringService) performCleanShutdown() {
	s.cancelServiceContext()
	s.cleanupResources()
}

// cancelServiceContext cancels the service context
func (s *MonitoringService) cancelServiceContext() {
	if s.serviceCancelFunc != nil {
		s.serviceCancelFunc()
	}
}

// cleanupResources cleans up service resources
func (s *MonitoringService) cleanupResources() {
	activeURLs := s.urlManager.GetCurrentURLs()
	s.mutexManager.CleanupUnusedMutexes(activeURLs)
}

// updateServiceContext updates the service context with a new parent context
func (s *MonitoringService) updateServiceContext(parentCtx context.Context) {
	s.cancelServiceContext()
	s.serviceCtx, s.serviceCancelFunc = context.WithCancel(parentCtx)

	s.logger.Debug().Msg("Updated service context with new parent")
}

// createCycleID creates a new cycle ID
func (s *MonitoringService) createCycleID() string {
	return fmt.Sprintf("monitor-%s", time.Now().Format("20060102-150405"))
}
