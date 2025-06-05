package datastore

import (
	"context"
	"encoding/json"
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

// ParquetWriterConfig holds configuration for ParquetWriter
type ParquetWriterConfig struct {
	CompressionType  string
	BatchSize        int
	EnableValidation bool
}

// DefaultParquetWriterConfig returns default configuration
func DefaultParquetWriterConfig() ParquetWriterConfig {
	return ParquetWriterConfig{
		CompressionType:  "zstd",
		BatchSize:        1000,
		EnableValidation: true,
	}
}

// ParquetWriter handles writing probe results to Parquet files.
type ParquetWriter struct {
	config       *config.StorageConfig
	logger       zerolog.Logger
	fileManager  *common.FileManager
	writerConfig ParquetWriterConfig
}

// ParquetWriterBuilder provides a fluent interface for creating ParquetWriter
type ParquetWriterBuilder struct {
	config       *config.StorageConfig
	logger       zerolog.Logger
	writerConfig ParquetWriterConfig
}

// NewParquetWriterBuilder creates a new ParquetWriterBuilder
func NewParquetWriterBuilder(logger zerolog.Logger) *ParquetWriterBuilder {
	return &ParquetWriterBuilder{
		logger:       logger.With().Str("component", "ParquetWriter").Logger(),
		writerConfig: DefaultParquetWriterConfig(),
	}
}

// WithStorageConfig sets the storage configuration
func (b *ParquetWriterBuilder) WithStorageConfig(cfg *config.StorageConfig) *ParquetWriterBuilder {
	b.config = cfg
	return b
}

// WithWriterConfig sets the writer configuration
func (b *ParquetWriterBuilder) WithWriterConfig(cfg ParquetWriterConfig) *ParquetWriterBuilder {
	b.writerConfig = cfg
	return b
}

// Build creates a new ParquetWriter instance
func (b *ParquetWriterBuilder) Build() (*ParquetWriter, error) {
	if b.config == nil {
		return nil, common.NewValidationError("config", b.config, "storage config cannot be nil")
	}

	if b.config.ParquetBasePath == "" {
		b.logger.Warn().Msg("ParquetBasePath is empty in config")
	}

	fileManager := common.NewFileManager(b.logger)

	return &ParquetWriter{
		config:       b.config,
		logger:       b.logger,
		fileManager:  fileManager,
		writerConfig: b.writerConfig,
	}, nil
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

// RecordTransformer handles transformation of records
type RecordTransformer struct {
	logger zerolog.Logger
}

// NewRecordTransformer creates a new RecordTransformer
func NewRecordTransformer(logger zerolog.Logger) *RecordTransformer {
	return &RecordTransformer{
		logger: logger.With().Str("component", "RecordTransformer").Logger(),
	}
}

// TransformToParquetResult converts a models.ProbeResult to a models.ParquetProbeResult
func (rt *RecordTransformer) TransformToParquetResult(pr models.ProbeResult, scanTime time.Time) models.ParquetProbeResult {
	headersJSON := rt.marshalHeaders(pr.Headers, pr.InputURL)
	techNames := rt.extractTechnologyNames(pr.Technologies)
	firstSeen := rt.determineFirstSeenTimestamp(pr.OldestScanTimestamp, scanTime)

	return models.ParquetProbeResult{
		OriginalURL:   pr.InputURL,
		FinalURL:      StringPtrOrNil(pr.FinalURL),
		StatusCode:    Int32PtrOrNilZero(int32(pr.StatusCode)),
		ContentLength: Int64PtrOrNilZero(pr.ContentLength),
		ContentType:   StringPtrOrNil(pr.ContentType),
		Title:         StringPtrOrNil(pr.Title),
		WebServer:     StringPtrOrNil(pr.WebServer),
		Technologies:  techNames,
		IPAddress:     pr.IPs,
		RootTargetURL: StringPtrOrNil(pr.RootTargetURL),
		ProbeError:    StringPtrOrNil(pr.Error),
		Method:        StringPtrOrNil(pr.Method),
		HeadersJSON:   headersJSON,

		DiffStatus:         StringPtrOrNil(pr.URLStatus),
		ScanTimestamp:      scanTime.UnixMilli(),
		FirstSeenTimestamp: models.TimePtrToUnixMilliOptional(firstSeen),
		LastSeenTimestamp:  models.TimePtrToUnixMilliOptional(scanTime),
	}
}

// marshalHeaders converts headers map to JSON string pointer
func (rt *RecordTransformer) marshalHeaders(headers map[string]string, inputURL string) *string {
	if len(headers) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(headers)
	if err != nil {
		rt.logger.Error().Err(err).Str("url", inputURL).Msg("Failed to marshal headers")
		return nil
	}

	strData := string(jsonData)
	return &strData
}

// extractTechnologyNames extracts technology names from Technology slice
func (rt *RecordTransformer) extractTechnologyNames(technologies []models.Technology) []string {
	var techNames []string
	for _, tech := range technologies {
		techNames = append(techNames, tech.Name)
	}
	return techNames
}

// determineFirstSeenTimestamp determines the first seen timestamp
func (rt *RecordTransformer) determineFirstSeenTimestamp(oldestScanTimestamp time.Time, scanTime time.Time) time.Time {
	if oldestScanTimestamp.IsZero() {
		return scanTime
	}
	return oldestScanTimestamp
}

// Write takes a slice of ProbeResult and writes them to a Parquet file
func (pw *ParquetWriter) Write(ctx context.Context, currentProbeResults []models.ProbeResult, scanSessionID string, rootTarget string) error {
	request := WriteRequest{
		ProbeResults:  currentProbeResults,
		ScanSessionID: scanSessionID,
		RootTarget:    rootTarget,
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

	sanitizedRootTarget := urlhandler.SanitizeFilename(request.RootTarget)
	if sanitizedRootTarget == "" {
		return common.NewValidationError("root_target", request.RootTarget, "sanitized root target is empty, cannot write parquet file")
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
func (pw *ParquetWriter) prepareOutputFile(rootTarget string) (string, error) {
	sanitizedRootTarget := urlhandler.SanitizeFilename(rootTarget)

	scanOutputDir := filepath.Join(pw.config.ParquetBasePath, "scan")
	if err := os.MkdirAll(scanOutputDir, 0755); err != nil {
		return "", common.WrapError(err, "failed to create scan-specific Parquet directory: "+scanOutputDir)
	}

	fileName := fmt.Sprintf("%s.parquet", sanitizedRootTarget)
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

		parquetResult := transformer.TransformToParquetResult(pr, request.ScanTime)
		parquetResults = append(parquetResults, parquetResult)
	}

	return parquetResults, nil
}

// writeToParquetFile writes the transformed results to a Parquet file
func (pw *ParquetWriter) writeToParquetFile(filePath string, parquetResults []models.ParquetProbeResult) (int, error) {
	pw.logger.Info().
		Str("file_path", filePath).
		Int("probe_count", len(parquetResults)).
		Msg("Writing probe results to Parquet file")

	file, err := pw.createParquetFile(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writer, err := pw.createParquetWriter(file)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

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
