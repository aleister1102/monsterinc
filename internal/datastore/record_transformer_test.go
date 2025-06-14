package datastore

import (
	"testing"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecordTransformer(t *testing.T) {
	logger := zerolog.Nop()

	transformer := NewRecordTransformer(logger)

	assert.NotNil(t, transformer)
	assert.NotNil(t, transformer.logger)
}

func TestRecordTransformer_TransformToParquetResult(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	scanSessionID := "test-session-123"

	probeResult := models.ProbeResult{
		InputURL:      "http://example.com",
		FinalURL:      "https://example.com",
		StatusCode:    200,
		ContentLength: 1024,
		ContentType:   "text/html",
		Title:         "Example Domain",
		WebServer:     "nginx/1.18.0",
		IPs:           []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"},
		RootTargetURL: "https://example.com",
		Error:         "",
		Method:        "GET",
		URLStatus:     "new",
		Technologies: []models.Technology{
			{Name: "nginx", Version: "1.18.0"},
			{Name: "html", Version: ""},
		},
		Headers: map[string]string{
			"Content-Type":    "text/html; charset=UTF-8",
			"Server":          "nginx/1.18.0",
			"X-Frame-Options": "DENY",
		},
		OldestScanTimestamp: time.Time{}, // Zero time
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	// Verify basic fields
	assert.Equal(t, "http://example.com", result.OriginalURL)
	assert.Equal(t, "https://example.com", *result.FinalURL)
	assert.Equal(t, int32(200), *result.StatusCode)
	assert.Equal(t, int64(1024), *result.ContentLength)
	assert.Equal(t, "text/html", *result.ContentType)
	assert.Equal(t, "Example Domain", *result.Title)
	assert.Equal(t, "nginx/1.18.0", *result.WebServer)
	assert.Equal(t, []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}, result.IPAddress)
	assert.Equal(t, "https://example.com", *result.RootTargetURL)
	assert.Equal(t, "GET", *result.Method)
	assert.Equal(t, "new", *result.DiffStatus)
	assert.Equal(t, "test-session-123", *result.ScanSessionID)

	// Verify technology extraction
	assert.Equal(t, []string{"nginx", "html"}, result.Technologies)

	// Verify timestamps
	assert.Equal(t, scanTime.UnixMilli(), result.ScanTimestamp)
	assert.Equal(t, scanTime.UnixMilli(), *result.FirstSeenTimestamp) // Should use scanTime when OldestScanTimestamp is zero
	assert.Equal(t, scanTime.UnixMilli(), *result.LastSeenTimestamp)

	// Verify headers JSON
	assert.NotNil(t, result.HeadersJSON)
	assert.Contains(t, *result.HeadersJSON, "Content-Type")
	assert.Contains(t, *result.HeadersJSON, "Server")
	assert.Contains(t, *result.HeadersJSON, "X-Frame-Options")

	// Verify ProbeError is nil for successful probe
	assert.Nil(t, result.ProbeError)
}

func TestRecordTransformer_TransformToParquetResult_WithError(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	scanSessionID := "test-session-456"

	probeResult := models.ProbeResult{
		InputURL:     "http://broken.example.com",
		FinalURL:     "",
		StatusCode:   0,
		ContentType:  "",
		Title:        "",
		WebServer:    "",
		IPs:          []string{},
		Error:        "connection timeout",
		Technologies: []models.Technology{},
		Headers:      map[string]string{},
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	assert.Equal(t, "http://broken.example.com", result.OriginalURL)
	assert.Nil(t, result.FinalURL)
	assert.Nil(t, result.StatusCode)
	assert.Nil(t, result.ContentType)
	assert.Nil(t, result.Title)
	assert.Nil(t, result.WebServer)
	assert.Equal(t, []string{}, result.IPAddress)
	assert.Equal(t, "connection timeout", *result.ProbeError)
	assert.Equal(t, []string{}, result.Technologies)
	assert.Nil(t, result.HeadersJSON)
}

func TestRecordTransformer_TransformToParquetResult_WithOldestTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	oldestTime := scanTime.Add(-24 * time.Hour) // 24 hours ago
	scanSessionID := "test-session-789"

	probeResult := models.ProbeResult{
		InputURL:            "http://example.com",
		FinalURL:            "https://example.com",
		StatusCode:          200,
		OldestScanTimestamp: oldestTime,
		Technologies:        []models.Technology{},
		Headers:             map[string]string{},
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	// First seen should be the oldest timestamp
	assert.Equal(t, oldestTime.UnixMilli(), *result.FirstSeenTimestamp)
	// Last seen should be the current scan time
	assert.Equal(t, scanTime.UnixMilli(), *result.LastSeenTimestamp)
}

func TestRecordTransformer_MarshalHeaders(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	tests := []struct {
		name       string
		headers    map[string]string
		inputURL   string
		expectNil  bool
		expectJson bool
	}{
		{
			name:       "empty headers",
			headers:    map[string]string{},
			inputURL:   "http://example.com",
			expectNil:  true,
			expectJson: false,
		},
		{
			name: "valid headers",
			headers: map[string]string{
				"Content-Type": "text/html",
				"Server":       "nginx",
			},
			inputURL:   "http://example.com",
			expectNil:  false,
			expectJson: true,
		},
		{
			name: "single header",
			headers: map[string]string{
				"Location": "https://example.com",
			},
			inputURL:   "http://example.com",
			expectNil:  false,
			expectJson: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.marshalHeaders(tt.headers, tt.inputURL)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				if tt.expectJson {
					assert.Contains(t, *result, "{")
					assert.Contains(t, *result, "}")
					// Verify it's valid JSON structure
					assert.True(t, len(*result) > 2) // More than just "{}"
				}
			}
		})
	}
}

