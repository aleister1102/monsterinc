package datastore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// FilePathGenerator handles file path generation for history files
type FilePathGenerator struct {
	logger     zerolog.Logger
	urlHashGen *URLHashGenerator
	basePath   string
}

// NewFilePathGenerator creates a new file path generator
func NewFilePathGenerator(basePath string, urlHashGen *URLHashGenerator, logger zerolog.Logger) *FilePathGenerator {
	return &FilePathGenerator{
		logger:     logger.With().Str("component", "FilePathGenerator").Logger(),
		urlHashGen: urlHashGen,
		basePath:   basePath,
	}
}

// GenerateHistoryFilePath returns the path to the Parquet file for a specific URL
func (fpg *FilePathGenerator) GenerateHistoryFilePath(recordURL string) (string, error) {
	hostnameWithPort, err := fpg.extractHostnamePort(recordURL)
	if err != nil {
		return "", err
	}

	sanitizedHostPort := urlhandler.SanitizeHostnamePort(hostnameWithPort)
	urlHash := fpg.urlHashGen.GenerateHash(recordURL)
	fileName := fmt.Sprintf("%s_history.parquet", urlHash)

	urlSpecificDir := filepath.Join(fpg.basePath, monitorDataDir, sanitizedHostPort)
	if err := fpg.ensureDirectoryExists(urlSpecificDir); err != nil {
		return "", err
	}

	filePath := filepath.Join(urlSpecificDir, fileName)
	fpg.logger.Debug().
		Str("url", recordURL).
		Str("file_path", filePath).
		Str("url_hash", urlHash).
		Msg("Generated history file path")

	return filePath, nil
}

// extractHostnamePort extracts hostname:port from URL
func (fpg *FilePathGenerator) extractHostnamePort(recordURL string) (string, error) {
	hostnameWithPort, err := urlhandler.ExtractHostnameWithPort(recordURL)
	if err != nil {
		fpg.logger.Error().Err(err).Str("url", recordURL).Msg("Failed to extract hostname:port for history file path")
		return "", common.WrapError(err, "failed to extract hostname:port from URL: "+recordURL)
	}
	return hostnameWithPort, nil
}

// ensureDirectoryExists creates directory if it doesn't exist
func (fpg *FilePathGenerator) ensureDirectoryExists(directory string) error {
	if err := os.MkdirAll(directory, 0755); err != nil {
		fpg.logger.Error().Err(err).Str("directory", directory).Msg("Failed to create URL-specific directory for history file")
		return common.WrapError(err, "failed to create directory: "+directory)
	}
	return nil
} 