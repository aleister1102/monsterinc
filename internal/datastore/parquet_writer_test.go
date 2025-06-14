package datastore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParquetWriter(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	cfg := &config.StorageConfig{
		ParquetBasePath: tempDir,
	}

	writer, err := NewParquetWriter(cfg, logger)

	require.NoError(t, err)
	assert.NotNil(t, writer)
	assert.Equal(t, tempDir, writer.config.ParquetBasePath)
}

func TestParquetWriter_Write_Success(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	cfg := &config.StorageConfig{
		ParquetBasePath: tempDir,
	}

	writer, err := NewParquetWriter(cfg, logger)
	require.NoError(t, err)

	probeResults := []models.ProbeResult{
		{
			InputURL:    "http://example.com",
			FinalURL:    "https://example.com",
			StatusCode:  200,
			ContentType: "text/html",
			Title:       "Example",
			WebServer:   "nginx",
			IPs:         []string{"1.2.3.4"},
			Technologies: []models.Technology{
				{Name: "nginx", Version: "1.18.0"},
			},
			Headers: map[string]string{
				"Content-Type": "text/html",
				"Server":       "nginx",
			},
		},
	}

	ctx := context.Background()
	err = writer.Write(ctx, probeResults, "test-session-123", "example.com")

	require.NoError(t, err)

	// Verify file was created
	expectedFile := filepath.Join(tempDir, "scan", "example.com.parquet")
	assert.FileExists(t, expectedFile)
}

