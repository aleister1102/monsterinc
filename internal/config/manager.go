package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
)

// ConfigManager provides centralized configuration management with caching and hot-reload capabilities
type ConfigManager struct {
	mu           sync.RWMutex
	config       *GlobalConfig
	configPath   string
	logger       zerolog.Logger
	watcher      *fsnotify.Watcher
	reloadChan   chan struct{}
	stopChan     chan struct{}
	lastModified time.Time

	// Configuration validation and caching
	validationEnabled bool
	cacheEnabled      bool

	// Hot-reload settings
	hotReloadEnabled bool
	reloadDelay      time.Duration
}

// ConfigManagerOptions holds options for creating a ConfigManager
type ConfigManagerOptions struct {
	Logger            zerolog.Logger
	ValidationEnabled bool
	CacheEnabled      bool
	HotReloadEnabled  bool
	ReloadDelay       time.Duration
}

// DefaultConfigManagerOptions returns default options for ConfigManager
func DefaultConfigManagerOptions() ConfigManagerOptions {
	return ConfigManagerOptions{
		Logger:            zerolog.Nop(),
		ValidationEnabled: true,
		CacheEnabled:      true,
		HotReloadEnabled:  false,
		ReloadDelay:       time.Second * 2, // 2 second delay to avoid rapid reloads
	}
}

// NewConfigManager creates a new centralized configuration manager
func NewConfigManager(configPath string, opts ConfigManagerOptions) (*ConfigManager, error) {
	cm := &ConfigManager{
		configPath:        configPath,
		logger:            opts.Logger.With().Str("component", "ConfigManager").Logger(),
		reloadChan:        make(chan struct{}, 1),
		stopChan:          make(chan struct{}),
		validationEnabled: opts.ValidationEnabled,
		cacheEnabled:      opts.CacheEnabled,
		hotReloadEnabled:  opts.HotReloadEnabled,
		reloadDelay:       opts.ReloadDelay,
	}

	// Load initial configuration
	if err := cm.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial configuration: %w", err)
	}

	// Setup file watcher for hot-reload if enabled
	if cm.hotReloadEnabled && cm.configPath != "" {
		if err := cm.setupFileWatcher(); err != nil {
			cm.logger.Warn().Err(err).Msg("Failed to setup file watcher, hot-reload disabled")
			cm.hotReloadEnabled = false
		}
	}

	return cm, nil
}

// GetConfig returns the current configuration (thread-safe)
func (cm *ConfigManager) GetConfig() *GlobalConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modifications
	if cm.config == nil {
		return NewDefaultGlobalConfig()
	}

	// Deep copy the configuration
	return cm.copyConfig(cm.config)
}

// ReloadConfig manually reloads the configuration from file
func (cm *ConfigManager) ReloadConfig() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.loadConfig()
}

// UpdateConfig updates the configuration and optionally saves to file
func (cm *ConfigManager) UpdateConfig(newConfig *GlobalConfig, saveToFile bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Validate new configuration if validation is enabled
	if cm.validationEnabled {
		if err := ValidateConfig(newConfig); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	// Update in-memory configuration
	cm.config = cm.copyConfig(newConfig)

	// Save to file if requested
	if saveToFile && cm.configPath != "" {
		if err := SaveGlobalConfig(newConfig, cm.configPath); err != nil {
			return fmt.Errorf("failed to save configuration to file: %w", err)
		}
		cm.logger.Info().Str("path", cm.configPath).Msg("Configuration saved to file")
	}

	cm.logger.Info().Msg("Configuration updated successfully")
	return nil
}

// GetConfigPath returns the current configuration file path
func (cm *ConfigManager) GetConfigPath() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configPath
}

// IsHotReloadEnabled returns whether hot-reload is enabled
func (cm *ConfigManager) IsHotReloadEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.hotReloadEnabled
}

// Close stops the configuration manager and cleans up resources
func (cm *ConfigManager) Close() error {
	close(cm.stopChan)

	if cm.watcher != nil {
		return cm.watcher.Close()
	}

	return nil
}

// StartHotReload starts the hot-reload goroutine (non-blocking)
func (cm *ConfigManager) StartHotReload(ctx context.Context) {
	if !cm.hotReloadEnabled {
		return
	}

	go cm.hotReloadLoop(ctx)
}

