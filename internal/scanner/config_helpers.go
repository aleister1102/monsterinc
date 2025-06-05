package scanner

import (
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/crawler"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
)

// prepareScanConfiguration prepares crawler configuration for the scan.
func (s *Scanner) prepareScanConfiguration(seedURLs []string, scanSessionID string) (*config.CrawlerConfig, string, error) {
	crawlerConfig := &s.config.CrawlerConfig
	currentCrawlerConfig := *crawlerConfig
	currentCrawlerConfig.SeedURLs = seedURLs

	if currentCrawlerConfig.AutoAddSeedHostnames && len(seedURLs) > 0 {
		s.autoAddSeedHostnames(&currentCrawlerConfig, seedURLs, scanSessionID)
	}

	primaryRootTargetURL := s.determinePrimaryRootTargetURL(seedURLs, scanSessionID)
	return &currentCrawlerConfig, primaryRootTargetURL, nil
}

// autoAddSeedHostnames automatically adds seed hostnames to allowed hostnames if configured.
func (s *Scanner) autoAddSeedHostnames(crawlerConfig *config.CrawlerConfig, seedURLs []string, scanSessionID string) {
	seedHostnames := crawler.ExtractHostnamesFromSeedURLs(seedURLs, s.logger)
	if len(seedHostnames) == 0 {
		return
	}

	originalAllowedHostnames := crawlerConfig.Scope.AllowedHostnames
	crawlerConfig.Scope.AllowedHostnames = crawler.MergeAllowedHostnames(
		crawlerConfig.Scope.AllowedHostnames,
		seedHostnames,
	)

	s.logger.Info().
		Strs("seed_hostnames", seedHostnames).
		Strs("original_allowed_hostnames", originalAllowedHostnames).
		Strs("final_allowed_hostnames", crawlerConfig.Scope.AllowedHostnames).
		Str("session_id", scanSessionID).
		Msg("Auto-added seed hostnames to allowed hostnames")
}

// determinePrimaryRootTargetURL determines the primary root target URL for the scan.
func (s *Scanner) determinePrimaryRootTargetURL(seedURLs []string, scanSessionID string) string {
	if len(seedURLs) > 0 {
		return seedURLs[0]
	}
	return "unknown_target_" + scanSessionID
}

// buildHTTPXConfig creates an httpx configuration from discovered URLs and scanner config.
func (s *Scanner) buildHTTPXConfig(discoveredURLs []string) *httpxrunner.Config {
	httpxConfig := s.config.HttpxRunnerConfig

	return &httpxrunner.Config{
		Targets:              discoveredURLs,
		Method:               httpxConfig.Method,
		RequestURIs:          httpxConfig.RequestURIs,
		FollowRedirects:      httpxConfig.FollowRedirects,
		Timeout:              httpxConfig.TimeoutSecs,
		Retries:              httpxConfig.Retries,
		Threads:              httpxConfig.Threads,
		CustomHeaders:        httpxConfig.CustomHeaders,
		Verbose:              httpxConfig.Verbose,
		TechDetect:           httpxConfig.TechDetect,
		ExtractTitle:         httpxConfig.ExtractTitle,
		ExtractStatusCode:    httpxConfig.ExtractStatusCode,
		ExtractLocation:      httpxConfig.ExtractLocation,
		ExtractContentLength: httpxConfig.ExtractContentLength,
		ExtractServerHeader:  httpxConfig.ExtractServerHeader,
		ExtractContentType:   httpxConfig.ExtractContentType,
		ExtractIPs:           httpxConfig.ExtractIPs,
		ExtractBody:          httpxConfig.ExtractBody,
		ExtractHeaders:       httpxConfig.ExtractHeaders,
	}
}
