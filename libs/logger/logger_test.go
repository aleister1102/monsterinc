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

func TestNewWithContext(t *testing.T) {
	logDir := t.TempDir()
	fileConfig := FileLogConfig{
		LogFile:   filepath.Join(logDir, "context.log"),
		LogLevel:  "info",
		LogFormat: "text",
	}
	scanID := "context-scan"
	cycleID := "context-cycle"

	// Test with ScanID
	builderScan := NewLoggerBuilder().WithFileConfig(fileConfig).WithScanID(scanID).WithConsole(false)
	loggerScan, err := builderScan.Build()
	require.NoError(t, err)
	defer loggerScan.Close()

	loggerScan.GetZerolog().Info().Msg("test")
	expectedPathScan := filepath.Join(logDir, "scans", scanID, "context.log")
	_, err = os.Stat(expectedPathScan)
	assert.NoError(t, err, "log file should be created for scan context")

	// Test with CycleID
	builderCycle := NewLoggerBuilder().WithFileConfig(fileConfig).WithCycleID(cycleID).WithConsole(false)
	loggerCycle, err := builderCycle.Build()
	require.NoError(t, err)
	defer loggerCycle.Close()

	loggerCycle.GetZerolog().Info().Msg("test")
	expectedPathCycle := filepath.Join(logDir, "monitors", cycleID, "context.log")
	_, err = os.Stat(expectedPathCycle)
	assert.NoError(t, err, "log file should be created for monitor context")
}
