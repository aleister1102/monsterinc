package config

import (
	"errors"
	"fmt"
	"strings"

	"monsterinc/internal/httpxrunner"
)

// HTTPXConfig holds the configuration for httpx probing
type HTTPXConfig struct {
	// Target configuration
	Targets []string `json:"targets" yaml:"targets"`

	// HTTP configuration
	Method          string   `json:"method" yaml:"method"`
	RequestURIs     []string `json:"request_uris" yaml:"request_uris"`
	FollowRedirects bool     `json:"follow_redirects" yaml:"follow_redirects"`
	Timeout         int      `json:"timeout" yaml:"timeout"`
	Retries         int      `json:"retries" yaml:"retries"`
	Threads         int      `json:"threads" yaml:"threads"`

	// Data extraction configuration
	ExtractStatus    bool `json:"extract_status" yaml:"extract_status"`
	ExtractLength    bool `json:"extract_length" yaml:"extract_length"`
	ExtractType      bool `json:"extract_type" yaml:"extract_type"`
	ExtractTitle     bool `json:"extract_title" yaml:"extract_title"`
	ExtractServer    bool `json:"extract_server" yaml:"extract_server"`
	ExtractTech      bool `json:"extract_tech" yaml:"extract_tech"`
	ExtractIP        bool `json:"extract_ip" yaml:"extract_ip"`
	ExtractCNAME     bool `json:"extract_cname" yaml:"extract_cname"`
	ExtractFinalURL  bool `json:"extract_final_url" yaml:"extract_final_url"`
	ExtractRedirects bool `json:"extract_redirects" yaml:"extract_redirects"`

	// Output configuration
	OutputFormat string `json:"output_format" yaml:"output_format"`

	// Headers and proxy
	CustomHeaders map[string]string `json:"custom_headers" yaml:"custom_headers"`
	Proxy         string            `json:"proxy" yaml:"proxy"`
}

// NewHTTPXConfig creates a new HTTPXConfig with default values
func NewHTTPXConfig() *HTTPXConfig {
	return &HTTPXConfig{
		// HTTP defaults
		Method:          "GET",
		FollowRedirects: true,
		Timeout:         5,
		Retries:         1,
		Threads:         40,

		// Data extraction defaults
		ExtractStatus:    true,
		ExtractLength:    true,
		ExtractType:      true,
		ExtractTitle:     true,
		ExtractServer:    true,
		ExtractTech:      true,
		ExtractIP:        true,
		ExtractCNAME:     true,
		ExtractFinalURL:  true,
		ExtractRedirects: true,

		// Output defaults
		OutputFormat: "json",
	}
}

// Validate checks if the configuration is valid
func (c *HTTPXConfig) Validate() error {
	if len(c.Targets) == 0 {
		return errors.New("no targets specified")
	}

	if c.Timeout < 1 {
		return errors.New("timeout must be greater than 0")
	}

	if c.Retries < 0 {
		return errors.New("retries must be non-negative")
	}

	if c.Threads < 1 {
		return errors.New("threads must be greater than 0")
	}

	if c.Method != "" {
		method := strings.ToUpper(c.Method)
		validMethods := map[string]bool{
			"GET":     true,
			"POST":    true,
			"PUT":     true,
			"DELETE":  true,
			"HEAD":    true,
			"OPTIONS": true,
			"PATCH":   true,
			"ALL":     true,
		}
		if !validMethods[method] {
			return fmt.Errorf("invalid HTTP method: %s", c.Method)
		}
	}

	if c.OutputFormat != "" {
		validFormats := map[string]bool{
			"json": true,
			"csv":  true,
			"yaml": true,
		}
		if !validFormats[c.OutputFormat] {
			return fmt.Errorf("invalid output format: %s", c.OutputFormat)
		}
	}

	return nil
}

// ToRunnerConfig converts HTTPXConfig to httpxrunner.Config
func (c *HTTPXConfig) ToRunnerConfig() *httpxrunner.Config {
	return &httpxrunner.Config{
		Targets:         c.Targets,
		Method:          c.Method,
		RequestURIs:     c.RequestURIs,
		FollowRedirects: c.FollowRedirects,
		Timeout:         c.Timeout,
		Retries:         c.Retries,
		Threads:         c.Threads,
		OutputFormat:    c.OutputFormat,
		CustomHeaders:   c.CustomHeaders,
		Proxy:           c.Proxy,
	}
}
