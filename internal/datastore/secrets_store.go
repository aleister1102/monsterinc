package datastore

import (
	"context"
	"os"
	"path/filepath"

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
func (ss *SecretsStore) StoreFindings(ctx context.Context, findings []models.SecretFinding) error {
	if len(findings) == 0 {
		return nil
	}

	if ss.config.ParquetBasePath == "" {
		return common.NewValidationError("parquet_base_path", ss.config.ParquetBasePath, "ParquetBasePath is not configured for secrets")
	}

	filePath, err := ss.prepareOutputFile()
	if err != nil {
		return err
	}

	var allFindings []models.SecretFinding
	if _, err := os.Stat(filePath); err == nil {
		// File exists, load existing data
		existingFindings, err := ss.loadFindingsFromFile(ctx, filePath)
		if err != nil {
			// If the file is corrupt, we log a warning and overwrite it.
			ss.logger.Warn().Err(err).Str("file_path", filePath).Msg("Failed to load existing secret findings, overwriting the file.")
			allFindings = findings
		} else {
			allFindings = append(existingFindings, findings...)
		}
	} else if os.IsNotExist(err) {
		// File doesn't exist, use only the new findings
		allFindings = findings
	} else {
		// Another error from os.Stat
		return common.WrapError(err, "failed to check secrets parquet file status")
	}

	err = ss.writeToParquetFile(filePath, allFindings)
	if err != nil {
		return err
	}

	ss.logger.Info().Str("file_path", filePath).Int("records_written", len(findings)).Msg("Successfully stored secret findings")
	return nil
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

	reader := parquet.NewGenericReader[models.SecretFinding](file)
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
