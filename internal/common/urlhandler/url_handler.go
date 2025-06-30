package urlhandler

import (
	"errors" // Assuming your error model is defined here
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
)

// Regex for cleaning filenames
var (
	unsafeFilenameCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
	multipleUnderscoresRegex = regexp.MustCompile(`_+`)
)

// NormalizeURL normalizes a URL by adding scheme if missing and lowercasing the domain
func NormalizeURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", errorwrapper.NewError("URL is empty")
	}

	// Add scheme if missing
	if !strings.HasPrefix(trimmedURL, "http://") && !strings.HasPrefix(trimmedURL, "https://") {
		trimmedURL = "https://" + trimmedURL
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", errorwrapper.WrapError(err, "could not parse URL '"+trimmedURL+"'")
	}

	// Lowercase the hostname
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	return parsedURL.String(), nil
}

// ResolveURL resolves a relative or absolute URL against a base URL
func ResolveURL(href string, base *url.URL) (string, error) {
	trimmedHref := strings.TrimSpace(href)
	if trimmedHref == "" {
		return "", errorwrapper.NewError("href is empty")
	}

	// Try to parse as absolute URL first
	if parsedHref, err := url.Parse(trimmedHref); err == nil && parsedHref.IsAbs() {
		return parsedHref.String(), nil
	}

	// Handle relative URL
	if base == nil {
		// Try to parse as standalone URL
		if _, parseErr := url.Parse(trimmedHref); parseErr != nil {
			return "", errorwrapper.WrapError(parseErr, "error parsing base-less href '"+trimmedHref+"'")
		}
		return "", errorwrapper.NewError("cannot process relative URL '" + trimmedHref + "' without a base URL")
	}

	// Parse the href to create proper URL struct
	parsedHref, err := url.Parse(trimmedHref)
	if err != nil {
		return "", errorwrapper.WrapError(err, "error parsing href '"+trimmedHref+"'")
	}

	// Resolve relative URL against base using proper URL struct
	resolvedURL := base.ResolveReference(parsedHref)
	if resolvedURL == nil {
		return "", errorwrapper.NewError("error resolving href '" + trimmedHref + "' with base '" + base.String() + "'")
	}

	return resolvedURL.String(), nil
}

// GetRootTargetForURL finds the original seed URL that matches the host of the discoveredURL.
// This helps in associating discovered content with its original crawl scope.
func GetRootTargetForURL(discoveredURL string, seedURLs []string) string {
	normalizedDiscovered, err := NormalizeURL(discoveredURL)
	if err != nil {
		// If the discovered URL is invalid, return the original URL itself as root target
		return discoveredURL
	}

	var discoveredHost string
	if parsedDiscovered, pErr := url.Parse(normalizedDiscovered); pErr == nil {
		discoveredHost = parsedDiscovered.Hostname()
	} else {
		// If parsing fails, return the discovered URL itself as root target
		return discoveredURL
	}

	if discoveredHost == "" {
		// If hostname is empty (e.g. file:// URLs without host), return the discovered URL itself
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

	// Instead of fallback to seedURLs[0], construct a proper root URL from the discovered URL
	// This ensures each domain gets its own root target
	if parsedDiscovered, err := url.Parse(normalizedDiscovered); err == nil {
		// Construct root URL as scheme://hostname
		rootURL := parsedDiscovered.Scheme + "://" + parsedDiscovered.Host
		return rootURL
	}

	// Absolute fallback - return the discovered URL itself
	return discoveredURL
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

	// 4. Clean up dots: remove multiple consecutive dots and dots next to underscores
	name = strings.ReplaceAll(name, "..", "_") // Replace .. with _
	name = strings.ReplaceAll(name, "_.", "_") // Replace _. with _
	name = strings.ReplaceAll(name, "._", "_") // Replace ._ with _

	// 5. Remove leading or trailing dots and underscores that might result from replacements
	name = strings.Trim(name, "_.")

	// If the name becomes empty after sanitization (e.g., input was just "http://"), provide a default.
	if name == "" {
		return "sanitized_empty_input"
	}

	return name
}

// ValidateURLFormat validates if a URL string has proper format
func ValidateURLFormat(rawURL string) error {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return errorwrapper.NewError("URL is empty")
	}

	_, err := url.Parse(trimmedURL)
	if err != nil {
		return errorwrapper.WrapError(err, "invalid URL format '"+trimmedURL+"'")
	}

	return nil
}

// ExtractHostname extracts hostname without port from a URL string
func ExtractHostname(urlString string) (string, error) {
	if urlString == "" {
		return "", errorwrapper.NewError("URL string is empty")
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", errorwrapper.WrapError(err, "could not parse URL '"+urlString+"'")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return "", errorwrapper.NewError("URL has no hostname component: " + urlString)
	}

	return hostname, nil
}
