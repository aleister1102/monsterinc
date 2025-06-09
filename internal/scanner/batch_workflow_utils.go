package scanner

import (
	"runtime"

	"github.com/aleister1102/monsterinc/internal/config"
)

// optimizeConfigForMemoryEfficiency tự động điều chỉnh config để tiết kiệm memory
func (bwo *BatchWorkflowOrchestrator) optimizeConfigForMemoryEfficiency(gCfg *config.GlobalConfig, batchSize int) {
	// Giảm concurrent requests cho crawler để tránh memory spike
	if gCfg.CrawlerConfig.MaxConcurrentRequests > 10 {
		originalConcurrent := gCfg.CrawlerConfig.MaxConcurrentRequests
		gCfg.CrawlerConfig.MaxConcurrentRequests = 10
		bwo.logger.Info().
			Int("original_concurrent", originalConcurrent).
			Int("optimized_concurrent", 10).
			Msg("Reduced crawler concurrent requests for memory efficiency")
	}

	// Giảm httpx threads nếu cần
	if gCfg.HttpxRunnerConfig.Threads > 30 {
		originalThreads := gCfg.HttpxRunnerConfig.Threads
		gCfg.HttpxRunnerConfig.Threads = 30
		bwo.logger.Info().
			Int("original_threads", originalThreads).
			Int("optimized_threads", 30).
			Msg("Reduced httpx threads for memory efficiency")
	}

	// Log optimization info
	bwo.logger.Info().
		Int("batch_size", batchSize).
		Int("crawler_concurrent", gCfg.CrawlerConfig.MaxConcurrentRequests).
		Int("httpx_threads", gCfg.HttpxRunnerConfig.Threads).
		Msg("Configuration optimized for memory efficiency during batch processing")
}

// logBatchProcessingSummary logs comprehensive summary of batch processing
func (bwo *BatchWorkflowOrchestrator) logBatchProcessingSummary(result *BatchScanResult) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	bwo.logger.Info().
		Int("total_batches", result.TotalBatches).
		Int("processed_batches", result.ProcessedBatches).
		Int("total_targets", result.SummaryData.TotalTargets).
		Int("successful_probes", result.SummaryData.ProbeStats.SuccessfulProbes).
		Int("failed_probes", result.SummaryData.ProbeStats.FailedProbes).
		Int("new_urls", result.SummaryData.DiffStats.New).
		Int("existing_urls", result.SummaryData.DiffStats.Existing).
		Int("old_urls", result.SummaryData.DiffStats.Old).
		Dur("total_duration", result.SummaryData.ScanDuration).
		Int("report_files", len(result.ReportFilePaths)).
		Bool("was_interrupted", result.InterruptedAt > 0).
		Uint64("final_memory_mb", memStats.Alloc/1024/1024).
		Str("status", result.SummaryData.Status).
		Msg("Batch processing summary completed")

	if result.InterruptedAt > 0 {
		bwo.logger.Warn().
			Int("interrupted_at_batch", result.InterruptedAt).
			Int("completed_batches", result.ProcessedBatches).
			Float64("completion_percentage", float64(result.ProcessedBatches)/float64(result.TotalBatches)*100).
			Msg("Batch processing was interrupted")
	}
}
