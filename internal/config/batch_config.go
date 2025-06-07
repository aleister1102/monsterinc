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
		MaxConcurrentBatch: 2,    // Higher concurrency for scan service
		BatchTimeoutMins:   45,   // Longer timeout for scan service
		ThresholdSize:      1000, // Higher threshold for scan service
	}
}

// MonitorBatchConfig defines configuration for monitor batch processing
type MonitorBatchConfig struct {
	BatchSize          int `json:"batch_size,omitempty" yaml:"batch_size,omitempty" validate:"omitempty,min=1"`
	MaxConcurrentBatch int `json:"max_concurrent_batch,omitempty" yaml:"max_concurrent_batch,omitempty" validate:"omitempty,min=1"`
	BatchTimeoutMins   int `json:"batch_timeout_mins,omitempty" yaml:"batch_timeout_mins,omitempty" validate:"omitempty,min=1"`
	ThresholdSize      int `json:"threshold_size,omitempty" yaml:"threshold_size,omitempty" validate:"omitempty,min=1"`
}

// NewDefaultMonitorBatchConfig creates default monitor batch configuration
func NewDefaultMonitorBatchConfig() MonitorBatchConfig {
	return MonitorBatchConfig{
		BatchSize:          50,  // Smaller batch size for monitor service
		MaxConcurrentBatch: 1,   // Sequential processing for monitor service
		BatchTimeoutMins:   20,  // Shorter timeout for monitor service
		ThresholdSize:      200, // Lower threshold for monitor service
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

// ToBatchProcessorConfig converts MonitorBatchConfig to common.BatchProcessorConfig
func (mbc MonitorBatchConfig) ToBatchProcessorConfig() common.BatchProcessorConfig {
	return common.BatchProcessorConfig{
		BatchSize:          mbc.BatchSize,
		MaxConcurrentBatch: mbc.MaxConcurrentBatch,
		BatchTimeout:       time.Duration(mbc.BatchTimeoutMins) * time.Minute,
		ThresholdSize:      mbc.ThresholdSize,
	}
}
