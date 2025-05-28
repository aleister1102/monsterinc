package urlhandler

import (
	"fmt"
	"github.com/aleister1102/monsterinc/internal/models" // Assuming your error model is defined here
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
		return "", &models.URLValidationError{URL: rawURL, Message: "input URL is empty"}
	}

	// Simplified logic: If the URL doesn't have a scheme, always prepend "http://".
	// This helps url.Parse consistently handle cases like "example.com/path".
	if !strings.Contains(trimmedURL, "://") && !strings.HasPrefix(trimmedURL, "//") {
		trimmedURL = "http://" + trimmedURL
	} else if strings.HasPrefix(trimmedURL, "//") { // Handle scheme-relative URLs like //example.com
		trimmedURL = "http:" + trimmedURL
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", &models.URLValidationError{URL: rawURL, Message: fmt.Sprintf("parsing failed: %v", err)}
	}

	// A valid URL must have both a scheme and a host.
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		// If scheme is missing, it might be an invalid input like "http:/example.com"
		// or if host is missing, it could be "http:///path" or "mailto:user" (which we might not support here if a host is expected)
		return "", &models.URLValidationError{URL: rawURL, Message: "URL lacks a scheme or host component after parsing"}
	}

	// Normalization:
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host) // Host includes port if present
	parsedURL.Fragment = ""                          // Remove fragment (#)

	// Optional: Remove trailing slash from path for further normalization,
	// but only if the path is not just "/"
	if len(parsedURL.Path) > 1 && strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	}
	// If path is empty after normalization (e.g. "http://example.com"), set it to "/"
	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	finalURL := parsedURL.String()

	// Final check to prevent URLs like "http://" or "scheme://" from being considered valid if they are effectively empty.
	if finalURL == parsedURL.Scheme+"://"+parsedURL.Host && parsedURL.Path == "/" && parsedURL.RawQuery == "" && parsedURL.Fragment == "" {
		// This is a base URL like "http://example.com", which is fine.
	} else if parsedURL.Host == "" { // Double check host, as some inputs might bypass earlier checks
		return "", &models.URLValidationError{URL: rawURL, Message: "normalization resulted in a URL without a host"}
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

// IsDomainOrSubdomain checks if `domain` is equal to `baseDomain` or is a subdomain of `baseDomain`.
// Both inputs should ideally be normalized (e.g., lowercase) before calling this function,
// or this function can normalize them internally.
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
