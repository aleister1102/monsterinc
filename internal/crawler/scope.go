package crawler

import (
	"net/url"
	"strings"

	"slices"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// ScopeSettings defines the rules for what URLs the crawler should visit
type ScopeSettings struct {
	disallowedHostnames      []string
	disallowedSubdomains     []string
	disallowedFileExtensions []string
	seedHostnames            []string // Hostnames from seed URLs (for auto-allow)
	logger                   zerolog.Logger
	includeSubdomains        bool
	autoAddSeedHostnames     bool
	originalSeedDomains      []string
}

// ScopeValidator provides methods for URL scope validation
type ScopeValidator struct {
	settings *ScopeSettings
}

// NewScopeValidator creates a new ScopeValidator instance
func NewScopeValidator(settings *ScopeSettings) *ScopeValidator {
	return &ScopeValidator{
		settings: settings,
	}
}

// NewScopeSettings creates a new ScopeSettings instance
func NewScopeSettings(
	rootURLHostname string,
	disallowedHostnames []string,
	disallowedSubdomains []string,
	disallowedFileExtensions []string,
	logger zerolog.Logger,
	includeSubdomains bool,
	autoAddSeedHostnames bool,
	originalSeedURLs []string,
) (*ScopeSettings, error) {
	scopeLogger := logger.With().Str("component", "ScopeSettings").Logger()

	settings := &ScopeSettings{
		disallowedHostnames:      disallowedHostnames,
		disallowedSubdomains:     disallowedSubdomains,
		disallowedFileExtensions: disallowedFileExtensions,
		logger:                   scopeLogger,
		includeSubdomains:        includeSubdomains,
		autoAddSeedHostnames:     autoAddSeedHostnames,
	}

	// Extract seed hostnames for auto-allow functionality
	if len(originalSeedURLs) > 0 {
		settings.seedHostnames = ExtractHostnamesFromSeedURLs(originalSeedURLs, scopeLogger)
		if autoAddSeedHostnames {
			scopeLogger.Info().
				Strs("seed_hostnames", settings.seedHostnames).
				Msg("Seed hostnames will be auto-allowed unless explicitly disallowed")
		}
	}

	if includeSubdomains {
		settings.originalSeedDomains = extractSeedDomains(originalSeedURLs, scopeLogger)
	}

	scopeLogger.Info().
		Strs("disallowed_hostnames", disallowedHostnames).
		Strs("disallowed_subdomains", disallowedSubdomains).
		Strs("disallowed_file_extensions", disallowedFileExtensions).
		Strs("seed_hostnames", settings.seedHostnames).
		Bool("include_subdomains", includeSubdomains).
		Bool("auto_add_seed_hostnames", autoAddSeedHostnames).
		Strs("original_seed_domains", settings.originalSeedDomains).
		Msg("ScopeSettings initialized")

	return settings, nil
}

// extractSeedDomains extracts base domains from seed URLs
func extractSeedDomains(seedURLs []string, logger zerolog.Logger) []string {
	var domains []string
	for _, seedURL := range seedURLs {
		if hostnames := ExtractHostnamesFromSeedURLs([]string{seedURL}, logger); len(hostnames) > 0 {
			domains = append(domains, hostnames[0])
		}
	}

	uniqueDomains := removeDuplicates(domains)
	logger.Debug().Strs("original_seed_domains", uniqueDomains).Msg("Extracted base domains from seed URLs")

	return uniqueDomains
}

// CheckHostnameScope checks if a given hostname is within the defined scope
func (s *ScopeSettings) CheckHostnameScope(hostname string) bool {
	// Priority 1: Check if explicitly disallowed first
	if containsString(hostname, s.disallowedHostnames) {
		s.logger.Debug().Str("hostname", hostname).Msg("Hostname explicitly disallowed")
		return false
	}

	// Check if subdomain of disallowed hostname
	for _, disallowedHost := range s.disallowedHostnames {
		if isSubdomainOf(hostname, disallowedHost) {
			s.logger.Debug().
				Str("hostname", hostname).
				Str("disallowed_base", disallowedHost).
				Msg("Hostname is subdomain of disallowed host")
			return false
		}
	}

	// Check subdomain parts against disallowed subdomains
	if s.hasDisallowedSubdomainPart(hostname) {
		return false
	}

	// Priority 2: Auto-allow seed hostnames (if feature enabled)
	if s.autoAddSeedHostnames && len(s.seedHostnames) > 0 && containsString(hostname, s.seedHostnames) {
		s.logger.Debug().Str("hostname", hostname).Msg("Hostname auto-allowed (seed hostname)")
		return true
	}

	// Priority 3: If includeSubdomains and matches seed domains, allow
	if s.includeSubdomains && s.isAllowedBySeedDomains(hostname) {
		return true
	}

	// Priority 4: Default allow
	s.logger.Debug().Str("hostname", hostname).Msg("Hostname allowed by default")
	return true
}

// hasDisallowedSubdomainPart checks if hostname has disallowed subdomain parts
func (s *ScopeSettings) hasDisallowedSubdomainPart(hostname string) bool {
	hostnameBase, err := urlhandler.GetBaseDomain(hostname)
	if err != nil || hostnameBase == "" {
		return false
	}

	if hostname == hostnameBase {
		return false
	}

	subdomainPart := strings.TrimSuffix(hostname, "."+hostnameBase)
	if subdomainPart == hostname || containsString(subdomainPart, s.disallowedSubdomains) {
		s.logger.Debug().
			Str("hostname", hostname).
			Str("subdomain_part", subdomainPart).
			Msg("Subdomain part is disallowed")
		return true
	}

	return false
}

// isAllowedBySeedDomains checks if hostname is allowed by seed domain policy
func (s *ScopeSettings) isAllowedBySeedDomains(hostname string) bool {
	for _, seedBaseDomain := range s.originalSeedDomains {
		if s.matchesSeedDomain(hostname, seedBaseDomain) {
			return true
		}
	}
	return false
}

// matchesSeedDomain checks if hostname matches or is subdomain of seed domain
func (s *ScopeSettings) matchesSeedDomain(hostname, seedBaseDomain string) bool {
	if hostname == seedBaseDomain {
		return true
	}

	if !strings.HasSuffix(hostname, "."+seedBaseDomain) {
		return false
	}

	// Check if specific subdomain is disallowed
	subdomainPart := strings.TrimSuffix(hostname, "."+seedBaseDomain)
	if containsString(subdomainPart, s.disallowedSubdomains) {
		s.logger.Debug().
			Str("hostname", hostname).
			Str("seed_base", seedBaseDomain).
			Str("subdomain_part", subdomainPart).
			Msg("Allowed by seed domains, but subdomain part is disallowed")
		return false
	}

	s.logger.Debug().
		Str("hostname", hostname).
		Str("seed_base_domain", seedBaseDomain).
		Msg("Hostname allowed by seed domain policy")
	return true
}

// checkPathScope checks if a given URL path is within the defined scope
func (s *ScopeSettings) checkPathScope(path string) bool {
	// Fast path: check disallowed file extensions
	for _, ext := range s.disallowedFileExtensions {
		if strings.HasSuffix(path, ext) {
			s.logger.Debug().
				Str("path", path).
				Str("extension", ext).
				Msg("Path matches disallowed file extension")
			return false
		}
	}

	s.logger.Debug().Str("path", path).Msg("Path allowed by default")
	return true
}

// IsURLAllowed checks if a URL is allowed based on hostname and path scope
func (s *ScopeSettings) IsURLAllowed(urlString string) (bool, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false, common.WrapError(err, "failed to parse URL for scope check")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return false, common.NewValidationError("hostname", hostname, "hostname cannot be empty")
	}

	if !s.CheckHostnameScope(hostname) {
		return false, nil
	}

	if !s.checkPathScope(parsedURL.Path) {
		return false, nil
	}

	return true, nil
}

