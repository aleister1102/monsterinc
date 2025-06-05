package datastore

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
)

// scanHistoryFile reads a history file and returns records that have diff data.
func (pfs *ParquetFileHistoryStore) scanHistoryFile(filePath string) ([]*models.FileHistoryRecord, error) {
	if strings.Contains(filepath.Base(filePath), "archived") {
		return []*models.FileHistoryRecord{}, nil
	}

	records, err := pfs.readRecordsFromFile(filePath)
	if err != nil {
		return nil, err
	}

	var diffRecords []*models.FileHistoryRecord
	for _, record := range records {
		if record.DiffResultJSON != nil && *record.DiffResultJSON != "" && *record.DiffResultJSON != "null" {
			diffRecords = append(diffRecords, record)
		}
	}

	return diffRecords, nil
}

// walkDirectoryForDiffs walks through the monitor directory to find history files with diffs
func (pfs *ParquetFileHistoryStore) walkDirectoryForDiffs(monitorBaseDir string) ([]*models.FileHistoryRecord, error) {
	allDiffRecords := make([]*models.FileHistoryRecord, 0)

	// Walk through the monitorBaseDir to find all host-specific directories
	// then look for *_history.parquet files in each.
	err := filepath.WalkDir(monitorBaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log the error but try to continue if possible, unless it's a critical error like permission denied to the root
			pfs.logger.Error().Err(err).Str("path", path).Msg("Error accessing path during walk for diff records")
			// If the error is related to the root directory itself, we probably can't proceed.
			if path == monitorBaseDir {
				return fmt.Errorf("error walking root monitor directory %s: %w", monitorBaseDir, err) // Stop walking
			}
			return nil // Skip this entry and continue
		}

		// We are looking for *_history.parquet files (new pattern)
		if !d.IsDir() && strings.HasSuffix(d.Name(), "_history.parquet") {
			diffRecords, scanErr := pfs.scanHistoryFile(path)
			if scanErr != nil {
				pfs.logger.Error().Err(scanErr).Str("file", path).Msg("Error scanning history file for diffs")
				return nil // Continue walking despite error
			}
			if diffRecords != nil {
				allDiffRecords = append(allDiffRecords, diffRecords...)
			}
		}
		return nil
	})

	if err != nil {
		// This error is from filepath.WalkDir itself (e.g., root dir not accessible)
		pfs.logger.Error().Err(err).Str("base_dir", monitorBaseDir).Msg("Failed to walk history directories to get records with diffs")
		return nil, fmt.Errorf("failed to walk history directories: %w", err)
	}

	return allDiffRecords, nil
}

// groupURLsByHost groups URLs by their hostname:port for optimized processing
func (pfs *ParquetFileHistoryStore) groupURLsByHost(urls []string) map[string]string {
	urlHostMap := make(map[string]string) // To optimize file reads, group URLs by host:port

	for _, u := range urls {
		hostnameWithPort, err := urlhandler.ExtractHostnameWithPort(u)
		if err != nil {
			pfs.logger.Warn().Err(err).Str("url", u).Msg("Failed to extract hostname:port from URL, skipping for latest diff result.")
			continue
		}
		urlHostMap[u] = hostnameWithPort
	}

	return urlHostMap
}

// GetHostnamesWithHistory retrieves a list of unique hostname:port combinations that have history records.
// This method scans the base monitor directory for subdirectories (each representing a hostname:port)
// and checks if they contain any *_history.parquet files.
func (pfs *ParquetFileHistoryStore) GetHostnamesWithHistory() ([]string, error) {
	hostnamesPorts := make([]string, 0)
	seenHostsPorts := make(map[string]bool)

	monitorBaseDir := filepath.Join(pfs.storageConfig.ParquetBasePath, monitorDataDir)

	entries, err := os.ReadDir(monitorBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return hostnamesPorts, nil
		}
		return nil, fmt.Errorf("failed to read monitor base directory '%s': %w", monitorBaseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			hostPortDirName := entry.Name()
			hostPortDir := filepath.Join(monitorBaseDir, hostPortDirName)

			// Check if this directory contains any *_history.parquet files
			dirEntries, err := os.ReadDir(hostPortDir)
			if err != nil {
				pfs.logger.Warn().Err(err).Str("dir", hostPortDir).Msg("Error reading host directory, skipping.")
				continue
			}

			hasHistoryFiles := false
			for _, dirEntry := range dirEntries {
				if !dirEntry.IsDir() && strings.HasSuffix(dirEntry.Name(), "_history.parquet") {
					hasHistoryFiles = true
					break
				}
			}

			if hasHistoryFiles {
				// Convert sanitized directory name back to hostname:port format
				hostPortRestored := urlhandler.RestoreHostnamePort(hostPortDirName)
				if !seenHostsPorts[hostPortRestored] {
					hostnamesPorts = append(hostnamesPorts, hostPortRestored)
					seenHostsPorts[hostPortRestored] = true
				}
			}
		}
	}

	pfs.logger.Info().Int("count", len(hostnamesPorts)).Msg("Successfully retrieved hostname:port combinations with history.")
	return hostnamesPorts, nil
} 