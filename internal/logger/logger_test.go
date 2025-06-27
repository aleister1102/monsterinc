package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerBuilder_Default(t *testing.T) {
	builder := NewLoggerBuilder()
	logger, err := builder.Build()
	require.NoError(t, err)
	require.NotNil(t, logger)

	config := logger.GetConfig()
	assert.Equal(t, zerolog.InfoLevel, config.Level)
	assert.Equal(t, FormatConsole, config.Format)
	assert.True(t, config.EnableConsole)
	assert.False(t, config.EnableFile)
}

func TestLoggerBuilder_FileLogging(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "test.log")

	builder := NewLoggerBuilder()
	logger, err := builder.
		WithLevel(zerolog.DebugLevel).
		WithFormat(FormatJSON).
		WithFile(logFile, 1, 1).
		WithConsole(false).
		Build()
	require.NoError(t, err)
	defer logger.Close()

	// Use the logger to log a message
	logger.GetZerolog().Debug().Msg("this is a test")

	// Verify file content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"level":"debug"`)
	assert.Contains(t, string(content), `"message":"this is a test"`)
}

func TestLoggerBuilder_WithScanID(t *testing.T) {
	logDir := t.TempDir()
	baseLogFile := "monsterinc.log"
	logFile := filepath.Join(logDir, baseLogFile)
	scanID := "test-scan-123"

	builder := NewLoggerBuilder()
	logger, err := builder.
		WithFile(logFile, 1, 1).
		WithScanID(scanID).
		WithConsole(false).
		Build()
	require.NoError(t, err)
	defer logger.Close()
	logger.GetZerolog().Info().Msg("testing scan id")

	expectedPath := filepath.Join(logDir, "scans", scanID, baseLogFile)
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "log file should be created in the correct scan directory")
}

func TestLoggerBuilder_WithCycleID(t *testing.T) {
	logDir := t.TempDir()
	baseLogFile := "monsterinc.log"
	logFile := filepath.Join(logDir, baseLogFile)
	cycleID := "test-cycle-456"

	builder := NewLoggerBuilder()
	logger, err := builder.
		WithFile(logFile, 1, 1).
		WithCycleID(cycleID).
		WithConsole(false).
		Build()
	require.NoError(t, err)
	defer logger.Close()
	logger.GetZerolog().Info().Msg("testing cycle id")

	expectedPath := filepath.Join(logDir, "monitors", cycleID, baseLogFile)
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "log file should be created in the correct monitor directory")
}

func TestConfigConverter(t *testing.T) {
	converter := NewConfigConverter()
	fileConfig := FileLogConfig{
		LogLevel:      "warn",
		LogFormat:     "json",
		LogFile:       "/tmp/test.log",
		MaxLogSizeMB:  50,
		MaxLogBackups: 5,
	}

	loggerConfig, err := converter.ConvertFileConfig(fileConfig)
	require.NoError(t, err)

	assert.Equal(t, zerolog.WarnLevel, loggerConfig.Level)
	assert.Equal(t, FormatJSON, loggerConfig.Format)
	assert.True(t, loggerConfig.EnableFile)
	assert.Equal(t, "/tmp/test.log", loggerConfig.FilePath)
	assert.Equal(t, 50, loggerConfig.MaxSizeMB)
	assert.Equal(t, 5, loggerConfig.MaxBackups)
}

func TestLogLevelParser(t *testing.T) {
	parser := NewLogLevelParser()
	level, err := parser.ParseLevel("debug")
	assert.NoError(t, err)
	assert.Equal(t, zerolog.DebugLevel, level)

	_, err = parser.ParseLevel("invalid-level")
	assert.Error(t, err)
}

func TestLogFormatParser(t *testing.T) {
	parser := NewLogFormatParser()
	assert.Equal(t, FormatJSON, parser.ParseFormat("json"))
	assert.Equal(t, FormatConsole, parser.ParseFormat("console"))
	assert.Equal(t, FormatText, parser.ParseFormat("text"))
	assert.Equal(t, FormatConsole, parser.ParseFormat("unknown-format")) // Fallback
}

func TestLoggerBuilder_Validation(t *testing.T) {
	// Test error when file logging is enabled but file path is empty
	_, err := NewLoggerBuilder().WithFile("", 1, 1).Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file path required")

	// Test error when max size is not positive
	_, err = NewLoggerBuilder().WithFile("test.log", 0, 1).Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max size must be positive")
}

func TestLogger_Reconfigure(t *testing.T) {
	logDir := t.TempDir()
	logFile1 := filepath.Join(logDir, "test1.log")
	logFile2 := filepath.Join(logDir, "test2.log")

	// Initial configuration
	builder := NewLoggerBuilder().
		WithFile(logFile1, 1, 1).
		WithConsole(false).
		WithLevel(zerolog.InfoLevel)
	logger, err := builder.Build()
	require.NoError(t, err)

	logger.GetZerolog().Info().Msg("initial message")

	// Reconfigure
	reconfig := FileLogConfig{
		LogLevel:  "debug",
		LogFormat: "json",
		LogFile:   logFile2,
	}
	err = logger.Reconfigure(reconfig)
	require.NoError(t, err)

	// Verify new configuration
	config := logger.GetConfig()
	assert.Equal(t, zerolog.DebugLevel, config.Level)
	assert.Equal(t, FormatJSON, config.Format)

	logger.GetZerolog().Debug().Msg("reconfigured message")
	logger.Close()

	// Check that initial log file exists and is not empty
	content1, err := os.ReadFile(logFile1)
	require.NoError(t, err)
	assert.NotEmpty(t, content1)

	// Check that reconfigured log file contains new message
	content2, err := os.ReadFile(logFile2)
	require.NoError(t, err)
	assert.Contains(t, string(content2), "reconfigured message")
	assert.Contains(t, string(content2), `"level":"debug"`)
}

func TestLogger_Close(t *testing.T) {
	logDir := t.TempDir()
	logFile := filepath.Join(logDir, "test_close.log")

	logger, err := NewLoggerBuilder().WithFile(logFile, 1, 1).Build()
	require.NoError(t, err)

	// First close should be successful
	err = logger.Close()
	assert.NoError(t, err)

	// Subsequent close should fail (or do nothing) as the file is already closed
	err = logger.Close()
	// The underlying lumberjack logger might not return an error on second close.
	// We just want to make sure it doesn't panic.
	// If it does return an error, we should check for it.
	// For now, we'll just assert no error to ensure no panics.
	assert.NoError(t, err, "subsequent close should not panic or return an error")
}
