package config

import (
	"encoding/json"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/aleister1102/monsterinc/internal/common/filemanager"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// GlobalConfig contains all configuration sections for the application
type GlobalConfig struct {
	CrawlerConfig      CrawlerConfig      `json:"crawler_config,omitempty" yaml:"crawler_config,omitempty"`
	HttpxRunnerConfig  HttpxRunnerConfig  `json:"httpx_runner_config,omitempty" yaml:"httpx_runner_config,omitempty"`
	LogConfig          LogConfig          `json:"log_config,omitempty" yaml:"log_config,omitempty"`
	Mode               string             `json:"mode,omitempty" yaml:"mode,omitempty" validate:"required,mode"`
	NotificationConfig NotificationConfig `json:"notification_config,omitempty" yaml:"notification_config,omitempty"`
	ReporterConfig     ReporterConfig     `json:"reporter_config,omitempty" yaml:"reporter_config,omitempty"`
	SchedulerConfig    SchedulerConfig    `json:"scheduler_config,omitempty" yaml:"scheduler_config,omitempty"`
	StorageConfig      StorageConfig      `json:"storage_config,omitempty" yaml:"storage_config,omitempty"`
	ScanBatchConfig    ScanBatchConfig    `json:"scan_batch_config,omitempty" yaml:"scan_batch_config,omitempty"`
}

// NewDefaultGlobalConfig creates a new GlobalConfig with default values
func NewDefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		CrawlerConfig:      NewDefaultCrawlerConfig(),
		HttpxRunnerConfig:  NewDefaultHTTPXRunnerConfig(),
		LogConfig:          NewDefaultLogConfig(),
		Mode:               "onetime",
		NotificationConfig: NewDefaultNotificationConfig(),
		ReporterConfig:     NewDefaultReporterConfig(),
		SchedulerConfig:    NewDefaultSchedulerConfig(),
		StorageConfig:      NewDefaultStorageConfig(),
		ScanBatchConfig:    NewDefaultScanBatchConfig(),
	}
}

// LoadGlobalConfig loads the configuration from a file or default locations.
// It determines the config file path using GetConfigPath, supports both JSON and YAML formats.
// YAML is preferred if the file extension is .yaml or .yml.
func LoadGlobalConfig(providedPath string, logger zerolog.Logger) (*GlobalConfig, error) {
	cfg := NewDefaultGlobalConfig()

	filePath := GetConfigPath(providedPath)
	if filePath == "" {
		return cfg, nil
	}

	fileManager := filemanager.NewFileManager(logger)
	if !fileManager.FileExists(filePath) {
		return nil, errorwrapper.NewValidationError("config_file", filePath, "config file does not exist")
	}

	data, err := loadConfigFileContent(fileManager, filePath)
	if err != nil {
		return nil, errorwrapper.WrapError(err, "failed to load config file content")
	}

	if err := parseConfigContent(data, filePath, cfg); err != nil {
		return nil, errorwrapper.WrapError(err, "failed to parse config content")
	}

	return cfg, nil
}

// loadConfigFileContent reads the config file using FileManager
func loadConfigFileContent(fileManager *filemanager.FileManager, filePath string) ([]byte, error) {
	opts := filemanager.DefaultFileReadOptions()
	opts.MaxSize = 10 * 1024 * 1024 // 10MB max config file size

	return fileManager.ReadFile(filePath, opts)
}

// parseConfigContent parses the config content based on file extension
func parseConfigContent(data []byte, filePath string, cfg *GlobalConfig) error {
	ext := filepath.Ext(filePath)
	if isYAMLFile(ext) {
		return parseYAMLConfig(data, filePath, cfg)
	}
	return parseJSONConfig(data, filePath, cfg)
}

// isYAMLFile checks if the file extension indicates a YAML file
func isYAMLFile(ext string) bool {
	return ext == ".yaml" || ext == ".yml"
}

// parseYAMLConfig parses YAML configuration
func parseYAMLConfig(data []byte, filePath string, cfg *GlobalConfig) error {
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return errorwrapper.NewError("failed to unmarshal YAML from '%s': %w", filePath, err)
	}
	return nil
}

// parseJSONConfig parses JSON configuration
func parseJSONConfig(data []byte, filePath string, cfg *GlobalConfig) error {
	if err := json.Unmarshal(data, cfg); err != nil {
		return errorwrapper.NewError("failed to unmarshal JSON from '%s': %w", filePath, err)
	}
	return nil
}
