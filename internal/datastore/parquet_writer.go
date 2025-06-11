package datastore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// ParquetWriter handles writing probe results to Parquet files.
type ParquetWriter struct {
	config       *config.StorageConfig
	logger       zerolog.Logger
	fileManager  *common.FileManager
	writerConfig ParquetWriterConfig
}

// NewParquetWriter creates a new ParquetWriter using builder pattern
func NewParquetWriter(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetWriter, error) {
	return NewParquetWriterBuilder(logger).
		WithStorageConfig(cfg).
		Build()
}

// WriteRequest encapsulates a write request
type WriteRequest struct {
	ProbeResults  []models.ProbeResult
	ScanSessionID string
	RootTarget    string
	ScanTime      time.Time
}

// WriteResult contains the result of a write operation
type WriteResult struct {
	FilePath       string
	RecordsWritten int
	FileSize       int64
	WriteTime      time.Duration
}

// Write takes a slice of ProbeResult and writes them to a Parquet file
func (pw *ParquetWriter) Write(ctx context.Context, currentProbeResults []models.ProbeResult, scanSessionID string, hostname string) error {
	request := WriteRequest{
		ProbeResults:  currentProbeResults,
		ScanSessionID: scanSessionID,
		RootTarget:    hostname,
		ScanTime:      time.Now(),
	}

	result, err := pw.writeProbeResults(ctx, request)
	if err != nil {
		return err
	}

	pw.logger.Info().
		Str("file_path", result.FilePath).
		Int("records_written", result.RecordsWritten).
		Dur("write_time", result.WriteTime).
		Msg("Successfully wrote probe results to Parquet file")

	return nil
}

// writeProbeResults performs the actual write operation
func (pw *ParquetWriter) writeProbeResults(ctx context.Context, request WriteRequest) (*WriteResult, error) {
	startTime := time.Now()

	if err := pw.validateWriteRequest(request); err != nil {
		return nil, err
	}

	if err := pw.checkCancellation(ctx, "write start"); err != nil {
		return nil, err
	}

	filePath, err := pw.prepareOutputFile(request.RootTarget)
	if err != nil {
		return nil, err
	}

	if err := pw.checkCancellation(ctx, "before file creation"); err != nil {
		return nil, err
	}

	transformedResults, err := pw.transformRecords(ctx, request)
	if err != nil {
		return nil, err
	}

	if err := pw.checkCancellation(ctx, "before parquet write"); err != nil {
		return nil, err
	}

	recordsWritten, err := pw.writeToParquetFile(filePath, transformedResults)
	if err != nil {
		return nil, err
	}

	fileInfo, _ := os.Stat(filePath)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	return &WriteResult{
		FilePath:       filePath,
		RecordsWritten: recordsWritten,
		FileSize:       fileSize,
		WriteTime:      time.Since(startTime),
	}, nil
}

// validateWriteRequest validates the write request parameters
func (pw *ParquetWriter) validateWriteRequest(request WriteRequest) error {
	if pw.config.ParquetBasePath == "" {
		return common.NewValidationError("parquet_base_path", pw.config.ParquetBasePath, "ParquetBasePath is not configured")
	}

	sanitizedHostname := urlhandler.SanitizeFilename(request.RootTarget)
	if sanitizedHostname == "" {
		return common.NewValidationError("hostname", request.RootTarget, "sanitized hostname is empty, cannot write parquet file")
	}

	return nil
}

// checkCancellation checks for context cancellation
func (pw *ParquetWriter) checkCancellation(ctx context.Context, operation string) error {
	if result := common.CheckCancellationWithLog(ctx, pw.logger, operation); result.Cancelled {
		return result.Error
	}
	return nil
}

