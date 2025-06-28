package scanner

import (
	"testing"
	"time"

	"github.com/aleister1102/monsterinc/internal/common/urlhandler"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

func TestURLPreprocessor_ParallelProcessing(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	// Create config with parallel processing enabled
	preprocessorConfig := URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   true,
		BatchSize:        50,
		MaxWorkers:       4,
		EnableParallel:   true,
	}

	preprocessor := NewURLPreprocessor(preprocessorConfig, logger)

	// Create a large number of test URLs to trigger parallel processing
	testURLs := make([]string, 100)
	for i := 0; i < 100; i++ {
		testURLs[i] = "https://example.com/page" + string(rune(i+65)) // Start from 'A' to avoid control chars
	}

	// Test parallel processing
	start := time.Now()
	result := preprocessor.PreprocessURLs(testURLs)
	duration := time.Since(start)

	// Verify results
	if len(result.ProcessedURLs) == 0 {
		t.Errorf("Expected processed URLs, got none")
	}

	if result.Stats.TotalProcessed != 100 {
		t.Errorf("Expected 100 processed URLs, got %d", result.Stats.TotalProcessed)
	}

	t.Logf("Parallel processing took %v for %d URLs", duration, len(testURLs))
	t.Logf("Processed %d URLs, final count: %d", result.Stats.TotalProcessed, result.Stats.FinalCount)
}

func TestURLPreprocessor_SetMaxWorkers(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	preprocessorConfig := URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   true,
		BatchSize:        50,
		MaxWorkers:       2,
		EnableParallel:   true,
	}

	preprocessor := NewURLPreprocessor(preprocessorConfig, logger)

	// Test initial workers
	if preprocessor.getEffectiveMaxWorkers() != 2 {
		t.Errorf("Expected 2 workers, got %d", preprocessor.getEffectiveMaxWorkers())
	}

	// Test setting new worker count
	preprocessor.SetMaxWorkers(8)
	if preprocessor.getEffectiveMaxWorkers() != 8 {
		t.Errorf("Expected 8 workers after SetMaxWorkers, got %d", preprocessor.getEffectiveMaxWorkers())
	}
}

func TestURLPreprocessor_ParallelVsSequential(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()

	// Create test URLs
	testURLs := make([]string, 200)
	for i := 0; i < 200; i++ {
		testURLs[i] = "https://example.com/test" + string(rune(i%26+65)) // Cycle through A-Z
	}

	// Test sequential processing
	seqConfig := URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   false,
		MaxWorkers:       1,
		EnableParallel:   false,
	}
	seqPreprocessor := NewURLPreprocessor(seqConfig, logger)

	start := time.Now()
	seqResult := seqPreprocessor.PreprocessURLs(testURLs)
	seqDuration := time.Since(start)

	// Test parallel processing
	parConfig := URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   true,
		BatchSize:        50,
		MaxWorkers:       4,
		EnableParallel:   true,
	}
	parPreprocessor := NewURLPreprocessor(parConfig, logger)

	start = time.Now()
	parResult := parPreprocessor.PreprocessURLs(testURLs)
	parDuration := time.Since(start)

	// Verify both produce same number of results
	if len(seqResult.ProcessedURLs) != len(parResult.ProcessedURLs) {
		t.Errorf("Sequential and parallel processing produced different result counts: seq=%d, par=%d",
			len(seqResult.ProcessedURLs), len(parResult.ProcessedURLs))
	}

	t.Logf("Sequential processing: %v for %d URLs", seqDuration, len(testURLs))
	t.Logf("Parallel processing: %v for %d URLs", parDuration, len(testURLs))

	// Note: We don't assert that parallel is faster since it depends on system resources
	// and the overhead might not be worth it for small test datasets
}

func BenchmarkURLPreprocessor_Sequential(b *testing.B) {
	logger := zerolog.Nop()

	config := URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   false,
		MaxWorkers:       1,
		EnableParallel:   false,
	}
	preprocessor := NewURLPreprocessor(config, logger)

	testURLs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testURLs[i] = "https://example.com/benchmark" + string(rune(i+1))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.PreprocessURLs(testURLs)
	}
}

func BenchmarkURLPreprocessor_Parallel(b *testing.B) {
	logger := zerolog.Nop()

	config := URLPreprocessorConfig{
		URLNormalization: urlhandler.DefaultURLNormalizationConfig(),
		AutoCalibrate:    config.NewDefaultAutoCalibrateConfig(),
		EnableBatching:   true,
		BatchSize:        100,
		MaxWorkers:       4,
		EnableParallel:   true,
	}
	preprocessor := NewURLPreprocessor(config, logger)

	testURLs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testURLs[i] = "https://example.com/benchmark" + string(rune(i+1))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.PreprocessURLs(testURLs)
	}
}
