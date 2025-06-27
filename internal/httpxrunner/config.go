package telescope

import "github.com/projectdiscovery/goflags"

// Config holds the configuration options for the runner.
// It is used to customize the behavior of the underlying httpx engine.
type Config struct {
	// Targets is a slice of hosts or URLs to scan.
	Targets []string `json:"targets"`

	// Threads is the number of concurrent probes to run.
	Threads int `json:"threads"`

	// Timeout is the timeout in seconds for each request.
	Timeout int `json:"timeout"`

	// Retries is the number of times to retry a failed request.
	Retries int `json:"retries"`

	// Method is the HTTP method to use (e.g., "GET", "POST").
	Method string `json:"method"`

	// FollowRedirects determines whether to follow HTTP redirects.
	FollowRedirects bool `json.com:"follow_redirects"`

	// RateLimit is the maximum requests per second. 0 means no limit.
	RateLimit int `json:"rate_limit"`

	// CustomHeaders is a map of custom headers to send with each request.
	CustomHeaders goflags.StringSlice

	// RequestURIs is a slice of URIs/paths to request for each target.
	RequestURIs []string `json:"request_uris"`

	// Verbose enables verbose logging from the underlying engine.
	Verbose bool `json:"verbose"`

	// ExtractTitle enables title extraction.
	ExtractTitle bool `json:"extract_title"`

	// ExtractStatusCode enables status code extraction.
	ExtractStatusCode bool `json:"extract_status_code"`

	// ExtractContentLength enables content length extraction.
	ExtractContentLength bool `json:"extract_content_length"`

	// ExtractBody enables body extraction.
	ExtractBody bool `json:"extract_body"`

	// ExtractHeaders enables headers extraction.
	ExtractHeaders bool `json:"extract_headers"`

	// TechDetect enables technology detection.
	TechDetect bool `json:"tech_detect"`
}

// DefaultConfig returns a new Config with default values.
// These values are sensible defaults for a general-purpose scan.
func DefaultConfig() *Config {
	return &Config{
		Threads:         25,
		Timeout:         10,
		Retries:         1,
		Method:          "GET",
		FollowRedirects: true,
		RateLimit:       0, // No limit
		CustomHeaders:   []string{},
		RequestURIs:     []string{},

		// Extraction flags
		ExtractTitle:         true,
		ExtractStatusCode:    true,
		ExtractContentLength: true,
		ExtractHeaders:       true,
		ExtractBody:          false, // Body can be large, so off by default
		TechDetect:           true,
	}
}
