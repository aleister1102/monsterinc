package datastore

import (
	"encoding/json"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/xitongsys/parquet-go-source/localfile"
	"github.com/xitongsys/parquet-go/writer"
)

// ParquetWriter handles writing probe results to Parquet files.
type ParquetWriter struct {
	config *config.StorageConfig
	logger zerolog.Logger
}

// NewParquetWriter creates a new ParquetWriter.
func NewParquetWriter(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetWriter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config cannot be nil")
	}
	if cfg.ParquetBasePath == "" {
		logger.Warn().Msg("ParquetBasePath is empty in config. Parquet writing will be effectively disabled for some operations or use defaults.")
		// Depending on strictness, could return an error or a disabled writer.
		// For now, allow creation but log a clear warning.
	}
	return &ParquetWriter{
		config: cfg,
		logger: logger.With().Str("component", "ParquetWriter").Logger(),
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

		// New/Updated fields
		DiffStatus:         strPtrOrNil(pr.URLStatus),
		ScanTimestamp:      scanTime.UnixMilli(),                                      // Current scan session time
		FirstSeenTimestamp: models.TimePtrToUnixMilliOptional(pr.OldestScanTimestamp), // When this URL was first ever recorded
		LastSeenTimestamp:  models.TimePtrToUnixMilliOptional(pr.Timestamp),           // Could be same as ScanTimestamp, or older if it's an 'old' record being re-saved for some reason (though usually 'old' are not re-written)

		// Fields that were removed from ParquetProbeResult are no longer mapped here:
		// Duration, CNAMEs, ASN, ASNOrg, TLSVersion, TLSCipher, TLSCertIssuer, TLSCertExpiry
	}
}

// Write takes a slice of ProbeResult, a scanSessionID (can be a timestamp or unique ID),
// and the rootTarget string, then writes them to a Parquet file specific to that rootTarget.
// The URL diffing and population of URLStatus and OldestScanTimestamp in ProbeResult
// is expected to have been done *before* calling this Write method.
func (pw *ParquetWriter) Write(probeResults []models.ProbeResult, scanSessionID string, rootTarget string) error {
	if len(probeResults) == 0 {
		pw.logger.Info().Str("target", rootTarget).Str("session", scanSessionID).Msg("No probe results to write for target")
		return nil
	}

	normalizedRootTarget, err := urlhandler.NormalizeURL(rootTarget)
	if err != nil {
		pw.logger.Error().Err(err).Str("target", rootTarget).Msg("Error normalizing root target URL")
		normalizedRootTarget = rootTarget // Fallback to raw if normalization fails
	}

	filename := urlhandler.SanitizeFilename(normalizedRootTarget) + ".parquet"
	filePath := filepath.Join(pw.config.ParquetBasePath, filename)
	pw.logger.Info().Int("num_results", len(probeResults)).Str("target", rootTarget).Str("session", scanSessionID).Str("file_path", filePath).Msg("Preparing to write probe results to Parquet file")

	// Ensure the directory exists
	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		pw.logger.Error().Err(err).Str("path", filepath.Dir(filePath)).Msg("Failed to create directory for Parquet file")
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(filePath), err)
	}

	fw, err := localfile.NewLocalFileWriter(filePath)
	if err != nil {
		pw.logger.Error().Err(err).Str("file_path", filePath).Msg("Failed to create local Parquet file writer")
		return fmt.Errorf("failed to create Parquet file writer for %s: %w", filePath, err)
	}
	defer fw.Close()

	pqWriter, err := writer.NewParquetWriter(fw, new(models.ParquetProbeResult), 4) // Concurrency for writing
	if err != nil {
		pw.logger.Error().Err(err).Msg("Failed to create Parquet writer instance")
		return fmt.Errorf("failed to create Parquet writer: %w", err)
	}

	scanTime := time.Now() // Use a consistent scan time for all records in this batch
	for _, pr := range probeResults {
		// Important: pr.URLStatus and pr.OldestScanTimestamp should be populated by the diffing logic *before* this point.
		if err = pqWriter.Write(pw.transformToParquetResult(pr, scanTime)); err != nil {
			pw.logger.Error().Err(err).Interface("problematic_record", pr).Msg("Failed to write record to Parquet file")
			// Decide if we should continue or abort. For now, log and continue.
		}
	}

	if err = pqWriter.WriteStop(); err != nil {
		pw.logger.Error().Err(err).Msg("Failed to stop Parquet writer")
		return fmt.Errorf("failed to stop Parquet writer: %w", err)
	}

	pw.logger.Info().Str("file_path", filePath).Int("records_written", len(probeResults)).Msg("Successfully wrote data to Parquet file")
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

// Helper to convert float64 to pointer, or nil if value is 0.0.
// Note: This might not always be desired if 0.0 is a valid meaningful value.
func float64PtrOrNilZero(f float64) *float64 {
	if f == 0.0 {
		return nil
	}
	return &f
}

// getParquetCompressionCodec is no longer needed as the logic is inlined in Write.
// func getParquetCompressionCodec(codecName string) (interface{}, error) {
// ...
// }
