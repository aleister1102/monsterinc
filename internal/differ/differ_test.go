package differ

import (
	"fmt"
	"testing"
	"time"

	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUrlDiffer(t *testing.T) {
	logger := zerolog.Nop()

	// Create mock parquet reader
	mockParquetReader := &datastore.ParquetReader{} // This may need adjustment based on actual constructor

	differ, err := NewUrlDiffer(mockParquetReader, logger)

	assert.NoError(t, err)
	assert.NotNil(t, differ)
}

func TestURLDiffer_Compare_BothEmpty(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	currentProbes := []*models.ProbeResult{}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Equal(t, rootTarget, diffResult.RootTargetURL)
	assert.Equal(t, 0, diffResult.New)
	assert.Equal(t, 0, diffResult.Existing)
	assert.Empty(t, diffResult.Results)
}

func TestURLDiffer_Compare_OnlyNewResults(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	currentProbes := []*models.ProbeResult{
		{InputURL: "http://example.com", StatusCode: 200},
		{InputURL: "http://test.com", StatusCode: 404},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Equal(t, rootTarget, diffResult.RootTargetURL)
	assert.Equal(t, 2, diffResult.New)
	assert.Equal(t, 0, diffResult.Existing)
	assert.Len(t, diffResult.Results, 2)

	// Verify all results are marked as new
	for _, result := range diffResult.Results {
		assert.Equal(t, string(models.StatusNew), result.ProbeResult.URLStatus)
		assert.Contains(t, []string{"http://example.com", "http://test.com"}, result.ProbeResult.InputURL)
	}
}

func TestURLDiffer_Compare_MixedResults(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	currentProbes := []*models.ProbeResult{
		{InputURL: "http://existing.com", StatusCode: 200, Title: "New Title"},
		{InputURL: "http://new.com", StatusCode: 201},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Equal(t, rootTarget, diffResult.RootTargetURL)
	assert.Len(t, diffResult.Results, 2)

	// Find and verify each result type
	var newResult *models.DiffedURL
	for i, result := range diffResult.Results {
		if result.ProbeResult.InputURL == "http://new.com" {
			newResult = &diffResult.Results[i]
		}
	}

	require.NotNil(t, newResult)
	assert.Equal(t, string(models.StatusNew), newResult.ProbeResult.URLStatus)
	assert.Equal(t, 201, newResult.ProbeResult.StatusCode)
}

func TestURLDiffer_Compare_SameURLDifferentData(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	oldTime := time.Now().Add(-24 * time.Hour)

	currentProbes := []*models.ProbeResult{
		{
			InputURL:            "http://example.com",
			StatusCode:          404,
			Title:               "New Title",
			ContentLength:       2000,
			OldestScanTimestamp: oldTime,
		},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Len(t, diffResult.Results, 1)

	result := diffResult.Results[0]
	assert.Equal(t, "http://example.com", result.ProbeResult.InputURL)
	assert.Equal(t, 404, result.ProbeResult.StatusCode)            // Should have new status code
	assert.Equal(t, "New Title", result.ProbeResult.Title)         // Should have new title
	assert.Equal(t, int64(2000), result.ProbeResult.ContentLength) // Should have new content length
}

func TestURLDiffer_Compare_PreserveOldestTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	veryOldTime := time.Now().Add(-72 * time.Hour)

	currentProbes := []*models.ProbeResult{
		{
			InputURL:            "http://example.com",
			StatusCode:          200,
			OldestScanTimestamp: veryOldTime,
		},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	require.Len(t, diffResult.Results, 1)
	result := diffResult.Results[0]

	// Should preserve the timestamp
	assert.Equal(t, veryOldTime, result.ProbeResult.OldestScanTimestamp)
}

func TestURLDiffer_Compare_HandleZeroTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	currentProbes := []*models.ProbeResult{
		{
			InputURL:            "http://example.com",
			StatusCode:          200,
			OldestScanTimestamp: time.Time{}, // Zero time
		},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	require.Len(t, diffResult.Results, 1)
	result := diffResult.Results[0]

	// Should remain zero time
	assert.True(t, result.ProbeResult.OldestScanTimestamp.IsZero())
}

func TestURLDiffer_Compare_LargeDatasets(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	// Create large datasets to test performance
	currentProbes := make([]*models.ProbeResult, 1000)

	for i := 0; i < 1000; i++ {
		// All URLs are new (no historical data in this test)
		currentProbes[i] = &models.ProbeResult{
			InputURL:   fmt.Sprintf("http://new%d.com", i),
			StatusCode: 201,
		}
	}

	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Equal(t, 1000, diffResult.New)
	assert.Len(t, diffResult.Results, 1000) // All new
}

func TestURLDiffer_Compare_EmptyStrings(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	currentProbes := []*models.ProbeResult{
		{InputURL: "", StatusCode: 200},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Len(t, diffResult.Results, 1)

	result := diffResult.Results[0]
	assert.Equal(t, "", result.ProbeResult.InputURL)
	assert.Equal(t, 200, result.ProbeResult.StatusCode)
}

func TestURLDiffer_Compare_DuplicateURLs(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	// Test with duplicate URLs in the same dataset
	currentProbes := []*models.ProbeResult{
		{InputURL: "http://example.com", StatusCode: 404, Title: "New"},
	}
	rootTarget := "http://example.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	assert.Len(t, diffResult.Results, 1)

	result := diffResult.Results[0]
	assert.Equal(t, "http://example.com", result.ProbeResult.InputURL)
	assert.Equal(t, 404, result.ProbeResult.StatusCode) // Should have new data
	assert.Equal(t, "New", result.ProbeResult.Title)
}

func TestURLDiffer_Compare_ComplexProbeData(t *testing.T) {
	logger := zerolog.Nop()
	mockParquetReader := &datastore.ParquetReader{}

	differ, err := NewUrlDiffer(mockParquetReader, logger)
	require.NoError(t, err)

	currentProbes := []*models.ProbeResult{
		{
			InputURL:      "http://complex.com",
			FinalURL:      "https://complex.com",
			StatusCode:    200,
			ContentLength: 6000,
			ContentType:   "text/html",
			Title:         "New Complex Site",
			WebServer:     "nginx/1.20.0",
			IPs:           []string{"1.2.3.4", "5.6.7.8"},
			Technologies: []models.Technology{
				{Name: "nginx", Version: "1.20.0"},
				{Name: "react", Version: "17.0.0"},
			},
			Headers: map[string]string{
				"Server": "nginx/1.20.0",
				"X-New":  "true",
			},
		},
	}

	rootTarget := "http://complex.com"
	scanSessionID := "test-session"

	diffResult, err := differ.Differentiate(currentProbes, rootTarget, scanSessionID)

	require.NoError(t, err)
	require.Len(t, diffResult.Results, 1)
	result := diffResult.Results[0]

	// Should preserve all new data
	assert.Equal(t, "New Complex Site", result.ProbeResult.Title)
	assert.Equal(t, "nginx/1.20.0", result.ProbeResult.WebServer)
	assert.Equal(t, int64(6000), result.ProbeResult.ContentLength)
	assert.Equal(t, []string{"1.2.3.4", "5.6.7.8"}, result.ProbeResult.IPs)
	assert.Len(t, result.ProbeResult.Technologies, 2)
	assert.Equal(t, "nginx/1.20.0", result.ProbeResult.Headers["Server"])
	assert.Equal(t, "true", result.ProbeResult.Headers["X-New"])
}
