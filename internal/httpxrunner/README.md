# Go-Telescope

A high-level, concurrent, and configurable Go wrapper for the powerful `projectdiscovery/httpx` library. This wrapper, **Telescope**, simplifies the process of running HTTP probes by providing a clean, fluent builder pattern, robust configuration, and structured, easy-to-use results.

## Overview

This library is designed to abstract away the complexity of configuring and running the underlying `httpx` engine. It allows you to quickly integrate HTTP probing capabilities into your Go applications with minimal setup. The focus is on ease of use, concurrent execution, and predictable results.

## Features

-   **Fluent Builder Pattern**: A clean, chainable API for building and configuring the runner.
-   **Simplified Configuration**: A single `Config` struct to control all major `httpx` options.
-   **Concurrent & Scalable**: Runs probes concurrently using goroutines, managed by the underlying `httpx` engine.
-   **Context-Aware Cancellation**: Gracefully handles `context.Context` cancellation to stop scans.
-   **Structured Results**: Maps `httpx` output to a clean, well-defined `TelescopeResult` struct.
-   **Thread-Safe Result Collection**: Safely collect results from multiple concurrent probes.

## Installation

```bash
go get github.com/monsterinc/telescope
```

## Usage

Here is a complete example of how to use the library to run a simple scan.

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/monsterinc/telescope"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// 1. Initialize a logger
	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// 2. Create a configuration
	// Start with the default config and customize it
	config := telescope.DefaultConfig()
	config.Targets = []string{"scanme.nmap.org", "example.com"}
	config.Threads = 10
	config.Timeout = 5 // 5 seconds
	config.FollowRedirects = true
	config.ExtractTitle = true
	config.ExtractStatusCode = true

	// 3. Use the builder to create a new runner
	// The root URL is an identifier for this specific scan instance
	runner, err := telescope.NewRunnerBuilder(logger).
		WithConfig(config).
		WithRootTargetURL("my-first-scan").
		Build()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to build runner")
	}

	// 4. Run the scan with a context for cancellation
	// (e.g., a 30-second timeout for the whole scan)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info().Msg("Starting HTTP scan...")
	if err := runner.Run(ctx); err != nil {
		// This error is typically a context cancellation error
		logger.Warn().Err(err).Msg("Runner execution finished with an error")
	}
	logger.Info().Msg("Scan complete.")

	// 5. Get the results
	results := runner.GetResults()

	fmt.Printf("\n--- Scan Results (%d found) ---\n", len(results))
	for _, result := range results {
		fmt.Printf("URL: %s\n", result.FinalURL)
		fmt.Printf("  Status Code: %d\n", result.StatusCode)
		fmt.Printf("  Title: %s\n", result.Title)
		fmt.Printf("  Content-Length: %d\n", result.ContentLength)
		if result.Error != nil {
			fmt.Printf("  Error: %v\n", result.Error)
		}
		fmt.Println("--------------------")
	}
}
```

## Configuration

The `Config` struct allows you to customize the runner's behavior. Create a config by calling `telescope.DefaultConfig()` and then modify its fields.

| Field                  | Type                | Description                                                               | Default             |
| ---------------------- | ------------------- | ------------------------------------------------------------------------- | ------------------- |
| `Targets`              | `[]string`          | A slice of target hosts or URLs to scan.                                  | `[]`                |
| `Threads`              | `int`               | The number of concurrent probes to run.                                   | `25`                |
| `Timeout`              | `int`               | The timeout in seconds for each individual request.                       | `10`                |
| `Retries`              | `int`               | The number of times to retry a failed request.                            | `1`                 |
| `Method`               | `string`            | The HTTP method to use (e.g., "GET", "POST").                             | `"GET"`             |
| `FollowRedirects`      | `bool`              | Whether to follow HTTP redirects.                                         | `true`              |
| `RateLimit`            | `int`               | Maximum requests per second. `0` means no limit.                          | `0`                 |
| `CustomHeaders`        | `map[string]string` | A map of custom headers to send with each request.                        | `(empty map)`       |
| `RequestURIs`          | `[]string`          | A slice of URIs/paths to request for each target.                         | `[]`                |
| `Verbose`              | `bool`              | Enables verbose logging from the underlying engine.                       | `false`             |
| `ExtractTitle`         | `bool`              | Enables title extraction.                                                 | `true`              |
| `ExtractStatusCode`    | `bool`              | Enables status code extraction.                                            | `true`              |
| `ExtractBody`          | `bool`              | Enables body extraction.                                                  | `true`              |
| `ExtractHeaders`       | `bool`              | Enables headers extraction.                                               | `true`              |
| `TechDetect`           | `bool`              | Enables technology detection.                                             | `true`              |


## Result Structure

The `GetResults()` method returns a slice of `TelescopeResult`. Each struct contains detailed information about a single successful probe.

| Field           | Type                  | Description                                            |
| --------------- | --------------------- | ------------------------------------------------------ |
| `InputURL`      | `string`              | The original target input.                             |
| `FinalURL`      | `string`              | The final URL after any redirects.                     |
| `RootTargetURL` | `string`              | The identifier provided to the builder.                |
| `StatusCode`    | `int`                 | The HTTP status code of the response.                  |
| `ContentLength` | `int64`               | The `Content-Length` of the response body.             |
| `Title`         | `string`              | The title of the HTML page.                            |
| `WebServer`     | `string`              | The `Server` header value.                             |
| `ContentType`   | `string`              | The `Content-Type` header value.                       |
| `Body`          | `[]byte`              | The raw response body (if `ExtractBody` is true).      |
| `Headers`       | `map[string]string`   | A map of all response headers.                         |
| `IPs`           | `[]string`            | A slice of resolved IP addresses for the host.         |
| `Technologies`  | `[]Technology`        | A slice of technologies detected on the page.          |
| `Duration`      | `float64`             | The total request duration in seconds.                 |
| `Timestamp`     | `time.Time`           | The timestamp of when the probe was completed.         |
| `Error`         | `error`               | Any error that occurred during the probe for this target. |
