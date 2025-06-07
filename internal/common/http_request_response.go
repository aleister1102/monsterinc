package common

import (
	"context"
	"io"
)

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    io.Reader
	Context context.Context
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}
