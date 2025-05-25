package datastore

import (
	"encoding/json"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"os"
	"path/filepath"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/brotli"
	"github.com/parquet-go/parquet-go/compress/gzip"
	"github.com/parquet-go/parquet-go/compress/lz4"
	"github.com/parquet-go/parquet-go/compress/snappy"
	"github.com/parquet-go/parquet-go/compress/uncompressed"
	"github.com/parquet-go/parquet-go/compress/zstd"
)

// ParquetWriter handles writing probe results to Parquet files.
type ParquetWriter struct {
	config *config.StorageConfig
	logger *log.Logger
}

// NewParquetWriter creates a new ParquetWriter.
func NewParquetWriter(cfg *config.StorageConfig, appLogger *log.Logger) (*ParquetWriter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config cannot be nil")
	}
	if appLogger == nil {
		appLogger = log.New(os.Stderr, "[ParquetWriter] ", log.LstdFlags)
		appLogger.Println("Warning: No logger provided, using default stderr logger.")
	}
	pw := &ParquetWriter{
		config: cfg,
		logger: appLogger,
	}
	err := os.MkdirAll(pw.config.ParquetBasePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet base directory %s: %w", pw.config.ParquetBasePath, err)
	}
	return pw, nil
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
			pw.logger.Printf("Warning: Failed to marshal headers for URL %s: %v", pr.InputURL, err)
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
		pw.logger.Printf("No probe results to write for target: %s, session: %s", rootTarget, scanSessionID)
		return nil
	}

	normalizedRootTarget, err := urlhandler.NormalizeURL(rootTarget)
	if err != nil {
		pw.logger.Printf("Error normalizing root target URL %s: %v. Using raw root target for filename.", rootTarget, err)
		normalizedRootTarget = rootTarget // Fallback to raw if normalization fails
	}

	filename := urlhandler.SanitizeFilename(normalizedRootTarget) + ".parquet"
	filePath := filepath.Join(pw.config.ParquetBasePath, filename)
	pw.logger.Printf("Preparing to write %d probe results for target '%s' (session: %s) to Parquet file: %s", len(probeResults), rootTarget, scanSessionID, filePath)

	// We will overwrite the file if it exists, as Parquet is better for full dataset writes than appends generally.
	// If append is needed, a different strategy (e.g., multiple files per scan, or reading existing then rewriting) would be required.
	file, err := os.Create(filePath) // Overwrites or creates new
	if err != nil {
		return fmt.Errorf("failed to create/truncate parquet file %s: %w", filePath, err)
	}
	defer file.Close()

	parquetRows := make([]models.ParquetProbeResult, 0, len(probeResults))
	scanTime := time.Now() // Use a consistent scan time for all records in this batch
	for _, pr := range probeResults {
		// Important: pr.URLStatus and pr.OldestScanTimestamp should be populated by the diffing logic *before* this point.
		parquetRows = append(parquetRows, pw.transformToParquetResult(pr, scanTime))
	}

	pw.logger.Printf("Writing %d transformed Parquet rows to %s", len(parquetRows), filePath)

	var writerOptions []parquet.WriterOption

	// Determine the compression codec
	codecName := pw.config.CompressionCodec
	switch codecName {
	case "snappy":
		writerOptions = append(writerOptions, parquet.Compression(&snappy.Codec{}))
		pw.logger.Printf("Using Parquet compression: snappy")
	case "gzip":
		writerOptions = append(writerOptions, parquet.Compression(&gzip.Codec{}))
		pw.logger.Printf("Using Parquet compression: gzip")
	case "brotli":
		writerOptions = append(writerOptions, parquet.Compression(&brotli.Codec{}))
		pw.logger.Printf("Using Parquet compression: brotli")
	case "zstd":
		writerOptions = append(writerOptions, parquet.Compression(&zstd.Codec{}))
		pw.logger.Printf("Using Parquet compression: zstd")
	case "lz4raw":
		writerOptions = append(writerOptions, parquet.Compression(&lz4.Codec{}))
		pw.logger.Printf("Using Parquet compression: lz4raw")
	case "none", "uncompressed", "": // Treat empty string as uncompressed/default
		writerOptions = append(writerOptions, parquet.Compression(&uncompressed.Codec{}))
		if codecName == "" {
			pw.logger.Printf("No Parquet compression codec specified, using uncompressed.")
		} else {
			pw.logger.Printf("Using Parquet compression: %s", codecName)
		}
	default:
		pw.logger.Printf("Warning: Unsupported compression codec '%s'. Defaulting to ZSTD.", codecName)
		writerOptions = append(writerOptions, parquet.Compression(&zstd.Codec{}))
	}

	writer := parquet.NewGenericWriter[models.ParquetProbeResult](file, writerOptions...)

	numWritten, err := writer.Write(parquetRows)
	if err != nil {
		// Attempt to close the writer to flush any pending data, though it might also fail.
		_ = writer.Close() // Best effort close
		return fmt.Errorf("error writing parquet rows to %s (wrote %d before error): %w", filePath, numWritten, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("error closing parquet writer for %s (after writing %d rows): %w", filePath, numWritten, err)
	}

	pw.logger.Printf("Successfully wrote %d Parquet rows to %s for target: %s", numWritten, filePath, rootTarget)
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
