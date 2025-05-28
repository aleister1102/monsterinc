package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the Discord bot configuration
type Config struct {
	Discord DiscordConfig `yaml:"discord"`
	Bot     BotConfig     `yaml:"bot"`
	Paths   PathsConfig   `yaml:"paths"`
	Service ServiceConfig `yaml:"service"`
}

// DiscordConfig contains Discord-specific settings
type DiscordConfig struct {
	Token      string `yaml:"token"`
	GuildID    string `yaml:"guild_id"`
	WebhookURL string `yaml:"webhook_url"`
}

// BotConfig contains bot behavior settings
type BotConfig struct {
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	CommandsPerMinute int           `yaml:"commands_per_minute"`
	BurstLimit        int           `yaml:"burst_limit"`
	WindowDuration    time.Duration `yaml:"window_duration"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// PathsConfig contains file paths
type PathsConfig struct {
	TargetsDir    string `yaml:"targets_dir"`
	URLsFile      string `yaml:"urls_file"`
	JSHTMLFile    string `yaml:"js_html_file"`
	MonsterIncBin string `yaml:"monsterinc_bin"`
}

// ServiceConfig contains MonsterInc service settings
type ServiceConfig struct {
	CheckTimeout    time.Duration `yaml:"check_timeout"`
	ExecuteTimeout  time.Duration `yaml:"execute_timeout"`
	ProcessName     string        `yaml:"process_name"`
	WatchdogEnabled bool          `yaml:"watchdog_enabled"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config if not exists
		defaultConfig := getDefaultConfig()
		if err := saveConfig(defaultConfig, configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return defaultConfig, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Override with environment variables if set
	if token := os.Getenv("DISCORD_BOT_TOKEN"); token != "" {
		config.Discord.Token = token
	}
	if webhookURL := os.Getenv("DISCORD_WEBHOOK_URL"); webhookURL != "" {
		config.Discord.WebhookURL = webhookURL
	}

	// Validate required fields
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// getDefaultConfig returns default configuration
func getDefaultConfig() *Config {
	return &Config{
		Discord: DiscordConfig{
			Token:      "", // Must be set via environment variable or config file
			GuildID:    "", // Must be set in config file
			WebhookURL: "", // Must be set via environment variable or config file
		},
		Bot: BotConfig{
			RateLimit: RateLimitConfig{
				CommandsPerMinute: 30,
				BurstLimit:        5,
				WindowDuration:    time.Minute,
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "console",
				File:   "",
			},
		},
		Paths: PathsConfig{
			TargetsDir:    "../../targets",
			URLsFile:      "urls.txt",
			JSHTMLFile:    "js_html.txt",
			MonsterIncBin: "../../monsterinc",
		},
		Service: ServiceConfig{
			CheckTimeout:    30 * time.Second,
			ExecuteTimeout:  5 * time.Minute,
			ProcessName:     "monsterinc",
			WatchdogEnabled: true,
		},
	}
}

// saveConfig saves configuration to YAML file
func saveConfig(config *Config, configPath string) error {
	// Create directory if not exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Discord.Token == "" {
		return fmt.Errorf("discord token is required (set DISCORD_BOT_TOKEN environment variable)")
	}
	if config.Discord.GuildID == "" {
		return fmt.Errorf("discord guild_id is required")
	}
	if config.Paths.TargetsDir == "" {
		return fmt.Errorf("targets directory path is required")
	}
	if config.Paths.MonsterIncBin == "" {
		return fmt.Errorf("monsterinc binary path is required")
	}
	return nil
}
