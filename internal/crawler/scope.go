package crawler

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// ScopeSettings defines the rules for what URLs the crawler should visit.
type ScopeSettings struct {
	AllowedHostnames       []string
	AllowedSubdomains      []string
	DisallowedHostnames    []string
	DisallowedSubdomains   []string
	AllowedPathPatterns    []*regexp.Regexp
	DisallowedPathPatterns []*regexp.Regexp
	logger                 zerolog.Logger

	// New fields for subdomain handling based on original seeds
	IncludeSubdomains   bool
	OriginalSeedDomains []string // Stores base domains of original seeds
}

// NewScopeSettings creates a new ScopeSettings instance with compiled regex patterns
func NewScopeSettings(
	rootURLHostname string, // This is typically the hostname of the first seed URL
	allowedHostnames, disallowedHostnames []string,
	allowedSubdomains, disallowedSubdomains []string,
	allowedPathRegexes, disallowedPathRegexes []string,
	logger zerolog.Logger,
	includeSubdomains bool, // New parameter
	originalSeedURLs []string, // New parameter
) (*ScopeSettings, error) {
	scopeLogger := logger.With().Str("component", "ScopeSettings").Logger()

	// Extract base domains from original seed URLs for subdomain checking
	var originalSeedDomains []string
	if includeSubdomains {
		for _, seedURL := range originalSeedURLs {
			if hostname := ExtractHostnamesFromSeedURLs([]string{seedURL}, scopeLogger); len(hostname) > 0 {
				originalSeedDomains = append(originalSeedDomains, hostname[0])
			}
		}
		originalSeedDomains = unique(originalSeedDomains)
		scopeLogger.Debug().Strs("original_seed_domains", originalSeedDomains).Msg("Extracted base domains from seed URLs")
	}

	// Compile regex patterns using common utility
	allowedPathPatterns := common.CompileRegexes(allowedPathRegexes, scopeLogger)
	disallowedPathPatterns := common.CompileRegexes(disallowedPathRegexes, scopeLogger)

	scopeSettings := &ScopeSettings{
		AllowedHostnames:       allowedHostnames,
		AllowedSubdomains:      allowedSubdomains,
		DisallowedHostnames:    disallowedHostnames,
		DisallowedSubdomains:   disallowedSubdomains,
		AllowedPathPatterns:    allowedPathPatterns,
		DisallowedPathPatterns: disallowedPathPatterns,
		logger:                 scopeLogger,
		IncludeSubdomains:      includeSubdomains,
		OriginalSeedDomains:    originalSeedDomains,
	}

	scopeLogger.Info().
		Strs("allowed_hostnames", allowedHostnames).
		Strs("disallowed_hostnames", disallowedHostnames).
		Strs("allowed_subdomains", allowedSubdomains).
		Strs("disallowed_subdomains", disallowedSubdomains).
		Int("allowed_path_patterns", len(allowedPathPatterns)).
		Int("disallowed_path_patterns", len(disallowedPathPatterns)).
		Bool("include_subdomains", includeSubdomains).
		Strs("original_seed_domains", originalSeedDomains).
		Msg("ScopeSettings initialized")

	return scopeSettings, nil
}

