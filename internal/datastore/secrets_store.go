package datastore

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// SecretsStore handles writing secret findings to Parquet files.
type SecretsStore struct {
	config      *config.StorageConfig
	logger      zerolog.Logger
	fileManager *common.FileManager
	mutex       sync.RWMutex // Protects concurrent access to parquet file
}

// NewSecretsStore creates a new SecretsStore.
func NewSecretsStore(cfg *config.StorageConfig, logger zerolog.Logger) (*SecretsStore, error) {
	return &SecretsStore{
		config:      cfg,
		logger:      logger.With().Str("module", "SecretsStore").Logger(),
		fileManager: common.NewFileManager(logger),
	}, nil
}

// StoreFindings writes a slice of SecretFinding to a Parquet file.
// If the file already exists, it appends the new findings to the existing ones.
// Duplicates are filtered out based on SourceURL and SecretText.
func (ss *SecretsStore) StoreFindings(ctx context.Context, findings []models.SecretFinding) error {
	if len(findings) == 0 {
		return nil
	}

	if ss.config.ParquetBasePath == "" {
		return common.NewValidationError("parquet_base_path", ss.config.ParquetBasePath, "ParquetBasePath is not configured for secrets")
	}

	// Lock to prevent concurrent access to parquet file
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	filePath, err := ss.prepareOutputFile()
	if err != nil {
		return err
	}

	var existingFindings []models.SecretFinding
	if _, err := os.Stat(filePath); err == nil {
		// File exists, load existing data
		existingFindings, err = ss.loadFindingsFromFile(ctx, filePath)
		if err != nil {
			// If the file is corrupt, we log a warning and overwrite it.
			ss.logger.Warn().Err(err).Str("file_path", filePath).Msg("Failed to load existing secret findings, overwriting the file.")
			existingFindings = []models.SecretFinding{}
		}
	} else if !os.IsNotExist(err) {
		// Another error from os.Stat
		return common.WrapError(err, "failed to check secrets parquet file status")
	}

	// Filter out duplicates from new findings
	uniqueFindings := ss.filterDuplicates(existingFindings, findings)

	if len(uniqueFindings) == 0 {
		ss.logger.Info().Int("total_duplicates", len(findings)).Msg("All secret findings are duplicates, skipping storage")
		return nil
	}

	// Combine existing and unique new findings
	allFindings := append(existingFindings, uniqueFindings...)

	err = ss.writeToParquetFile(filePath, allFindings)
	if err != nil {
		return err
	}

	ss.logger.Info().
		Str("file_path", filePath).
		Int("new_records", len(uniqueFindings)).
		Int("duplicates_filtered", len(findings)-len(uniqueFindings)).
		Int("total_records", len(allFindings)).
		Msg("Successfully stored secret findings")
	return nil
}

// filterDuplicates removes findings that already exist based on SourceURL and SecretText
func (ss *SecretsStore) filterDuplicates(existing []models.SecretFinding, newFindings []models.SecretFinding) []models.SecretFinding {
	// Create a map of existing findings for fast lookup
	existingMap := make(map[string]bool)
	for _, finding := range existing {
		key := ss.createFindingKey(finding)
		existingMap[key] = true
	}

	// Filter out duplicates from new findings
	var uniqueFindings []models.SecretFinding
	for _, finding := range newFindings {
		key := ss.createFindingKey(finding)
		if !existingMap[key] {
			uniqueFindings = append(uniqueFindings, finding)
			existingMap[key] = true // Add to map to prevent duplicates within the same batch
		}
	}

	return uniqueFindings
}

// createFindingKey creates a unique key for a SecretFinding based on SourceURL and SecretText
func (ss *SecretsStore) createFindingKey(finding models.SecretFinding) string {
	return finding.SourceURL + "|" + finding.SecretText
}

func (ss *SecretsStore) prepareOutputFile() (string, error) {
	secretsDir := filepath.Join(ss.config.ParquetBasePath, "secrets")
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return "", common.WrapError(err, "failed to create secrets Parquet directory: "+secretsDir)
	}
	fileName := "secrets.parquet"
	return filepath.Join(secretsDir, fileName), nil
}

func (ss *SecretsStore) writeToParquetFile(filePath string, findings []models.SecretFinding) error {
	// Open the file with truncation to overwrite existing content.
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return common.WrapError(err, "failed to open/create secret findings parquet file for writing: "+filePath)
	}
	defer file.Close()

	writer := parquet.NewGenericWriter[models.SecretFinding](file, parquet.Compression(&parquet.Zstd))

	if len(findings) > 0 {
		_, err = writer.Write(findings)
		if err != nil {
			_ = writer.Close()
			return common.WrapError(err, "failed to write secret findings to parquet file")
		}
	}

	return writer.Close()
}

// LoadFindings reads all secret findings from the Parquet file.
func (ss *SecretsStore) LoadFindings(ctx context.Context) ([]models.SecretFinding, error) {
	if ss.config.ParquetBasePath == "" {
		return nil, common.NewValidationError("parquet_base_path", ss.config.ParquetBasePath, "ParquetBasePath is not configured for secrets")
	}

	// Use read lock to allow concurrent reads but prevent writes
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	filePath, err := ss.prepareOutputFile()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		ss.logger.Warn().Str("file_path", filePath).Msg("Secrets parquet file does not exist, returning empty list.")
		return []models.SecretFinding{}, nil
	}

	return ss.loadFindingsFromFile(ctx, filePath)
}

func (ss *SecretsStore) loadFindingsFromFile(ctx context.Context, filePath string) ([]models.SecretFinding, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, common.WrapError(err, "failed to open secret findings parquet file for reading: "+filePath)
	}
	defer file.Close()

	// Check if file is empty or corrupted
	stat, err := file.Stat()
	if err != nil {
		return nil, common.WrapError(err, "failed to get file stats")
	}
	if stat.Size() == 0 {
		ss.logger.Warn().Str("file_path", filePath).Msg("Parquet file is empty, returning empty list")
		return []models.SecretFinding{}, nil
	}

	reader, err := func() (*parquet.GenericReader[models.SecretFinding], error) {
		defer func() {
			if r := recover(); r != nil {
				ss.logger.Error().Interface("panic", r).Str("file_path", filePath).Msg("Panic while creating parquet reader")
			}
		}()
		return parquet.NewGenericReader[models.SecretFinding](file), nil
	}()

	if err != nil {
		return nil, common.WrapError(err, "failed to create parquet reader")
	}
	if reader == nil {
		return nil, common.NewValidationError("parquet_reader", "nil", "failed to create parquet reader")
	}
	defer reader.Close()

	findings := make([]models.SecretFinding, 0)
	for {
		if err := ss.checkCancellation(ctx, "load secret findings"); err != nil {
			return nil, err
		}

		batch := make([]models.SecretFinding, 100) // Read in batches of 100
		n, err := reader.Read(batch)
		if n > 0 {
			findings = append(findings, batch[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, common.WrapError(err, "failed to read secret findings from parquet file")
		}
	}

	ss.logger.Info().Int("records_read", len(findings)).Str("file_path", filePath).Msg("Successfully loaded secret findings")
	return findings, nil
}

func (ss *SecretsStore) checkCancellation(ctx context.Context, operation string) error {
	if result := common.CheckCancellationWithLog(ctx, ss.logger, operation); result.Cancelled {
		return result.Error
	}
	return nil
}
