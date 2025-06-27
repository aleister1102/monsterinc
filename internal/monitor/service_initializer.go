package monitor

import (
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/extractor"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/reporter"
	httpclient "github.com/aleister1102/go-comet"
	limiter "github.com/aleister1102/go-rslimiter"
	"github.com/rs/zerolog"
)

// validateMonitoringConfig validates the monitoring configuration
func validateMonitoringConfig(gCfg *config.GlobalConfig) error {
	if gCfg == nil || !gCfg.MonitorConfig.Enabled {
		return fmt.Errorf("global configuration is nil or monitoring is disabled")
	}
	return nil
}

// initializeResourceLimiter creates and configures the resource limiter
func initializeResourceLimiter(gCfg *config.GlobalConfig, logger zerolog.Logger) *limiter.ResourceLimiter {
	// Convert config to limiter.ResourceLimiterConfig
	limiterConfig := limiter.ResourceLimiterConfig{
		MaxMemoryMB:        gCfg.ResourceLimiterConfig.MaxMemoryMB,
		MaxGoroutines:      gCfg.ResourceLimiterConfig.MaxGoroutines,
		CheckInterval:      time.Duration(gCfg.ResourceLimiterConfig.CheckIntervalSecs) * time.Second,
		MemoryThreshold:    gCfg.ResourceLimiterConfig.MemoryThreshold,
		GoroutineWarning:   gCfg.ResourceLimiterConfig.GoroutineWarning,
		SystemMemThreshold: gCfg.ResourceLimiterConfig.SystemMemThreshold,
		CPUThreshold:       gCfg.ResourceLimiterConfig.CPUThreshold,
		EnableAutoShutdown: gCfg.ResourceLimiterConfig.EnableAutoShutdown,
	}

	return limiter.NewResourceLimiter(limiterConfig, logger)
}

// initializeHistoryStore creates and configures the history store
func initializeHistoryStore(gCfg *config.GlobalConfig, logger zerolog.Logger) (*datastore.ParquetFileHistory, error) {
	historyStore, err := datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ParquetFileHistoryStore: %w", err)
	}
	return historyStore, nil
}

// initializeHTTPFetcher creates and configures the HTTP fetcher
func initializeHTTPFetcher(gCfg *config.GlobalConfig, logger zerolog.Logger) (*httpclient.Fetcher, error) {
	httpTimeout := determineHTTPTimeout(gCfg, logger)

	httpClientBuilder := httpclient.NewHTTPClientBuilder(logger).
		WithTimeout(httpTimeout).
		WithInsecureSkipVerify(gCfg.MonitorConfig.MonitorInsecureSkipVerify).
		WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36").
		WithFollowRedirects(true).
		WithMaxRedirects(5).
		WithConnectionPooling(50, 10, 0).
		WithHTTP2(true)

	httpClient, err := httpClientBuilder.Build()
	if err != nil {
		return nil, WrapError(err, "failed to create HTTP client for monitoring")
	}

	fetcherConfig := &httpclient.HTTPClientFetcherConfig{
		MaxContentSize: gCfg.MonitorConfig.MaxContentSize,
	}

	return httpclient.NewFetcher(httpClient, logger, fetcherConfig), nil
}

// determineHTTPTimeout determines the HTTP timeout from configuration
func determineHTTPTimeout(gCfg *config.GlobalConfig, logger zerolog.Logger) time.Duration {
	timeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
		logger.Warn().
			Int("configured_timeout", gCfg.MonitorConfig.HTTPTimeoutSeconds).
			Dur("default_timeout", timeout).
			Msg("Invalid timeout configured, using default")
	}
	return timeout
}

// initializeContentDiffer creates the content differ
func initializeContentDiffer(gCfg *config.GlobalConfig, logger zerolog.Logger) *differ.ContentDiffer {
	contentDiffer, err := differ.NewContentDiffer(logger, &gCfg.DiffReporterConfig)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create content differ, using basic differ")
		return nil
	}
	return contentDiffer
}

// initializePathExtractor creates the path extractor
func initializePathExtractor(gCfg *config.GlobalConfig, logger zerolog.Logger) *extractor.PathExtractor {
	pathExtractor, err := extractor.NewPathExtractor(gCfg.ExtractorConfig, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create path extractor")
		return nil
	}
	return pathExtractor
}

// initializeHtmlDiffReporter creates the HTML diff reporter
func initializeHtmlDiffReporter(
	gCfg *config.GlobalConfig,
	historyStore models.FileHistoryStore,
	logger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) *reporter.HtmlDiffReporter {
	if historyStore == nil {
		return nil
	}

	htmlDiffReporter, err := reporter.NewHtmlDiffReporter(logger, historyStore, &gCfg.MonitorConfig)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create HTML diff reporter")
		return nil
	}

	if notificationHelper != nil {
		notificationHelper.SetDiffReportCleaner(htmlDiffReporter)
	}

	return htmlDiffReporter
}

// createInitialCycleTracker creates the initial cycle tracker
func createInitialCycleTracker() *CycleTracker {
	// The concept of an initial cycle ID is deprecated.
	// The tracker is now initialized with max cycles from config.
	// Providing a default of 0 (infinite) to fix compilation.
	return NewCycleTracker(0)
}
