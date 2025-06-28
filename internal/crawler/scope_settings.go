package crawler

import (
	"fmt"
	"net/url"
	"strings"

	"slices"

	"github.com/aleister1102/monsterinc/internal/common/errors"
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

	autoAddSeedHostnames bool
}

// String returns a string representation of ScopeSettings for logging
func (s *ScopeSettings) String() string {
	return fmt.Sprintf("ScopeSettings{disallowed_hostnames:%v, disallowed_subdomains:%v, disallowed_file_extensions:%v, seed_hostnames:%v, auto_add_seed_hostnames:%t}",
		s.disallowedHostnames,
		s.disallowedSubdomains,
		s.disallowedFileExtensions,
		s.seedHostnames,
		s.autoAddSeedHostnames,
	)
}

// NewScopeSettings creates a new ScopeSettings instance
func NewScopeSettings(
	rootURLHostname string,
	disallowedHostnames []string,
	disallowedSubdomains []string,
	disallowedFileExtensions []string,
	logger zerolog.Logger,
	autoAddSeedHostnames bool,
	originalSeedURLs []string,
) (*ScopeSettings, error) {
	scopeLogger := logger.With().Str("component", "ScopeSettings").Logger()

	settings := &ScopeSettings{
		disallowedHostnames:      disallowedHostnames,
		disallowedSubdomains:     disallowedSubdomains,
		disallowedFileExtensions: disallowedFileExtensions,
		logger:                   scopeLogger,
		autoAddSeedHostnames:     autoAddSeedHostnames,
	}

	// Extract seed hostnames for auto-allow functionality
	if len(originalSeedURLs) > 0 {
		settings.seedHostnames = extractHostnamesFromSeedURLs(originalSeedURLs, scopeLogger)
		if autoAddSeedHostnames {
			scopeLogger.Info().
				Strs("seed_hostnames", settings.seedHostnames).
				Msg("Seed hostnames will be auto-allowed unless explicitly disallowed")
		}
	}

	scopeLogger.Info().
		Strs("disallowed_hostnames", disallowedHostnames).
		Strs("disallowed_subdomains", disallowedSubdomains).
		Strs("disallowed_file_extensions", disallowedFileExtensions).
		Strs("seed_hostnames", settings.seedHostnames).
		Bool("auto_add_seed_hostnames", autoAddSeedHostnames).
		Msg("ScopeSettings initialized")

	return settings, nil
}

// extractHostnamesFromSeedURLs extracts hostnames from a list of seed URLs
func extractHostnamesFromSeedURLs(seedURLs []string, logger zerolog.Logger) []string {
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

// IsURLAllowed checks if a URL is allowed based on hostname and path scope
func (s *ScopeSettings) IsURLAllowed(urlString string) (bool, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false, errors.WrapError(err, "failed to parse URL for scope check")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return false, errors.NewValidationError("hostname", hostname, "hostname cannot be empty")
	}

	if !s.checkHostnameScope(hostname) {
		return false, nil
	}

	if !s.checkPathScope(parsedURL.Path) {
		return false, nil
	}

	return true, nil
}

// checkHostnameScope checks if a given hostname is within the defined scope
func (s *ScopeSettings) checkHostnameScope(hostname string) bool {
	s.logger.Debug().
		Str("hostname", hostname).
		Strs("seed_hostnames", s.seedHostnames).
		Strs("disallowed_hostnames", s.disallowedHostnames).
		Bool("auto_add_seed_hostnames", s.autoAddSeedHostnames).
		Msg("Starting hostname scope check")

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

	// Priority 3: Only allow exact hostname matches with seed hostnames
	if len(s.seedHostnames) > 0 {
		if containsString(hostname, s.seedHostnames) {
			s.logger.Debug().Str("hostname", hostname).Msg("Hostname matches exact seed hostname")
			return true
		} else {
			s.logger.Debug().
				Str("hostname", hostname).
				Strs("seed_hostnames", s.seedHostnames).
				Msg("Hostname not in seed hostnames")
			return false
		}
	}

	// Priority 4: Default behavior when no seed URLs provided
	// Only allow if there are no hostname restrictions at all
	if len(s.seedHostnames) == 0 && len(s.disallowedHostnames) == 0 {
		s.logger.Debug().
			Str("hostname", hostname).
			Msg("Hostname allowed by default (no seed or hostname restrictions)")
		return true
	}

	// Fallback: allow if only disallowed hostnames are configured (and hostname wasn't disallowed above)
	s.logger.Debug().
		Str("hostname", hostname).
		Msg("Hostname allowed (only disallowed hostnames configured)")
	return true
}

// containsString checks if string exists in slice
func containsString(str string, slice []string) bool {
	return slices.Contains(slice, str)
}

// isSubdomainOf checks if hostname is a subdomain of baseHostname
func isSubdomainOf(hostname, baseHostname string) bool {
	return strings.HasSuffix(hostname, "."+baseHostname) && hostname != baseHostname
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

// checkPathScope checks if a given URL path is within the defined scope
func (s *ScopeSettings) checkPathScope(path string) bool {
	// Strip query parameters and fragments from path for extension checking
	cleanPath := path
	if queryIndex := strings.Index(cleanPath, "?"); queryIndex != -1 {
		cleanPath = cleanPath[:queryIndex]
	}
	if fragmentIndex := strings.Index(cleanPath, "#"); fragmentIndex != -1 {
		cleanPath = cleanPath[:fragmentIndex]
	}

	// Fast path: check disallowed file extensions
	for _, ext := range s.disallowedFileExtensions {
		if strings.HasSuffix(cleanPath, ext) {
			s.logger.Debug().
				Str("path", path).
				Str("clean_path", cleanPath).
				Str("extension", ext).
				Msg("Path matches disallowed file extension")
			return false
		}
	}

	s.logger.Debug().
		Str("path", path).
		Str("clean_path", cleanPath).
		Msg("Path allowed by default")
	return true
}
