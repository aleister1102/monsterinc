package crawler

import (
	"errors"
	"fmt"
	"log" // For logging regex compilation errors
	"net/url"
	"regexp" // For path restriction logic
	"strings"
	// "log" // For debugging scope decisions later
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
	// TODO: Add Robots.txt handling (Task 2.3)
}

// NewScopeSettings creates a new ScopeSettings with provided rules.
// For now, these are passed directly. Later, this will come from config.
func NewScopeSettings(
	allowedHostnames, allowedSubdomains,
	disallowedHostnames, disallowedSubdomains,
	allowedPathRegexes, disallowedPathRegexes []string, // Task 2.2: Added path regexes
) *ScopeSettings {
	// Normalize all inputs to lowercase for case-insensitive matching
	normalize := func(items []string) []string {
		normalized := make([]string, len(items))
		for i, item := range items {
			normalized[i] = strings.ToLower(strings.TrimSpace(item))
		}
		return normalized
	}

	compileRegexes := func(patterns []string) []*regexp.Regexp {
		compiled := make([]*regexp.Regexp, 0, len(patterns))
		for _, pattern := range patterns {
			if strings.TrimSpace(pattern) == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				log.Printf("[ERROR] ScopeSettings: Failed to compile regex pattern '%s': %v. Skipping pattern.", pattern, err)
				continue
			}
			compiled = append(compiled, re)
		}
		return compiled
	}

	return &ScopeSettings{
		AllowedHostnames:       normalize(allowedHostnames),
		AllowedSubdomains:      normalize(allowedSubdomains),
		DisallowedHostnames:    normalize(disallowedHostnames),
		DisallowedSubdomains:   normalize(disallowedSubdomains),
		AllowedPathPatterns:    compileRegexes(allowedPathRegexes),    // Task 2.2
		DisallowedPathPatterns: compileRegexes(disallowedPathRegexes), // Task 2.2
	}
}

// isDomainOrSubdomain checks if `domain` is equal to `baseDomain` or is a subdomain of `baseDomain`.
// Both inputs should be normalized (e.g., lowercase).
func isDomainOrSubdomain(domain, baseDomain string) bool {
	if domain == baseDomain {
		return true // Exact match
	}
	// Check for subdomain: domain must end with ".baseDomain"
	return strings.HasSuffix(domain, "."+baseDomain)
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
			// log.Printf("[DEBUG] Scope: Hostname '%s' disallowed by DisallowedHostnames.", hostname)
			return false
		}
	}

	// 2. Check DisallowedSubdomains
	for _, disallowedSubdomain := range ss.DisallowedSubdomains {
		// This checks if normalizedHostname is "disallowedSubdomain" or "anything.disallowedSubdomain"
		// Example: if "internal.example.com" is disallowed, then "internal.example.com" and "api.internal.example.com" are out.
		if isDomainOrSubdomain(normalizedHostname, disallowedSubdomain) {
			// log.Printf("[DEBUG] Scope: Hostname '%s' disallowed by DisallowedSubdomains ('%s').", hostname, disallowedSubdomain)
			return false
		}
	}

	// 3. Check AllowedHostnames
	// If AllowedHostnames is not set, any hostname is potentially allowed (subject to disallows which were already checked).
	if len(ss.AllowedHostnames) == 0 {
		// log.Printf("[DEBUG] Scope: Hostname '%s' allowed (no AllowedHostnames defined, passed disallows).", hostname)
		return true // Allowed by default if no specific allowed hostnames, and not disallowed.
	}

	// If AllowedHostnames is set, the hostname must match one of them or be a permitted subdomain.
	for _, allowedHost := range ss.AllowedHostnames {
		if normalizedHostname == allowedHost { // Exact match for an allowed hostname
			// log.Printf("[DEBUG] Scope: Hostname '%s' allowed by AllowedHostnames.", hostname)
			return true
		}

		// Check for subdomain allowance if it's a subdomain of an allowedHost.
		// AllowedSubdomains list refines this. If AllowedSubdomains is empty, any subdomain is fine.
		// If AllowedSubdomains is *not* empty, it must be one of *those* specific subdomains.
		if strings.HasSuffix(normalizedHostname, "."+allowedHost) { // It's a potential subdomain
			if len(ss.AllowedSubdomains) == 0 {
				// No specific subdomains listed, so any subdomain of allowedHost is fine.
				// log.Printf("[DEBUG] Scope: Hostname '%s' (subdomain of '%s') allowed (no specific AllowedSubdomains).", hostname, allowedHost)
				return true
			}
			// Specific AllowedSubdomains are listed. Check if this hostname matches one.
			// The entries in AllowedSubdomains are full hostnames, e.g., "sub.example.com".
			for _, allowedSub := range ss.AllowedSubdomains {
				if normalizedHostname == allowedSub {
					// log.Printf("[DEBUG] Scope: Hostname '%s' allowed by specific AllowedSubdomains.", hostname)
					return true
				}
			}
			// It's a subdomain of an allowedHost, but not in the specific AllowedSubdomains list. So, not allowed.
			// log.Printf("[DEBUG] Scope: Hostname '%s' (subdomain of '%s') not in specific AllowedSubdomains list.", hostname, allowedHost)
			// Continue checking other AllowedHostnames. This specific one didn't grant permission via its subdomains.
		}
	}

	// If we went through all allowed hostnames and found no match (direct or via subdomain rules)
	// log.Printf("[DEBUG] Scope: Hostname '%s' not allowed (did not match any AllowedHostnames or their subdomain rules).", hostname)
	return false
}

