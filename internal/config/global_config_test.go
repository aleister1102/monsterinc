package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultGlobalConfig(t *testing.T) {
	cfg := NewDefaultGlobalConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "onetime", cfg.Mode)
	assert.NotNil(t, cfg.CrawlerConfig)
	assert.NotNil(t, cfg.DiffConfig)
	assert.NotNil(t, cfg.ExtractorConfig)
	assert.NotNil(t, cfg.LogConfig)
	assert.NotNil(t, cfg.MonitorConfig)
	assert.NotNil(t, cfg.NotificationConfig)
	assert.NotNil(t, cfg.ReporterConfig)
	assert.NotNil(t, cfg.ResourceLimiterConfig)
	assert.NotNil(t, cfg.SchedulerConfig)
	assert.NotNil(t, cfg.StorageConfig)
	assert.NotNil(t, cfg.ScanBatchConfig)
	assert.NotNil(t, cfg.MonitorBatchConfig)
	assert.NotNil(t, cfg.ProgressConfig)
}

func TestLoadGlobalConfig_NoConfigFile(t *testing.T) {
	logger := zerolog.Nop()

	cfg, err := LoadGlobalConfig("", logger)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "onetime", cfg.Mode)
}

func TestLoadGlobalConfig_NonExistentFile(t *testing.T) {
	logger := zerolog.Nop()

	cfg, err := LoadGlobalConfig("/nonexistent/config.json", logger)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config file does not exist")
}

func TestLoadGlobalConfig_JSONFile(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.json")

	configData := `{
		"mode": "continuous",
		"log_config": {
			"level": "debug"
		},
		"crawler_config": {
			"user_agent": "test-agent"
		}
	}`

	err := os.WriteFile(configFile, []byte(configData), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "continuous", cfg.Mode)
	assert.Equal(t, "debug", cfg.LogConfig.LogLevel)
	assert.Equal(t, "test-agent", cfg.CrawlerConfig.UserAgent)
}

func TestLoadGlobalConfig_YAMLFile(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configData := `
mode: continuous
log_config:
  level: debug
crawler_config:
  user_agent: test-agent
notification_config:
  notify_on_success: true
  scan_service_discord_webhook_url: https://example.com/webhook
`

	err := os.WriteFile(configFile, []byte(configData), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "continuous", cfg.Mode)
	assert.Equal(t, "debug", cfg.LogConfig.LogLevel)
	assert.Equal(t, "test-agent", cfg.CrawlerConfig.UserAgent)
	assert.Equal(t, "https://example.com/webhook", cfg.NotificationConfig.ScanServiceDiscordWebhookURL)
}

func TestLoadGlobalConfig_YMLFile(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yml")

	configData := `
mode: scheduled
scheduler_config:
  cycle_minutes: 60
`

	err := os.WriteFile(configFile, []byte(configData), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "scheduled", cfg.Mode)
	assert.Equal(t, 60, cfg.SchedulerConfig.CycleMinutes)
}

func TestLoadGlobalConfig_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.json")

	invalidJSON := `{"mode": "test",}`

	err := os.WriteFile(configFile, []byte(invalidJSON), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to unmarshal JSON")
}

