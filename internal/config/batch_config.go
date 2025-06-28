package config

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
)

// ScanBatchConfig defines configuration for scan batch processing
type ScanBatchConfig struct {
	BatchSize          int `json:"batch_size,omitempty" yaml:"batch_size,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentBatch int `json:"max_concurrent_batch,omitempty" yaml:"max_concurrent_batch,omitempty" validate:"omitempty,min=1"`
	BatchTimeoutMins   int `json:"batch_timeout_mins,omitempty" yaml:"batch_timeout_mins,omitempty" validate:"omitempty,min=1"`
	ThresholdSize      int `json:"threshold_size,omitempty" yaml:"threshold_size,omitempty" validate:"omitempty,min=1"`
}

// NewDefaultScanBatchConfig creates default scan batch configuration
func NewDefaultScanBatchConfig() ScanBatchConfig {
	return ScanBatchConfig{
		BatchSize:          200,  // Larger batch size for scan service
		MaxConcurrentBatch: 0,    // Will be set based on crawler thread count
		BatchTimeoutMins:   45,   // Longer timeout for scan service
		ThresholdSize:      1000, // Higher threshold for scan service
	}
}

// BatchConfig interface for converting to common.BatchProcessorConfig
type BatchConfig interface {
	ToBatchProcessorConfig() common.BatchProcessorConfig
}

// ToBatchProcessorConfig converts ScanBatchConfig to common.BatchProcessorConfig
func (sbc ScanBatchConfig) ToBatchProcessorConfig() common.BatchProcessorConfig {
	return common.BatchProcessorConfig{
		BatchSize:          sbc.BatchSize,
		MaxConcurrentBatch: sbc.MaxConcurrentBatch,
		BatchTimeout:       time.Duration(sbc.BatchTimeoutMins) * time.Minute,
		ThresholdSize:      sbc.ThresholdSize,
	}
}

// SetMaxConcurrentFromCrawlerThreads sets MaxConcurrentBatch based on crawler thread count
func (sbc *ScanBatchConfig) SetMaxConcurrentFromCrawlerThreads(crawlerThreads int) {
	if sbc.MaxConcurrentBatch == 0 {
		// Use 50% of crawler threads for batch concurrency, minimum 1, maximum 8
		maxConcurrent := crawlerThreads / 2
		if maxConcurrent < 1 {
			maxConcurrent = 1
		}
		if maxConcurrent > 8 {
			maxConcurrent = 8
		}
		sbc.MaxConcurrentBatch = maxConcurrent
	}
}

// GetEffectiveMaxConcurrentBatch returns the effective MaxConcurrentBatch value
func (sbc ScanBatchConfig) GetEffectiveMaxConcurrentBatch() int {
	if sbc.MaxConcurrentBatch <= 0 {
		return 2 // fallback default
	}
	return sbc.MaxConcurrentBatch
}
