package datastore

import (
	"encoding/json"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/models"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
)

// helperCreateTestProbeResults remains the same or can be adjusted if new fields in ProbeResult need specific test values.
func helperCreateTestProbeResults() []models.ProbeResult {
	return []models.ProbeResult{
		{
			InputURL:      "http://example.com/page1",
			Method:        "GET",
			Timestamp:     time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			Duration:      0.5,
			StatusCode:    200,
			ContentLength: 1024,
			ContentType:   "text/html",
			Headers:       map[string]string{"X-Test-Header": "Value1", "Content-Type": "text/html"},
			Title:         "Example Page 1",
			WebServer:     "TestServer/1.0",
			FinalURL:      "http://example.com/page1",
			IPs:           []string{"192.168.1.1", "192.168.1.2"},
			CNAMEs:        []string{"cname.example.com", "cname2.example.com"},
			ASN:           12345,
			ASNOrg:        "Test ASN Org",
			Technologies:  []models.Technology{{Name: "TestTech", Version: "1.0"}, {Name: "AnotherTech"}},
			TLSVersion:    "TLSv1.2",
			TLSCipher:     "AES256-GCM-SHA384",
			TLSCertIssuer: "Test CA",
			TLSCertExpiry: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			RootTargetURL: "http://example.com",
		},
		{
			InputURL:      "https://example.org/another",
			Method:        "POST",
			Timestamp:     time.Date(2023, 1, 2, 10, 0, 0, 0, time.UTC),
			Duration:      1.2,
			Error:         "Connection timed out",
			StatusCode:    0,
			RootTargetURL: "https://example.org",
			// Optional fields will be nil or empty slices
			Headers:      map[string]string{}, // Ensure empty map, not nil for consistency if needed by transform
			IPs:          []string{},
			CNAMEs:       []string{},
			Technologies: []models.Technology{},
		},
		{
			InputURL:      "http://test.local/empty",
			Method:        "GET",
			Timestamp:     time.Date(2023, 1, 3, 8, 0, 0, 0, time.UTC),
			StatusCode:    404,
			ContentType:   "text/plain",
			RootTargetURL: "http://test.local",
			ContentLength: 0, // Explicitly 0
			Headers:       map[string]string{"X-Minimal": "true"},
		},
	}
}

func TestParquetWriter_WriteAndRead_ParquetGo(t *testing.T) {
	tempDir := t.TempDir()
	storageCfg := &config.StorageConfig{
		ParquetBasePath:  tempDir,
		CompressionCodec: "GZIP", // Test with GZIP
	}
	appLogger := log.New(os.Stdout, "[TestParquetWriterGoLib] ", log.LstdFlags)

	pw, err := NewParquetWriter(storageCfg, appLogger)
	if err != nil {
		t.Fatalf("NewParquetWriter() error = %v", err)
	}

	originalProbeResults := helperCreateTestProbeResults()
	scanSessionID := "test-session-parquet-go-gzip"
	rootTarget := "http://example.com/parquet-go-gzip"

	err = pw.Write(originalProbeResults, scanSessionID, rootTarget)
	if err != nil {
		t.Fatalf("ParquetWriter.Write() error = %v", err)
	}

	dateStr := time.Now().Format("20060102") // Date used in filePath generation
	datedPath := filepath.Join(tempDir, dateStr)
	safeRootTarget := strings.ReplaceAll(strings.ReplaceAll(rootTarget, "https://", ""), "http://", "")
	safeRootTarget = strings.ReplaceAll(safeRootTarget, ":", "_")
	safeRootTarget = strings.ReplaceAll(safeRootTarget, "/", "_")
	fileName := fmt.Sprintf("%s_%s.parquet", safeRootTarget, scanSessionID)
	filePath := filepath.Join(datedPath, fileName)

	if _, errStat := os.Stat(filePath); os.IsNotExist(errStat) {
		t.Fatalf("Parquet file was not created at %s", filePath)
	}

	// Read the file back
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open Parquet file for reading '%s': %v", filePath, err)
	}
	defer file.Close()

	reader := parquet.NewGenericReader[models.ParquetProbeResult](file)
	defer reader.Close() // Ensure reader is closed

	// Create a buffer to hold all expected results
	readProbeResultsBuffer := make([]models.ParquetProbeResult, len(originalProbeResults))

	totalRowsRead := 0
	// Loop to read all rows, as reader.Read might not fill the whole buffer in one go.
	for totalRowsRead < len(originalProbeResults) {
		rowsReadThisIteration, err := reader.Read(readProbeResultsBuffer[totalRowsRead:])
		if err != nil {
			// EOF is expected when all rows are read.
			// If rowsReadThisIteration > 0, it means some rows were read before EOF, which is fine.
			if err.Error() == "EOF" {
				totalRowsRead += rowsReadThisIteration // Add rows read in this final iteration
				break
			}
			t.Fatalf("reader.Read() error = %v", err) // Any other error is fatal
		}
		if rowsReadThisIteration == 0 {
			// This condition means Read returned 0 rows without an EOF or other error.
			// This implies there are no more rows to read, so we can break.
			// This might happen if the file has fewer rows than expected but ends cleanly.
			break
		}
		totalRowsRead += rowsReadThisIteration
	}

	// Slice the buffer to the actual number of rows read.
	readProbeResults := readProbeResultsBuffer[:totalRowsRead]

	if len(readProbeResults) != len(originalProbeResults) {
		t.Fatalf("Expected %d records, got %d", len(originalProbeResults), len(readProbeResults))
	}

	// Prepare expected results
	// For ScanTimestamp, we take the one from the first read record, as it's set by time.Now() during Write.
	var actualScanTime int64
	if len(readProbeResults) > 0 {
		actualScanTime = readProbeResults[0].ScanTimestamp
	} else if len(originalProbeResults) > 0 {
		// This case should not happen if read was successful and matched length
		t.Fatal("No records read, cannot determine actualScanTime for comparison, but expected records.")
	}

	expectedParquetResults := make([]models.ParquetProbeResult, len(originalProbeResults))
	for i, pr := range originalProbeResults {
		// Use the ParquetWriter's own transformation logic to create the expected structure
		// Use the actualScanTime obtained from the read data for consistent comparison
		expectedParquetResults[i] = pw.transformToParquetResult(pr, actualScanTime, rootTarget)
	}

	for i := 0; i < len(originalProbeResults); i++ {
		expected := expectedParquetResults[i]
		actual := readProbeResults[i]

		// Since ScanTimestamp is now aligned, we can use DeepEqual directly if other fields are consistent
		if !reflect.DeepEqual(expected, actual) {
			expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
			actualJSON, _ := json.MarshalIndent(actual, "", "  ")
			t.Errorf("Record %d mismatch:\nExpected:\n%s\nGot:\n%s", i, string(expectedJSON), string(actualJSON))
		}
	}
}

// No need for normalizeParquetProbeResultSlices with parquet-go/parquet-go
// as it handles nil/empty slices more directly if the struct tags are appropriate
// and our transformToParquetResult already ensures empty slices instead of nil.
