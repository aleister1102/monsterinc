package datastore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"os"
	"path/filepath"
	"sort"

	// Needed for GetLastKnownRecord sorting
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// ParquetFileHistoryStore implements the models.FileHistoryStore interface using Parquet files.
// Each monitored URL will have its history stored in a separate Parquet file.
type ParquetFileHistoryStore struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
}

// NewParquetFileHistoryStore creates a new ParquetFileHistoryStore.
func NewParquetFileHistoryStore(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetFileHistoryStore, error) {
	store := &ParquetFileHistoryStore{
		storageConfig: cfg,
		logger:        logger.With().Str("component", "ParquetFileHistoryStore").Logger(),
	}

	baseMonitorPath := filepath.Join(cfg.ParquetBasePath, "monitor_history")
	if err := os.MkdirAll(baseMonitorPath, 0755); err != nil {
		store.logger.Error().Err(err).Str("path", baseMonitorPath).Msg("Failed to create monitor history base directory")
		return nil, fmt.Errorf("creating monitor history base directory %s: %w", baseMonitorPath, err)
	}
	store.logger.Info().Str("path", baseMonitorPath).Msg("Monitor history base directory ensured.")

	return store, nil
}

// getHistoryFilePath generates a unique file path for a monitored URL's history.
func (pfs *ParquetFileHistoryStore) getHistoryFilePath(url string) string {
	hash := sha256.Sum256([]byte(url))
	fileName := fmt.Sprintf("%x.parquet", hash)
	return filepath.Join(pfs.storageConfig.ParquetBasePath, "monitor_history", fileName)
}

// Helper function to read all records from a Parquet file for FileHistoryRecord
func readFileHistoryRecords(filePath string, logger zerolog.Logger) ([]models.FileHistoryRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.FileHistoryRecord{}, nil
		}
		logger.Error().Err(err).Str("path", filePath).Msg("Failed to open history file for reading")
		return nil, fmt.Errorf("opening history file %s: %w", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		logger.Error().Err(err).Str("path", filePath).Msg("Failed to stat history file")
		return nil, fmt.Errorf("statting history file %s: %w", filePath, err)
	}

	if stat.Size() == 0 {
		return []models.FileHistoryRecord{}, nil
	}

	reader := parquet.NewReader(file)
	records := make([]models.FileHistoryRecord, reader.NumRows())

	if err := reader.Read(&records); err != nil {
		logger.Error().Err(err).Str("path", filePath).Msg("Failed to read records from Parquet history file")
		return nil, fmt.Errorf("reading Parquet history file %s: %w", filePath, err)
	}

	logger.Debug().Str("path", filePath).Int64("num_records", reader.NumRows()).Msg("Successfully read records from history file")
	return records, nil
}

// StoreFileRecord stores a new version of a monitored file.
func (pfs *ParquetFileHistoryStore) StoreFileRecord(record models.FileHistoryRecord) error {
	pfs.logger.Debug().Str("url", record.URL).Msg("Attempting to store file record")
	historyFilePath := pfs.getHistoryFilePath(record.URL)

	existingRecords, err := readFileHistoryRecords(historyFilePath, pfs.logger)
	if err != nil {
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Failed to read existing history records before storing new one")
		return fmt.Errorf("reading existing history from %s: %w", historyFilePath, err)
	}

	allRecords := append(existingRecords, record)

	fileCtx, err := os.OpenFile(historyFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Failed to open/create history file for writing")
		return fmt.Errorf("opening/creating history file %s for writing: %w", historyFilePath, err)
	}
	defer fileCtx.Close()

	writerOptions := []parquet.WriterOption{
		// Compression is handled by struct tags `parquet:",zstd"`.
		// If a general compression option for the writer is available and preferred over tags,
		// it would be set here. For example, if parquet.Compression(&parquet.Zstd) were a valid WriterOption:
		// writerOptions = append(writerOptions, parquet.Compression(&parquet.Zstd))
	}
	pfs.logger.Debug().Str("codec_via_tags", "zstd").Msg("Using ZSTD compression via struct tags for columns.")

	// Based on the persistent linter error indicating NewWriter signature is `func(output io.Writer, options ...WriterOption) *Writer`,
	// we attempt to initialize NewWriter without an explicit schema argument, hoping it's inferred or handled by options/first write.
	writer := parquet.NewWriter(fileCtx, writerOptions...)

	// If allRecords is empty (e.g. first record and existingRecords was empty),
	// writing an empty slice or calling Write with no data might be an issue or might do nothing.
	// However, allRecords will have at least one record (the new `record`).

	for i := range allRecords {
		if err := writer.Write(&allRecords[i]); err != nil {
			pfs.logger.Error().Err(err).Str("url", record.URL).Str("path", historyFilePath).Msg("Failed to write record to Parquet history file")
			_ = writer.Close()
			return fmt.Errorf("writing record to Parquet history file %s for URL %s: %w", historyFilePath, record.URL, err)
		}
	}

	if err := writer.Close(); err != nil {
		pfs.logger.Error().Err(err).Str("url", record.URL).Str("path", historyFilePath).Msg("Failed to close Parquet writer")
		return fmt.Errorf("closing Parquet writer for %s (URL %s): %w", historyFilePath, record.URL, err)
	}

	pfs.logger.Info().Str("url", record.URL).Str("path", historyFilePath).Int("total_records", len(allRecords)).Msg("Successfully stored/updated file history record.")
	return nil
}

// GetLastKnownRecord retrieves the most recent FileHistoryRecord for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownRecord(url string) (*models.FileHistoryRecord, error) {
	pfs.logger.Debug().Str("url", url).Msg("Attempting to get last known record")
	historyFilePath := pfs.getHistoryFilePath(url)

	records, err := readFileHistoryRecords(historyFilePath, pfs.logger)
	if err != nil {
		pfs.logger.Error().Err(err).Str("path", historyFilePath).Msg("Failed to read history records for GetLastKnownRecord")
		return nil, fmt.Errorf("reading history from %s for GetLastKnownRecord: %w", historyFilePath, err)
	}

	if len(records) == 0 {
		pfs.logger.Debug().Str("url", url).Msg("No records found for URL")
		return nil, models.ErrRecordNotFound
	}

	// Sort records by Timestamp in descending order to get the latest
	sort.Slice(records, func(i, j int) bool {
		return records[j].Timestamp.Before(records[i].Timestamp)
	})

	pfs.logger.Debug().Str("url", url).Time("latest_timestamp", records[0].Timestamp).Msg("Found last known record")
	return &records[0], nil
}

// GetLastKnownHash retrieves the most recent hash for a given URL.
func (pfs *ParquetFileHistoryStore) GetLastKnownHash(url string) (string, error) {
	record, err := pfs.GetLastKnownRecord(url)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			return "", models.ErrRecordNotFound
		}
		return "", err
	}
	if record == nil {
		return "", models.ErrRecordNotFound
	}
	return record.Hash, nil
}