func TestRecordTransformer_ExtractTechnologyNames(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	tests := []struct {
		name         string
		technologies []models.Technology
		expected     []string
	}{
		{
			name:         "empty technologies",
			technologies: []models.Technology{},
			expected:     []string{},
		},
		{
			name: "single technology",
			technologies: []models.Technology{
				{Name: "nginx", Version: "1.18.0"},
			},
			expected: []string{"nginx"},
		},
		{
			name: "multiple technologies",
			technologies: []models.Technology{
				{Name: "nginx", Version: "1.18.0"},
				{Name: "html", Version: ""},
				{Name: "javascript", Version: "ES6"},
			},
			expected: []string{"nginx", "html", "javascript"},
		},
		{
			name: "technologies with empty names",
			technologies: []models.Technology{
				{Name: "nginx", Version: "1.18.0"},
				{Name: "", Version: "1.0"},
				{Name: "html", Version: ""},
			},
			expected: []string{"nginx", "", "html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.extractTechnologyNames(tt.technologies)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecordTransformer_DetermineFirstSeenTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	oldestTime := scanTime.Add(-2 * time.Hour)

	tests := []struct {
		name                string
		oldestScanTimestamp time.Time
		scanTime            time.Time
		expected            time.Time
	}{
		{
			name:                "zero oldest timestamp",
			oldestScanTimestamp: time.Time{},
			scanTime:            scanTime,
			expected:            scanTime,
		},
		{
			name:                "valid oldest timestamp",
			oldestScanTimestamp: oldestTime,
			scanTime:            scanTime,
			expected:            oldestTime,
		},
		{
			name:                "oldest timestamp same as scan time",
			oldestScanTimestamp: scanTime,
			scanTime:            scanTime,
			expected:            scanTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.determineFirstSeenTimestamp(tt.oldestScanTimestamp, tt.scanTime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecordTransformer_TransformToParquetResult_AllNilValues(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	scanSessionID := "test-session-nil"

	// ProbeResult with all zero/empty values
	probeResult := models.ProbeResult{
		InputURL:      "http://example.com", // This is required
		FinalURL:      "",
		StatusCode:    0,
		ContentLength: 0,
		ContentType:   "",
		Title:         "",
		WebServer:     "",
		IPs:           []string{},
		RootTargetURL: "",
		Error:         "",
		Method:        "",
		URLStatus:     "",
		Technologies:  []models.Technology{},
		Headers:       map[string]string{},
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	// Only OriginalURL and timestamps should be non-nil
	assert.Equal(t, "http://example.com", result.OriginalURL)
	assert.Nil(t, result.FinalURL)
	assert.Nil(t, result.StatusCode)
	assert.Nil(t, result.ContentLength)
	assert.Nil(t, result.ContentType)
	assert.Nil(t, result.Title)
	assert.Nil(t, result.WebServer)
	assert.Equal(t, []string{}, result.IPAddress)
	assert.Nil(t, result.RootTargetURL)
	assert.Nil(t, result.ProbeError)
	assert.Nil(t, result.Method)
	assert.Nil(t, result.DiffStatus)
	assert.Equal(t, "test-session-nil", *result.ScanSessionID)
	assert.Equal(t, []string{}, result.Technologies)
	assert.Nil(t, result.HeadersJSON)

	// Timestamps should still be set
	assert.Equal(t, scanTime.UnixMilli(), result.ScanTimestamp)
	assert.Equal(t, scanTime.UnixMilli(), *result.FirstSeenTimestamp)
	assert.Equal(t, scanTime.UnixMilli(), *result.LastSeenTimestamp)
}

func TestRecordTransformer_TransformToParquetResult_ComplexHeaders(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	scanSessionID := "test-session-complex"

	probeResult := models.ProbeResult{
		InputURL: "http://example.com",
		Headers: map[string]string{
			"Content-Type":              "text/html; charset=UTF-8",
			"Content-Security-Policy":   "default-src 'self'; script-src 'self' 'unsafe-inline'",
			"X-Frame-Options":           "SAMEORIGIN",
			"X-Content-Type-Options":    "nosniff",
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		},
		Technologies: []models.Technology{},
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	require.NotNil(t, result.HeadersJSON)
	headerJSON := *result.HeadersJSON

	// Verify all headers are present in JSON
	assert.Contains(t, headerJSON, "Content-Type")
	assert.Contains(t, headerJSON, "Content-Security-Policy")
	assert.Contains(t, headerJSON, "X-Frame-Options")
	assert.Contains(t, headerJSON, "X-Content-Type-Options")
	assert.Contains(t, headerJSON, "Strict-Transport-Security")

	// Verify special characters are handled correctly
	assert.Contains(t, headerJSON, "charset=UTF-8")
	assert.Contains(t, headerJSON, "includeSubDomains")
}

func TestRecordTransformer_TransformToParquetResult_EdgeCases(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	scanSessionID := "test-session-edge"

	// Test with very large values
	probeResult := models.ProbeResult{
		InputURL:      "http://example.com",
		StatusCode:    999,
		ContentLength: 9223372036854775807, // Max int64
		Technologies: []models.Technology{
			{Name: "very-long-technology-name-that-exceeds-normal-length", Version: "1.0.0"},
		},
		Headers: map[string]string{
			"X-Very-Long-Header-Name": "Very long header value that might cause issues with JSON marshaling or storage",
		},
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	assert.Equal(t, int32(999), *result.StatusCode)
	assert.Equal(t, int64(9223372036854775807), *result.ContentLength)
	assert.Equal(t, []string{"very-long-technology-name-that-exceeds-normal-length"}, result.Technologies)
	assert.NotNil(t, result.HeadersJSON)
	assert.Contains(t, *result.HeadersJSON, "X-Very-Long-Header-Name")
}

func TestRecordTransformer_TransformToParquetResult_SpecialCharactersInHeaders(t *testing.T) {
	logger := zerolog.Nop()
	transformer := NewRecordTransformer(logger)

	scanTime := time.Now()
	scanSessionID := "test-session-special"

	probeResult := models.ProbeResult{
		InputURL: "http://example.com",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Custom":     "value with \"quotes\" and \\backslashes\\",
			"X-Unicode":    "测试中文字符 and émojis 🚀",
			"X-Newlines":   "line1\nline2\r\nline3",
		},
		Technologies: []models.Technology{},
	}

	result := transformer.TransformToParquetResult(probeResult, scanTime, scanSessionID)

	require.NotNil(t, result.HeadersJSON)

	// JSON should be properly escaped
	headerJSON := *result.HeadersJSON
	assert.Contains(t, headerJSON, "application/json")
	assert.Contains(t, headerJSON, "quotes")
	assert.Contains(t, headerJSON, "backslashes")
	assert.Contains(t, headerJSON, "测试中文字符")
	assert.Contains(t, headerJSON, "🚀")

	// Verify it's valid JSON by checking it starts and ends with braces
	assert.True(t, headerJSON[0] == '{')
	assert.True(t, headerJSON[len(headerJSON)-1] == '}')
}
