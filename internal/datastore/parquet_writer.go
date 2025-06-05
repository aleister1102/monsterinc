package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
)

// ParquetWriter handles writing probe results to Parquet files.
type ParquetWriter struct {
	config *config.StorageConfig
	logger zerolog.Logger
	// parquetReader *ParquetReader // Removed: No longer needed for internal merge
}

// NewParquetWriter creates a new ParquetWriter.
// It no longer takes a ParquetReader.
func NewParquetWriter(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetWriter, error) {
	if cfg == nil {
		return nil, common.NewValidationError("config", cfg, "storage config cannot be nil")
	}
	// Removed reader nil check as it's no longer a parameter
	if cfg.ParquetBasePath == "" {
		logger.Warn().Msg("ParquetBasePath is empty in config. Parquet writing will be effectively disabled for some operations or use defaults.")
	}
	return &ParquetWriter{
		config: cfg,
		logger: logger.With().Str("component", "ParquetWriter").Logger(),
		// parquetReader: reader, // Removed
	}, nil
}

// transformToParquetResult converts a models.ProbeResult to a models.ParquetProbeResult.
// scanTime is the timestamp of the current scan session.
func (pw *ParquetWriter) transformToParquetResult(pr models.ProbeResult, scanTime time.Time) models.ParquetProbeResult {
	var headersJSON *string
	if len(pr.Headers) > 0 {
		jsonData, err := json.Marshal(pr.Headers)
		if err == nil {
			strData := string(jsonData)
			headersJSON = &strData
		} else {
			pw.logger.Error().Err(err).Str("url", pr.InputURL).Msg("Failed to marshal headers for URL")
		}
	}

	// Convert []models.Technology to []string for Parquet
	var techNames []string
	for _, tech := range pr.Technologies {
		techNames = append(techNames, tech.Name)
	}

	// Ensure FirstSeenTimestamp is set correctly if OldestScanTimestamp is zero (new item)
	// For a simple writer, OldestScanTimestamp might just be the current scan time if not provided
	firstSeen := pr.OldestScanTimestamp
	if firstSeen.IsZero() {
		firstSeen = scanTime
	}

	return models.ParquetProbeResult{
		OriginalURL:   pr.InputURL,
		FinalURL:      strPtrOrNil(pr.FinalURL),
		StatusCode:    int32PtrOrNilZero(int32(pr.StatusCode)),
		ContentLength: int64PtrOrNilZero(pr.ContentLength),
		ContentType:   strPtrOrNil(pr.ContentType),
		Title:         strPtrOrNil(pr.Title),
		WebServer:     strPtrOrNil(pr.WebServer),
		Technologies:  techNames,
		IPAddress:     pr.IPs,
		RootTargetURL: strPtrOrNil(pr.RootTargetURL),
		ProbeError:    strPtrOrNil(pr.Error),
		Method:        strPtrOrNil(pr.Method),
		HeadersJSON:   headersJSON,

		DiffStatus:         strPtrOrNil(pr.URLStatus),
		ScanTimestamp:      scanTime.UnixMilli(),
		FirstSeenTimestamp: models.TimePtrToUnixMilliOptional(firstSeen), // Will be scanTime if OldestScanTimestamp was zero
		LastSeenTimestamp:  models.TimePtrToUnixMilliOptional(scanTime),  // Always current scan time for this record version
	}
}

// Write takes a slice of ProbeResult, a scanSessionID (can be a timestamp or unique ID),
// and the rootTarget string, then writes them to a Parquet file specific to that rootTarget.
// This version overwrites the file with the currentProbeResults, it does not merge with historical data.
// It now accepts a context for cancellation.
func (pw *ParquetWriter) Write(ctx context.Context, currentProbeResults []models.ProbeResult, scanSessionID string, rootTarget string) error {
	if pw.config.ParquetBasePath == "" {
		return common.NewValidationError("parquet_base_path", pw.config.ParquetBasePath, "ParquetBasePath is not configured")
	}

	sanitizedRootTarget := urlhandler.SanitizeFilename(rootTarget)
	if sanitizedRootTarget == "" {
		return common.NewValidationError("root_target", rootTarget, "sanitized root target is empty, cannot write parquet file for: "+rootTarget)
	}

	// Check for cancellation before starting
	if result := common.CheckCancellationWithLog(ctx, pw.logger, "parquet write start"); result.Cancelled {
		return result.Error
	}

	// Create scan-specific directory
	scanOutputDir := filepath.Join(pw.config.ParquetBasePath, "scan")
	if err := os.MkdirAll(scanOutputDir, 0755); err != nil {
		return common.WrapError(err, "failed to create scan-specific Parquet directory '"+scanOutputDir+"'")
	}

	// Generate file path
	fileName := fmt.Sprintf("%s.parquet", sanitizedRootTarget)
	filePath := filepath.Join(scanOutputDir, fileName)

	pw.logger.Info().
		Str("file_path", filePath).
		Str("scan_session_id", scanSessionID).
		Str("root_target", rootTarget).
		Int("probe_count", len(currentProbeResults)).
		Msg("Writing probe results to Parquet file")

	// Check for cancellation before file operations
	if result := common.CheckCancellationWithLog(ctx, pw.logger, "before file creation"); result.Cancelled {
		return result.Error
	}

	// Create/truncate the file
	file, err := os.Create(filePath)
	if err != nil {
		return common.WrapError(err, "failed to create/truncate parquet file "+filePath)
	}
	defer file.Close()

	// Transform probe results to parquet format
	scanTime := time.Now()
	var parquetResults []models.ParquetProbeResult

	for _, pr := range currentProbeResults {
		// Check for cancellation during transformation
		select {
		case <-ctx.Done():
			result := common.CheckCancellationWithLog(ctx, pw.logger, "during result transformation")
			if result.Cancelled {
				return result.Error
			}
		default:
		}

		parquetResult := pw.transformToParquetResult(pr, scanTime)
		parquetResults = append(parquetResults, parquetResult)
	}

	// Check for cancellation before writing
	if result := common.CheckCancellationWithLog(ctx, pw.logger, "before parquet write"); result.Cancelled {
		return result.Error
	}

	// Write to Parquet file
	writer := parquet.NewGenericWriter[models.ParquetProbeResult](file, parquet.Compression(&parquet.Zstd))
	defer writer.Close()

	_, err = writer.Write(parquetResults)
	if err != nil {
		return common.WrapError(err, "failed to write probe results to parquet file")
	}

	pw.logger.Info().
		Str("file_path", filePath).
		Int("records_written", len(parquetResults)).
		Msg("Successfully wrote probe results to Parquet file")

	return nil
}

// Helper to convert string to pointer, or nil if string is empty.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Helper to convert int32 to pointer, or nil if value is 0.
func int32PtrOrNilZero(i int32) *int32 {
	if i == 0 {
		return nil
	}
	return &i
}

// Helper to convert int64 to pointer, or nil if value is 0.
func int64PtrOrNilZero(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}