// CheckHostnameScope checks if a given hostname is within the defined scope.
// It prioritizes disallowed lists, then checks allowed lists and subdomain rules.
func (s *ScopeSettings) CheckHostnameScope(hostname string) bool {
	// 1. Check DisallowedHostnames: Exact matches for disallowed hostnames.
	if isStringInSlice(hostname, s.DisallowedHostnames) {
		s.logger.Debug().Str("hostname", hostname).Msg("Hostname explicitly disallowed.")
		return false
	}

	// 2. Check DisallowedSubdomains:
	//    A hostname is disallowed if it's a subdomain of a DisallowedHostname,
	//    OR if its specific subdomain part is listed in DisallowedSubdomains for a matching base domain.
	for _, disallowedHost := range s.DisallowedHostnames {
		if strings.HasSuffix(hostname, "."+disallowedHost) && hostname != disallowedHost {
			s.logger.Debug().Str("hostname", hostname).Str("disallowed_base", disallowedHost).Msg("Hostname is a subdomain of an explicitly disallowed host.")
			return false
		}
	}

	hostnameBase, err := urlhandler.GetBaseDomain(hostname)
	if err == nil && hostnameBase != "" {
		isPotentiallyAllowedBase := isStringInSlice(hostnameBase, s.AllowedHostnames) || (s.IncludeSubdomains && isStringInSlice(hostnameBase, s.OriginalSeedDomains))
		// If hostname is a subdomain of an explicitly allowed host, it's also potentially allowed based on its base.
		if !isPotentiallyAllowedBase && hostname != hostnameBase {
			for _, allowedHost := range s.AllowedHostnames {
				if hostnameBase == allowedHost { // hostname = sub.example.com, hostnameBase = example.com, allowedHost = example.com
					isPotentiallyAllowedBase = true
					break
				}
			}
		}

		if isPotentiallyAllowedBase {
			subdomainPart := strings.TrimSuffix(hostname, "."+hostnameBase)
			if hostname != hostnameBase && subdomainPart != hostname && isStringInSlice(subdomainPart, s.DisallowedSubdomains) {
				s.logger.Debug().Str("hostname", hostname).Str("subdomain_part", subdomainPart).Msg("Subdomain part is in DisallowedSubdomains for an otherwise allowed base domain.")
				return false
			}
		}
	}

	// 3. If IncludeSubdomains is true, check against OriginalSeedDomains
	if s.IncludeSubdomains {
		for _, seedBaseDomain := range s.OriginalSeedDomains {
			if hostname == seedBaseDomain || strings.HasSuffix(hostname, "."+seedBaseDomain) {
				// Check if this specific subdomain is disallowed
				if hostname != seedBaseDomain {
					subdomainPart := strings.TrimSuffix(hostname, "."+seedBaseDomain)
					if isStringInSlice(subdomainPart, s.DisallowedSubdomains) {
						s.logger.Debug().Str("hostname", hostname).Str("seed_base", seedBaseDomain).Str("subdomain_part", subdomainPart).Msg("Allowed by IncludeSubdomains, but specific subdomain part is disallowed.")
						return false
					}
				}
				s.logger.Debug().Str("hostname", hostname).Str("seed_base_domain", seedBaseDomain).Msg("Hostname allowed by IncludeSubdomains policy.")
				return true
			}
		}
	}

	// 4. Check AllowedHostnames: If this list is populated, the hostname MUST be in it or be an allowed subdomain of one of its entries.
	if len(s.AllowedHostnames) > 0 {
		if isStringInSlice(hostname, s.AllowedHostnames) {
			s.logger.Debug().Str("hostname", hostname).Msg("Hostname explicitly allowed.")
			return true
		}
		// Check if it's an allowed subdomain of an explicitly AllowedHostname
		for _, allowedHost := range s.AllowedHostnames {
			if strings.HasSuffix(hostname, "."+allowedHost) && hostname != allowedHost {
				subdomainPart := strings.TrimSuffix(hostname, "."+allowedHost)
				if subdomainPart != "" && subdomainPart != hostname { // Ensure it's a real subdomain part
					if isStringInSlice(subdomainPart, s.AllowedSubdomains) {
						// Check if this specific subdomain is also in DisallowedSubdomains
						if isStringInSlice(subdomainPart, s.DisallowedSubdomains) {
							s.logger.Debug().Str("hostname", hostname).Str("base", allowedHost).Str("subdomain_part", subdomainPart).Msg("Allowed subdomain, but also in DisallowedSubdomains list.")
							return false
						}
						s.logger.Debug().Str("hostname", hostname).Str("base", allowedHost).Str("subdomain_part", subdomainPart).Msg("Hostname is an allowed subdomain of an explicitly allowed host.")
						return true
					}
				}
			}
		}
		// If AllowedHostnames is populated and hostname didn't match any rule above, it's disallowed.
		s.logger.Debug().Str("hostname", hostname).Strs("allowed_hostnames", s.AllowedHostnames).Msg("Hostname not in allowed list and not an allowed subdomain (when AllowedHostnames is restrictive).")
		return false
	}

	// 5. Default allow: If no AllowedHostnames are specified (list is empty), and not disallowed by previous rules.
	s.logger.Debug().Str("hostname", hostname).Msg("Hostname allowed by default (no specific allow/disallow host rule matched, and AllowedHostnames is not restrictive).")
	return true
}

