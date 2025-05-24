package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// CrawlerScopeConfig is now defined in config.go
// type CrawlerScopeConfig struct {
// 	AllowedHostnames      []string `json:"allowed_hostnames" yaml:"allowed_hostnames"`
// 	AllowedSubdomains     []string `json:"allowed_subdomains" yaml:"allowed_subdomains"`
// 	DisallowedHostnames   []string `json:"disallowed_hostnames" yaml:"disallowed_hostnames"`
// 	DisallowedSubdomains  []string `json:"disallowed_subdomains" yaml:"disallowed_subdomains"`
// 	AllowedPathRegexes    []string `json:"allowed_path_regexes" yaml:"allowed_path_regexes"`
// 	DisallowedPathRegexes []string `json:"disallowed_path_regexes" yaml:"disallowed_path_regexes"`
// }

// CrawlerConfig is now defined in config.go
// // Task 4.1: Define crawler settings in configuration structure.
// type CrawlerConfig struct {
// 	SeedURLs         []string           `json:"seed_urls" yaml:"seed_urls"`
// 	UserAgent        string             `json:"user_agent" yaml:"user_agent"`
// 	RequestTimeout   time.Duration      `json:"request_timeout" yaml:"request_timeout"` // e.g., "10s", "1m"
// 	Threads          int                `json:"threads" yaml:"threads"` // Will map to MaxConcurrentRequests in the main CrawlerConfig
// 	MaxDepth         int                `json:"max_depth" yaml:"max_depth"`
// 	RespectRobotsTxt bool               `json:"respect_robots_txt" yaml:"respect_robots_txt"`
// 	Scope            CrawlerScopeConfig `json:"scope" yaml:"scope"`
// 	// Add other configurations as needed, e.g., Proxy, Delay, OutputDir, etc.
// }

// NewDefaultCrawlerConfig creates a CrawlerConfig with default values.
// It now returns the main config.CrawlerConfig type with crawler-specific defaults.
func NewDefaultCrawlerConfig() *CrawlerConfig { // Return type is now the one from config.go
	return &CrawlerConfig{
		UserAgent:             DefaultCrawlerUserAgent,      // Using constants from config.go
		RequestTimeout:        DefaultCrawlerRequestTimeout, // Using constants from config.go
		MaxConcurrentRequests: DefaultCrawlerThreads,        // This field in the main config.CrawlerConfig corresponds to Threads
		MaxDepth:              DefaultCrawlerMaxDepth,       // Using constants from config.go
		RespectRobotsTxt:      DefaultCrawlerRespectRobots,  // Using constants from config.go
		Scope: CrawlerScopeConfig{ // Default scope allows crawling anywhere (empty lists imply no restriction initially)
			AllowedHostnames:      nil, // Empty means any hostname if not disallowed
			AllowedSubdomains:     nil,
			DisallowedHostnames:   nil,
			DisallowedSubdomains:  nil,
			AllowedPathRegexes:    nil, // Empty means any path if not disallowed
			DisallowedPathRegexes: nil,
		},
		// SeedURLs should be provided by the user or a higher-level config.
	}
}

// LoadCrawlerConfigFromFile loads crawler configuration from a JSON file.
// Task 4.2: Implement configuration loading for crawler settings.
// It now returns the main config.CrawlerConfig type.
func LoadCrawlerConfigFromFile(filePath string) (*CrawlerConfig, error) { // Return type is now the one from config.go
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	config := NewDefaultCrawlerConfig() // Start with defaults, then override
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from '%s': %w", filePath, err)
	}

	// Basic validation for essential fields
	if len(config.SeedURLs) == 0 {
		log.Println("[WARN] Config: No seed URLs provided in the configuration file.")
		// Depending on requirements, you might want to return an error here:
		// return nil, fmt.Errorf("no seed URLs provided in config file '%s'", filePath)
	}

	return config, nil
}
