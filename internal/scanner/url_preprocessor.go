package scanner

import (
	"runtime"
	"sync"

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
	MaxWorkers       int                               `json:"max_workers,omitempty" yaml:"max_workers,omitempty"`
	EnableParallel   bool                              `json:"enable_parallel" yaml:"enable_parallel"`
}

// DefaultURLPreprocessorConfig returns default configuration
func DefaultURLPreprocessorConfig() URLPreprocessorConfig {
	return URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   true,
		BatchSize:        1000,
		MaxWorkers:       0, // Will be set based on crawler threads
		EnableParallel:   true,
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
	// Determine if we should use parallel processing
	maxWorkers := up.getEffectiveMaxWorkers()

	if up.config.EnableParallel && maxWorkers > 1 && len(inputURLs) > maxWorkers*10 {
		return up.processParallel(inputURLs, maxWorkers)
	}

	// Fall back to sequential batching
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

// getEffectiveMaxWorkers returns the effective number of workers to use
func (up *URLPreprocessor) getEffectiveMaxWorkers() int {
	if up.config.MaxWorkers > 0 {
		return up.config.MaxWorkers
	}
	// Default to number of CPU cores if not set
	return runtime.NumCPU()
}

// processParallel processes URLs using multiple workers in parallel
func (up *URLPreprocessor) processParallel(inputURLs []string, maxWorkers int) []string {
	up.logger.Info().
		Int("url_count", len(inputURLs)).
		Int("workers", maxWorkers).
		Msg("Starting parallel URL preprocessing")

	// Create work channel with buffer
	urlChan := make(chan string, maxWorkers*2)

	// Create result channel to collect processed URLs
	resultChan := make(chan string, len(inputURLs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go up.parallelWorker(urlChan, resultChan, &wg, i)
	}

	// Send URLs to workers
	go func() {
		defer close(urlChan)
		for _, url := range inputURLs {
			urlChan <- url
		}
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make([]string, 0, len(inputURLs))
	processedCount := 0

	for processedURL := range resultChan {
		if processedURL != "" {
			results = append(results, processedURL)
			processedCount++
		}
	}

	up.logger.Info().
		Int("processed_count", processedCount).
		Int("workers_used", maxWorkers).
		Msg("Parallel URL preprocessing completed")

	return results
}

// parallelWorker processes URLs from the work channel
func (up *URLPreprocessor) parallelWorker(urlChan <-chan string, resultChan chan<- string, wg *sync.WaitGroup, workerID int) {
	defer wg.Done()

	up.logger.Debug().
		Int("worker_id", workerID).
		Msg("Starting URL preprocessing worker")

	processed := 0
	for rawURL := range urlChan {
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

		// Step 2: Check for duplicates (thread-safe)
		if up.statsTracker.IsURLSeen(normalizedURL) {
			up.statsTracker.IncrementSkippedDuplicate()
			continue
		}

		// Step 3: Auto-calibrate filtering (thread-safe)
		if up.config.AutoCalibrate.Enabled && up.patternDetector.ShouldSkipByPattern(normalizedURL) {
			up.statsTracker.IncrementSkippedByPattern()
			continue
		}

		// Mark URL as seen and send result (thread-safe)
		up.statsTracker.MarkURLSeen(normalizedURL)

		select {
		case resultChan <- normalizedURL:
			processed++
		default:
			up.logger.Warn().Str("url", normalizedURL).Msg("Result channel full, dropping URL")
		}
	}

	up.logger.Debug().
		Int("worker_id", workerID).
		Int("processed", processed).
		Msg("URL preprocessing worker completed")
}

// SetMaxWorkers sets the maximum number of workers for parallel processing
func (up *URLPreprocessor) SetMaxWorkers(maxWorkers int) {
	up.config.MaxWorkers = maxWorkers
	up.logger.Info().
		Int("max_workers", maxWorkers).
		Msg("URL preprocessor max workers updated")
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
