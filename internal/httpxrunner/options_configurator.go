package telescope

import (
	"fmt"

	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
)

// OptionsConfigurator is responsible for converting the high-level `Config`
// into the low-level `runner.Options` required by the `httpx` engine.
type OptionsConfigurator struct {
	config   *Config
	rootURL  string
	onResult func(result runner.Result)
}

// NewOptionsConfigurator creates a new options configurator.
func NewOptionsConfigurator(config *Config, rootURL string, onResult func(result runner.Result)) *OptionsConfigurator {
	return &OptionsConfigurator{
		config:   config,
		rootURL:  rootURL,
		onResult: onResult,
	}
}

// GetOptions builds and returns the `httpx` runner options.
func (oc *OptionsConfigurator) GetOptions() *runner.Options {
	// httpx expects a comma-separated string for multiple targets
	// and for multiple request URIs. We handle the slice to string conversion here.
	requestURIs := ""
	if len(oc.config.RequestURIs) > 0 {
		// For simplicity, this implementation just takes the first URI.
		// A more complete implementation might join them.
		requestURIs = oc.config.RequestURIs[0]
	}

	options := &runner.Options{
		// Basic settings
		Methods:         oc.config.Method,
		Silent:          true, // Always silent; we use our own logger
		Verbose:         oc.config.Verbose,
		Timeout:         oc.config.Timeout,
		Retries:         oc.config.Retries,
		FollowRedirects: oc.config.FollowRedirects,
		RateLimit:       oc.config.RateLimit,

		// Input handling
		InputTargetHost: oc.config.Targets,
		RequestURI:      requestURIs,

		// Concurrency
		Threads: oc.config.Threads,

		// Result callback
		OnResult: oc.onResult,

		// Data extraction
		ExtractTitle:            oc.config.ExtractTitle,
		StatusCode:              oc.config.ExtractStatusCode,
		ContentLength:           oc.config.ExtractContentLength,
		OmitBody:                !oc.config.ExtractBody,
		ResponseHeadersInStdout: oc.config.ExtractHeaders,
		TechDetect:              oc.config.TechDetect,
		OutputServerHeader:      true, // Always try to get the server header
		OutputContentType:       true, // Always try to get the content type
		ResponseInStdout:        oc.config.ExtractBody,
		ChainInStdout:           true, // Needed for final URL
		HostMaxErrors:           -1,   // Disable host error limit
		CustomHeaders:           oc.getCustomHeaders(),
	}

	return options
}

func (oc *OptionsConfigurator) getCustomHeaders() customheader.CustomHeaders {
	headers := customheader.CustomHeaders{}
	for _, h := range oc.config.CustomHeaders {
		if err := headers.Set(h); err != nil {
			// In a real application, you'd want to log this error.
			// For this example, we'll just print it.
			fmt.Printf("Warning: could not set custom header %q: %v\n", h, err)
		}
	}
	return headers
}
