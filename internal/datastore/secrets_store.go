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

	err = ss.writeToParquetFile(filePath, findings)
	if err != nil {
		return err
	}

	ss.logger.Info().Str("file_path", filePath).Int("records_written", len(findings)).Msg("Successfully wrote secret findings to Parquet file")
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
	// We open the file in append mode or create it if it doesn't exist.
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return common.WrapError(err, "failed to open/create secret findings parquet file: "+filePath)
	}
	defer file.Close()

	writer := parquet.NewGenericWriter[models.SecretFinding](file, parquet.Compression(&parquet.Zstd))

	_, err = writer.Write(findings)
	if err != nil {
		_ = writer.Close()
		return common.WrapError(err, "failed to write secret findings to parquet file")
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