func TestParquetWriter_Write_EmptyResults(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	cfg := &config.StorageConfig{
		ParquetBasePath: tempDir,
	}

	writer, err := NewParquetWriter(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()
	err = writer.Write(ctx, []models.ProbeResult{}, "test-session", "example.com")

	require.NoError(t, err)

	// File should still be created even with empty results
	expectedFile := filepath.Join(tempDir, "scan", "example.com.parquet")
	assert.FileExists(t, expectedFile)
}

func TestParquetWriter_ValidateWriteRequest(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name        string
		config      *config.StorageConfig
		request     WriteRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid request",
			config: &config.StorageConfig{
				ParquetBasePath: "/tmp/test",
			},
			request: WriteRequest{
				RootTarget: "example.com",
			},
			expectError: false,
		},
		{
			name: "empty base path",
			config: &config.StorageConfig{
				ParquetBasePath: "",
			},
			request: WriteRequest{
				RootTarget: "example.com",
			},
			expectError: true,
			errorMsg:    "ParquetBasePath is not configured",
		},
		{
			name: "invalid hostname",
			config: &config.StorageConfig{
				ParquetBasePath: "/tmp/test",
			},
			request: WriteRequest{
				RootTarget: "///invalid///",
			},
			expectError: true,
			errorMsg:    "sanitized hostname is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &ParquetWriter{
				config: tt.config,
				logger: logger,
			}

			err := writer.validateWriteRequest(tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParquetWriter_CheckCancellation(t *testing.T) {
	logger := zerolog.Nop()
	writer := &ParquetWriter{logger: logger}

	t.Run("normal context", func(t *testing.T) {
		ctx := context.Background()
		err := writer.checkCancellation(ctx, "test operation")
		assert.NoError(t, err)
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := writer.checkCancellation(ctx, "test operation")
		assert.Error(t, err)
	})
}

func TestParquetWriter_PrepareOutputFile(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	writer := &ParquetWriter{
		config: &config.StorageConfig{
			ParquetBasePath: tempDir,
		},
		logger: logger,
	}

	filePath, err := writer.prepareOutputFile("example.com")

	require.NoError(t, err)
	expectedPath := filepath.Join(tempDir, "scan", "example.com.parquet")
	assert.Equal(t, expectedPath, filePath)

	// Verify directory was created
	scanDir := filepath.Join(tempDir, "scan")
	assert.DirExists(t, scanDir)
}

func TestParquetWriter_PrepareOutputFile_SpecialCharacters(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	writer := &ParquetWriter{
		config: &config.StorageConfig{
			ParquetBasePath: tempDir,
		},
		logger: logger,
	}

	// Test hostname with special characters that need sanitization
	filePath, err := writer.prepareOutputFile("example.com/path?query=value")

	require.NoError(t, err)
	// Should be sanitized to remove special characters
	assert.Contains(t, filePath, ".parquet")
	assert.NotContains(t, filepath.Base(filePath), "/")
	assert.NotContains(t, filepath.Base(filePath), "?")
}

func TestParquetWriter_TransformRecords(t *testing.T) {
	logger := zerolog.Nop()
	writer := &ParquetWriter{logger: logger}

	probeResults := []models.ProbeResult{
		{
			InputURL:    "http://example.com",
			FinalURL:    "https://example.com",
			StatusCode:  200,
			ContentType: "text/html",
			Title:       "Example",
			WebServer:   "nginx",
			IPs:         []string{"1.2.3.4"},
			Technologies: []models.Technology{
				{Name: "nginx", Version: "1.18.0"},
				{Name: "html", Version: ""},
			},
			Headers: map[string]string{
				"Content-Type": "text/html",
				"Server":       "nginx",
			},
		},
	}

	request := WriteRequest{
		ProbeResults:  probeResults,
		ScanSessionID: "test-session",
		ScanTime:      time.Now(),
	}

	ctx := context.Background()
	results, err := writer.transformRecords(ctx, request)

	require.NoError(t, err)
	assert.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, "http://example.com", result.OriginalURL)
	assert.Equal(t, "https://example.com", *result.FinalURL)
	assert.Equal(t, int32(200), *result.StatusCode)
	assert.Equal(t, "text/html", *result.ContentType)
	assert.Equal(t, "Example", *result.Title)
	assert.Equal(t, "nginx", *result.WebServer)
	assert.Equal(t, []string{"1.2.3.4"}, result.IPAddress)
	assert.Equal(t, []string{"nginx", "html"}, result.Technologies)
	assert.Equal(t, "test-session", *result.ScanSessionID)
	assert.NotNil(t, result.HeadersJSON)
}

func TestParquetWriter_TransformRecords_ContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	writer := &ParquetWriter{logger: logger}

	probeResults := []models.ProbeResult{
		{InputURL: "http://example.com"},
	}

	request := WriteRequest{
		ProbeResults: probeResults,
		ScanTime:     time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := writer.transformRecords(ctx, request)

	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestParquetWriter_WriteToParquetFile(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	writer := &ParquetWriter{
		logger: logger,
		writerConfig: ParquetWriterConfig{
			CompressionType: "snappy",
		},
	}

	parquetResults := []models.ParquetProbeResult{
		{
			OriginalURL:   "http://example.com",
			FinalURL:      StringPtrOrNil("https://example.com"),
			StatusCode:    Int32PtrOrNilZero(200),
			ContentType:   StringPtrOrNil("text/html"),
			Title:         StringPtrOrNil("Example"),
			WebServer:     StringPtrOrNil("nginx"),
			Technologies:  []string{"nginx"},
			IPAddress:     []string{"1.2.3.4"},
			ScanSessionID: StringPtrOrNil("test-session"),
			ScanTimestamp: time.Now().UnixMilli(),
		},
	}

	filePath := filepath.Join(tempDir, "test.parquet")
	recordsWritten, err := writer.writeToParquetFile(filePath, parquetResults)

	require.NoError(t, err)
	assert.Equal(t, 1, recordsWritten)
	assert.FileExists(t, filePath)

	// Verify file has content
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestParquetWriter_WriteResult(t *testing.T) {
	result := &WriteResult{
		FilePath:       "/tmp/test.parquet",
		RecordsWritten: 100,
		FileSize:       1024,
		WriteTime:      time.Second,
	}

	assert.Equal(t, "/tmp/test.parquet", result.FilePath)
	assert.Equal(t, 100, result.RecordsWritten)
	assert.Equal(t, int64(1024), result.FileSize)
	assert.Equal(t, time.Second, result.WriteTime)
}

func TestParquetWriter_UtilityFunctions(t *testing.T) {
	t.Run("StringPtrOrNil", func(t *testing.T) {
		tests := []struct {
			input    string
			expected *string
		}{
			{"test", func() *string { s := "test"; return &s }()},
			{"", nil},
		}

		for _, tt := range tests {
			result := StringPtrOrNil(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		}
	})

	t.Run("Int32PtrOrNilZero", func(t *testing.T) {
		tests := []struct {
			input    int32
			expected *int32
		}{
			{100, func() *int32 { i := int32(100); return &i }()},
			{0, nil},
		}

		for _, tt := range tests {
			result := Int32PtrOrNilZero(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		}
	})

	t.Run("Int64PtrOrNilZero", func(t *testing.T) {
		tests := []struct {
			input    int64
			expected *int64
		}{
			{200, func() *int64 { i := int64(200); return &i }()},
			{0, nil},
		}

		for _, tt := range tests {
			result := Int64PtrOrNilZero(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		}
	})
}

func TestParquetWriter_GetCompressionOption(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name            string
		compressionType string
	}{
		{"snappy", "snappy"},
		{"gzip", "gzip"},
		{"none", "none"},
		{"unknown", "snappy"}, // should default to snappy
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &ParquetWriter{
				logger: logger,
				writerConfig: ParquetWriterConfig{
					CompressionType: tt.compressionType,
				},
			}

			option := writer.getCompressionOption()
			assert.NotNil(t, option)
		})
	}
}

func TestParquetWriter_WriteProbeResults_FullFlow(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	cfg := &config.StorageConfig{
		ParquetBasePath: tempDir,
	}

	writer, err := NewParquetWriter(cfg, logger)
	require.NoError(t, err)

	request := WriteRequest{
		ProbeResults: []models.ProbeResult{
			{
				InputURL:     "http://example.com",
				FinalURL:     "https://example.com",
				StatusCode:   200,
				ContentType:  "text/html",
				Title:        "Example Domain",
				WebServer:    "nginx/1.18.0",
				IPs:          []string{"93.184.216.34"},
				Technologies: []models.Technology{{Name: "nginx"}},
				Headers:      map[string]string{"Server": "nginx"},
			},
		},
		ScanSessionID: "test-session-456",
		RootTarget:    "example.com",
		ScanTime:      time.Now(),
	}

	ctx := context.Background()
	result, err := writer.writeProbeResults(ctx, request)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.RecordsWritten)
	assert.Greater(t, result.FileSize, int64(0))
	assert.Greater(t, result.WriteTime, time.Duration(0))
	assert.Contains(t, result.FilePath, "example.com.parquet")
	assert.FileExists(t, result.FilePath)
}

func TestParquetWriter_CreateParquetFile_InvalidPath(t *testing.T) {
	logger := zerolog.Nop()
	writer := &ParquetWriter{logger: logger}

	// Try to create file in non-existent directory without permission
	invalidPath := "/nonexistent/path/test.parquet"
	file, err := writer.createParquetFile(invalidPath)

	assert.Error(t, err)
	assert.Nil(t, file)
}

func TestParquetWriter_WriteRecords_EmptySlice(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()

	writer := &ParquetWriter{
		logger: logger,
		writerConfig: ParquetWriterConfig{
			CompressionType: "snappy",
		},
	}

	filePath := filepath.Join(tempDir, "empty.parquet")
	recordsWritten, err := writer.writeToParquetFile(filePath, []models.ParquetProbeResult{})

	require.NoError(t, err)
	assert.Equal(t, 0, recordsWritten)
	assert.FileExists(t, filePath)
}
