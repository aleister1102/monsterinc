package datastore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// getAndSortRecordsForURL is an internal helper to get the file path for a URL,
// read its records using readFileHistoryRecords, which also sorts them (newest first).
func (pfs *ParquetFileHistoryStore) getAndSortRecordsForURL(recordURL string) ([]models.FileHistoryRecord, error) {
	historyFilePath, err := pfs.getHistoryFilePath(recordURL)
	if err != nil {
		// Error already logged by getHistoryFilePath if it's critical for path formation
		return nil, fmt.Errorf("failed to get history file path for '%s': %w", recordURL, err)
	}

	// readFileHistoryRecords reads, sorts (newest first), and handles os.IsNotExist by returning ([]models.FileHistoryRecord{}, nil)
	records, err := readFileHistoryRecords(historyFilePath, pfs.logger)
	if err != nil {
		// This 'err' would be a more severe error than file not found, as that case is handled in readFileHistoryRecords.
		pfs.logger.Error().Err(err).Str("url", recordURL).Str("path", historyFilePath).Msg("Error reading and sorting records from history file")
		return nil, fmt.Errorf("error reading history from '%s': %w", historyFilePath, err)
	}
	return records, nil
}

// readFileHistoryRecords reads all records from the specified Parquet file.
func readFileHistoryRecords(filePath string, logger zerolog.Logger) ([]models.FileHistoryRecord, error) {
	osFile, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info().Str("file", filePath).Msg("History file does not exist, returning empty records.")
			return []models.FileHistoryRecord{}, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open history file '%s': %w", filePath, err)
	}
	defer osFile.Close()

	stat, err := osFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat history file '%s': %w", filePath, err)
	}
	if stat.Size() == 0 {
		logger.Info().Str("file", filePath).Msg("History file is empty, returning empty records.")
		return []models.FileHistoryRecord{}, nil // Return empty slice if file is empty
	}

	pqFile, err := parquet.OpenFile(osFile, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file '%s': %w", filePath, err)
	}

	// According to parquet-go/parquet-go README examples:
	// reader := parquet.NewReader(f) // where f is *parquet.File
	// Then loop reader.Read(&row)
	reader := parquet.NewReader(pqFile)

	var records []models.FileHistoryRecord
	for {
		var record models.FileHistoryRecord
		if err := reader.Read(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break // End of file
			}
			logger.Error().Err(err).Str("file", filePath).Msg("Error reading record from Parquet file")
			return nil, fmt.Errorf("error reading record from parquet file '%s': %w", filePath, err)
		}
		records = append(records, record)
	}

	// Sort records by Timestamp descending (newest first)
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Timestamp > records[j].Timestamp // Compare int64 directly
	})

	// The osFile is closed by defer. The parquet.File (pqFile) does not seem to have/need an explicit Close method
	// in the examples when created from an os.File that is managed separately.
	// reader also does not show an explicit Close method in the simple Read loop example.

	logger.Debug().Int("count", len(records)).Str("file", filePath).Msg("Successfully read and sorted records from history file.")
	return records, nil
}

// Helper to read all records from a single parquet file.
func (pfs *ParquetFileHistoryStore) readRecordsFromFile(filePath string) ([]*models.FileHistoryRecord, error) {
	records, err := readFileHistoryRecords(filePath, pfs.logger)
	if err != nil {
		return nil, err
	}

	recordPtrs := make([]*models.FileHistoryRecord, len(records))
	for i, rec := range records {
		recordPtrs[i] = &rec
	}
	return recordPtrs, nil
}
