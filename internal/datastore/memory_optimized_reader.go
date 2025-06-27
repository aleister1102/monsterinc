package datastore

import (
	"context"
	"io"
	"os"

	httpx "github.com/aleister1102/go-telescope"
	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// StreamingParquetReader provides memory-efficient streaming reads
type StreamingParquetReader struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	fileManager   *common.FileManager
	bufferPool    *common.BufferPool
}

// NewStreamingParquetReader creates a new streaming parquet reader
func NewStreamingParquetReader(cfg *config.StorageConfig, logger zerolog.Logger) *StreamingParquetReader {
	return &StreamingParquetReader{
		storageConfig: cfg,
		logger:        logger.With().Str("component", "StreamingParquetReader").Logger(),
		fileManager:   common.NewFileManager(logger),
		bufferPool:    common.NewBufferPool(1024 * 1024), // 1MB buffers
	}
}

// StreamProbeResultsCallback defines callback function for streaming results
type StreamProbeResultsCallback func(result httpx.ProbeResult) error

// StreamFileHistoryCallback defines callback function for streaming file history
type StreamFileHistoryCallback func(record models.FileHistoryRecord) error

// StreamProbeResults streams probe results without loading all into memory
func (spr *StreamingParquetReader) StreamProbeResults(
	ctx context.Context,
	rootTargetURL string,
	callback StreamProbeResultsCallback,
) error {
	filePath, err := spr.buildParquetFilePath(rootTargetURL)
	if err != nil {
		return err
	}

	file, err := spr.openParquetFile(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			spr.logger.Error().Err(closeErr).Str("file", filePath).Msg("Failed to close parquet file")
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return WrapError(err, "failed to stat parquet file")
	}

	pqFile, err := parquet.OpenFile(file, stat.Size())
	if err != nil {
		return WrapError(err, "failed to open parquet file for reading")
	}

	reader := parquet.NewReader(pqFile)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var parquetResult models.ParquetProbeResult
		if err := reader.Read(&parquetResult); err != nil {
			if err == io.EOF {
				break
			}
			return WrapError(err, "failed to read parquet record")
		}

		// Convert and call callback
		probeResult := parquetResult.ToProbeResult()
		if err := callback(probeResult); err != nil {
			return WrapError(err, "callback error during streaming")
		}
	}

	return nil
}

// StreamFileHistory streams file history records without loading all into memory
func (spr *StreamingParquetReader) StreamFileHistory(
	ctx context.Context,
	filePath string,
	callback StreamFileHistoryCallback,
) error {
	file, err := spr.openParquetFile(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			spr.logger.Error().Err(closeErr).Str("file", filePath).Msg("Failed to close parquet file")
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return WrapError(err, "failed to stat parquet file")
	}

	pqFile, err := parquet.OpenFile(file, stat.Size())
	if err != nil {
		return WrapError(err, "failed to open parquet file for reading")
	}

	reader := parquet.NewReader(pqFile)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var record models.FileHistoryRecord
		if err := reader.Read(&record); err != nil {
			if err == io.EOF {
				break
			}
			return WrapError(err, "failed to read file history record")
		}

		if err := callback(record); err != nil {
			return WrapError(err, "callback error during streaming")
		}
	}

	return nil
}

// CountRecords efficiently counts records without loading them into memory
func (spr *StreamingParquetReader) CountRecords(ctx context.Context, filePath string) (int64, error) {
	file, err := spr.openParquetFile(filePath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			spr.logger.Error().Err(closeErr).Str("file", filePath).Msg("Failed to close parquet file")
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return 0, WrapError(err, "failed to stat parquet file")
	}

	pqFile, err := parquet.OpenFile(file, stat.Size())
	if err != nil {
		return 0, WrapError(err, "failed to open parquet file for reading")
	}

	// Get total number of rows efficiently
	return pqFile.NumRows(), nil
}

// Helper methods

func (spr *StreamingParquetReader) buildParquetFilePath(rootTargetURL string) (string, error) {
	// Implementation similar to ParquetReader.buildParquetFilePath
	urlHashGen := NewURLHashGenerator(16)
	urlHash := urlHashGen.GenerateHash(rootTargetURL)

	return spr.storageConfig.ParquetBasePath + "/" + urlHash + "_results.parquet", nil
}

func (spr *StreamingParquetReader) openParquetFile(filePath string) (*os.File, error) {
	if !spr.fileManager.FileExists(filePath) {
		return nil, NewError("parquet file does not exist: " + filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, WrapError(err, "failed to open parquet file")
	}

	return file, nil
}

// BatchProcessor for processing results in batches
type BatchProcessor struct {
	batchSize int
	buffer    []httpx.ProbeResult
	callback  func(batch []httpx.ProbeResult) error
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(batchSize int, callback func(batch []httpx.ProbeResult) error) *BatchProcessor {
	return &BatchProcessor{
		batchSize: batchSize,
		buffer:    make([]httpx.ProbeResult, 0, batchSize),
		callback:  callback,
	}
}

// Add adds a result to the batch and processes if batch is full
func (bp *BatchProcessor) Add(result httpx.ProbeResult) error {
	bp.buffer = append(bp.buffer, result)

	if len(bp.buffer) >= bp.batchSize {
		return bp.Flush()
	}

	return nil
}

// Flush processes any remaining results in the buffer
func (bp *BatchProcessor) Flush() error {
	if len(bp.buffer) == 0 {
		return nil
	}

	err := bp.callback(bp.buffer)
	bp.buffer = bp.buffer[:0] // Reset buffer while keeping capacity
	return err
}
