package datastore

import (
	"fmt"
	"io"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler" // Added urlhandler
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
	}
	return &ParquetReader{
		storageConfig: cfg,
		logger:        logger.With().Str("module", "ParquetReader").Logger(),
	}
}

// FindHistoricalDataForTarget finds the historical scan data for a given rootTargetURL
// and returns it as a slice of models.ProbeResult.
func (pr *ParquetReader) FindHistoricalDataForTarget(rootTargetURL string) ([]models.ProbeResult, error) {
	pr.logger.Debug().Str("root_target_url", rootTargetURL).Msg("Attempting to find historical data")
	sanitizedTargetName := urlhandler.SanitizeFilename(rootTargetURL)
	baseDir := filepath.Join(pr.storageConfig.ParquetBasePath, sanitizedTargetName)

	var allResults []models.ProbeResult

	// Check if the base directory for the target exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		pr.logger.Info().Str("root_target_url", rootTargetURL).Str("directory", baseDir).Msg("No historical data directory found for target.")
		return nil, nil // No directory means no historical data, not an error in this context
	}

	// Read all session directories for the target
	sessionDirs, err := os.ReadDir(baseDir)
	if err != nil {
		pr.logger.Error().Err(err).Str("directory", baseDir).Msg("Failed to read session directories")
		return nil, fmt.Errorf("failed to read session directories in %s: %w", baseDir, err)
	}

	for _, sessionDir := range sessionDirs {
		if sessionDir.IsDir() {
			parquetFilePath := filepath.Join(baseDir, sessionDir.Name(), "data.parquet")
			if _, err := os.Stat(parquetFilePath); !os.IsNotExist(err) {
				pr.logger.Debug().Str("file", parquetFilePath).Msg("Reading historical data from file")
				results, err := pr.readProbeResultsFromSpecificFile(parquetFilePath, rootTargetURL)
				if err != nil {
					pr.logger.Warn().Err(err).Str("file", parquetFilePath).Msg("Failed to read or parse historical Parquet file, skipping this file")
					continue // Skip this file and try others
				}
				allResults = append(allResults, results...)
			} else {
				pr.logger.Debug().Str("file", parquetFilePath).Msg("Parquet file not found in session directory, skipping")
			}
		}
	}
	pr.logger.Info().Int("record_count", len(allResults)).Str("root_target_url", rootTargetURL).Msg("Finished reading historical data")
	return allResults, nil
}

// readProbeResultsFromSpecificFile reads full ProbeResult records from a given Parquet file.
func (pr *ParquetReader) readProbeResultsFromSpecificFile(filePathToRead string, contextualRootTargetURL string) ([]models.ProbeResult, error) {
	file, err := os.Open(filePathToRead)
	if err != nil {
		pr.logger.Error().Err(err).Str("file", filePathToRead).Msg("Failed to open parquet file")
		return nil, fmt.Errorf("failed to open parquet file %s: %w", filePathToRead, err)
	}
	defer file.Close()

	// File info is not directly used by NewReader, but good to have for potential future use or debugging
	_, err = file.Stat() // Call Stat to check for errors, but don't need info for NewReader
	if err != nil {
		pr.logger.Error().Err(err).Str("file", filePathToRead).Msg("Failed to get file info for parquet file")
		return nil, fmt.Errorf("failed to stat parquet file %s: %w", filePathToRead, err)
	}

	reader := parquet.NewReader(file) // Corrected: NewReader expects io.ReadSeeker and optional ReaderOption(s)
	defer reader.Close()              // Ensure the reader is closed

	var results []models.ProbeResult
	row := models.ParquetProbeResult{}

	for {
		if err := reader.Read(&row); err != nil {
			if err == io.EOF {
				break
			}
			pr.logger.Error().Err(err).Str("file", filePathToRead).Msg("Failed to read row from parquet file")
			return nil, fmt.Errorf("failed to read row from %s: %w", filePathToRead, err)
		}

		// Convert ParquetProbeResult to ProbeResult
		probeResult := row.ToProbeResult()
		// Ensure RootTargetURL is set from the context of this read operation if not present in file
		if probeResult.RootTargetURL == "" {
			probeResult.RootTargetURL = contextualRootTargetURL
		}
		results = append(results, probeResult)
	}

	pr.logger.Debug().Int("record_count", len(results)).Str("file", filePathToRead).Msg("Successfully read records from Parquet file")
	return results, nil
}

// FindMostRecentScanURLs finds the most recent scan's Parquet file for a target and returns all URLs from it.
func (pr *ParquetReader) FindMostRecentScanURLs(rootTargetURL string) ([]string, error) {
	pr.logger.Debug().Str("root_target_url", rootTargetURL).Msg("Finding most recent scan URLs")
	sanitizedTargetName := urlhandler.SanitizeFilename(rootTargetURL)
	baseDir := filepath.Join(pr.storageConfig.ParquetBasePath, sanitizedTargetName)

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		pr.logger.Info().Str("directory", baseDir).Msg("No data directory found for target, cannot find most recent scan.")
		return nil, nil // No directory means no data
	}

	sessionDirs, err := os.ReadDir(baseDir)
	if err != nil {
		pr.logger.Error().Err(err).Str("directory", baseDir).Msg("Failed to read session directories for finding most recent scan")
		return nil, fmt.Errorf("failed to read session directories in %s: %w", baseDir, err)
	}

	var mostRecentSessionTime time.Time
	var mostRecentParquetFile string

	for _, sessionDir := range sessionDirs {
		if sessionDir.IsDir() {
			// Assuming session directory name is a timestamp like "20230101-150405"
			sessionTime, err := time.Parse("20060102-150405", sessionDir.Name())
			if err != nil {
				pr.logger.Warn().Err(err).Str("session_dir", sessionDir.Name()).Msg("Could not parse session directory name as timestamp, skipping")
				continue
			}

			parquetFilePath := filepath.Join(baseDir, sessionDir.Name(), "data.parquet")
			if _, statErr := os.Stat(parquetFilePath); !os.IsNotExist(statErr) {
				if sessionTime.After(mostRecentSessionTime) {
					mostRecentSessionTime = sessionTime
					mostRecentParquetFile = parquetFilePath
				}
			}
		}
	}

	if mostRecentParquetFile == "" {
		pr.logger.Info().Str("root_target_url", rootTargetURL).Msg("No valid Parquet files found in any session for target.")
		return nil, nil // No valid parquet file found
	}

	pr.logger.Info().Str("file", mostRecentParquetFile).Msg("Identified most recent Parquet file for URL extraction.")
	probeResults, err := pr.readProbeResultsFromSpecificFile(mostRecentParquetFile, rootTargetURL)
	if err != nil {
		pr.logger.Error().Err(err).Str("file", mostRecentParquetFile).Msg("Failed to read probe results from most recent Parquet file")
		return nil, fmt.Errorf("failed to read most recent parquet file %s: %w", mostRecentParquetFile, err)
	}

	urls := make([]string, 0, len(probeResults))
	for _, pr := range probeResults {
		urls = append(urls, pr.InputURL) // Or pr.FinalURL depending on which is needed
	}
	pr.logger.Info().Int("url_count", len(urls)).Str("root_target_url", rootTargetURL).Msg("Extracted URLs from most recent scan.")
	return urls, nil
}
