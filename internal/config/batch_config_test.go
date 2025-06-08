package config

import (
	"testing"
)

func TestScanBatchConfig_SetMaxConcurrentFromCrawlerThreads(t *testing.T) {
	tests := []struct {
		name           string
		initialConfig  ScanBatchConfig
		crawlerThreads int
		expectedMax    int
	}{
		{
			name:           "Zero initial config with 10 threads",
			initialConfig:  ScanBatchConfig{MaxConcurrentBatch: 0},
			crawlerThreads: 10,
			expectedMax:    5, // 50% of 10
		},
		{
			name:           "Zero initial config with 2 threads",
			initialConfig:  ScanBatchConfig{MaxConcurrentBatch: 0},
			crawlerThreads: 2,
			expectedMax:    1, // minimum 1
		},
		{
			name:           "Zero initial config with 20 threads",
			initialConfig:  ScanBatchConfig{MaxConcurrentBatch: 0},
			crawlerThreads: 20,
			expectedMax:    8, // maximum 8
		},
		{
			name:           "Already set config should not change",
			initialConfig:  ScanBatchConfig{MaxConcurrentBatch: 3},
			crawlerThreads: 10,
			expectedMax:    3, // unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.initialConfig
			config.SetMaxConcurrentFromCrawlerThreads(tt.crawlerThreads)

			if config.MaxConcurrentBatch != tt.expectedMax {
				t.Errorf("SetMaxConcurrentFromCrawlerThreads() = %d, want %d",
					config.MaxConcurrentBatch, tt.expectedMax)
			}
		})
	}
}

func TestMonitorBatchConfig_SetMaxConcurrentFromMonitorWorkers(t *testing.T) {
	tests := []struct {
		name           string
		initialConfig  MonitorBatchConfig
		monitorWorkers int
		expectedMax    int
	}{
		{
			name:           "Zero initial config with 10 workers",
			initialConfig:  MonitorBatchConfig{MaxConcurrentBatch: 0},
			monitorWorkers: 10,
			expectedMax:    4, // maximum 4
		},
		{
			name:           "Zero initial config with 2 workers",
			initialConfig:  MonitorBatchConfig{MaxConcurrentBatch: 0},
			monitorWorkers: 2,
			expectedMax:    1, // minimum 1
		},
		{
			name:           "Zero initial config with 6 workers",
			initialConfig:  MonitorBatchConfig{MaxConcurrentBatch: 0},
			monitorWorkers: 6,
			expectedMax:    3, // 50% of 6
		},
		{
			name:           "Already set config should not change",
			initialConfig:  MonitorBatchConfig{MaxConcurrentBatch: 2},
			monitorWorkers: 10,
			expectedMax:    2, // unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.initialConfig
			config.SetMaxConcurrentFromMonitorWorkers(tt.monitorWorkers)

			if config.MaxConcurrentBatch != tt.expectedMax {
				t.Errorf("SetMaxConcurrentFromMonitorWorkers() = %d, want %d",
					config.MaxConcurrentBatch, tt.expectedMax)
			}
		})
	}
}

func TestGetEffectiveMaxConcurrentBatch(t *testing.T) {
	t.Run("ScanBatchConfig effective value", func(t *testing.T) {
		// Test zero value returns fallback
		config := ScanBatchConfig{MaxConcurrentBatch: 0}
		if got := config.GetEffectiveMaxConcurrentBatch(); got != 2 {
			t.Errorf("GetEffectiveMaxConcurrentBatch() = %d, want 2", got)
		}

		// Test positive value returns itself
		config = ScanBatchConfig{MaxConcurrentBatch: 5}
		if got := config.GetEffectiveMaxConcurrentBatch(); got != 5 {
			t.Errorf("GetEffectiveMaxConcurrentBatch() = %d, want 5", got)
		}
	})

	t.Run("MonitorBatchConfig effective value", func(t *testing.T) {
		// Test zero value returns fallback
		config := MonitorBatchConfig{MaxConcurrentBatch: 0}
		if got := config.GetEffectiveMaxConcurrentBatch(); got != 1 {
			t.Errorf("GetEffectiveMaxConcurrentBatch() = %d, want 1", got)
		}

		// Test positive value returns itself
		config = MonitorBatchConfig{MaxConcurrentBatch: 3}
		if got := config.GetEffectiveMaxConcurrentBatch(); got != 3 {
			t.Errorf("GetEffectiveMaxConcurrentBatch() = %d, want 3", got)
		}
	})
}
