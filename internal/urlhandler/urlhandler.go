package urlhandler

import (
	"fmt"
	"net/url"
	"strings"

	"monsterinc/internal/models" // Import the models package
)

// URLValidationError is now defined in internal/models/error.go
// type URLValidationError struct {
// 	URL     string
// 	Message string
// }
//
// func (e *URLValidationError) Error() string {
// 	return fmt.Sprintf("invalid URL %s: %s", e.URL, e.Message)
// }

// ValidateURL validates a single URL
func ValidateURL(rawURL string) error {
	// Use NormalizeURL to validate
	_, err := NormalizeURL(rawURL)
	if err != nil {
		// Ensure we are returning the new models.URLValidationError type
		if modelsErr, ok := err.(*models.URLValidationError); ok {
			return modelsErr // Return as is if it's already the correct type (e.g. from NormalizeURL if it's updated)
		}
		// If NormalizeURL returns a generic error, wrap it.
		// However, NormalizeURL itself should ideally return models.URLValidationError for consistency.
		// For now, let's assume NormalizeURL might return a generic error related to validation.
		return &models.URLValidationError{URL: rawURL, Message: err.Error()}
	}
	return nil
}

// ValidateURLs validates multiple URLs and returns a map of invalid URLs and their errors
func ValidateURLs(urls []string) map[string]error {
	errors := make(map[string]error)
	for _, u := range urls {
		if err := ValidateURL(u); err != nil {
			errors[u] = err
		}
	}
	return errors
}

// NormalizeURL takes a raw URL string, parses it, and applies normalization rules.
// Task 1.1: Implement basic URL parsing.
// Task 1.2: Implement scheme and hostname normalization.
// Task 1.3: Implement URL fragment removal.
func NormalizeURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", &models.URLValidationError{URL: rawURL, Message: "input URL is empty"} // Return specific error type
	}

	// Preserve the original fragment before parsing, as url.Parse might handle it.
	// We explicitly want to remove it based on PRD.
	urlToParse := trimmedURL
	if idx := strings.Index(urlToParse, "#"); idx != -1 {
		// This step is a bit redundant if url.Parse correctly handles fragments,
		// but the PRD explicitly states "Remove any URL fragment".
		// We'll rely on parsedURL.Fragment = "" later for robustness.
	}

	// Attempt to parse the URL.
	parsedURL, err := url.Parse(urlToParse)
	if err != nil {
		return "", &models.URLValidationError{URL: rawURL, Message: fmt.Sprintf("parsing failed: %s", err.Error())}
	}

	// Task 1.2: Add default scheme if missing.
	// PRD: "If no scheme (e.g., http://, https://) is present, prepend http:// by default."
	if parsedURL.Scheme == "" {
		// Re-parse with "http://" prepended if the original lacked a scheme.
		// This is important because url.Parse("example.com/path") treats "example.com/path" as Path, not Host+Path.
		// And url.Parse("example.com#fragment") also treats example.com as Path.
		// Prepending a scheme ensures Host is correctly identified.
		parsedURL, err = url.Parse("http://" + trimmedURL) // Use original trimmedURL for re-parsing
		if err != nil {
			return "", &models.URLValidationError{URL: rawURL, Message: fmt.Sprintf("parsing with default scheme failed: %s", err.Error())}
		}
	}

	// Task 1.2: Convert scheme and hostname to lowercase.
	// PRD: "Convert the scheme and the hostname components of the URL to lowercase."
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host) // Host includes hostname and port if present. url.Hostname() gives just hostname.

	// Task 1.3: Remove URL fragment.
	// PRD: "Remove any URL fragment (the part of the URL after a # symbol)."
	parsedURL.Fragment = ""

	// For task 1.1, the primary goal is to ensure the URL is parsable and to set up the function.
	// The url.Parse function itself might perform some minimal normalization (e.g., on escape sequences).
	// The .String() method reassembles the URL from the parsed components.
	// More specific normalization rules (like scheme defaulting, case normalization, fragment removal)
	// will be implemented in subsequent tasks (1.2, 1.3).
	finalURL := parsedURL.String()
	if finalURL == "" || finalURL == "http://" || finalURL == "https://" { // Basic check for empty result post-normalization
		return "", &models.URLValidationError{URL: rawURL, Message: "normalization resulted in an empty or scheme-only URL"}
	}

	return finalURL, nil
}

// NormalizeURLs normalizes multiple URLs and returns a map of original URLs to their normalized forms
func NormalizeURLs(urls []string) (map[string]string, map[string]error) {
	normalized := make(map[string]string)
	errors := make(map[string]error)

	for _, u := range urls {
		normalizedURL, err := NormalizeURL(u)
		if err != nil {
			errors[u] = err
			continue
		}
		normalized[u] = normalizedURL
	}

	return normalized, errors
}

// IsValidURL returns true if the URL is valid
func IsValidURL(rawURL string) bool {
	return ValidateURL(rawURL) == nil
}

// GetBaseURL returns the base URL (scheme + host) of a URL
func GetBaseURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host), nil
}

// isDomainOrSubdomain checks if `domain` is equal to `baseDomain` or is a subdomain of `baseDomain`.
// Both inputs should be normalized (e.g., lowercase).
func IsDomainOrSubdomain(domain, baseDomain string) bool {
	if domain == baseDomain {
		return true // Exact match
	}
	// Check for subdomain: domain must end with ".baseDomain"
	return strings.HasSuffix(domain, "."+baseDomain)
}

// ResolveURL resolves a (possibly relative) URL string against a base URL.
func ResolveURL(href string, base *url.URL) (string, error) {
	if base == nil { // If no base, href must be absolute
		parsedHref, err := url.Parse(strings.TrimSpace(href))
		if err != nil {
			return "", fmt.Errorf("error parsing base-less href '%s': %w", href, err)
		}
		if !parsedHref.IsAbs() {
			return "", fmt.Errorf("cannot process relative URL '%s' without a base URL", href)
		}
		return parsedHref.String(), nil
	}

	resolved, err := base.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", fmt.Errorf("error parsing href '%s' against base '%s': %w", href, base.String(), err)
	}
	return resolved.String(), nil
}
