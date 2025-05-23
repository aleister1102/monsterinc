package normalizer

import (
	"errors"
	"net/url"
	"strings"
)

// NormalizeURL takes a raw URL string, parses it, and applies normalization rules.
// Task 1.1: Implement basic URL parsing.
// Task 1.2: Implement scheme and hostname normalization.
// Task 1.3: Implement URL fragment removal.
func NormalizeURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		// According to the PRD, empty lines in the input file should be skipped.
		// This function signals an empty URL to the caller (e.g., file processing logic)
		// which can then decide to skip it.
		return "", errors.New("input URL is empty")
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
		// This indicates a malformed URL that cannot be processed.
		return "", err
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
			// This could happen if, even after prepending http://, the URL is malformed.
			return "", err
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
	return parsedURL.String(), nil
}
