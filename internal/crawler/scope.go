package crawler

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
	"slices"
)

// ScopeSettings defines the rules for what URLs the crawler should visit
type ScopeSettings struct {
	allowedHostnames       []string
	allowedSubdomains      []string
	disallowedHostnames    []string
	disallowedSubdomains   []string
	allowedPathPatterns    []*regexp.Regexp
	disallowedPathPatterns []*regexp.Regexp
	logger                 zerolog.Logger
	includeSubdomains      bool
	originalSeedDomains    []string
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

// NewScopeSettings creates a new ScopeSettings instance with compiled regex patterns
func NewScopeSettings(
	rootURLHostname string,
	allowedHostnames, disallowedHostnames []string,
	allowedSubdomains, disallowedSubdomains []string,
	allowedPathRegexes, disallowedPathRegexes []string,
	logger zerolog.Logger,
	includeSubdomains bool,
	originalSeedURLs []string,
) (*ScopeSettings, error) {
	scopeLogger := logger.With().Str("component", "ScopeSettings").Logger()

	builder := NewScopeSettingsBuilder(scopeLogger).
		WithHostnames(allowedHostnames, disallowedHostnames).
		WithSubdomains(allowedSubdomains, disallowedSubdomains).
		WithPathRegexes(allowedPathRegexes, disallowedPathRegexes).
		WithSubdomainPolicy(includeSubdomains).
		WithSeedURLs(originalSeedURLs)

	return builder.Build()
}

// ScopeSettingsBuilder provides a fluent interface for creating ScopeSettings
type ScopeSettingsBuilder struct {
	settings *ScopeSettings
	logger   zerolog.Logger
}

// NewScopeSettingsBuilder creates a new ScopeSettingsBuilder instance
func NewScopeSettingsBuilder(logger zerolog.Logger) *ScopeSettingsBuilder {
	return &ScopeSettingsBuilder{
		settings: &ScopeSettings{},
		logger:   logger,
	}
}

// WithHostnames sets allowed and disallowed hostnames
func (sb *ScopeSettingsBuilder) WithHostnames(allowed, disallowed []string) *ScopeSettingsBuilder {
	sb.settings.allowedHostnames = allowed
	sb.settings.disallowedHostnames = disallowed
	return sb
}

// WithSubdomains sets allowed and disallowed subdomains
func (sb *ScopeSettingsBuilder) WithSubdomains(allowed, disallowed []string) *ScopeSettingsBuilder {
	sb.settings.allowedSubdomains = allowed
	sb.settings.disallowedSubdomains = disallowed
	return sb
}

// WithPathRegexes sets allowed and disallowed path regex patterns
func (sb *ScopeSettingsBuilder) WithPathRegexes(allowed, disallowed []string) *ScopeSettingsBuilder {
	sb.settings.allowedPathPatterns = common.CompileRegexes(allowed, sb.logger)
	sb.settings.disallowedPathPatterns = common.CompileRegexes(disallowed, sb.logger)
	return sb
}

// WithSubdomainPolicy sets the subdomain inclusion policy
func (sb *ScopeSettingsBuilder) WithSubdomainPolicy(includeSubdomains bool) *ScopeSettingsBuilder {
	sb.settings.includeSubdomains = includeSubdomains
	return sb
}

// WithSeedURLs sets the original seed URLs for domain extraction
func (sb *ScopeSettingsBuilder) WithSeedURLs(seedURLs []string) *ScopeSettingsBuilder {
	if sb.settings.includeSubdomains {
		sb.settings.originalSeedDomains = sb.extractSeedDomains(seedURLs)
	}
	return sb
}

// extractSeedDomains extracts base domains from seed URLs
func (sb *ScopeSettingsBuilder) extractSeedDomains(seedURLs []string) []string {
	var domains []string
	for _, seedURL := range seedURLs {
		if hostnames := ExtractHostnamesFromSeedURLs([]string{seedURL}, sb.logger); len(hostnames) > 0 {
			domains = append(domains, hostnames[0])
		}
	}

	uniqueDomains := removeDuplicates(domains)
	sb.logger.Debug().Strs("original_seed_domains", uniqueDomains).Msg("Extracted base domains from seed URLs")

	return uniqueDomains
}

// Build creates the final ScopeSettings instance
func (sb *ScopeSettingsBuilder) Build() (*ScopeSettings, error) {
	sb.settings.logger = sb.logger

	sb.logConfiguration()
	return sb.settings, nil
}

// logConfiguration logs the scope configuration
func (sb *ScopeSettingsBuilder) logConfiguration() {
	sb.logger.Info().
		Strs("allowed_hostnames", sb.settings.allowedHostnames).
		Strs("disallowed_hostnames", sb.settings.disallowedHostnames).
		Strs("allowed_subdomains", sb.settings.allowedSubdomains).
		Strs("disallowed_subdomains", sb.settings.disallowedSubdomains).
		Int("allowed_path_patterns", len(sb.settings.allowedPathPatterns)).
		Int("disallowed_path_patterns", len(sb.settings.disallowedPathPatterns)).
		Bool("include_subdomains", sb.settings.includeSubdomains).
		Strs("original_seed_domains", sb.settings.originalSeedDomains).
		Msg("ScopeSettings initialized")
}

// CheckHostnameScope checks if a given hostname is within the defined scope
func (s *ScopeSettings) CheckHostnameScope(hostname string) bool {
	validator := NewHostnameValidator(s)
	return validator.IsAllowed(hostname)
}

// HostnameValidator handles hostname scope validation logic
type HostnameValidator struct {
	settings *ScopeSettings
}

// NewHostnameValidator creates a new HostnameValidator instance
func NewHostnameValidator(settings *ScopeSettings) *HostnameValidator {
	return &HostnameValidator{
		settings: settings,
	}
}

// IsAllowed checks if hostname is allowed based on scope rules
func (hv *HostnameValidator) IsAllowed(hostname string) bool {
	// Priority: disallowed > specific allowed > subdomain policies > default allow

	if hv.isExplicitlyDisallowed(hostname) {
		return false
	}

	if hv.isDisallowedSubdomain(hostname) {
		return false
	}

	if hv.settings.includeSubdomains && hv.isAllowedBySeedDomains(hostname) {
		return true
	}

	if len(hv.settings.allowedHostnames) > 0 {
		return hv.isInAllowedHostnames(hostname)
	}

	hv.settings.logger.Debug().Str("hostname", hostname).Msg("Hostname allowed by default")
	return true
}

// isExplicitlyDisallowed checks if hostname is explicitly disallowed
func (hv *HostnameValidator) isExplicitlyDisallowed(hostname string) bool {
	if containsString(hostname, hv.settings.disallowedHostnames) {
		hv.settings.logger.Debug().Str("hostname", hostname).Msg("Hostname explicitly disallowed")
		return true
	}
	return false
}

// isDisallowedSubdomain checks if hostname is a disallowed subdomain
func (hv *HostnameValidator) isDisallowedSubdomain(hostname string) bool {
	// Check if hostname is subdomain of disallowed hostname
	for _, disallowedHost := range hv.settings.disallowedHostnames {
		if hv.isSubdomainOf(hostname, disallowedHost) {
			hv.settings.logger.Debug().
				Str("hostname", hostname).
				Str("disallowed_base", disallowedHost).
				Msg("Hostname is subdomain of disallowed host")
			return true
		}
	}

	// Check subdomain parts against disallowed subdomains
	return hv.hasDisallowedSubdomainPart(hostname)
}

// isSubdomainOf checks if hostname is a subdomain of baseHostname
func (hv *HostnameValidator) isSubdomainOf(hostname, baseHostname string) bool {
	return strings.HasSuffix(hostname, "."+baseHostname) && hostname != baseHostname
}

// hasDisallowedSubdomainPart checks if hostname has disallowed subdomain parts
func (hv *HostnameValidator) hasDisallowedSubdomainPart(hostname string) bool {
	hostnameBase, err := urlhandler.GetBaseDomain(hostname)
	if err != nil || hostnameBase == "" {
		return false
	}

	if !hv.isBaseAllowed(hostnameBase, hostname) {
		return false
	}

	if hostname == hostnameBase {
		return false
	}

	subdomainPart := strings.TrimSuffix(hostname, "."+hostnameBase)
	if subdomainPart == hostname || containsString(subdomainPart, hv.settings.disallowedSubdomains) {
		hv.settings.logger.Debug().
			Str("hostname", hostname).
			Str("subdomain_part", subdomainPart).
			Msg("Subdomain part is disallowed")
		return true
	}

	return false
}

// isBaseAllowed checks if the base domain is allowed
func (hv *HostnameValidator) isBaseAllowed(hostnameBase, originalHostname string) bool {
	// Check if base is in allowed hostnames
	if containsString(hostnameBase, hv.settings.allowedHostnames) {
		return true
	}

	// Check if base is in original seed domains (when includeSubdomains is true)
	if hv.settings.includeSubdomains && containsString(hostnameBase, hv.settings.originalSeedDomains) {
		return true
	}

	// Check if original hostname has an allowed base in allowed hostnames
	for _, allowedHost := range hv.settings.allowedHostnames {
		if hostnameBase == allowedHost {
			return true
		}
	}

	return false
}

// isAllowedBySeedDomains checks if hostname is allowed by seed domain policy
func (hv *HostnameValidator) isAllowedBySeedDomains(hostname string) bool {
	for _, seedBaseDomain := range hv.settings.originalSeedDomains {
		if hv.matchesSeedDomain(hostname, seedBaseDomain) {
			return true
		}
	}
	return false
}

// matchesSeedDomain checks if hostname matches or is subdomain of seed domain
func (hv *HostnameValidator) matchesSeedDomain(hostname, seedBaseDomain string) bool {
	if hostname == seedBaseDomain {
		return true
	}

	if !strings.HasSuffix(hostname, "."+seedBaseDomain) {
		return false
	}

	// Check if specific subdomain is disallowed
	subdomainPart := strings.TrimSuffix(hostname, "."+seedBaseDomain)
	if containsString(subdomainPart, hv.settings.disallowedSubdomains) {
		hv.settings.logger.Debug().
			Str("hostname", hostname).
			Str("seed_base", seedBaseDomain).
			Str("subdomain_part", subdomainPart).
			Msg("Allowed by seed domains, but subdomain part is disallowed")
		return false
	}

	hv.settings.logger.Debug().
		Str("hostname", hostname).
		Str("seed_base_domain", seedBaseDomain).
		Msg("Hostname allowed by seed domain policy")
	return true
}

// isInAllowedHostnames checks if hostname is in allowed hostnames list
func (hv *HostnameValidator) isInAllowedHostnames(hostname string) bool {
	// Check exact match
	if containsString(hostname, hv.settings.allowedHostnames) {
		hv.settings.logger.Debug().Str("hostname", hostname).Msg("Hostname explicitly allowed")
		return true
	}

	// Check if it's allowed subdomain
	return hv.isAllowedSubdomainOfAllowedHost(hostname)
}

// isAllowedSubdomainOfAllowedHost checks if hostname is allowed subdomain of explicitly allowed host
func (hv *HostnameValidator) isAllowedSubdomainOfAllowedHost(hostname string) bool {
	for _, allowedHost := range hv.settings.allowedHostnames {
		if hv.isAllowedSubdomainOf(hostname, allowedHost) {
			return true
		}
	}

	hv.settings.logger.Debug().
		Str("hostname", hostname).
		Strs("allowed_hostnames", hv.settings.allowedHostnames).
		Msg("Hostname not in allowed list")
	return false
}

// isAllowedSubdomainOf checks if hostname is an allowed subdomain of baseHost
func (hv *HostnameValidator) isAllowedSubdomainOf(hostname, baseHost string) bool {
	if !hv.isSubdomainOf(hostname, baseHost) {
		return false
	}

	subdomainPart := strings.TrimSuffix(hostname, "."+baseHost)
	if subdomainPart == "" || subdomainPart == hostname {
		return false
	}

	if !containsString(subdomainPart, hv.settings.allowedSubdomains) {
		return false
	}

	if containsString(subdomainPart, hv.settings.disallowedSubdomains) {
		hv.settings.logger.Debug().
			Str("hostname", hostname).
			Str("base", baseHost).
			Str("subdomain_part", subdomainPart).
			Msg("Allowed subdomain, but also in disallowed list")
		return false
	}

	hv.settings.logger.Debug().
		Str("hostname", hostname).
		Str("base", baseHost).
		Str("subdomain_part", subdomainPart).
		Msg("Hostname is allowed subdomain of allowed host")
	return true
}

// checkPathScope checks if a given URL path is within the defined scope using regex patterns
func (s *ScopeSettings) checkPathScope(path string) bool {
	pathValidator := NewPathValidator(s)
	return pathValidator.IsAllowed(path)
}

// PathValidator handles path scope validation logic
type PathValidator struct {
	settings *ScopeSettings
}

// NewPathValidator creates a new PathValidator instance
func NewPathValidator(settings *ScopeSettings) *PathValidator {
	return &PathValidator{
		settings: settings,
	}
}

// IsAllowed checks if path is allowed based on regex patterns
func (pv *PathValidator) IsAllowed(path string) bool {
	// Check disallowed patterns first
	if pv.matchesDisallowedPattern(path) {
		return false
	}

	// If allowed patterns exist, path must match at least one
	if len(pv.settings.allowedPathPatterns) > 0 {
		return pv.matchesAllowedPattern(path)
	}

	pv.settings.logger.Debug().Str("path", path).Msg("Path allowed by default")
	return true
}

// matchesDisallowedPattern checks if path matches any disallowed pattern
func (pv *PathValidator) matchesDisallowedPattern(path string) bool {
	for _, regex := range pv.settings.disallowedPathPatterns {
		if regex.MatchString(path) {
			pv.settings.logger.Debug().
				Str("path", path).
				Str("regex", regex.String()).
				Msg("Path matches disallowed pattern")
			return true
		}
	}
	return false
}

// matchesAllowedPattern checks if path matches any allowed pattern
func (pv *PathValidator) matchesAllowedPattern(path string) bool {
	for _, regex := range pv.settings.allowedPathPatterns {
		if regex.MatchString(path) {
			pv.settings.logger.Debug().
				Str("path", path).
				Str("regex", regex.String()).
				Msg("Path matches allowed pattern")
			return true
		}
	}

	pv.settings.logger.Debug().Str("path", path).Msg("Path does not match any allowed pattern")
	return false
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
	parsed, err := url.Parse(seedURL)
	if err != nil {
		logger.Warn().Str("seed_url", seedURL).Err(err).Msg("Failed to parse seed URL")
		return ""
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		logger.Warn().Str("seed_url", seedURL).Msg("Empty hostname in seed URL")
		return ""
	}

	return hostname
}

// MergeAllowedHostnames merges existing hostnames with seed hostnames, removing duplicates
func MergeAllowedHostnames(existingHostnames, seedHostnames []string) []string {
	merged := append(existingHostnames, seedHostnames...)
	return removeDuplicates(merged)
}
