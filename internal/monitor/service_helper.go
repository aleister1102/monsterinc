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
	var reportPath string

	// Only generate report if there are changes and reporter is available
	if len(changedURLs) > 0 && s.urlChecker.htmlDiffReporter != nil {
		reportPaths, err := s.urlChecker.htmlDiffReporter.GenerateDiffReport(monitoredURLs, cycleID)
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to generate cycle end diff report")
		} else if len(reportPaths) > 0 {
			// Use the first report path for notification (main report)
			reportPath = reportPaths[0]
			s.logger.Info().
				Str("main_report_path", reportPath).
				Int("total_reports", len(reportPaths)).
				Msg("Generated cycle end diff report(s)")
		}
	} else if len(changedURLs) == 0 {
		s.logger.Info().Int("monitored_count", len(monitoredURLs)).Msg("No changes detected - sending notification without report")
	} else {
		s.logger.Warn().Msg("HtmlDiffReporter is not available, sending notification without report")
	}

	// Always send cycle complete notification
	s.sendCycleCompleteNotification(cycleID, changedURLs, reportPath, len(monitoredURLs))
}

// sendCycleCompleteNotification sends a notification when a monitoring cycle completes
func (s *MonitoringService) sendCycleCompleteNotification(cycleID string, changedURLs []string, reportPath string, totalMonitored int) {
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
		ReportPath:     reportPath,
		TotalMonitored: totalMonitored,
		Timestamp:      time.Now(),
		BatchStats:     batchStats,
	}
	s.notificationHelper.SendMonitorCycleCompleteNotification(s.serviceCtx, data)
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
