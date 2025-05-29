package datastore

import (
	"fmt"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// SecretsStore defines the interface for storing and retrieving secret findings.
// Implementations could include Parquet, databases, etc.
type SecretsStore interface {
	// StoreSecretFindings saves a list of secret findings.
	StoreSecretFindings(findings []models.SecretFinding) error

	// GetSecretFindings retrieves findings, potentially with filtering (to be defined).
	// GetSecretFindings(filterCriteria map[string]string) ([]models.SecretFinding, error)
}

// ParquetSecretsStore implements SecretsStore using Parquet files.
type ParquetSecretsStore struct {
	storageConfig *config.StorageConfig
	logger        zerolog.Logger
	mu            sync.Mutex // To protect file access if multiple goroutines call StoreSecretFindings
}

// NewParquetSecretsStore creates a new ParquetSecretsStore.
func NewParquetSecretsStore(cfg *config.StorageConfig, log zerolog.Logger) (*ParquetSecretsStore, error) {
	store := &ParquetSecretsStore{
		storageConfig: cfg,
		logger:        log.With().Str("component", "ParquetSecretsStore").Logger(),
	}
	// Ensure the base directory exists
	basePath := filepath.Join(cfg.ParquetBasePath, "secrets") // Storing secrets in a 'secrets' subdirectory
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create secrets parquet base directory %s: %w", basePath, err)
	}
	store.logger.Info().Str("path", basePath).Msg("ParquetSecretsStore initialized. Secrets will be stored here.")
	return store, nil
}

// StoreSecretFindings saves a list of secret findings to a Parquet file.
// It reads existing findings, appends new ones, and then overwrites the file.
func (s *ParquetSecretsStore) StoreSecretFindings(findings []models.SecretFinding) error {
	if len(findings) == 0 {
		s.logger.Info().Msg("No new secret findings to store.")
		// If there are no new findings, we might still want to ensure the file exists
		// or just do nothing if no existing file. For now, if new findings are empty, return.
		// This means if the file exists with old data, it won't be touched.
		// If the goal is to "consolidate" even with no new findings, this logic would change.
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.storageConfig.ParquetBasePath, "secrets", "secrets_findings.parquet")
	s.logger.Info().Int("new_findings_count", len(findings)).Str("file", filePath).Msg("Attempting to store secret findings to Parquet file")

	var allFindings []models.SecretFinding

	// 1. Read existing records if file exists
	if _, statErr := os.Stat(filePath); statErr == nil {
		s.logger.Debug().Str("file", filePath).Msg("Existing secrets file found, reading records to append.")
		existingRecs, readErr := s.readSecretsFromFile(filePath)
		if readErr != nil {
			s.logger.Error().Err(readErr).Str("file", filePath).Msg("Failed to read existing secrets from Parquet file. Will attempt to overwrite with current findings only.")
			// If read fails, proceed with only the new findings. This is a choice: risk losing old data vs. not storing new.
			// For secret storage, preserving new findings might be more critical.
		} else {
			allFindings = append(allFindings, existingRecs...)
		}
	} else if !os.IsNotExist(statErr) {
		s.logger.Error().Err(statErr).Str("file", filePath).Msg("Error stating existing secrets file. Will proceed as if file does not exist.")
	}

	// 2. Append new findings
	allFindings = append(allFindings, findings...)

	if len(allFindings) == 0 {
		s.logger.Info().Str("file", filePath).Msg("No findings (neither existing nor new) to write. Skipping file operation.")
		return nil
	}
	s.logger.Info().Int("total_findings_to_write", len(allFindings)).Str("file", filePath).Msg("Preparing to write all findings.")

	// 3. Write all findings back to the file (overwrite)
	file, err := os.Create(filePath) // os.Create truncates if file exists, or creates if not.
	if err != nil {
		return fmt.Errorf("failed to create/truncate parquet file %s: %w", filePath, err)
	}
	defer file.Close()

	options := make([]parquet.WriterOption, 0, 1) // Initialize with capacity for compression only

	var compressionOption parquet.WriterOption
	switch strings.ToLower(s.storageConfig.CompressionCodec) {
	case "snappy":
		compressionOption = parquet.Compression(&parquet.Snappy)
	case "gzip":
		compressionOption = parquet.Compression(&parquet.Gzip)
	case "zstd":
		compressionOption = parquet.Compression(&parquet.Zstd)
	case "none", "":
		compressionOption = parquet.Compression(&parquet.Uncompressed)
	default:
		s.logger.Warn().Str("codec", s.storageConfig.CompressionCodec).Msg("Unsupported compression codec for secrets, using ZSTD as default")
		compressionOption = parquet.Compression(&parquet.Zstd)
	}
	options = append(options, compressionOption)

	// Create writer with schema inferred from the type
	writer := parquet.NewGenericWriter[models.SecretFinding](file, options...)

	for _, finding := range allFindings {
		if _, err := writer.Write([]models.SecretFinding{finding}); err != nil {
			s.logger.Error().Err(err).Interface("finding_rule_id", finding.RuleID).Msg("Failed to write secret finding to Parquet")
			// Continue to try and write other records, but the file might be incomplete.
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close parquet writer for %s: %w", filePath, err)
	}

	s.logger.Info().Int("total_written_count", len(allFindings)).Str("file", filePath).Msg("Successfully stored secret findings.")
	return nil
}

// readSecretsFromFile is a helper to read SecretFinding records from a parquet file.
func (s *ParquetSecretsStore) readSecretsFromFile(filePath string) ([]models.SecretFinding, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Debug().Str("file", filePath).Msg("Secrets file does not exist, no records to read.")
			return []models.SecretFinding{}, nil
		}
		return nil, fmt.Errorf("failed to open secrets file for reading %s: %w", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat secrets file %s: %w", filePath, err)
	}
	if stat.Size() == 0 {
		s.logger.Debug().Str("file", filePath).Msg("Secrets file is empty, no records to read.")
		return []models.SecretFinding{}, nil
	}

	pqFile, err := parquet.OpenFile(file, stat.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to open parquet file object %s: %w", filePath, err)
	}

	reader := parquet.NewGenericReader[models.SecretFinding](pqFile)
	var records []models.SecretFinding

	// Read records in batches
	for {
		batch := make([]models.SecretFinding, 1000) // Read in batches of 1000
		n, err := reader.Read(batch)
		if err != nil {
			// Check for EOF
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("error reading secret findings from %s: %w", filePath, err)
		}

		// Append only the records that were actually read
		records = append(records, batch[:n]...)

		// If we read fewer records than the batch size, we're at the end
		if n < len(batch) {
			break
		}
	}

	s.logger.Debug().Int("count", len(records)).Str("file", filePath).Msg("Successfully read existing secrets.")
	return records, nil
}

// deduplicateSecrets removes duplicate secret findings based on key fields
func (s *ParquetSecretsStore) deduplicateSecrets(findings []models.SecretFinding) []models.SecretFinding {
	// Simple deduplication based on SourceURL, RuleID, and SecretText
	seen := make(map[string]bool)
	var deduplicated []models.SecretFinding

	for _, finding := range findings {
		key := fmt.Sprintf("%s-%s-%s", finding.SourceURL, finding.RuleID, finding.SecretText)
		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, finding)
		}
	}

	if len(deduplicated) != len(findings) {
		s.logger.Info().Int("original", len(findings)).Int("deduplicated", len(deduplicated)).Msg("Removed duplicate secret findings")
	}

	return deduplicated
}
