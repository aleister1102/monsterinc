package datastore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/aleister1102/monsterinc/internal/common/filemanager"
	"github.com/aleister1102/monsterinc/internal/common/urlhandler"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"

	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// ParquetReader handles reading data from Parquet files.
type ParquetReader struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	fileManager   *filemanager.FileManager
	config        ParquetReaderConfig
}

// NewParquetReader creates a new ParquetReader using builder pattern
func NewParquetReader(cfg *config.StorageConfig, logger zerolog.Logger) *ParquetReader {
	reader, _ := NewParquetReaderBuilder(logger).
		WithStorageConfig(cfg).
		Build()
	return reader
}

// ProbeResultQuery encapsulates query parameters for probe results
type ProbeResultQuery struct {
	RootTargetURL string
	Limit         int
	Offset        int
}

// ProbeResultSearchResult contains search results and metadata
type ProbeResultSearchResult struct {
	Results      []httpxrunner.ProbeResult
	TotalCount   int
	LastModified time.Time
	FilePath     string
}

// FindAllProbeResultsForTarget reads all probe results for a given rootTargetURL
// from its consolidated Parquet file (e.g., database/example.com/data.parquet).
// Returns the results and the last modification time of the file.
func (pr *ParquetReader) FindAllProbeResultsForTarget(rootTargetURL string) ([]httpxrunner.ProbeResult, time.Time, error) {
	query := ProbeResultQuery{
		RootTargetURL: rootTargetURL,
	}

	result, err := pr.searchProbeResults(query)
	if err != nil {
		return nil, time.Time{}, err
	}

	return result.Results, result.LastModified, nil
}

// searchProbeResults performs the actual search operation
func (pr *ParquetReader) searchProbeResults(query ProbeResultQuery) (*ProbeResultSearchResult, error) {
	pr.logger.Debug().
		Str("root_target_url", query.RootTargetURL).
		Msg("Searching probe results for target")

	if err := pr.validateConfiguration(); err != nil {
		return nil, err
	}

	filePath, err := pr.buildParquetFilePath(query.RootTargetURL)
	if err != nil {
		return nil, err
	}

	fileInfo, err := pr.validateFileExists(filePath)
	if err != nil {
		return nil, err
	}
	if fileInfo == nil {
		// File doesn't exist - return empty results
		return &ProbeResultSearchResult{
			Results:      []httpxrunner.ProbeResult{},
			TotalCount:   0,
			LastModified: time.Time{},
			FilePath:     filePath,
		}, nil
	}

	results, err := pr.readProbeResultsFromFile(filePath, query.RootTargetURL)
	if err != nil {
		return nil, errorwrapper.WrapError(err, "failed to read probe results from file")
	}

	pr.logger.Info().
		Int("record_count", len(results)).
		Str("root_target_url", query.RootTargetURL).
		Msg("Successfully retrieved probe results for target")

	return &ProbeResultSearchResult{
		Results:      results,
		TotalCount:   len(results),
		LastModified: fileInfo.ModTime(),
		FilePath:     filePath,
	}, nil
}

// validateConfiguration checks if the reader is properly configured
func (pr *ParquetReader) validateConfiguration() error {
	if pr.storageConfig == nil || pr.storageConfig.ParquetBasePath == "" {
		return errorwrapper.NewValidationError("parquet_base_path", pr.storageConfig, "ParquetBasePath is not configured")
	}
	return nil
}

// buildParquetFilePath constructs the file path for a given root target URL
func (pr *ParquetReader) buildParquetFilePath(rootTargetURL string) (string, error) {
	sanitizedTargetName := urlhandler.SanitizeFilename(rootTargetURL)
	if sanitizedTargetName == "" {
		return "", errorwrapper.NewValidationError("root_target_url", rootTargetURL, "sanitized to empty string, cannot determine Parquet file path")
	}

	fileName := fmt.Sprintf("%s.parquet", sanitizedTargetName)
	return filepath.Join(pr.storageConfig.ParquetBasePath, "scan", fileName), nil
}

// validateFileExists checks if the target file exists and returns file info
func (pr *ParquetReader) validateFileExists(filePath string) (os.FileInfo, error) {
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// pr.logger.Info().Str("file", filePath).Msg("Parquet file not found for target")
		return nil, nil // Not an error - file simply doesn't exist yet
	}
	if err != nil {
		return nil, errorwrapper.WrapError(err, "failed to stat Parquet file: "+filePath)
	}
	return fileInfo, nil
}

