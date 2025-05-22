package normalizer

import (
	"net/url"
	"strings"
)

// NormalizeURL takes a raw URL string and returns a normalized version.
// Normalization includes:
// - Adding a default scheme (http) if missing.
// - Lowercasing the scheme and host.
// - Removing the fragment.
func NormalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, try to prepend http:// and parse again
		// This handles cases like "example.com"
		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			u, err = url.Parse("http://" + rawURL)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	if u.Scheme == "" {
		u.Scheme = "http" // Default to http if no scheme is present
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = "" // Remove fragment

	return u.String(), nil
}
