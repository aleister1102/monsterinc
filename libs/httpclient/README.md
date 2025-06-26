# MonsterInc HTTP Client

This library provides a robust and configurable HTTP client for Go applications, designed for making HTTP/HTTPS requests with support for retries, timeouts, and other advanced features.

## Features

- **Fluent Builder API**: Easily construct HTTP client instances with a chained builder pattern.
- **Retry Mechanism**: Built-in support for request retries with exponential backoff.
- **Timeout Configuration**: Fine-grained control over various timeouts (request, dial, TLS handshake).
- **Connection Pooling**: Configure connection pooling for performance.
- **HTTP/2 Support**: Enable or disable HTTP/2.
- **Redirect Handling**: Control how redirects are followed.
- **Custom Headers and User-Agent**: Easily set custom headers and user-agent for all requests.
- **Independent Module**: Designed as a standalone library.

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
		WithHTTP2(true).
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