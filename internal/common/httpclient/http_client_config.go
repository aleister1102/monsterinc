package httpclient

import (
	"time"
)

// HTTPClientConfig holds configuration for HTTP clients
type HTTPClientConfig struct {
	Timeout               time.Duration     // Request timeout
	InsecureSkipVerify    bool              // Skip TLS verification
	FollowRedirects       bool              // Whether to follow redirects
	MaxRedirects          int               // Maximum number of redirects to follow
	Proxy                 string            // Proxy URL (HTTP/SOCKS)
	CustomHeaders         map[string]string // Custom headers to add to all requests
	MaxIdleConns          int               // Maximum idle connections
	MaxIdleConnsPerHost   int               // Maximum idle connections per host
	MaxConnsPerHost       int               // Maximum connections per host
	IdleConnTimeout       time.Duration     // Idle connection timeout
	TLSHandshakeTimeout   time.Duration     // TLS handshake timeout
	ExpectContinueTimeout time.Duration     // Expect 100-continue timeout
	DialTimeout           time.Duration     // Connection dial timeout
	KeepAlive             time.Duration     // Keep-alive duration
	EnableHTTP2           bool              // Enable HTTP/2 support (default: true)
}

// DefaultHTTPClientConfig returns the default HTTP client configuration
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:               30 * time.Second,
		InsecureSkipVerify:    true,
		FollowRedirects:       true,
		MaxRedirects:          10,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // 0 means no limit
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialTimeout:           10 * time.Second,
		KeepAlive:             30 * time.Second,
		EnableHTTP2:           true, // Enable HTTP/2 by default
		CustomHeaders: map[string]string{
			"Accept":                    "*/*",
			"Accept-Language":           "en-US,en;q=0.9",
			"Connection":                "keep-alive",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Upgrade-Insecure-Requests": "1",
		},
	}
}
