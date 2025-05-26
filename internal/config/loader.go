package config

import (
	"os"
	"path/filepath"
)

// GetConfigPath determines the configuration file path based on command-line flags,
// environment variables, and default locations.
// Priority:
// 1. -config command-line flag
// 2. MONSTERINC_CONFIG_PATH environment variable
// 3. config.yaml in the current working directory
// 4. config.json in the current working directory
// 5. config.yaml in the executable's directory
// 6. config.json in the executable's directory
// If a configFilePath is provided as an argument to this function (e.g. from main), it overrides all others.
func GetConfigPath(configFilePathFlag string) string {
	// 1. Command-line flag (highest priority if provided directly to this function)
	if configFilePathFlag != "" {
		if _, err := os.Stat(configFilePathFlag); err == nil {
			return configFilePathFlag
		}
	}

	// 2. Environment variable
	envPath := os.Getenv("MONSTERINC_CONFIG_PATH")
	if envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// Get current working directory and executable directory
	cwd, errCwd := os.Getwd()
	exePath, errExe := os.Executable()
	exeDir := ""
	if errExe == nil {
		exeDir = filepath.Dir(exePath)
	}

	// Default paths to check
	defaultFiles := []string{"config.yaml", "config.json"}
	locations := []string{}

	if errCwd == nil {
		locations = append(locations, cwd)
	}
	if errExe == nil && exeDir != "" && (errCwd == nil || exeDir != cwd) { // Avoid duplicate check if cwd is exeDir
		locations = append(locations, exeDir)
	}

	for _, loc := range locations {
		for _, file := range defaultFiles {
			path := filepath.Join(loc, file)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return "" // No config file found
}

// Helper function to check if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
