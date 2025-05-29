package crawler

import (
	"errors"
	"fmt" // For logging regex compilation errors
	"net/url"
	"regexp" // For path restriction logic
	"strings"

	// "log" // For debugging scope decisions later

	"github.com/rs/zerolog"
)

// ScopeSettings defines the rules for what URLs the crawler is allowed to visit.
// Task 2.1: Define structures for hostname and subdomain control.
type ScopeSettings struct {
	AllowedHostnames     []string // If empty, any hostname is allowed (unless disallowed).
	AllowedSubdomains    []string // Only effective if AllowedHostnames is also set. If empty, any subdomain of an allowed hostname is permitted.
	DisallowedHostnames  []string // Specific hostnames to never visit.
	DisallowedSubdomains []string // Specific subdomains to never visit.

	AllowedPathPatterns    []*regexp.Regexp // Task 2.2: Regex for allowed paths
	DisallowedPathPatterns []*regexp.Regexp // Task 2.2: Regex for disallowed paths
	logger                 zerolog.Logger   // Added logger
}

// NewScopeSettings creates a new ScopeSettings with provided rules.
// For now, these are passed directly. Later, this will come from config.
func NewScopeSettings(
	allowedHostnames, allowedSubdomains,
	disallowedHostnames, disallowedSubdomains,
	allowedPathRegexes, disallowedPathRegexes []string, // Task 2.2: Added path regexes
	logger zerolog.Logger, // Added logger parameter
) *ScopeSettings {
	scopeLogger := logger.With().Str("component", "ScopeSettings").Logger()

	normalize := func(items []string) []string {
		normalized := make([]string, len(items))
		for i, item := range items {
			normalized[i] = strings.ToLower(strings.TrimSpace(item))
		}
		return normalized
	}

	ss := &ScopeSettings{
		AllowedHostnames:     normalize(allowedHostnames),
		AllowedSubdomains:    normalize(allowedSubdomains),
		DisallowedHostnames:  normalize(disallowedHostnames),
		DisallowedSubdomains: normalize(disallowedSubdomains),
		logger:               scopeLogger,
	}

	compileRegexes := func(patterns []string) []*regexp.Regexp {
		compiled := make([]*regexp.Regexp, 0, len(patterns))
		for _, pattern := range patterns {
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				ss.logger.Error().Err(err).Str("regex_pattern", pattern).Msg("Failed to compile regex. Skipping pattern.")
				continue
			}
			compiled = append(compiled, re)
		}
		return compiled
	}

	ss.AllowedPathPatterns = compileRegexes(allowedPathRegexes)
	ss.DisallowedPathPatterns = compileRegexes(disallowedPathRegexes)

	return ss
}

// CheckHostnameScope evaluates if the given hostname is within the configured scope
// based on AllowedHostnames, AllowedSubdomains, DisallowedHostnames, and DisallowedSubdomains.
// Task 2.1: Implement hostname and subdomain control logic.
func (ss *ScopeSettings) CheckHostnameScope(hostname string) bool {
	if hostname == "" {
		return false // Cannot determine scope for empty hostname
	}
	normalizedHostname := strings.ToLower(strings.TrimSpace(hostname))

	// 1. Check DisallowedHostnames (highest precedence for direct disallow)
	for _, disallowedHost := range ss.DisallowedHostnames {
		if normalizedHostname == disallowedHost {
			return false
		}
	}

	// 2. Check DisallowedSubdomains
	for _, disallowedSubdomain := range ss.DisallowedSubdomains {
		if strings.HasSuffix(normalizedHostname, "."+disallowedSubdomain) || normalizedHostname == disallowedSubdomain {
			return false
		}
	}

	// 3. If AllowedHostnames is empty, all hostnames that are not disallowed are implicitly allowed.
	if len(ss.AllowedHostnames) == 0 {
		return true
	}

	// 4. Check AllowedHostnames and AllowedSubdomains
	for _, allowedHost := range ss.AllowedHostnames {
		if normalizedHostname == allowedHost {
			return true
		}
		// Check if it's an allowed subdomain for this specific allowedHost
		if (strings.HasSuffix(normalizedHostname, "."+allowedHost)) && (len(ss.AllowedSubdomains) == 0 || isStringInSlice(allowedHost, ss.AllowedSubdomains)) {
			return true
		}
	}

	return false
}

