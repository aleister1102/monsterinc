package summary

// BatchInfo holds information about a monitoring batch
type BatchInfo struct {
	BatchNumber      int `json:"batch_number"`       // Current batch number (1-based)
	TotalBatches     int `json:"total_batches"`      // Total number of batches in the cycle
	BatchSize        int `json:"batch_size"`         // Number of URLs in this batch
	ProcessedInBatch int `json:"processed_in_batch"` // Number of URLs processed in this batch so far
}

// NewBatchInfo creates a new BatchInfo instance
func NewBatchInfo(batchNumber, totalBatches, batchSize, processedInBatch int) *BatchInfo {
	return &BatchInfo{
		BatchNumber:      batchNumber,
		TotalBatches:     totalBatches,
		BatchSize:        batchSize,
		ProcessedInBatch: processedInBatch,
	}
}
