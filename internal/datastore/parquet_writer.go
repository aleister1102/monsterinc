package datastore

import (
	"encoding/json"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"
)

// ParquetWriter is responsible for writing probe results to Parquet files
// using the parquet-go/parquet-go library.
type ParquetWriter struct {
	config *config.StorageConfig
	logger *log.Logger
}

// NewParquetWriter creates a new ParquetWriter for parquet-go/parquet-go.
func NewParquetWriter(cfg *config.StorageConfig, appLogger *log.Logger) (*ParquetWriter, error) {
	if cfg == nil {
		cfg = &config.StorageConfig{ParquetBasePath: "./parquet_data", CompressionCodec: "ZSTD"}
		log.Println("[WARN] ParquetWriter (parquet-go/parquet-go): StorageConfig is nil, using default values.")
	}
	if appLogger == nil {
		appLogger = log.New(os.Stdout, "[ParquetWriterGo] ", log.LstdFlags)
	}

	pw := &ParquetWriter{
		config: cfg,
		logger: appLogger,
	}

	if pw.config.ParquetBasePath == "" {
		pw.config.ParquetBasePath = "./parquet_data" // Default path
		appLogger.Printf("ParquetBasePath not set, using default: %s", pw.config.ParquetBasePath)
	}

	err := os.MkdirAll(pw.config.ParquetBasePath, 0755)
	if err != nil {
		appLogger.Printf("Error creating base Parquet directory '%s': %v", pw.config.ParquetBasePath, err)
		return nil, err
	}

	appLogger.Printf("ParquetWriter (parquet-go/parquet-go) initialized. Base path: %s, Compression: %s",
		pw.config.ParquetBasePath, pw.config.CompressionCodec)

	return pw, nil
}

// transformToParquetResult converts models.ProbeResult to models.ParquetProbeResult.
func (pw *ParquetWriter) transformToParquetResult(pr models.ProbeResult, scanTime int64, rootTarget string) models.ParquetProbeResult {
	var headersJSON *string
	if len(pr.Headers) > 0 {
		jsonData, err := json.Marshal(pr.Headers)
		if err == nil {
			s := string(jsonData)
			headersJSON = &s
		} else {
			pw.logger.Printf("[WARN] Failed to marshal headers to JSON for URL %s: %v", pr.InputURL, err)
		}
	}

	pString := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	pInt32 := func(i int) *int32 {
		val := int32(i)
		if i == 0 && pr.Error != "" {
			return nil
		}
		return &val
	}
	pInt64 := func(i int64) *int64 {
		if i == 0 && pr.StatusCode == 0 && pr.Error != "" {
			return nil
		}
		return &i
	}
	pFloat64 := func(f float64) *float64 {
		if f == 0.0 && pr.StatusCode == 0 && pr.Error != "" {
			return nil
		}
		return &f
	}
	pTimeInt64 := func(t time.Time) *int64 {
		if t.IsZero() {
			return nil
		}
		val := t.UnixMilli()
		return &val
	}

	parquetPr := models.ParquetProbeResult{
		OriginalURL:   pr.InputURL,
		FinalURL:      pString(pr.FinalURL),
		StatusCode:    pInt32(pr.StatusCode),
		ContentLength: pInt64(pr.ContentLength),
		ContentType:   pString(pr.ContentType),
		Title:         pString(pr.Title),
		WebServer:     pString(pr.WebServer),
		ScanTimestamp: scanTime,
		RootTargetURL: pString(rootTarget),
		ProbeError:    pString(pr.Error),
		Method:        pString(pr.Method),
		Duration:      pFloat64(pr.Duration),
		HeadersJSON:   headersJSON,
		ASN:           pInt32(pr.ASN),
		ASNOrg:        pString(pr.ASNOrg),
		TLSVersion:    pString(pr.TLSVersion),
		TLSCipher:     pString(pr.TLSCipher),
		TLSCertIssuer: pString(pr.TLSCertIssuer),
		TLSCertExpiry: pTimeInt64(pr.TLSCertExpiry),
	}

	if len(pr.Technologies) > 0 {
		parquetPr.Technologies = make([]string, len(pr.Technologies))
		for i, tech := range pr.Technologies {
			parquetPr.Technologies[i] = tech.Name
		}
	} else {
		parquetPr.Technologies = []string{}
	}

	parquetPr.IPAddress = pr.IPs
	if parquetPr.IPAddress == nil {
		parquetPr.IPAddress = []string{}
	}

	parquetPr.CNAMEs = pr.CNAMEs
	if parquetPr.CNAMEs == nil {
		parquetPr.CNAMEs = []string{}
	}
	return parquetPr
}