func TestLoadGlobalConfig_InvalidYAML(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.yaml")

	invalidYAML := `
mode: test
  invalid_indent: value
`

	err := os.WriteFile(configFile, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to unmarshal YAML")
}

func TestLoadConfigFileContent_FileManager(t *testing.T) {
	logger := zerolog.Nop()
	fileManager := common.NewFileManager(logger)
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.json")

	testData := `{"test": "data"}`
	err := os.WriteFile(testFile, []byte(testData), 0644)
	require.NoError(t, err)

	data, err := loadConfigFileContent(fileManager, testFile)

	require.NoError(t, err)
	assert.Equal(t, []byte(testData), data)
}

func TestParseConfigContent_JSON(t *testing.T) {
	data := []byte(`{"mode": "test"}`)
	cfg := &GlobalConfig{}

	err := parseConfigContent(data, "config.json", cfg)

	require.NoError(t, err)
	assert.Equal(t, "test", cfg.Mode)
}

func TestParseConfigContent_YAML(t *testing.T) {
	data := []byte(`mode: test`)
	cfg := &GlobalConfig{}

	err := parseConfigContent(data, "config.yaml", cfg)

	require.NoError(t, err)
	assert.Equal(t, "test", cfg.Mode)
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".yaml", true},
		{".yml", true},
		{".json", false},
		{".txt", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := isYAMLFile(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseYAMLConfig(t *testing.T) {
	data := []byte(`
mode: yaml_test
log_config:
  level: warn
`)
	cfg := &GlobalConfig{}

	err := parseYAMLConfig(data, "test.yaml", cfg)

	require.NoError(t, err)
	assert.Equal(t, "yaml_test", cfg.Mode)
	assert.Equal(t, "warn", cfg.LogConfig.LogLevel)
}

func TestParseJSONConfig(t *testing.T) {
	data := []byte(`{
		"mode": "json_test",
		"log_config": {
			"level": "info"
		}
	}`)
	cfg := &GlobalConfig{}

	err := parseJSONConfig(data, "test.json", cfg)

	require.NoError(t, err)
	assert.Equal(t, "json_test", cfg.Mode)
	assert.Equal(t, "info", cfg.LogConfig.LogLevel)
}

func TestLoadGlobalConfig_LargeConfigFile(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "large_config.json")

	// Create a large config file (over 10MB would fail due to size limit)
	largeConfig := `{"mode": "test"`
	for i := 0; i < 1000; i++ {
		largeConfig += `, "key` + string(rune(i)) + `": "value` + string(rune(i)) + `"`
	}
	largeConfig += `}`

	err := os.WriteFile(configFile, []byte(largeConfig), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "test", cfg.Mode)
}

func TestLoadGlobalConfig_CompleteConfiguration(t *testing.T) {
	logger := zerolog.Nop()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "complete.yaml")

	completeConfig := `
mode: continuous
log_config:
  log_level: debug
  log_format: json
crawler_config:
  user_agent: complete-test-agent
  request_timeout_secs: 30
  max_depth: 5
scheduler_config:
  cycle_minutes: 120
  retry_attempts: 3
monitor_config:
  enabled: true
  check_interval_seconds: 60
storage_config:
  parquet_base_path: /tmp/test
notification_config:
  notify_on_success: true
  scan_service_discord_webhook_url: https://example.com/webhook
`

	err := os.WriteFile(configFile, []byte(completeConfig), 0644)
	require.NoError(t, err)

	cfg, err := LoadGlobalConfig(configFile, logger)

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "continuous", cfg.Mode)
	assert.Equal(t, "debug", cfg.LogConfig.LogLevel)
	assert.Equal(t, "json", cfg.LogConfig.LogFormat)
	assert.Equal(t, "complete-test-agent", cfg.CrawlerConfig.UserAgent)
	assert.Equal(t, 30, cfg.CrawlerConfig.RequestTimeoutSecs)
	assert.Equal(t, 5, cfg.CrawlerConfig.MaxDepth)
	assert.Equal(t, 120, cfg.SchedulerConfig.CycleMinutes)
	assert.Equal(t, 3, cfg.SchedulerConfig.RetryAttempts)
	assert.True(t, cfg.MonitorConfig.Enabled)
	assert.Equal(t, 60, cfg.MonitorConfig.CheckIntervalSeconds)
	assert.Equal(t, "/tmp/test", cfg.StorageConfig.ParquetBasePath)
	assert.True(t, cfg.NotificationConfig.NotifyOnSuccess)
	assert.Equal(t, "https://example.com/webhook", cfg.NotificationConfig.ScanServiceDiscordWebhookURL)
}