// loadConfig loads configuration from file (internal method, assumes lock is held)
func (cm *ConfigManager) loadConfig() error {
	// Determine config path if not set
	if cm.configPath == "" {
		cm.configPath = GetConfigPath("")
	}

	// Load configuration using existing LoadGlobalConfig function
	config, err := LoadGlobalConfig(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration if validation is enabled
	if cm.validationEnabled {
		if err := ValidateConfig(config); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	// Update last modified time if file exists
	if cm.configPath != "" {
		if stat, err := os.Stat(cm.configPath); err == nil {
			cm.lastModified = stat.ModTime()
		}
	}

	cm.config = config
	cm.logger.Info().Str("path", cm.configPath).Msg("Configuration loaded successfully")

	return nil
}

// setupFileWatcher sets up file system watcher for hot-reload
func (cm *ConfigManager) setupFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch the directory containing the config file
	configDir := filepath.Dir(cm.configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch config directory '%s': %w", configDir, err)
	}

	cm.watcher = watcher
	cm.logger.Info().Str("directory", configDir).Msg("File watcher setup for hot-reload")

	return nil
}

// hotReloadLoop runs the hot-reload monitoring loop
func (cm *ConfigManager) hotReloadLoop(ctx context.Context) {
	if cm.watcher == nil {
		return
	}

	reloadTimer := time.NewTimer(0)
	reloadTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			cm.logger.Info().Msg("Hot-reload loop stopped due to context cancellation")
			return

		case <-cm.stopChan:
			cm.logger.Info().Msg("Hot-reload loop stopped")
			return

		case event, ok := <-cm.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if event.Name == cm.configPath && (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				cm.logger.Debug().Str("file", event.Name).Str("op", event.Op.String()).Msg("Config file change detected")

				// Reset timer to delay reload (avoid rapid successive reloads)
				reloadTimer.Reset(cm.reloadDelay)
			}

		case err, ok := <-cm.watcher.Errors:
			if !ok {
				return
			}
			cm.logger.Error().Err(err).Msg("File watcher error")

		case <-reloadTimer.C:
			// Check if file was actually modified
			if stat, err := os.Stat(cm.configPath); err == nil {
				if stat.ModTime().After(cm.lastModified) {
					cm.logger.Info().Msg("Reloading configuration due to file change")
					if err := cm.ReloadConfig(); err != nil {
						cm.logger.Error().Err(err).Msg("Failed to reload configuration")
					} else {
						cm.logger.Info().Msg("Configuration reloaded successfully")
					}
				}
			}
		}
	}
}

// copyConfig creates a deep copy of the configuration
func (cm *ConfigManager) copyConfig(src *GlobalConfig) *GlobalConfig {
	if src == nil {
		return NewDefaultGlobalConfig()
	}

	// For now, we'll use a simple approach
	// In a production environment, you might want to use a more sophisticated deep copy method
	dst := NewDefaultGlobalConfig()

	// Copy all fields (this is a simplified version)
	*dst = *src

	// Deep copy slices and maps
	dst.InputConfig.InputURLs = make([]string, len(src.InputConfig.InputURLs))
	copy(dst.InputConfig.InputURLs, src.InputConfig.InputURLs)

	dst.HttpxRunnerConfig.RequestURIs = make([]string, len(src.HttpxRunnerConfig.RequestURIs))
	copy(dst.HttpxRunnerConfig.RequestURIs, src.HttpxRunnerConfig.RequestURIs)

	dst.HttpxRunnerConfig.CustomHeaders = make(map[string]string)
	for k, v := range src.HttpxRunnerConfig.CustomHeaders {
		dst.HttpxRunnerConfig.CustomHeaders[k] = v
	}

	// Copy other slice fields
	dst.CrawlerConfig.SeedURLs = make([]string, len(src.CrawlerConfig.SeedURLs))
	copy(dst.CrawlerConfig.SeedURLs, src.CrawlerConfig.SeedURLs)

	dst.NotificationConfig.MentionRoleIDs = make([]string, len(src.NotificationConfig.MentionRoleIDs))
	copy(dst.NotificationConfig.MentionRoleIDs, src.NotificationConfig.MentionRoleIDs)

	dst.PathExtractorDomains = make([]string, len(src.PathExtractorDomains))
	copy(dst.PathExtractorDomains, src.PathExtractorDomains)

	return dst
}

// GetConfigHealth returns health information about the configuration manager
func (cm *ConfigManager) GetConfigHealth() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	health := map[string]interface{}{
		"config_loaded":      cm.config != nil,
		"config_path":        cm.configPath,
		"hot_reload_enabled": cm.hotReloadEnabled,
		"validation_enabled": cm.validationEnabled,
		"cache_enabled":      cm.cacheEnabled,
	}

	if cm.configPath != "" {
		if stat, err := os.Stat(cm.configPath); err == nil {
			health["file_exists"] = true
			health["last_modified"] = stat.ModTime()
			health["file_size"] = stat.Size()
		} else {
			health["file_exists"] = false
			health["file_error"] = err.Error()
		}
	}

	return health
}