// prepareOutputFile prepares the output directory and file path
func (pw *ParquetWriter) prepareOutputFile(hostname string) (string, error) {
	sanitizedHostname := urlhandler.SanitizeFilename(hostname)

	scanOutputDir := filepath.Join(pw.config.ParquetBasePath, "scan")
	if err := os.MkdirAll(scanOutputDir, 0755); err != nil {
		return "", common.WrapError(err, "failed to create scan-specific Parquet directory: "+scanOutputDir)
	}

	fileName := fmt.Sprintf("%s.parquet", sanitizedHostname)
	filePath := filepath.Join(scanOutputDir, fileName)

	return filePath, nil
}

// transformRecords transforms probe results to parquet format
func (pw *ParquetWriter) transformRecords(ctx context.Context, request WriteRequest) ([]models.ParquetProbeResult, error) {
	transformer := NewRecordTransformer(pw.logger)
	var parquetResults []models.ParquetProbeResult

	for _, pr := range request.ProbeResults {
		// Check for cancellation during transformation
		if err := pw.checkCancellation(ctx, "during result transformation"); err != nil {
			return nil, err
		}

		parquetResult := transformer.TransformToParquetResult(pr, request.ScanTime, request.ScanSessionID)
		parquetResults = append(parquetResults, parquetResult)
	}

	return parquetResults, nil
}

// writeToParquetFile writes the transformed results to a Parquet file
func (pw *ParquetWriter) writeToParquetFile(filePath string, parquetResults []models.ParquetProbeResult) (int, error) {
	// pw.logger.Info().
	// 	Str("file_path", filePath).
	// 	Int("probe_count", len(parquetResults)).
	// 	Msg("Writing probe results to Parquet file")

	file, err := pw.createParquetFile(filePath)
	if err != nil {
		return 0, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			pw.logger.Error().Err(err).Str("file", filePath).Msg("Failed to close Parquet file")
		}
	}()

	writer, err := pw.createParquetWriter(file)
	if err != nil {
		return 0, err
	}
	defer func() {
		err := writer.Close()
		if err != nil {
			pw.logger.Error().Err(err).Str("file", filePath).Msg("Failed to close Parquet writer")
		}
	}()

	recordsWritten, err := pw.writeRecords(writer, parquetResults)
	if err != nil {
		return 0, common.WrapError(err, "failed to write probe results to parquet file")
	}

	return recordsWritten, nil
}

// createParquetFile creates and opens a new Parquet file for writing
func (pw *ParquetWriter) createParquetFile(filePath string) (*os.File, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, common.WrapError(err, "failed to create/truncate parquet file: "+filePath)
	}
	return file, nil
}

// createParquetWriter creates a configured Parquet writer
func (pw *ParquetWriter) createParquetWriter(file *os.File) (*parquet.GenericWriter[models.ParquetProbeResult], error) {
	compressionOption := pw.getCompressionOption()
	writer := parquet.NewGenericWriter[models.ParquetProbeResult](file, compressionOption)
	return writer, nil
}

// getCompressionOption returns the compression option based on configuration
func (pw *ParquetWriter) getCompressionOption() parquet.WriterOption {
	switch pw.writerConfig.CompressionType {
	case "gzip":
		return parquet.Compression(&parquet.Gzip)
	case "snappy":
		return parquet.Compression(&parquet.Snappy)
	case "zstd":
		return parquet.Compression(&parquet.Zstd)
	default:
		return parquet.Compression(&parquet.Zstd) // Default to Zstd
	}
}

// writeRecords writes all records to the Parquet writer
func (pw *ParquetWriter) writeRecords(writer *parquet.GenericWriter[models.ParquetProbeResult], parquetResults []models.ParquetProbeResult) (int, error) {
	recordsWritten, err := writer.Write(parquetResults)
	if err != nil {
		return 0, err
	}
	return recordsWritten, nil
}

// Helper functions

// StringPtrOrNil converts string to pointer, or nil if string is empty
func StringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Int32PtrOrNilZero converts int32 to pointer, or nil if value is 0
func Int32PtrOrNilZero(i int32) *int32 {
	if i == 0 {
		return nil
	}
	return &i
}

// Int64PtrOrNilZero converts int64 to pointer, or nil if value is 0
func Int64PtrOrNilZero(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}
