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
	// NormalizeURL should consistently return *models.URLValidationError or nil.
	return err
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
	tailoredURL := strings.TrimSpace(rawURL)
	if tailoredURL == "" {
		return "", &models.URLValidationError{URL: rawURL, Message: "input URL is empty"}
	}

	// Attempt to parse the URL.
	// Scheme and Host will be populated correctly by url.Parse if a scheme is present.
	// If no scheme, url.Parse treats the whole thing as a path for non-authority-form URLs.
	parsedURL, err := url.Parse(tailoredURL)
	if err != nil {
		// If initial parsing fails, it might be because there's no scheme and it's an authority form like "example.com"
		// or just a path. Try prepending a scheme to see if it becomes a valid absolute URL.
		parsedWithScheme, errWithScheme := url.Parse("http://" + tailoredURL)
		if errWithScheme != nil {
			// If it still fails even with a default scheme, then it's likely fundamentally malformed.
			return "", &models.URLValidationError{URL: rawURL, Message: fmt.Sprintf("parsing failed: %s (also with default http scheme: %s)", err.Error(), errWithScheme.Error())}
		}
		// If parsing with a scheme succeeded, use that result.
		parsedURL = parsedWithScheme
	} else if parsedURL.Scheme == "" {
		// If initial parsing succeeded BUT no scheme was found (e.g. "example.com/path"),
		// it means url.Parse treated "example.com/path" as the Path component.
		// We need to re-parse it with a scheme to correctly identify Host and Path.
		parsedURL, err = url.Parse("http://" + tailoredURL) // Use original tailoredURL for re-parsing
		if err != nil {
			return "", &models.URLValidationError{URL: rawURL, Message: fmt.Sprintf("parsing with default scheme failed: %s", err.Error())}
		}
	}

	// Ensure Host is present after scheme handling, as scheme-only URLs are not valid for us.
	if parsedURL.Host == "" {
		return "", &models.URLValidationError{URL: rawURL, Message: "URL lacks a host component after scheme processing"}
	}

	// Task 1.2: Convert scheme and hostname to lowercase.
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host) // Host includes hostname and port if present.

	// Task 1.3: Remove URL fragment.
	parsedURL.Fragment = ""

	finalURL := parsedURL.String()
	// Check for effectively empty URLs post-normalization (e.g. if path was "/" and everything else was stripped)
	// A URL like "http://" is not useful.
	if finalURL == "" || finalURL == parsedURL.Scheme+"://" || (parsedURL.Scheme != "" && parsedURL.Host == "" && parsedURL.Path == "" && parsedURL.Fragment == "" && parsedURL.RawQuery == "") {
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
