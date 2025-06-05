package datastore

import (
	"fmt"

	"github.com/aleister1102/monsterinc/internal/models"
)

// StoreFileRecord stores a new version of a monitored file.
func (pfs *ParquetFileHistoryStore) StoreFileRecord(record models.FileHistoryRecord) error {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(record.URL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	historyFilePath, err := pfs.getHistoryFilePath(record.URL)
	if err != nil {
		return err // Error already logged in getHistoryFilePath
	}

	pfs.logger.Info().Str("url", record.URL).Str("path", historyFilePath).Msg("Storing file record")

	// Load existing records
	existingRecords, err := pfs.loadExistingRecords(historyFilePath)
	if err != nil {
		return err
	}

	// Check if this exact record already exists (prevent duplicates)
	for _, existingRecord := range existingRecords {
		if existingRecord.URL == record.URL &&
			existingRecord.Hash == record.Hash &&
			existingRecord.Timestamp == record.Timestamp {
			pfs.logger.Debug().Str("url", record.URL).Str("hash", record.Hash).Int64("timestamp", record.Timestamp).Msg("Record already exists, skipping duplicate")
			return nil
		}
	}

	// Always append the new record
	allRecords := append(existingRecords, record)

	// Create parquet file for writing
	file, compressionOption, err := pfs.createParquetFile(historyFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write all records to the file
	if err := pfs.writeParquetData(file, compressionOption, allRecords); err != nil {
		return fmt.Errorf("writing parquet data to '%s': %w", historyFilePath, err)
	}

	pfs.logger.Info().Str("url", record.URL).Int("total_records", len(allRecords)).Msg("Successfully stored/updated file history record.")
	return nil
}

// GetLastKnownRecord retrieves the most recent FileHistoryRecord for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownRecord(recordURL string) (*models.FileHistoryRecord, error) {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(recordURL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	records, err := pfs.getAndSortRecordsForURL(recordURL)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	record := &records[0] // Since records are sorted newest first
	return record, nil
}

// GetLatestRecord retrieves the most recent FileHistoryRecord for a given URL.
// This is functionally an alias for GetLastKnownRecord in the current implementation.
func (pfs *ParquetFileHistoryStore) GetLatestRecord(recordURL string) (*models.FileHistoryRecord, error) {
	return pfs.GetLastKnownRecord(recordURL)
}

// GetRecordsForURL retrieves a limited number of records for a URL, for potential future use.
func (pfs *ParquetFileHistoryStore) GetRecordsForURL(recordURL string, limit int) ([]*models.FileHistoryRecord, error) {
	// Get URL-specific mutex to ensure thread-safety
	urlMutex := pfs.getURLMutex(recordURL)
	urlMutex.Lock()
	defer urlMutex.Unlock()

	records, err := pfs.getAndSortRecordsForURL(recordURL)
	if err != nil {
		return nil, err
	}

	// Apply limit if specified
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	// Convert to slice of pointers
	var resultRecords []*models.FileHistoryRecord
	for i := range records {
		resultRecords = append(resultRecords, &records[i])
	}

	return resultRecords, nil
}

// GetLastKnownHash retrieves the most recent hash for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownHash(url string) (string, error) {
	record, err := pfs.GetLastKnownRecord(url)
	if err != nil {
		return "", err // Propagate error
	}
	if record == nil {
		return "", nil // No record found, so no hash
	}
	return record.Hash, nil
}
