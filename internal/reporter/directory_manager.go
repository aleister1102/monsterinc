package reporter

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

// DirectoryManager manages creating and managing directories for reports
type DirectoryManager struct {
	logger zerolog.Logger
}

// NewDirectoryManager creates a new DirectoryManager
func NewDirectoryManager(logger zerolog.Logger) *DirectoryManager {
	return &DirectoryManager{
		logger: logger,
	}
}

// EnsureOutputDirectories ensures output directories are created
func (dm *DirectoryManager) EnsureOutputDirectories(outputDir string) error {
	if err := dm.createDirectory(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}
	return nil
}

// EnsureDiffReportDirectories ensures directories for diff reports are created
func (dm *DirectoryManager) EnsureDiffReportDirectories() error {
	// Create main diff report directory
	if err := dm.createDirectory(DefaultDiffReportDir); err != nil {
		return fmt.Errorf("failed to create diff report output directory %s: %w", DefaultDiffReportDir, err)
	}

	// Create assets directory for diff reports
	if err := dm.createDirectory(DefaultDiffReportAssetsDir); err != nil {
		return fmt.Errorf("failed to create diff report assets directory %s: %w", DefaultDiffReportAssetsDir, err)
	}

	return nil
}

// createDirectory creates directory with standard permissions
func (dm *DirectoryManager) createDirectory(path string) error {
	if err := os.MkdirAll(path, DirPermissions); err != nil {
		dm.logger.Error().Err(err).Str("path", path).Msg("Failed to create directory")
		return err
	}

	dm.logger.Debug().Str("path", path).Msg("Directory created successfully")
	return nil
}

// LogWorkingDirectory logs current working directory for debugging
func (dm *DirectoryManager) LogWorkingDirectory(targetDir string) {
	if wd, err := os.Getwd(); err == nil {
		dm.logger.Info().Str("working_directory", wd).Str("target_dir", targetDir).Msg("Current working directory info")
	}
}