func (pw *ParquetWriter) Write(probeResults []models.ProbeResult, scanSessionID string, rootTarget string) error {
	if len(probeResults) == 0 {
		pw.logger.Println("No probe results to write. Skipping Parquet file generation (parquet-go/parquet-go).")
		return nil
	}

	dateStr := time.Now().Format("20060102")
	datedPath := filepath.Join(pw.config.ParquetBasePath, dateStr)
	if err := os.MkdirAll(datedPath, 0755); err != nil {
		pw.logger.Printf("Error creating dated Parquet directory '%s': %v", datedPath, err)
		return err
	}

	fileNameBase := scanSessionID
	if fileNameBase == "" {
		fileNameBase = time.Now().Format("150405")
	}
	safeRootTarget := strings.ReplaceAll(strings.ReplaceAll(rootTarget, "https://", ""), "http://", "")
	safeRootTarget = strings.ReplaceAll(safeRootTarget, ":", "_")
	safeRootTarget = strings.ReplaceAll(safeRootTarget, "/", "_")
	if safeRootTarget == "" {
		safeRootTarget = "unknown_target"
	}
	fileName := fmt.Sprintf("scan_results_%s.parquet", fileNameBase)
	filePath := filepath.Join(datedPath, fileName)

	pw.logger.Printf("Preparing to write %d results to Parquet file (parquet-go/parquet-go): %s", len(probeResults), filePath)

	outputFile, err := os.Create(filePath)
	if err != nil {
		pw.logger.Printf("Failed to create Parquet file '%s': %v", filePath, err)
		return err
	}
	defer outputFile.Close()

	var writerOptions []parquet.WriterOption
	configCompression := strings.ToUpper(pw.config.CompressionCodec)
	pw.logger.Printf("Attempting to use %s compression for Parquet file (parquet-go/parquet-go).", configCompression)

	switch configCompression {
	case "ZSTD":
		writerOptions = append(writerOptions, parquet.Compression(&parquet.Zstd))
	case "SNAPPY":
		writerOptions = append(writerOptions, parquet.Compression(&parquet.Snappy))
	case "GZIP":
		writerOptions = append(writerOptions, parquet.Compression(&parquet.Gzip))
	case "UNCOMPRESSED":
		writerOptions = append(writerOptions, parquet.Compression(&parquet.Uncompressed))
	default:
		pw.logger.Printf("Unsupported compression codec '%s' for parquet-go/parquet-go, defaulting to UNCOMPRESSED.", configCompression)
		writerOptions = append(writerOptions, parquet.Compression(&parquet.Uncompressed))
	}

	// Schema is inferred from models.ParquetProbeResult by NewGenericWriter
	parquetFileWriter := parquet.NewGenericWriter[models.ParquetProbeResult](outputFile, writerOptions...)

	scanTime := time.Now().UnixMilli()
	dataToWrite := make([]models.ParquetProbeResult, len(probeResults))
	for i, pr := range probeResults {
		dataToWrite[i] = pw.transformToParquetResult(pr, scanTime, rootTarget)
	}

	_, err = parquetFileWriter.Write(dataToWrite)
	if err != nil {
		pw.logger.Printf("Error writing records to Parquet file '%s': %v", filePath, err)
		// It's good practice to attempt to close the writer even on error to release resources,
		// though some errors might prevent successful closing.
		_ = parquetFileWriter.Close() // Best effort close
		return err
	}

	if err := parquetFileWriter.Close(); err != nil {
		pw.logger.Printf("Error closing Parquet writer (finalizing file) '%s': %v", filePath, err)
		return err
	}

	pw.logger.Printf("Successfully wrote %d records to Parquet file: %s (compression: %s via parquet-go/parquet-go)",
		len(probeResults), filePath, configCompression)
	return nil
}

// Pointer helper functions can be kept if they are generally useful
// func pString(s string) *string { if s == "" { return nil }; return &s }
// func pInt32(i int32) *int32 { return &i } // For xitongsys, check if it needs *int32 or int32 for optional
