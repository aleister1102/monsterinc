package urlhandler

import (
	"errors" // Assuming your error model is defined here
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Regex for cleaning filenames
var (
	unsafeFilenameCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
	multipleUnderscoresRegex = regexp.MustCompile(`_+`)
)

// NormalizeURL normalizes a URL string, ensuring it has a scheme, lowercase host, and no fragment.
func NormalizeURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", errors.New("URL is empty or only whitespace")
	}

	// Add scheme if missing
	if !strings.Contains(trimmedURL, "://") && !strings.HasPrefix(trimmedURL, "//") {
		trimmedURL = "http://" + trimmedURL
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", fmt.Errorf("could not parse URL '%s': %w", trimmedURL, err)
	}

	if parsedURL.Host == "" {
		return "", errors.New("URL lacks a valid hostname")
	}

	finalURL := parsedURL.String()

	if finalURL == parsedURL.Scheme+"://"+parsedURL.Host && parsedURL.Path == "/" && parsedURL.RawQuery == "" && parsedURL.Fragment == "" {
		// Base URL like "http://example.com" is valid
	} else if len(parsedURL.Host) == 0 {
		return "", errors.New("URL appears to be invalid after parsing")
	}

	return finalURL, nil
}

// ValidateURL checks if a URL is valid by attempting to normalize it.
func ValidateURL(rawURL string) error {
	_, err := NormalizeURL(rawURL)
	return err
}

// ValidateURLs validates a slice of URLs.
func ValidateURLs(urls []string) map[string]error {
	errorsMap := make(map[string]error)
	for _, u := range urls {
		if err := ValidateURL(u); err != nil {
			errorsMap[u] = err
		}
	}
	return errorsMap
}

// NormalizeURLs normalizes a slice of URLs.
func NormalizeURLs(urls []string) (map[string]string, map[string]error) {
	normalizedMap := make(map[string]string)
	errorsMap := make(map[string]error)
	for _, u := range urls {
		normalizedURL, err := NormalizeURL(u)
		if err != nil {
			errorsMap[u] = err
			continue
		}
		normalizedMap[u] = normalizedURL
	}
	return normalizedMap, errorsMap
}

// IsValidURL is a utility function for a quick validity check.
func IsValidURL(rawURL string) bool {
	return ValidateURL(rawURL) == nil
}

// GetBaseURL returns the base part of a URL (e.g., "http://example.com").
// Uses NormalizeURL to ensure the input is processed correctly.
func GetBaseURL(rawURL string) (string, error) {
	normalizedURLString, err := NormalizeURL(rawURL)
	if err != nil {
		return "", err
	}
	// Parsing here is guaranteed to succeed because NormalizeURL already did.
	parsedURL, _ := url.Parse(normalizedURLString)
	return fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host), nil
}

// IsDomainOrSubdomain checks if domain is equal to baseDomain or is a subdomain of baseDomain.
func IsDomainOrSubdomain(domain, baseDomain string) bool {
	// Normalize to lowercase for case-insensitive comparison.
	domain = strings.ToLower(strings.TrimSpace(domain))
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))

	if domain == "" || baseDomain == "" {
		return false
	}
	return domain == baseDomain || strings.HasSuffix(domain, "."+baseDomain)
}

// ResolveURL resolves a (possibly relative) URL string against a base URL.
// The returned URL is also normalized.
func ResolveURL(href string, base *url.URL) (string, error) {
	trimmedHref := strings.TrimSpace(href)
	if trimmedHref == "" {
		return "", fmt.Errorf("href is empty")
	}

	var resolvedURL *url.URL

	if base == nil {
		// If no base, href must be an absolute URL.
		parsedHref, parseErr := url.Parse(trimmedHref)
		if parseErr != nil {
			return "", fmt.Errorf("error parsing base-less href '%s': %w", trimmedHref, parseErr)
		}
		if !parsedHref.IsAbs() {
			return "", fmt.Errorf("cannot process relative URL '%s' without a base URL", trimmedHref)
		}
		resolvedURL = parsedHref
	} else {
		// Resolve href against the base URL.
		resolved, resolveErr := base.Parse(trimmedHref)
		if resolveErr != nil {
			return "", fmt.Errorf("error resolving href '%s' with base '%s': %w", trimmedHref, base.String(), resolveErr)
		}
		resolvedURL = resolved
	}

	// Normalize the successfully resolved URL.
	return NormalizeURL(resolvedURL.String())
}

