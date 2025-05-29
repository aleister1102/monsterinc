package datastore

import (
	"fmt"
	"io"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"os"
	"path/filepath"
	"time"

	// "regexp" // No longer needed for local sanitization

	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// ParquetReader handles reading data from Parquet files.
type ParquetReader struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
}

// NewParquetReader creates a new ParquetReader.
func NewParquetReader(cfg *config.StorageConfig, logger zerolog.Logger) *ParquetReader {
	if cfg == nil || cfg.ParquetBasePath == "" {
		logger.Warn().Msg("ParquetReader: StorageConfig or ParquetBasePath is not properly configured.")
		// Return a functional reader, but operations might fail if path is needed and empty.
	}
	return &ParquetReader{
		storageConfig: cfg,
		logger:        logger.With().Str("module", "ParquetReader").Logger(),
	}
}

// FindAllProbeResultsForTarget reads all probe results for a given rootTargetURL
// from its consolidated Parquet file (e.g., database/example.com/data.parquet).
// Returns the results and the last modification time of the file.
func (pr *ParquetReader) FindAllProbeResultsForTarget(rootTargetURL string) ([]models.ProbeResult, time.Time, error) {
	pr.logger.Debug().Str("root_target_url", rootTargetURL).Msg("Attempting to find all probe results for target")

	if pr.storageConfig == nil || pr.storageConfig.ParquetBasePath == "" {
		msg := "ParquetBasePath is not configured. Cannot read Parquet file."
		pr.logger.Error().Msg(msg)
		return nil, time.Time{}, fmt.Errorf(msg)
	}

	sanitizedTargetName := urlhandler.SanitizeFilename(rootTargetURL)
	if sanitizedTargetName == "" {
		msg := fmt.Sprintf("Root target sanitized to empty string, cannot determine path for Parquet file: %s", rootTargetURL)
		pr.logger.Error().Str("original_target", rootTargetURL).Msg(msg)
		return nil, time.Time{}, fmt.Errorf(msg)
	}

	// Path is now <base_path>/<sanitized_rootTarget>.parquet
	fileName := fmt.Sprintf("%s.parquet", sanitizedTargetName)
	parquetFilePath := filepath.Join(pr.storageConfig.ParquetBasePath, "scan", fileName)

	fileInfo, err := os.Stat(parquetFilePath)
	if os.IsNotExist(err) {
		pr.logger.Info().Str("root_target_url", rootTargetURL).Str("file", parquetFilePath).Msg("No consolidated Parquet file found for target.")
		return nil, time.Time{}, nil // No file means no historical data, not an error in this context
	} else if err != nil {
		pr.logger.Error().Err(err).Str("file", parquetFilePath).Msg("Failed to stat Parquet file")
		return nil, time.Time{}, fmt.Errorf("failed to stat parquet file %s: %w", parquetFilePath, err)
	}

	pr.logger.Debug().Str("file", parquetFilePath).Msg("Reading all probe results from consolidated Parquet file")
	results, readErr := pr.readProbeResultsFromSpecificFile(parquetFilePath, rootTargetURL)
	if readErr != nil {
		// Error already logged by readProbeResultsFromSpecificFile
		return nil, time.Time{}, fmt.Errorf("failed to read consolidated parquet file %s: %w", parquetFilePath, readErr)
	}

	pr.logger.Info().Int("record_count", len(results)).Str("root_target_url", rootTargetURL).Msg("Finished reading all probe results for target")
	return results, fileInfo.ModTime(), nil
}

// readProbeResultsFromSpecificFile reads full ProbeResult records from a given Parquet file.
// contextualRootTargetURL is used if a record in the Parquet file doesn't have RootTargetURL set.
func (pr *ParquetReader) readProbeResultsFromSpecificFile(filePathToRead string, contextualRootTargetURL string) ([]models.ProbeResult, error) {
	file, err := os.Open(filePathToRead)
	if err != nil {
		pr.logger.Error().Err(err).Str("file", filePathToRead).Msg("Failed to open parquet file")
		return nil, fmt.Errorf("failed to open parquet file %s: %w", filePathToRead, err)
	}
	defer file.Close()

	readerOptions := []parquet.ReaderOption{}
	// Example: if you knew the file size, you could pass it for optimization:
	// fileInfo, err := file.Stat()
	// if err == nil {
	// readerOptions = append(readerOptions, parquet.ReadBufferSize(int(fileInfo.Size())))
	// }

	reader := parquet.NewReader(file, readerOptions...)
	defer reader.Close()

	var results []models.ProbeResult
	row := models.ParquetProbeResult{} // Reusable buffer for each row

	for {
		if err := reader.Read(&row); err != nil {
			if err == io.EOF {
				break // End of file
			}
			pr.logger.Error().Err(err).Str("file", filePathToRead).Msg("Failed to read row from parquet file")
			return nil, fmt.Errorf("failed to read row from %s: %w", filePathToRead, err)
		}

		probeResult := row.ToProbeResult() // Convert Parquet specific struct to general model

		// Ensure RootTargetURL is set, using the context if the record itself lacks it.
		// This is important if the Parquet files were generated without this field or it's sometimes optional.
		if probeResult.RootTargetURL == "" && contextualRootTargetURL != "" {
			probeResult.RootTargetURL = contextualRootTargetURL
		}
		results = append(results, probeResult)
	}

	pr.logger.Debug().Int("record_count", len(results)).Str("file", filePathToRead).Msg("Successfully read records from Parquet file")
	return results, nil
}

// Note: FindMostRecentScanURLs has been removed as the concept of "most recent scan file"
// is replaced by a single, consolidated Parquet file per target. The `LastSeenTimestamp`
// within the records of this consolidated file indicates recency for each specific URL.
