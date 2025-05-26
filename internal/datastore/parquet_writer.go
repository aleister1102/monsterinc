package datastore

import (
	"encoding/json"
	"fmt"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/rs/zerolog"
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
	pw.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Int("result_count", len(probeResults)).Msg("Starting Parquet write operation")

	if pw.config == nil || pw.config.ParquetBasePath == "" {
		pw.logger.Error().Msg("ParquetBasePath is not configured. Cannot write Parquet file.")
		return fmt.Errorf("ParquetWriter: ParquetBasePath is not configured")
	}
	if len(probeResults) == 0 {
		pw.logger.Info().Str("root_target", rootTarget).Msg("No probe results to write for target. Skipping Parquet file creation.")
		return nil
	}

	sanitizedTargetName := urlhandler.SanitizeFilename(rootTarget)
	if sanitizedTargetName == "" {
		pw.logger.Error().Str("original_target", rootTarget).Msg("Root target sanitized to empty string, cannot create valid path for Parquet file.")
		return fmt.Errorf("sanitized root target is empty, cannot write parquet file for: %s", rootTarget)
	}

	// Create a unique directory for this scan session under the target's directory
	sessionDir := filepath.Join(pw.config.ParquetBasePath, sanitizedTargetName, scanSessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		pw.logger.Error().Err(err).Str("path", sessionDir).Msg("Failed to create session directory for Parquet file")
		return fmt.Errorf("failed to create session directory '%s': %w", sessionDir, err)
	}

	fileName := "data.parquet"
	filePath := filepath.Join(sessionDir, fileName)
	pw.logger.Info().Str("path", filePath).Msg("Target Parquet file path determined")

	// Create the output file
	file, err := os.Create(filePath)
	if err != nil {
		pw.logger.Error().Err(err).Str("path", filePath).Msg("Failed to create Parquet file")
		return fmt.Errorf("failed to create parquet file %s: %w", filePath, err)
	}
	defer func() {
		if ferr := file.Close(); ferr != nil {
			pw.logger.Error().Err(ferr).Str("path", filePath).Msg("Failed to close Parquet file after writing")
			// Decide if this should overwrite the original error or be appended.
			// For now, just log it, as the write operation might have already failed.
		}
	}()

	// Configure Parquet writer options
	options := []parquet.WriterOption{
		parquet.Compression(&parquet.Zstd),
	}
	// Allow overriding compression codec via config
	if pw.config.CompressionCodec != "" {
		switch strings.ToLower(pw.config.CompressionCodec) {
		case "snappy":
			options[0] = parquet.Compression(&parquet.Snappy)
			pw.logger.Debug().Msg("Using Snappy compression for Parquet.")
		case "gzip":
			options[0] = parquet.Compression(&parquet.Gzip)
			pw.logger.Debug().Msg("Using Gzip compression for Parquet.")
		case "none":
			options[0] = parquet.Compression(&parquet.Uncompressed)
			pw.logger.Debug().Msg("Using Uncompressed for Parquet.")
		case "zstd":
			// Already default, but good to be explicit
			pw.logger.Debug().Msg("Using Zstd compression for Parquet.")
		default:
			pw.logger.Warn().Str("codec", pw.config.CompressionCodec).Msg("Unsupported compression codec specified, defaulting to Zstd.")
		}
	}

	// Create a schema from the ParquetProbeResult struct
	schemaPtr := parquet.SchemaOf(models.ParquetProbeResult{})
	if schemaPtr == nil {
		pw.logger.Error().Msg("Failed to generate parquet schema, cannot create writer.")
		return fmt.Errorf("failed to generate parquet schema")
	}

	// Add schema as a writer option
	options = append(options, schemaPtr) // Pass the schema pointer directly

	w := parquet.NewWriter(file, options...)

	scanTime := time.Now() // Consistent timestamp for all records in this batch

	for _, pr := range probeResults {
		parquetResult := pw.transformToParquetResult(pr, scanTime)
		if err := w.Write(&parquetResult); err != nil {
			pw.logger.Error().Err(err).Str("path", filePath).Msg("Failed to write record to Parquet file")
			// Attempt to close and remove the partially written file to avoid corruption
			_ = w.Close()           // Best effort close
			_ = file.Close()        // Best effort close underlying file
			_ = os.Remove(filePath) // Best effort remove
			return fmt.Errorf("failed to write record to parquet file %s: %w", filePath, err)
		}
	}

	if err := w.Close(); err != nil {
		pw.logger.Error().Err(err).Str("path", filePath).Msg("Failed to close Parquet writer")
		return fmt.Errorf("failed to close parquet writer for %s: %w", filePath, err)
	}

	pw.logger.Info().Str("path", filePath).Int("record_count", len(probeResults)).Msg("Successfully wrote Parquet file")
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