// GetRootTargetForURL finds the original seed URL that matches the host of the discoveredURL.
// This helps in associating discovered content with its original crawl scope.
func GetRootTargetForURL(discoveredURL string, seedURLs []string) string {
	normalizedDiscovered, err := NormalizeURL(discoveredURL)
	if err != nil {
		// If the discovered URL is invalid, fallback.
		if len(seedURLs) > 0 {
			return seedURLs[0] // Return the first seed as a default.
		}
		return discoveredURL // Or the original invalid URL if no seeds.
	}

	var discoveredHost string
	if parsedDiscovered, pErr := url.Parse(normalizedDiscovered); pErr == nil {
		discoveredHost = parsedDiscovered.Hostname()
	} else { // Should not happen if NormalizeURL succeeded without error
		if len(seedURLs) > 0 {
			return seedURLs[0]
		}
		return discoveredURL
	}

	if discoveredHost == "" { // If hostname is empty (e.g. file:// URLs without host)
		if len(seedURLs) > 0 {
			return seedURLs[0]
		}
		return discoveredURL
	}

	// Find a seed URL that has the same hostname.
	for _, seed := range seedURLs {
		normalizedSeed, sErr := NormalizeURL(seed)
		if sErr == nil {
			if parsedSeed, psErr := url.Parse(normalizedSeed); psErr == nil {
				if parsedSeed.Hostname() == discoveredHost {
					return seed // Return the original seed URL, not its normalized version.
				}
			}
		}
	}

	// Fallback if no matching seed host is found.
	if len(seedURLs) > 0 {
		return seedURLs[0]
	}
	return discoveredURL // Absolute fallback.
}

// SanitizeFilename creates a safe filename string from a URL or any input string.
// It removes the protocol, replaces unsafe characters with underscores, and cleans up underscores.
func SanitizeFilename(input string) string {
	// 1. Remove scheme (e.g., "http://", "https://") if present.
	name := input
	if i := strings.Index(name, "://"); i != -1 {
		name = name[i+3:] // Get the part after "://"
	}

	// 2. Replace all characters not in the safe set (letters, numbers, underscore, dot, hyphen) with an underscore.
	name = unsafeFilenameCharsRegex.ReplaceAllString(name, "_")

	// 3. Replace multiple consecutive underscores with a single underscore.
	name = multipleUnderscoresRegex.ReplaceAllString(name, "_")

	// 4. Remove leading or trailing underscores that might result from replacements at the start/end.
	name = strings.Trim(name, "_")

	// If the name becomes empty after sanitization (e.g., input was just "http://"), provide a default.
	if name == "" {
		return "sanitized_empty_input"
	}

	return name
}

// ExtractHostname extracts hostname from a URL string
func ExtractHostname(urlString string) (string, error) {
	if strings.TrimSpace(urlString) == "" {
		return "", fmt.Errorf("URL string is empty")
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("could not parse URL '%s': %w", urlString, err)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return "", fmt.Errorf("URL has no hostname component: %s", urlString)
	}

	return strings.ToLower(strings.TrimSpace(hostname)), nil
}

// ExtractHostnamesFromURLs extracts unique hostnames from a list of URLs
func ExtractHostnamesFromURLs(urls []string) ([]string, map[string]error) {
	hostnameSet := make(map[string]bool)
	errors := make(map[string]error)

	for _, urlString := range urls {
		if strings.TrimSpace(urlString) == "" {
			continue
		}

		hostname, err := ExtractHostname(urlString)
		if err != nil {
			errors[urlString] = err
			continue
		}

		hostnameSet[hostname] = true
	}

	// Convert map to slice
	hostnames := make([]string, 0, len(hostnameSet))
	for hostname := range hostnameSet {
		hostnames = append(hostnames, hostname)
	}

	return hostnames, errors
}

// MergeHostnames merges two slices of hostnames, removing duplicates
func MergeHostnames(existing, additional []string) []string {
	hostnameSet := make(map[string]bool)

	// Add existing hostnames
	for _, hostname := range existing {
		normalizedHostname := strings.ToLower(strings.TrimSpace(hostname))
		if normalizedHostname != "" {
			hostnameSet[normalizedHostname] = true
		}
	}

	// Add additional hostnames
	for _, hostname := range additional {
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

// ValidateURLFormat validates URL format using net/url parsing (for config validation)
func ValidateURLFormat(rawURL string) error {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return fmt.Errorf("URL is empty")
	}

	_, err := url.ParseRequestURI(trimmedURL)
	if err != nil {
		return fmt.Errorf("invalid URL format '%s': %w", trimmedURL, err)
	}

	return nil
}

// ValidateURLFormats validates a slice of URLs using ParseRequestURI (for config validation)
func ValidateURLFormats(urls []string) map[string]error {
	errors := make(map[string]error)
	for _, u := range urls {
		if err := ValidateURLFormat(u); err != nil {
			errors[u] = err
		}
	}
	return errors
}

// IsAbsoluteURL checks if a URL is absolute (has scheme and hostname)
func IsAbsoluteURL(urlString string) bool {
	if strings.TrimSpace(urlString) == "" {
		return false
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	return parsedURL.IsAbs() && parsedURL.Hostname() != ""
}

// HasValidScheme checks if URL has http or https scheme
func HasValidScheme(urlString string) bool {
	if strings.TrimSpace(urlString) == "" {
		return false
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	return scheme == "http" || scheme == "https"
}