// readProbeResultsFromFile reads all probe results from a specific Parquet file
func (pr *ParquetReader) readProbeResultsFromFile(filePath, contextualRootTargetURL string) ([]httpxrunner.ProbeResult, error) {
	pr.logger.Debug().Str("file", filePath).Msg("Reading probe results from Parquet file")

	file, err := pr.openParquetFile(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			pr.logger.Error().Err(err).Str("file", filePath).Msg("Failed to close Parquet file")
		}
	}()

	// Check file size before attempting to create reader
	if err := pr.validateParquetFile(file, filePath); err != nil {
		// If file is too small, return empty results instead of error
		pr.logger.Info().
			Str("file", filePath).
			Msg("Parquet file is too small or invalid, returning empty results")
		return []httpxrunner.ProbeResult{}, nil
	}

	reader, err := pr.createParquetReader(file)
	if err != nil {
		// If parquet reader creation fails, return empty results instead of error
		pr.logger.Info().
			Str("file", filePath).
			Err(err).
			Msg("Failed to create Parquet reader, returning empty results")
		return []httpxrunner.ProbeResult{}, nil
	}
	defer func() {
		err := reader.Close()
		if err != nil {
			pr.logger.Error().Err(err).Str("file", filePath).Msg("Failed to close Parquet reader")
		}
	}()

	results, err := pr.readAllRecords(reader, contextualRootTargetURL)
	if err != nil {
		return nil, errorwrapper.WrapError(err, "failed to read records from Parquet file")
	}

	pr.logger.Debug().
		Int("record_count", len(results)).
		Str("file", filePath).
		Msg("Successfully read records from Parquet file")

	return results, nil
}

// openParquetFile opens and validates a Parquet file
func (pr *ParquetReader) openParquetFile(filePath string) (*os.File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		pr.logger.Error().Err(err).Str("file", filePath).Msg("Failed to open Parquet file")
		return nil, errorwrapper.WrapError(err, "failed to open Parquet file: "+filePath)
	}
	return file, nil
}

// validateParquetFile checks if the Parquet file is valid and not empty
func (pr *ParquetReader) validateParquetFile(file *os.File, filePath string) error {
	// Get file stats to check size
	stat, err := file.Stat()
	if err != nil {
		return errorwrapper.WrapError(err, "failed to get file stats for: "+filePath)
	}

	// Check if file is empty or too small to contain a valid Parquet file
	// Parquet files need at least 12 bytes for magic header and footer
	if stat.Size() < 12 {
		pr.logger.Warn().
			Int64("file_size", stat.Size()).
			Str("file", filePath).
			Msg("Parquet file is too small or empty, returning empty results")
		return errorwrapper.NewValidationError("file_size", stat.Size(), "Parquet file is too small to be valid (minimum 12 bytes required)")
	}

	return nil
}

// createParquetReader creates a configured Parquet reader
func (pr *ParquetReader) createParquetReader(file *os.File) (reader *parquet.GenericReader[ParquetProbeResult], err error) {
	defer func() {
		if r := recover(); r != nil {
			pr.logger.Error().
				Interface("panic", r).
				Str("file", file.Name()).
				Msg("Panic occurred while creating Parquet reader")
			err = errorwrapper.WrapError(fmt.Errorf("panic creating parquet reader: %v", r), "failed to create parquet reader due to panic")
			reader = nil
		}
	}()

	readerOptions := pr.buildReaderOptions()

	// Try to create reader with proper error handling
	reader = parquet.NewGenericReader[ParquetProbeResult](file, readerOptions...)
	return reader, nil
}

// buildReaderOptions constructs reader options based on configuration
func (pr *ParquetReader) buildReaderOptions() []parquet.ReaderOption {
	var options []parquet.ReaderOption

	return options
}

// readAllRecords reads all records from the Parquet reader
func (pr *ParquetReader) readAllRecords(reader *parquet.GenericReader[ParquetProbeResult], contextualRootTargetURL string) ([]httpxrunner.ProbeResult, error) {
	var results []httpxrunner.ProbeResult

	// Read all rows using a buffer and loop
	const batchSize = 1000
	rows := make([]ParquetProbeResult, batchSize)

	for {
		n, err := reader.Read(rows)
		if err != nil && err != io.EOF {
			pr.logger.Error().Err(err).Msg("Failed to read rows from Parquet file")
			return nil, errorwrapper.WrapError(err, "failed to read rows from Parquet file")
		}

		// Process the read rows
		for i := 0; i < n; i++ {
			probeResult := pr.convertParquetRecord(rows[i], contextualRootTargetURL)
			results = append(results, probeResult)
		}

		// Break if we've reached the end of file
		if err == io.EOF {
			break
		}
	}

	return results, nil
}

// convertParquetRecord converts a Parquet record to ProbeResult
func (pr *ParquetReader) convertParquetRecord(row ParquetProbeResult, contextualRootTargetURL string) httpxrunner.ProbeResult {
	probeResult := row.ToProbeResult()

	// Ensure RootTargetURL is set using context if missing
	if probeResult.RootTargetURL == "" && contextualRootTargetURL != "" {
		probeResult.RootTargetURL = contextualRootTargetURL
	}

	return probeResult
}

func (pr *ParquetReader) Read(ctx context.Context, filePath string) ([]httpxrunner.ProbeResult, error) {
	// ... implementation
	return nil, nil
}
