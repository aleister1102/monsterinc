package datastore

import (
	"context"
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
	// parquetReader *ParquetReader // Removed: No longer needed for internal merge
}

// NewParquetWriter creates a new ParquetWriter.
// It no longer takes a ParquetReader.
func NewParquetWriter(cfg *config.StorageConfig, logger zerolog.Logger) (*ParquetWriter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config cannot be nil")
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

/*
// mergeProbeResults function is commented out as the writer no longer merges internally.
// This logic might reside in the orchestrator or be handled differently if true historical append is needed.
func (pw *ParquetWriter) mergeProbeResults(currentProbes []models.ProbeResult, historicalProbes []models.ProbeResult, currentScanTime time.Time) []models.ProbeResult {
	pw.logger.Debug().Int("current_count", len(currentProbes)).Int("historical_count", len(historicalProbes)).Msg("Starting merge of probe results")
	mergedResultsMap := make(map[string]models.ProbeResult)

	for _, hp := range historicalProbes {
		key := hp.InputURL
		if key == "" && hp.FinalURL != "" {
			key = hp.FinalURL
		}
		if key == "" {
			pw.logger.Warn().Interface("probe", hp).Msg("Historical probe with no InputURL or FinalURL, cannot merge.")
			continue
		}
		if hp.OldestScanTimestamp.IsZero() && !hp.Timestamp.IsZero() {
			hp.OldestScanTimestamp = hp.Timestamp
		}
		mergedResultsMap[key] = hp
	}
	pw.logger.Debug().Int("historical_map_size", len(mergedResultsMap)).Msg("Historical probes mapped.")

	for _, cp := range currentProbes {
		key := cp.InputURL
		if key == "" && cp.FinalURL != "" {
			key = cp.FinalURL
		}
		if key == "" {
			pw.logger.Warn().Interface("probe", cp).Msg("Current probe with no InputURL or FinalURL, cannot merge.")
			continue
		}

		if existingProbe, found := mergedResultsMap[key]; found {
			updatedProbe := existingProbe
			updatedProbe.FinalURL = cp.FinalURL
			updatedProbe.StatusCode = cp.StatusCode
			updatedProbe.ContentLength = cp.ContentLength
			updatedProbe.ContentType = cp.ContentType
			updatedProbe.Title = cp.Title
			updatedProbe.WebServer = cp.WebServer
			updatedProbe.Technologies = cp.Technologies
			updatedProbe.IPs = cp.IPs
			updatedProbe.Error = cp.Error
			updatedProbe.Method = cp.Method
			updatedProbe.Headers = cp.Headers
			updatedProbe.Duration = cp.Duration
			updatedProbe.CNAMEs = cp.CNAMEs
			updatedProbe.ASN = cp.ASN
			updatedProbe.ASNOrg = cp.ASNOrg
			updatedProbe.TLSVersion = cp.TLSVersion
			updatedProbe.TLSCipher = cp.TLSCipher
			updatedProbe.TLSCertIssuer = cp.TLSCertIssuer
			updatedProbe.TLSCertExpiry = cp.TLSCertExpiry
			updatedProbe.Timestamp = currentScanTime
			updatedProbe.URLStatus = cp.URLStatus
			mergedResultsMap[key] = updatedProbe
			pw.logger.Debug().Str("url", key).Msg("Updated existing probe in merge map.")
		} else {
			newProbe := cp
			newProbe.OldestScanTimestamp = currentScanTime
			newProbe.Timestamp = currentScanTime
			mergedResultsMap[key] = newProbe
			pw.logger.Debug().Str("url", key).Msg("Added new probe to merge map.")
		}
	}

	finalMergedProbes := make([]models.ProbeResult, 0, len(mergedResultsMap))
	for _, probe := range mergedResultsMap {
		finalMergedProbes = append(finalMergedProbes, probe)
	}
	pw.logger.Debug().Int("final_merged_count", len(finalMergedProbes)).Msg("Probe results merged.")
	return finalMergedProbes
}
*/

// Write takes a slice of ProbeResult, a scanSessionID (can be a timestamp or unique ID),
// and the rootTarget string, then writes them to a Parquet file specific to that rootTarget.
// This version overwrites the file with the currentProbeResults, it does not merge with historical data.
// It now accepts a context for cancellation.
func (pw *ParquetWriter) Write(ctx context.Context, currentProbeResults []models.ProbeResult, scanSessionID string, rootTarget string) error {
	currentScanTime := time.Now() // Consistent timestamp for this write operation
	pw.logger.Info().Str("root_target", rootTarget).Str("session_id", scanSessionID).Int("current_result_count", len(currentProbeResults)).Msg("Starting Parquet write operation (overwrite)")

	// Check for context cancellation at the beginning
	select {
	case <-ctx.Done():
		pw.logger.Info().Str("root_target", rootTarget).Msg("Parquet write cancelled before starting.")
		return ctx.Err()
	default:
	}

	if pw.config == nil || pw.config.ParquetBasePath == "" {
		pw.logger.Error().Msg("ParquetBasePath is not configured. Cannot write Parquet file.")
		return fmt.Errorf("ParquetWriter: ParquetBasePath is not configured")
	}

	sanitizedTargetName := urlhandler.SanitizeFilename(rootTarget)
	if sanitizedTargetName == "" {
		pw.logger.Error().Str("original_target", rootTarget).Msg("Root target sanitized to empty string, cannot create valid path for Parquet file.")
		return fmt.Errorf("sanitized root target is empty, cannot write parquet file for: %s", rootTarget)
	}

	// Ensure the base directory for Parquet files exists
	if err := os.MkdirAll(pw.config.ParquetBasePath, 0755); err != nil {
		pw.logger.Error().Err(err).Str("path", pw.config.ParquetBasePath).Msg("Failed to create base Parquet directory")
		return fmt.Errorf("failed to create base Parquet directory '%s': %w", pw.config.ParquetBasePath, err)
	}

	fileName := fmt.Sprintf("%s.parquet", sanitizedTargetName)     // Filename is <sanitized_rootTarget>.parquet
	filePath := filepath.Join(pw.config.ParquetBasePath, fileName) // Path is <base_path>/<sanitized_rootTarget>.parquet
	pw.logger.Info().Str("path", filePath).Msg("Target Parquet file path for writing (overwrite)")

	// Removed historical data reading and merging logic
	// var historicalProbes []models.ProbeResult
	// ... read logic ...
	// allProbesToStore := pw.mergeProbeResults(currentProbeResults, historicalProbes, currentScanTime)

	// Directly use currentProbeResults
	allProbesToStore := currentProbeResults

	if len(allProbesToStore) == 0 {
		pw.logger.Info().Str("root_target", rootTarget).Msg("No probe results to write. Skipping Parquet file operation.")
		// Optionally, delete the file if it exists and we are writing an empty set
		// For now, if it exists and allProbesToStore is empty, it will be overwritten with an empty parquet file.
		// If it doesn't exist, an empty parquet file will be created.
	}

	file, err := os.Create(filePath) // This will truncate/overwrite if the file exists
	if err != nil {
		pw.logger.Error().Err(err).Str("path", filePath).Msg("Failed to create/truncate Parquet file for writing")
		return fmt.Errorf("failed to create/truncate parquet file %s: %w", filePath, err)
	}
	defer func() {
		if ferr := file.Close(); ferr != nil {
			pw.logger.Error().Err(ferr).Str("path", filePath).Msg("Failed to close Parquet file after writing")
		}
	}()

	options := []parquet.WriterOption{
		parquet.Compression(&parquet.Zstd),
	}
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
			pw.logger.Debug().Msg("Using Zstd compression for Parquet.")
		default:
			pw.logger.Warn().Str("codec", pw.config.CompressionCodec).Msg("Unsupported compression codec specified, defaulting to Zstd.")
		}
	}

	schemaPtr := parquet.SchemaOf(models.ParquetProbeResult{})
	if schemaPtr == nil {
		pw.logger.Error().Msg("Failed to generate parquet schema, cannot create writer.")
		return fmt.Errorf("failed to generate parquet schema")
	}
	options = append(options, schemaPtr)

	w := parquet.NewWriter(file, options...)

	writeCount := 0
	for i, pr := range allProbesToStore {
		// Check for context cancellation inside the loop
		if i%100 == 0 { // Check periodically, e.g., every 100 records
			select {
			case <-ctx.Done():
				pw.logger.Info().Str("path", filePath).Int("records_written", writeCount).Msg("Parquet write cancelled during record writing.")
				_ = w.Close()           // Attempt to close writer
				_ = file.Close()        // Attempt to close file
				_ = os.Remove(filePath) // Attempt to clean up partially written file
				return ctx.Err()
			default:
			}
		}

		parquetResult := pw.transformToParquetResult(pr, currentScanTime)
		if err := w.Write(&parquetResult); err != nil {
			pw.logger.Error().Err(err).Str("path", filePath).Msg("Failed to write record to Parquet file")
			_ = w.Close()
			_ = file.Close()
			_ = os.Remove(filePath) // Attempt to clean up
			return fmt.Errorf("failed to write record to parquet file %s: %w", filePath, err)
		}
		writeCount++
	}

	if err := w.Close(); err != nil {
		pw.logger.Error().Err(err).Str("path", filePath).Msg("Failed to close Parquet writer")
		return fmt.Errorf("failed to close parquet writer for %s: %w", filePath, err)
	}

	pw.logger.Info().Str("path", filePath).Int("record_count", writeCount).Msg("Successfully wrote (overwrote) Parquet file")
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
