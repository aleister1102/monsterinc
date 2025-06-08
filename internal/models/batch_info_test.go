package models

import (
	"testing"
)

func TestNewBatchInfo(t *testing.T) {
	batchInfo := NewBatchInfo(2, 5, 10, 3)

	if batchInfo.BatchNumber != 2 {
		t.Errorf("Expected BatchNumber 2, got %d", batchInfo.BatchNumber)
	}

	if batchInfo.TotalBatches != 5 {
		t.Errorf("Expected TotalBatches 5, got %d", batchInfo.TotalBatches)
	}

	if batchInfo.BatchSize != 10 {
		t.Errorf("Expected BatchSize 10, got %d", batchInfo.BatchSize)
	}

	if batchInfo.ProcessedInBatch != 3 {
		t.Errorf("Expected ProcessedInBatch 3, got %d", batchInfo.ProcessedInBatch)
	}
}

func TestNewBatchStats(t *testing.T) {
	batchStats := NewBatchStats(true, 3, 2, 10, 15, 25)

	if !batchStats.UsedBatching {
		t.Error("Expected UsedBatching to be true")
	}

	if batchStats.TotalBatches != 3 {
		t.Errorf("Expected TotalBatches 3, got %d", batchStats.TotalBatches)
	}

	if batchStats.CompletedBatches != 2 {
		t.Errorf("Expected CompletedBatches 2, got %d", batchStats.CompletedBatches)
	}

	if batchStats.AvgBatchSize != 10 {
		t.Errorf("Expected AvgBatchSize 10, got %d", batchStats.AvgBatchSize)
	}

	if batchStats.MaxBatchSize != 15 {
		t.Errorf("Expected MaxBatchSize 15, got %d", batchStats.MaxBatchSize)
	}

	if batchStats.TotalURLsProcessed != 25 {
		t.Errorf("Expected TotalURLsProcessed 25, got %d", batchStats.TotalURLsProcessed)
	}
}

func TestFileChangeInfoWithBatchInfo(t *testing.T) {
	batchInfo := NewBatchInfo(1, 3, 5, 2)

	fileChange := FileChangeInfo{
		URL:       "https://example.com/test.js",
		OldHash:   "old123",
		NewHash:   "new456",
		CycleID:   "cycle-123",
		BatchInfo: batchInfo,
	}

	if fileChange.BatchInfo == nil {
		t.Error("Expected BatchInfo to be set")
	}

	if fileChange.BatchInfo.BatchNumber != 1 {
		t.Errorf("Expected BatchNumber 1, got %d", fileChange.BatchInfo.BatchNumber)
	}
}

func TestMonitorFetchErrorInfoWithBatchInfo(t *testing.T) {
	batchInfo := NewBatchInfo(2, 4, 8, 5)

	errorInfo := MonitorFetchErrorInfo{
		URL:       "https://example.com/error.js",
		Error:     "Connection timeout",
		CycleID:   "cycle-456",
		BatchInfo: batchInfo,
	}

	if errorInfo.BatchInfo == nil {
		t.Error("Expected BatchInfo to be set")
	}

	if errorInfo.BatchInfo.TotalBatches != 4 {
		t.Errorf("Expected TotalBatches 4, got %d", errorInfo.BatchInfo.TotalBatches)
	}
}
