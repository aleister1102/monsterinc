package config

import (
	"os"
	"path/filepath"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/rs/zerolog"
)

// GetConfigPath determines the configuration file path based on command-line flags,
// environment variables, and default locations.
// Priority:
//
// 1. configFilePathFlag (command-line flag)
//
// 2. MONSTERINC_CONFIG_PATH environment variable
//
// 3. config.yaml in the current working directory
//
// 4. config.json in the current working directory
//
// 5. config.yaml in the executable's directory
//
// 6. config.json in the executable's directory
func GetConfigPath(configFilePathFlag string) string {
	logger := zerolog.Nop() // Use nop logger for backward compatibility
	locator := NewConfigFileLocator(logger)
	return locator.FindConfigFile(configFilePathFlag)
}

// ConfigFileLocator handles the logic for finding configuration files
type ConfigFileLocator struct {
	fileManager *common.FileManager
	logger      zerolog.Logger
}

// NewConfigFileLocator creates a new ConfigFileLocator
func NewConfigFileLocator(logger zerolog.Logger) *ConfigFileLocator {
	return &ConfigFileLocator{
		fileManager: common.NewFileManager(logger),
		logger:      logger,
	}
}

// FindConfigFile locates the configuration file using the priority order
func (cfl *ConfigFileLocator) FindConfigFile(configFilePathFlag string) string {
	// Priority 1: Command-line flag
	if path := cfl.checkProvidedPath(configFilePathFlag); path != "" {
		return path
	}

	// Priority 2: Environment variable
	if path := cfl.checkEnvironmentPath(); path != "" {
		return path
	}

	// Priority 3-6: Default locations
	return cfl.checkDefaultLocations()
}

// checkProvidedPath validates the provided config file path
func (cfl *ConfigFileLocator) checkProvidedPath(configPath string) string {
	if configPath == "" {
		return ""
	}

	if cfl.fileManager.FileExists(configPath) {
		cfl.logger.Debug().Str("path", configPath).Msg("Using provided config file")
		return configPath
	}

	cfl.logger.Warn().Str("path", configPath).Msg("Provided config file does not exist")
	return ""
}

// checkEnvironmentPath checks for config path in environment variable
func (cfl *ConfigFileLocator) checkEnvironmentPath() string {
	envPath := os.Getenv("MONSTERINC_CONFIG_PATH")
	if envPath == "" {
		return ""
	}

	if cfl.fileManager.FileExists(envPath) {
		cfl.logger.Debug().Str("path", envPath).Msg("Using config file from environment variable")
		return envPath
	}

	cfl.logger.Warn().Str("path", envPath).Msg("Config file from environment variable does not exist")
	return ""
}

// checkDefaultLocations searches for config files in default locations
func (cfl *ConfigFileLocator) checkDefaultLocations() string {
	locations := cfl.getSearchLocations()
	defaultFiles := []string{"config.yaml", "config.json"}

	for _, location := range locations {
		for _, fileName := range defaultFiles {
			path := filepath.Join(location, fileName)
			if cfl.fileManager.FileExists(path) {
				cfl.logger.Debug().Str("path", path).Msg("Found config file in default location")
				return path
			}
		}
	}

	return ""
}

// getSearchLocations returns the directories to search for config files
func (cfl *ConfigFileLocator) getSearchLocations() []string {
	var locations []string

	// Add current working directory
	if cwd := cfl.getCurrentWorkingDirectory(); cwd != "" {
		locations = append(locations, cwd)
	}

	// Add executable directory (if different from CWD)
	if exeDir := cfl.getExecutableDirectory(); exeDir != "" {
		// Only add if different from CWD to avoid duplicate checks
		if len(locations) == 0 || locations[0] != exeDir {
			locations = append(locations, exeDir)
		}
	}

	return locations
}

// getCurrentWorkingDirectory gets the current working directory
func (cfl *ConfigFileLocator) getCurrentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		cfl.logger.Warn().Err(err).Msg("Failed to get current working directory")
		return ""
	}
	return cwd
}

// getExecutableDirectory gets the directory containing the executable
func (cfl *ConfigFileLocator) getExecutableDirectory() string {
	exePath, err := os.Executable()
	if err != nil {
		cfl.logger.Warn().Err(err).Msg("Failed to get executable path")
		return ""
	}
	return filepath.Dir(exePath)
}
