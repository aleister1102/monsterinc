package datastore

import (
	"fmt"
	"io"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler" // Added urlhandler
	"os"
	"path/filepath"

	// "regexp" // No longer needed for local sanitization
	// "sort"
	// "strings"
	// "time"

	"github.com/parquet-go/parquet-go"
)

// ParquetReader handles reading data from Parquet files.
type ParquetReader struct {
	storageConfig *config.StorageConfig
	logger        *log.Logger
}

// NewParquetReader creates a new ParquetReader.
func NewParquetReader(cfg *config.StorageConfig, logger *log.Logger) *ParquetReader {
	if cfg == nil || cfg.ParquetBasePath == "" {
		logger.Println("[WARN] ParquetReader: StorageConfig or ParquetBasePath is not properly configured.")
	}
	return &ParquetReader{
		storageConfig: cfg,
		logger:        logger,
	}
}

// FindHistoricalDataForTarget finds the historical scan data for a given rootTargetURL
// and returns it as a slice of models.ProbeResult.
func (pr *ParquetReader) FindHistoricalDataForTarget(rootTargetURL string) ([]models.ProbeResult, error) {
	if pr.storageConfig == nil || pr.storageConfig.ParquetBasePath == "" {
		pr.logger.Println("[ERROR] ParquetReader: ParquetBasePath is not configured.")
		return nil, fmt.Errorf("ParquetBasePath not configured")
	}

	sanitizedTargetName := urlhandler.SanitizeFilename(rootTargetURL)
	if sanitizedTargetName == "sanitized_empty_input" || sanitizedTargetName == "" {
		pr.logger.Printf("[WARN] FindHistoricalDataForTarget: Root target '%s' sanitized to empty. Cannot find file.", rootTargetURL)
		return []models.ProbeResult{}, nil
	}

	fileName := fmt.Sprintf("%s.parquet", sanitizedTargetName)
	filePathToRead := filepath.Join(pr.storageConfig.ParquetBasePath, fileName)

	pr.logger.Printf("Attempting to read historical ProbeResult data for target %s from: %s", rootTargetURL, filePathToRead)
	return pr.readProbeResultsFromSpecificFile(filePathToRead, rootTargetURL)
}

// readProbeResultsFromSpecificFile reads full ProbeResult records from a given Parquet file.
func (pr *ParquetReader) readProbeResultsFromSpecificFile(filePathToRead string, contextualRootTargetURL string) ([]models.ProbeResult, error) {
	pr.logger.Printf("Reading historical ProbeResult data for context target '%s' from specific file: %s", contextualRootTargetURL, filePathToRead)

	file, err := os.Open(filePathToRead)
	if err != nil {
		if os.IsNotExist(err) {
			pr.logger.Printf("Parquet file does not exist at %s (context: %s). No historical data.", filePathToRead, contextualRootTargetURL)
			return []models.ProbeResult{}, nil
		}
		pr.logger.Printf("Error opening parquet file %s: %v.", filePathToRead, err)
		return nil, fmt.Errorf("error opening parquet file %s: %w", filePathToRead, err)
	}
	defer file.Close()

	// Log file size for debugging
	fileInfo, statErr := file.Stat()
	if statErr == nil {
		pr.logger.Printf("File %s opened successfully. Size: %d bytes.", filePathToRead, fileInfo.Size())
	} else {
		pr.logger.Printf("Could not get file stats for %s: %v", filePathToRead, statErr)
	}

	reader := parquet.NewGenericReader[models.ParquetProbeResult](file)
	var results []models.ProbeResult
	rowsBuffer := make([]models.ParquetProbeResult, 100)
	totalRowsRead := 0

	// Log schema if possible - parquet-go might not expose this easily for GenericReader
	// pr.logger.Printf("Schema of Parquet file %s: %s", filePathToRead, reader.Schema().String()) // This line might not work directly

	for {
		n, errRead := reader.Read(rowsBuffer)
		if errRead != nil && errRead != io.EOF { // Check for io.EOF explicitly
			pr.logger.Printf("Error reading batch from parquet file %s (context: %s, total rows processed so far: %d): %v.", filePathToRead, contextualRootTargetURL, totalRowsRead, errRead)
			return nil, fmt.Errorf("error reading rows from %s: %w", filePathToRead, errRead)
		}

		if n > 0 {
			pr.logger.Printf("Read %d ParquetProbeResult rows in current batch from %s.", n, filePathToRead)
			for i := 0; i < n; i++ {
				results = append(results, rowsBuffer[i].ToProbeResult())
			}
			totalRowsRead += n
		}

		if errRead == io.EOF {
			pr.logger.Printf("EOF reached for Parquet file %s after processing %d rows in this batch. Total rows for file: %d.", filePathToRead, n, totalRowsRead)
			break
		}
		// If n == 0 and errRead == nil, it might also indicate end of file or an issue.
		if n == 0 && errRead == nil {
			pr.logger.Printf("Read 0 rows and no error (errRead is nil), assuming EOF for Parquet file %s. Total rows for file: %d.", filePathToRead, totalRowsRead)
			break
		}
	}

	if errClose := reader.Close(); errClose != nil {
		pr.logger.Printf("Error closing Parquet reader for %s: %v (context: %s)", filePathToRead, errClose, contextualRootTargetURL)
		// Not returning error on close if we already got data, but logging it.
	}

	pr.logger.Printf("Successfully read and converted %d total ProbeResults from Parquet file: %s (context: %s)", len(results), filePathToRead, contextualRootTargetURL)
	return results, nil
}

// FindMostRecentScanURLs is DEPRECATED and callers should be updated.
// If needed for backward compatibility, it would call FindHistoricalDataForTarget and extract URLs.
func (pr *ParquetReader) FindMostRecentScanURLs(rootTargetURL string) ([]string, error) {
	pr.logger.Printf("[WARN] FindMostRecentScanURLs is deprecated and will be removed. Update callers to use FindHistoricalDataForTarget or alternative. For target: %s", rootTargetURL)
	historicalData, err := pr.FindHistoricalDataForTarget(rootTargetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data via FindHistoricalDataForTarget: %w", err)
	}
	urls := make([]string, len(historicalData))
	for i, data := range historicalData {
		// Determine which URL to return based on previous logic (e.g., InputURL or FinalURL)
		// Assuming InputURL was the key previously.
		urls[i] = data.InputURL
	}
	return urls, nil
}
