package scanner

import (
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// URLPreprocessorConfig configures URL preprocessing behavior
type URLPreprocessorConfig struct {
	URLNormalization urlhandler.URLNormalizationConfig `json:"url_normalization,omitempty" yaml:"url_normalization,omitempty"`
	AutoCalibrate    config.AutoCalibrateConfig        `json:"auto_calibrate,omitempty" yaml:"auto_calibrate,omitempty"`
	EnableBatching   bool                              `json:"enable_batching" yaml:"enable_batching"`
	BatchSize        int                               `json:"batch_size,omitempty" yaml:"batch_size,omitempty"`
}

// DefaultURLPreprocessorConfig returns default configuration
func DefaultURLPreprocessorConfig() URLPreprocessorConfig {
	return URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   true,
		BatchSize:        1000,
	}
}

// URLPreprocessor handles URL normalization and auto-calibrate filtering
type URLPreprocessor struct {
	config          URLPreprocessorConfig
	logger          zerolog.Logger
	normalizer      *urlhandler.URLNormalizer
	patternDetector *URLPatternDetector
	statsTracker    *URLStatsTracker
}

// URLPreprocessorResult contains the result of URL preprocessing
type URLPreprocessorResult struct {
	ProcessedURLs []string             `json:"processed_urls"`
	Stats         URLPreprocessorStats `json:"stats"`
	PatternStats  map[string]int       `json:"pattern_stats,omitempty"`
}

// NewURLPreprocessor creates a new URL preprocessor
func NewURLPreprocessor(config URLPreprocessorConfig, logger zerolog.Logger) *URLPreprocessor {
	preprocessorLogger := logger.With().Str("component", "URLPreprocessor").Logger()

	return &URLPreprocessor{
		config:          config,
		logger:          preprocessorLogger,
		normalizer:      urlhandler.NewURLNormalizer(config.URLNormalization),
		patternDetector: NewURLPatternDetector(config.AutoCalibrate, preprocessorLogger),
		statsTracker:    NewURLStatsTracker(preprocessorLogger),
	}
}

// PreprocessURLs normalizes and filters URLs before scanning
func (up *URLPreprocessor) PreprocessURLs(inputURLs []string) *URLPreprocessorResult {
	up.logger.Info().
		Int("input_count", len(inputURLs)).
		Bool("normalization_enabled", true).
		Bool("auto_calibrate_enabled", up.config.AutoCalibrate.Enabled).
		Msg("Starting URL preprocessing")

	// Reset stats
	up.statsTracker.ResetStats()

	var processedURLs []string

	if up.config.EnableBatching && len(inputURLs) > up.config.BatchSize {
		processedURLs = up.processBatched(inputURLs)
	} else {
		processedURLs = up.processSequential(inputURLs)
	}

	// Update final count
	up.statsTracker.SetFinalCount(len(processedURLs))

	up.statsTracker.LogProcessingResults()

	return &URLPreprocessorResult{
		ProcessedURLs: processedURLs,
		Stats:         up.statsTracker.GetStats(),
		PatternStats:  up.patternDetector.GetPatternStats(),
	}
}

// processBatched processes URLs in batches for memory efficiency
func (up *URLPreprocessor) processBatched(inputURLs []string) []string {
	var processedURLs []string

	for i := 0; i < len(inputURLs); i += up.config.BatchSize {
		end := i + up.config.BatchSize
		if end > len(inputURLs) {
			end = len(inputURLs)
		}

		batch := inputURLs[i:end]
		batchResult := up.processSequential(batch)
		processedURLs = append(processedURLs, batchResult...)

		up.logger.Debug().
			Int("batch_start", i).
			Int("batch_end", end).
			Int("batch_processed", len(batchResult)).
			Msg("Processed URL batch")
	}

	return processedURLs
}

// processSequential processes URLs sequentially
func (up *URLPreprocessor) processSequential(inputURLs []string) []string {
	var processedURLs []string

	for _, rawURL := range inputURLs {
		up.statsTracker.IncrementProcessed()

		// Step 1: Normalize URL
		normalizedURL, err := up.normalizer.NormalizeURL(rawURL)
		if err != nil {
			up.logger.Debug().Err(err).Str("url", rawURL).Msg("Failed to normalize URL, skipping")
			continue
		}

		// If URL changed during normalization, count it
		if normalizedURL != rawURL {
			up.statsTracker.IncrementNormalized()
		}

		// Step 2: Check for duplicates
		if up.statsTracker.IsURLSeen(normalizedURL) {
			up.statsTracker.IncrementSkippedDuplicate()
			continue
		}

		// Step 3: Auto-calibrate filtering
		if up.config.AutoCalibrate.Enabled && up.patternDetector.ShouldSkipByPattern(normalizedURL) {
			up.statsTracker.IncrementSkippedByPattern()
			continue
		}

		// Mark URL as seen and add to results
		up.statsTracker.MarkURLSeen(normalizedURL)
		processedURLs = append(processedURLs, normalizedURL)
	}

	return processedURLs
}
