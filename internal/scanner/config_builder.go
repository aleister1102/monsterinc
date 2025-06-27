package scanner

import (
	httpx "github.com/aleister1102/go-telescope"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// ConfigBuilder helps build configurations for various scanner components
// Separates configuration building logic from main scanner operations
type ConfigBuilder struct {
	globalConfig *config.GlobalConfig
	logger       zerolog.Logger
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder(globalConfig *config.GlobalConfig, logger zerolog.Logger) *ConfigBuilder {
	return &ConfigBuilder{
		globalConfig: globalConfig,
		logger:       logger.With().Str("module", "ConfigBuilder").Logger(),
	}
}

// BuildCrawlerConfig creates crawler configuration with seed URLs
func (cb *ConfigBuilder) BuildCrawlerConfig(seedURLs []string, scanSessionID string) (*config.CrawlerConfig, string, error) {
	// Create a copy of the crawler config to avoid modifying the global config
	crawlerConfig := cb.globalConfig.CrawlerConfig
	crawlerConfig.SeedURLs = make([]string, len(seedURLs))
	copy(crawlerConfig.SeedURLs, seedURLs)

	primaryRootTargetURL := cb.determinePrimaryRootTarget(seedURLs, scanSessionID)
	return &crawlerConfig, primaryRootTargetURL, nil
}

// BuildHTTPXConfig creates HTTPX runner configuration from global config
func (cb *ConfigBuilder) BuildHTTPXConfig(targets []string) *httpx.Config {
	httpxCfg := &cb.globalConfig.HttpxRunnerConfig

	return &httpx.Config{
		Targets:              targets,
		Method:               httpxCfg.Method,
		RequestURIs:          httpxCfg.RequestURIs,
		FollowRedirects:      httpxCfg.FollowRedirects,
		Timeout:              httpxCfg.TimeoutSecs,
		Retries:              httpxCfg.Retries,
		Threads:              httpxCfg.Threads,
		CustomHeaders:        httpxCfg.CustomHeaders,
		Verbose:              httpxCfg.Verbose,
		TechDetect:           httpxCfg.TechDetect,
		ExtractASN:           httpxCfg.ExtractASN,
		ExtractTitle:         httpxCfg.ExtractTitle,
		ExtractStatusCode:    httpxCfg.ExtractStatusCode,
		ExtractLocation:      httpxCfg.ExtractLocation,
		ExtractContentLength: httpxCfg.ExtractContentLength,
		ExtractServerHeader:  httpxCfg.ExtractServerHeader,
		ExtractContentType:   httpxCfg.ExtractContentType,
		ExtractIPs:           httpxCfg.ExtractIPs,
		ExtractBody:          httpxCfg.ExtractBody,
		ExtractHeaders:       httpxCfg.ExtractHeaders,
	}
}

// determinePrimaryRootTarget determines the primary root target URL
func (cb *ConfigBuilder) determinePrimaryRootTarget(seedURLs []string, scanSessionID string) string {
	if len(seedURLs) > 0 {
		return seedURLs[0]
	}
	return "unknown_target_" + scanSessionID
}