// checkPathScope checks if a given URL path is within the defined scope using regex patterns.
func (s *ScopeSettings) checkPathScope(path string) bool {
	// Check disallowed patterns first
	for _, re := range s.DisallowedPathPatterns {
		if re.MatchString(path) {
			s.logger.Debug().Str("path", path).Str("regex", re.String()).Msg("Path matches disallowed regex pattern.")
			return false // Path is disallowed
		}
	}

	// If allowed patterns are defined, path must match at least one
	if len(s.AllowedPathPatterns) > 0 {
		for _, re := range s.AllowedPathPatterns {
			if re.MatchString(path) {
				s.logger.Debug().Str("path", path).Str("regex", re.String()).Msg("Path matches allowed regex pattern.")
				return true // Path is allowed
			}
		}
		s.logger.Debug().Str("path", path).Msg("Path does not match any allowed regex pattern (when allowed patterns are defined).")
		return false // Path is not in the allowed list
	}

	// Default: If no allowed patterns, allow (unless disallowed)
	s.logger.Debug().Str("path", path).Msg("Path allowed by default (no specific path patterns matched or defined).")
	return true
}

// IsURLAllowed checks if a given URL string is within the defined scope.
func (s *ScopeSettings) IsURLAllowed(urlString string) (bool, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(urlString))
	if err != nil {
		s.logger.Warn().Str("url", urlString).Err(err).Msg("Failed to parse URL for scope check.")
		return false, err
	}

	hostname := parsedURL.Hostname()
	if !s.CheckHostnameScope(hostname) {
		// s.logger.Debug().Str("url", urlString).Str("hostname", hostname).Msg("URL disallowed by hostname scope.")
		return false, nil
	}

	if !s.checkPathScope(parsedURL.Path) {
		// s.logger.Debug().Str("url", urlString).Str("path", parsedURL.Path).Msg("URL disallowed by path scope.")
		return false, nil
	}

	// s.logger.Debug().Str("url", urlString).Msg("URL is within scope.")
	return true, nil
}

// unique returns a slice with unique strings from the input slice.
func unique(slice []string) []string {
	if len(slice) == 0 {
		return []string{}
	}
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// isStringInSlice checks if a string exists in a slice of strings.
// Helper function for scope checking.
func isStringInSlice(str string, slice []string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// ExtractHostnamesFromSeedURLs extracts unique hostnames from a list of seed URLs.
func ExtractHostnamesFromSeedURLs(seedURLs []string, logger zerolog.Logger) []string {
	hostnames := make(map[string]bool)
	for _, seed := range seedURLs {
		parsedURL, err := url.Parse(seed)
		if err != nil {
			logger.Warn().Str("seed_url", seed).Err(err).Msg("Failed to parse seed URL for hostname extraction")
			continue
		}
		if parsedURL.Hostname() != "" {
			hostnames[parsedURL.Hostname()] = true
		}
	}
	uniqueHostnames := make([]string, 0, len(hostnames))
	for host := range hostnames {
		uniqueHostnames = append(uniqueHostnames, host)
	}
	return uniqueHostnames
}

// MergeAllowedHostnames merges two slices of hostnames, ensuring uniqueness.
func MergeAllowedHostnames(existingHostnames, seedHostnames []string) []string {
	merged := make(map[string]bool)
	for _, h := range existingHostnames {
		merged[h] = true
	}
	for _, h := range seedHostnames {
		merged[h] = true
	}
	result := make([]string, 0, len(merged))
	for h := range merged {
		result = append(result, h)
	}
	return result
}