// checkPathScope evaluates if the given URL path is within the configured path regexes.
// Task 2.2: Implement path restriction logic.
func (ss *ScopeSettings) checkPathScope(path string) bool {
	// 1. Check DisallowedPathPatterns
	for _, re := range ss.DisallowedPathPatterns {
		if re.MatchString(path) {
			return false
		}
	}

	// 2. If AllowedPathPatterns is empty, all paths not disallowed are implicitly allowed.
	if len(ss.AllowedPathPatterns) == 0 {
		return true
	}

	// 3. Check AllowedPathPatterns
	for _, re := range ss.AllowedPathPatterns {
		if re.MatchString(path) {
			return true
		}
	}

	return false
}

// IsURLAllowed determines if a given URL string is within the defined crawling scope.
// This method will be added to the Crawler struct later and will use ScopeSettings.
// For now, this is a standalone helper that would be called by the Crawler.
// It primarily uses CheckHostnameScope for Task 2.1.
func (ss *ScopeSettings) IsURLAllowed(urlString string) (bool, error) {
	if strings.TrimSpace(urlString) == "" {
		return false, errors.New("URL string is empty")
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false, fmt.Errorf("could not parse URL '%s': %w", urlString, err)
	}

	if !parsedURL.IsAbs() {
		// Depending on policy, relative URLs might be considered in scope if their base is,
		// but for a direct check, we need an absolute URL.
		// For now, let's say non-absolute URLs need resolution first.
		return false, errors.New("URL is not absolute, cannot check hostname scope directly")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		// This can happen for URLs like "file:///path/to/file" or malformed ones
		return false, errors.New("URL has no hostname component")
	}

	// 1. Check hostname scope
	if !ss.CheckHostnameScope(hostname) {
		return false, nil // Hostname not allowed
	}

	// 2. Check path scope (Task 2.2)
	// The path component for regex matching typically includes the leading slash.
	// Example: for "http://example.com/foo/bar?q=1", path is "/foo/bar"
	// url.URL.Path gives the path part. url.URL.RequestURI() includes path and query. PRD implies path.
	path := parsedURL.Path
	if path == "" { // For URLs like "http://example.com"
		path = "/"
	}

	if !ss.checkPathScope(path) {
		return false, nil // Path not allowed
	}

	// If all checks pass
	return true, nil
}

// isStringInSlice checks if a string is present in a slice of strings.
func isStringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// ExtractHostnamesFromSeedURLs extracts unique hostnames from a list of seed URLs
func ExtractHostnamesFromSeedURLs(seedURLs []string, logger zerolog.Logger) []string {
	hostnameSet := make(map[string]bool)

	for _, seedURL := range seedURLs {
		if strings.TrimSpace(seedURL) == "" {
			continue
		}

		parsedURL, err := url.Parse(seedURL)
		if err != nil {
			logger.Warn().Str("seed_url", seedURL).Err(err).Msg("Failed to parse seed URL for hostname extraction")
			continue
		}

		hostname := parsedURL.Hostname()
		if hostname == "" {
			logger.Warn().Str("seed_url", seedURL).Msg("Seed URL has no hostname component")
			continue
		}

		// Normalize hostname to lowercase
		normalizedHostname := strings.ToLower(strings.TrimSpace(hostname))
		hostnameSet[normalizedHostname] = true
	}

	// Convert map to slice
	hostnames := make([]string, 0, len(hostnameSet))
	for hostname := range hostnameSet {
		hostnames = append(hostnames, hostname)
	}

	return hostnames
}

// MergeAllowedHostnames merges extracted seed hostnames with existing allowed hostnames
func MergeAllowedHostnames(existingHostnames, seedHostnames []string) []string {
	hostnameSet := make(map[string]bool)

	// Add existing hostnames
	for _, hostname := range existingHostnames {
		normalizedHostname := strings.ToLower(strings.TrimSpace(hostname))
		if normalizedHostname != "" {
			hostnameSet[normalizedHostname] = true
		}
	}

	// Add seed hostnames
	for _, hostname := range seedHostnames {
		normalizedHostname := strings.ToLower(strings.TrimSpace(hostname))
		if normalizedHostname != "" {
			hostnameSet[normalizedHostname] = true
		}
	}

	// Convert back to slice
	mergedHostnames := make([]string, 0, len(hostnameSet))
	for hostname := range hostnameSet {
		mergedHostnames = append(mergedHostnames, hostname)
	}

	return mergedHostnames
}
