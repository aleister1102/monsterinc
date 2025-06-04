package urlhandler

import (
	"errors" // Assuming your error model is defined here
	"fmt"
	"net"
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

// GetBaseDomain extracts the base domain (e.g., "example.com" from "sub.example.com", or "example.co.uk" from "www.example.co.uk").
// It tries to handle common TLDs; for more complex scenarios, a proper library might be needed.
func GetBaseDomain(hostname string) (string, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return "", errors.New("hostname is empty")
	}

	// Remove port if present
	if strings.Contains(hostname, ":") {
		host, _, err := net.SplitHostPort(hostname)
		if err == nil {
			hostname = host
		}
	}

	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		// Cannot be a base domain like example.com, could be localhost or single label
		return hostname, nil // Or return error if single label isn't desired
	}

	// Common two-part TLDs (add more as needed or use a library)
	// This is a simplified approach. For comprehensive TLD handling, consider a library like "golang.org/x/net/publicsuffix".
	twoPartTLDs := map[string]bool{
		"co.uk": true, "com.au": true, "com.sg": true, "com.cn": true, "org.uk": true, // etc.
		"gov.uk": true, "ac.uk": true, "net.au": true, "com.br": true, "com.mx": true,
	}

	if len(parts) > 2 {
		// Check for common two-part TLDs like "co.uk"
		potentialTwoPartTLD := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if twoPartTLDs[potentialTwoPartTLD] {
			if len(parts) > 2 { // Need at least three parts for domain.co.uk
				return parts[len(parts)-3] + "." + potentialTwoPartTLD, nil
			}
			// Edge case: something like "co.uk" itself - treat as is if it was the input
			return potentialTwoPartTLD, nil
		}
	}

	// Standard case: example.com -> take last two parts
	return parts[len(parts)-2] + "." + parts[len(parts)-1], nil
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

// ExtractHostnameWithPort extracts hostname:port from a URL string
// For URLs without explicit port, it returns hostname:default_port (80 for http, 443 for https)
func ExtractHostnameWithPort(urlString string) (string, error) {
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

	port := parsedURL.Port()
	if port == "" {
		// Use default ports for common schemes
		switch strings.ToLower(parsedURL.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			port = "80" // Default fallback
		}
	}

	return strings.ToLower(strings.TrimSpace(hostname)) + ":" + port, nil
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

// SanitizeHostnamePort creates a safe filename string from hostname:port format.
// It specifically handles the colon character by replacing it with an underscore.
// This allows for easier reversal compared to the general SanitizeFilename function.
func SanitizeHostnamePort(hostnamePort string) string {
	// Simply replace colon with underscore for hostname:port format
	// This preserves the structure and allows for easy reversal
	return strings.ReplaceAll(hostnamePort, ":", "_")
}

// RestoreHostnamePort converts a sanitized hostname_port back to hostname:port format.
// This assumes the input was sanitized using SanitizeHostnamePort.
func RestoreHostnamePort(sanitizedHostnamePort string) string {
	// Find the last underscore, which should be the port separator
	lastUnderscore := strings.LastIndex(sanitizedHostnamePort, "_")
	if lastUnderscore == -1 {
		// No underscore found, return as-is (shouldn't happen with valid input)
		return sanitizedHostnamePort
	}

	// Replace the last underscore with colon to restore hostname:port format
	return sanitizedHostnamePort[:lastUnderscore] + ":" + sanitizedHostnamePort[lastUnderscore+1:]
}