// Utility functions

// removeDuplicates removes duplicate strings from slice
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// containsString checks if string exists in slice
func containsString(str string, slice []string) bool {
	return slices.Contains(slice, str)
}

// isSubdomainOf checks if hostname is a subdomain of baseHostname
func isSubdomainOf(hostname, baseHostname string) bool {
	return strings.HasSuffix(hostname, "."+baseHostname) && hostname != baseHostname
}

// ExtractHostnamesFromSeedURLs extracts hostnames from a list of seed URLs
func ExtractHostnamesFromSeedURLs(seedURLs []string, logger zerolog.Logger) []string {
	var hostnames []string

	for _, seedURL := range seedURLs {
		if hostname := extractSingleHostname(seedURL, logger); hostname != "" {
			hostnames = append(hostnames, hostname)
		}
	}

	return removeDuplicates(hostnames)
}

// extractSingleHostname extracts hostname from a single URL
func extractSingleHostname(seedURL string, logger zerolog.Logger) string {
	// Validate URL format using urlhandler
	if err := urlhandler.ValidateURLFormat(seedURL); err != nil {
		logger.Warn().Str("seed_url", seedURL).Err(err).Msg("Invalid URL format")
		return ""
	}

	// Extract hostname without port for scope validation
	parsed, err := url.Parse(seedURL)
	if err != nil {
		logger.Warn().Str("seed_url", seedURL).Err(err).Msg("Failed to parse seed URL")
		return ""
	}

	return parsed.Hostname()
}
