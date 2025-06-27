# Comet HTTP Client

This library provides a robust and configurable HTTP client for Go applications, designed for making HTTP/HTTPS requests with support for retries, timeouts, and other advanced features.

## Features

- **Fluent Builder API**: Easily construct HTTP client instances with a chained builder pattern.
- **Built-in Retries**: Automatic request retries with exponential backoff for transient errors.
- **Content Fetching**: Specialized method for fetching content with support for conditional requests (ETag/Last-Modified) and content size limits.
- **Fine-grained Timeouts**: Control over request, dial, and TLS handshake timeouts.
- **Connection Pooling**: Configure connection pooling for high performance.
- **Redirect Handling**: Control how redirects are followed.
- **Customization**: Set custom headers, user-agent, proxy, and more.
- **HTTP/2 Support**: Enable or disable HTTP/2.

## Installation

```bash
go get github.com/monsterinc/httpclient
```

## Usage

### Basic GET Request

```go
package main

import (
	"fmt"
	"log"

	"github.com/monsterinc/httpclient"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.Nop()
	client, err := httpclient.NewHTTPClientBuilder(logger).Build()
	if err != nil {
		log.Fatalf("failed to create http client: %v", err)
	}

	req := &httpclient.HTTPRequest{
		URL:    "https://api.github.com/zen",
		Method: "GET",
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to make request: %v", err)
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %s\n", string(resp.Body))
}
```

### Advanced Configuration

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/monsterinc/httpclient"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.Nop()

	client, err := httpclient.NewHTTPClientBuilder(logger).
		WithTimeout(15 * time.Second).
		WithFollowRedirects(false).
		WithUserAgent("MyAwesomeApp/1.0").
		WithMaxContentSize(1024 * 1024). // 1MB limit
		Build()

	if err != nil {
		log.Fatalf("failed to create http client: %v", err)
	}

	req := &httpclient.HTTPRequest{
		URL:    "https://httpbin.org/redirect/1",
		Method: "GET",
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to make request: %v", err)
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Location Header: %s\n", resp.Headers["Location"])
}
```

### Content Fetching

The `FetchContent` method simplifies fetching resources with support for conditional requests, which is useful for reducing bandwidth and handling cached content.

```go
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/monsterinc/httpclient"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.Nop()
	client, err := httpclient.NewHTTPClientBuilder(logger).Build()
	if err != nil {
		log.Fatalf("failed to create http client: %v", err)
	}

	// Assume we have a stored ETag from a previous request
	previousETag := "some-etag-from-before"

	input := httpclient.FetchContentInput{
		URL:          "https://httpbin.org/cache",
		PreviousETag: previousETag,
	}

	result, err := client.FetchContent(input)
	if err != nil {
		// Handle "Not Modified" status
		if errors.Is(err, httpclient.ErrNotModified) {
			log.Printf("Content not modified (HTTP 304). ETag: %s", result.ETag)
			return
		}
		log.Fatalf("Failed to fetch content: %v", err)
	}

	fmt.Printf("Successfully fetched content.\n")
	fmt.Printf("Status: %d\n", result.HTTPStatusCode)
	fmt.Printf("ETag: %s\n", result.ETag)
	fmt.Printf("Content Type: %s\n", result.ContentType)
	fmt.Printf("Body: %s\n", string(result.Content))
} 
```