// checkPathScope evaluates if the given URL path is within the configured path regexes.
// Task 2.2: Implement path restriction logic.
func (ss *ScopeSettings) checkPathScope(path string) bool {
	// 1. Check DisallowedPathPatterns first
	for _, re := range ss.DisallowedPathPatterns {
		if re.MatchString(path) {
			// log.Printf("[DEBUG] Scope: Path '%s' disallowed by pattern '%s'.", path, re.String())
			return false
		}
	}

	// 2. If AllowedPathPatterns is defined, path must match at least one.
	// If AllowedPathPatterns is empty, any path is allowed (if not disallowed).
	if len(ss.AllowedPathPatterns) > 0 {
		pathIsAllowed := false
		for _, re := range ss.AllowedPathPatterns {
			if re.MatchString(path) {
				// log.Printf("[DEBUG] Scope: Path '%s' allowed by pattern '%s'.", path, re.String())
				pathIsAllowed = true
				break
			}
		}
		if !pathIsAllowed {
			// log.Printf("[DEBUG] Scope: Path '%s' did not match any AllowedPathPatterns.", path)
			return false // Not explicitly allowed, and allowed patterns are defined.
		}
	}
	// If we reach here, either:
	// - No disallowed patterns matched.
	// - And ( (AllowedPathPatterns is empty) OR (path matched an allowed pattern) )
	// log.Printf("[DEBUG] Scope: Path '%s' allowed by path scope rules.", path)
	return true
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
		// log.Printf("[DEBUG] Scope: URL '%s' is not absolute, cannot determine hostname scope accurately without resolving.", urlString)
		// Depending on policy, relative URLs might be considered in scope if their base is,
		// but for a direct check, we need an absolute URL.
		// For now, let's say non-absolute URLs need resolution first.
		return false, errors.New("URL is not absolute, cannot check hostname scope directly")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		// This can happen for URLs like "file:///path/to/file" or malformed ones
		// log.Printf("[DEBUG] Scope: URL '%s' has no hostname component.", urlString)
		return false, errors.New("URL has no hostname component")
	}

	// 1. Check hostname scope
	if !ss.CheckHostnameScope(hostname) {
		// log.Printf("[DEBUG] Scope: URL '%s' (hostname '%s') failed hostname scope.", urlString, hostname)
		return false, nil // Hostname not allowed
	}
	// log.Printf("[DEBUG] Scope: URL '%s' (hostname '%s') passed hostname scope.", urlString, hostname)

	// 2. Check path scope (Task 2.2)
	// The path component for regex matching typically includes the leading slash.
	// Example: for "http://example.com/foo/bar?q=1", path is "/foo/bar"
	// url.URL.Path gives the path part. url.URL.RequestURI() includes path and query. PRD implies path.
	path := parsedURL.Path
	if path == "" { // For URLs like "http://example.com"
		path = "/"
	}

	if !ss.checkPathScope(path) {
		// log.Printf("[DEBUG] Scope: URL '%s' (path '%s') failed path scope.", urlString, path)
		return false, nil // Path not allowed
	}
	// log.Printf("[DEBUG] Scope: URL '%s' (path '%s') passed path scope.", urlString, path)

	// If all checks pass
	return true, nil
}
