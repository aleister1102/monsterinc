package logger

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerBuilder_Build(t *testing.T) {
	t.Run("build with console", func(t *testing.T) {
		builder := NewLoggerBuilder().WithConsole(true).WithFile("", 0, 0)
		logger, err := builder.Build()
		require.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.GetConfig().EnableConsole)
		assert.False(t, logger.GetConfig().EnableFile)
	})

	t.Run("build with file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")
		builder := NewLoggerBuilder().WithConsole(false).WithFile(logFile, 10, 1)
		logger, err := builder.Build()
		require.NoError(t, err)
		assert.NotNil(t, logger)
		assert.False(t, logger.GetConfig().EnableConsole)
		assert.True(t, logger.GetConfig().EnableFile)
		assert.Equal(t, logFile, logger.GetConfig().FilePath)
	})

	t.Run("build with file and subdirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")
		builder := NewLoggerBuilder().
			WithConsole(false).
			WithFile(logFile, 10, 1).
			WithScanID("test-scan").
			WithSubdirs(true)

		logger, err := builder.Build()
		require.NoError(t, err)
		expectedPath := filepath.Join(tmpDir, "scans", "test-scan", "test.log")
		// This is a bit indirect, but we check if the file gets created where we expect
		logger.GetZerolog().Info().Msg("test")
		_, err = os.Stat(expectedPath)
		assert.NoError(t, err, "log file should be created in subdirectory")
	})

	t.Run("build no writers", func(t *testing.T) {
		builder := NewLoggerBuilder().WithConsole(false)
		_, err := builder.Build()
		assert.Error(t, err)
	})

	t.Run("validation error - no file path", func(t *testing.T) {
		builder := NewLoggerBuilder().WithConsole(false).WithFile("", 10, 1)
		// This combination doesn't make sense as WithFile(path) would set EnableFile to true.
		// The builder enables file logging if path is set.
		// Let's adjust the config directly to create the invalid state.
		builder.config.EnableFile = true
		builder.config.FilePath = ""
		_, err := builder.Build()
		assert.Error(t, err)
	})
}

func TestLogger_Reconfigure(t *testing.T) {
	// 1. Initial logger
	builder := NewLoggerBuilder().WithConsole(true).WithLevel(zerolog.InfoLevel)
	logger, err := builder.Build()
	require.NoError(t, err)
	assert.Equal(t, zerolog.InfoLevel, logger.GetConfig().Level)

	// Capture output to test
	var buf bytes.Buffer
	logger.zerolog = logger.zerolog.Output(&buf)

	// Log with info level, should be visible
	logger.GetZerolog().Info().Msg("info message")
	assert.Contains(t, buf.String(), "info message")
	buf.Reset()

	// Log with debug level, should be hidden
	logger.GetZerolog().Debug().Msg("debug message")
	assert.NotContains(t, buf.String(), "debug message")
	buf.Reset()

	// 2. Reconfigure to a different level
	newCfg := FileLogConfig{
		LogLevel: "debug", // Change level
	}
	err = logger.Reconfigure(newCfg)
	require.NoError(t, err)
	assert.Equal(t, zerolog.DebugLevel, logger.GetConfig().Level)

	// Need to re-set the output on the new zerolog instance inside the logger
	logger.zerolog = logger.zerolog.Output(&buf)

	// Log with debug level, should now be visible
	logger.GetZerolog().Debug().Msg("debug message after reconfig")
	assert.Contains(t, buf.String(), "debug message after reconfig")
}

func TestNew_Functions(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("New", func(t *testing.T) {
		cfg := NewDefaultFileLogConfig()
		cfg.LogFile = filepath.Join(tmpDir, "new.log")
		log, err := New(cfg)
		require.NoError(t, err)
		log.Info().Msg("test from New")
		content, _ := ioutil.ReadFile(cfg.LogFile)
		assert.Contains(t, string(content), "test from New")
	})

	t.Run("NewWithScanID", func(t *testing.T) {
		cfg := NewDefaultFileLogConfig()
		cfg.LogFile = filepath.Join(tmpDir, "scan.log")
		log, err := NewWithScanID(cfg, "scan123")
		require.NoError(t, err)
		log.Info().Msg("test scan")

		expectedPath := filepath.Join(tmpDir, "scans", "scan123", "scan.log")
		content, err := ioutil.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test scan")
	})

	t.Run("NewWithCycleID", func(t *testing.T) {
		cfg := NewDefaultFileLogConfig()
		cfg.LogFile = filepath.Join(tmpDir, "cycle.log")
		log, err := NewWithCycleID(cfg, "cycle456")
		require.NoError(t, err)
		log.Info().Msg("test cycle")

		expectedPath := filepath.Join(tmpDir, "monitors", "cycle456", "cycle.log")
		content, err := ioutil.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test cycle")
	})

	t.Run("NewWithContext", func(t *testing.T) {
		cfg := NewDefaultFileLogConfig()
		cfg.LogFile = filepath.Join(tmpDir, "context.log")
		log, err := NewWithContext(cfg, "scan-ctx", "")
		require.NoError(t, err)
		log.Info().Msg("test context scan")

		expectedPath := filepath.Join(tmpDir, "scans", "scan-ctx", "context.log")
		content, err := ioutil.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "test context scan")
	})
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	builder := NewLoggerBuilder().
		WithConsole(true).
		WithFormat(FormatJSON).
		WithLevel(zerolog.InfoLevel)

	// We can't build and then set output easily because the writer is already set.
	// So we create the writer manually for the test.
	builder.config.EnableConsole = true // ensure console writer is created
	consoleWriter := builder.factory.CreateConsoleWriter(FormatJSON)
	logger := zerolog.New(consoleWriter).Output(&buf)

	logger.Info().Str("key", "value").Msg("json test")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err, "log output should be valid json")
	assert.Equal(t, "value", logEntry["key"])
	assert.Equal(t, "json test", logEntry["message"])
	assert.Equal(t, "info", logEntry["level"])
}

func TestLoggerBuilder_WithFileConfig(t *testing.T) {
	cfg := FileLogConfig{
		LogLevel:  "warn",
		LogFormat: "json",
	}
	builder := NewLoggerBuilder().WithFileConfig(cfg)
	builtConfig := builder.config

	assert.Equal(t, zerolog.WarnLevel, builtConfig.Level)
	assert.Equal(t, FormatJSON, builtConfig.Format)
}
