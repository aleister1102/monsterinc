# `httpxrunner` Package Documentation

## Overview

The `httpxrunner` package serves as a MonsterInc-specific wrapper around the powerful `github.com/projectdiscovery/httpx/runner` library. It simplifies the integration of HTTP/S probing capabilities into the MonsterInc application by providing a tailored configuration layer and result handling mechanism.

Key responsibilities include:
- Translating MonsterInc's `httpxrunner.Config` into the underlying `httpx.Options`.
- Managing the lifecycle of the `httpx.Runner` (initialization, execution, closure).
- Processing results from `httpx` and mapping them to MonsterInc's `models.ProbeResult` struct.
- Providing channels for asynchronous consumption of probe results and errors.

This wrapper allows MonsterInc to leverage `httpx`'s features such as technology detection, status code extraction, header/body inspection, and more, while abstracting away some of its direct complexities.

## Core Components

### 1. `Runner` Struct

The main struct for the wrapper.

```go
package httpxrunner

import (
	"monsterinc/internal/models"
	"sync"
	"github.com/projectdiscovery/httpx/runner"
)

type Runner struct {
	httpxRunner *runner.Runner
	options     *runner.Options
	config      *Config
	results     chan *models.ProbeResult
	errors      chan error
	wg          sync.WaitGroup
}
```

### 2. `Config` Struct

Defines the configuration for the MonsterInc `httpxrunner`.

```go
package httpxrunner

// Config holds the configuration for the httpx runner
type Config struct {
	// Target configuration
	Targets []string // Can be URLs or hostnames

	// HTTP configuration
	Method          string
	RequestURIs     []string // Paths to append to targets (httpx uses the first one)
	FollowRedirects bool
	Timeout         int // Timeout in seconds
	Retries         int
	Threads         int
	RateLimit       int // Requests per second

	// Output configuration (Verbose controls httpx's Silent option)
	Verbose bool

	// Headers and proxy
	CustomHeaders map[string]string
	Proxy         string

	// Data extraction flags - mapped to httpx.Options
	TechDetect           bool
	ExtractTitle         bool
	ExtractStatusCode    bool
	ExtractLocation      bool // For redirect locations
	ExtractContentLength bool
	ExtractServerHeader  bool
	ExtractContentType   bool
	ExtractIPs           bool
	ExtractBody          bool
	ExtractHeaders       bool
	ExtractCNAMEs        bool
	ExtractASN           bool
	ExtractTLSData       bool
}
```

## Key Functions and Methods

1.  **`NewRunner(config *Config) *Runner`**
    *   **Purpose:** Creates a new `httpxrunner.Runner` instance.
    *   **Operation:**
        1.  Initializes `results` and `errors` channels.
        2.  Creates an `httpx.Options` struct with sensible defaults for library usage (e.g., `NoColor: true`, `Silent: !config.Verbose`).
        3.  Maps fields from the input `httpxrunner.Config` to the `httpx.Options` (e.g., `Timeout`, `Threads`, `RateLimit`, `CustomHeaders`, `Proxy`, data extraction flags like `TechDetect`, `ExtractTitle`, `OutputCName`, `Asn`, `TLSProbe`).
        4.  Sets up the `httpx.Options.OnResult` callback. This callback is crucial as it transforms the `runner.Result` from the `httpx` library into MonsterInc's `models.ProbeResult` and sends it to the `results` channel.
            *   It maps fields like `Input`, `URL` (final URL), `StatusCode`, `ContentLength`, `ContentType`, `Title`, `WebServer`, `ResponseBody`, `ResponseTime`, `ResponseHeaders`, `Technologies`, `A` (IPs), `CNAMEs`.
            *   It also attempts to map ASN information (`res.ASN.AsNumber`, `res.ASN.AsName`) and TLS data (`res.TLSData.Version`, `res.TLSData.Cipher`, and certificate details like issuer and expiry if available from `res.TLSData.CertificateResponse`).
    *   **Returns:** A pointer to the new `Runner`.

2.  **`Runner.Initialize() error`**
    *   **Purpose:** Sets up the internal `httpx.Runner` instance.
    *   **Operation:**
        1.  Returns `nil` if already initialized or if `config.Targets` is empty (logging this info).
        2.  Assigns `config.Targets` to `options.InputTargetHost`.
        3.  Calls `runner.New(options)` from the `httpx` library to create the underlying runner.
        4.  Stores the created `httpx.Runner`.
    *   **Returns:** An error if `runner.New()` fails (e.g., invalid options for `httpx`).

3.  **`Runner.Run() error`**
    *   **Purpose:** Executes the HTTP/S probing operation.
    *   **Operation:**
        1.  Returns an error if `config` is `nil`.
        2.  Calls `Initialize()`. If an error occurs (meaning `httpx.Runner` couldn't be created for reasons other than no targets), it returns the error.
        3.  If `Initialize()` ran but `r.httpxRunner` is `nil` (because no targets were provided), it logs this, closes result/error channels, and returns `nil`.
        4.  Starts three goroutines managed by a `sync.WaitGroup`:
            *   **`httpx` Execution:** Runs `r.httpxRunner.RunEnumeration()`. This is a blocking call. `defer r.Close()` is called here to ensure resources are cleaned up and channels are closed when enumeration finishes or panics.
            *   **Results Processing:** Reads from `r.Results()` channel and logs each `models.ProbeResult` (success or failure).
            *   **Errors Processing:** Reads from `r.Errors()` channel and logs any global errors from the runner.
        5.  Waits for all goroutines to complete using `r.wg.Wait()`.
    *   **Returns:** `nil` if successful, or an error from initialization.

4.  **`Runner.Close()`**
    *   **Purpose:** Cleans up resources.
    *   **Operation:**
        1.  Calls `r.httpxRunner.Close()` to close the underlying `httpx` runner.
        2.  Closes the `r.results` and `r.errors` channels.

5.  **`Runner.Results() <-chan *models.ProbeResult`**
    *   **Returns:** A read-only channel from which `models.ProbeResult` can be consumed.

6.  **`Runner.Errors() <-chan error`**
    *   **Returns:** A read-only channel from which probing errors (not individual request errors, which are part of `ProbeResult`) can be consumed.

## `result.go` Utilities

Located in the same package, `result.go` provides helper functions for `models.ProbeResult`:

-   **`SetProbeError(r *models.ProbeResult, errMsg string)`:** Sets an error message on the result and clears out other potentially inconsistent fields (like `StatusCode`, `Body`, etc.).
-   **`IsProbeSuccess(r *models.ProbeResult) bool`:** Checks if `r.Error` is empty.
-   **`ProbeHasTechnologies(r *models.ProbeResult) bool`:** Checks if `len(r.Technologies)` is greater than 0.
-   **`ProbeHasTLS(r *models.ProbeResult) bool`:** Checks if `r.TLSVersion` is not empty.

## How to Use

1.  **Create Configuration:** Populate an `httpxrunner.Config` struct.
    ```go
    import "monsterinc/internal/httpxrunner"

    probeConfig := &httpxrunner.Config{
        Targets:         []string{"http://example.com", "https://google.com"},
        Method:          "GET",
        Timeout:         10,    // seconds
        Threads:         50,
        RateLimit:       100,   // rps
        FollowRedirects: true,
        Verbose:         false, // Set to true for more httpx logs

        ExtractTitle:         true,
        ExtractStatusCode:    true,
        ExtractTechDetect:    true,
        ExtractIPs:           true,
        ExtractCNAMEs:        true,
        ExtractASN:           true,
        ExtractTLSData:       true,
        ExtractHeaders:       true,
        // Set ExtractBody to true if response bodies are needed
    }
    ```

2.  **Initialize Runner:** Create a new runner instance.
    ```go
    runner := httpxrunner.NewRunner(probeConfig)
    ```

3.  **Process Results and Errors (Asynchronously):** Start goroutines to listen on the `Results()` and `Errors()` channels.
    ```go
    go func() {
        for err := range runner.Errors() {
            log.Printf("[AppLevelERROR] HTTPX Runner Error: %v", err)
        }
    }()

    go func() {
        for result := range runner.Results() {
            if result.Error != "" {
                log.Printf("Probe FAILED for %s: %s", result.InputURL, result.Error)
            } else {
                log.Printf("Probe SUCCESS for %s: Status %d, Title: %s", result.InputURL, result.StatusCode, result.Title)
                // Access other fields: result.IPs, result.Technologies, result.ASNOrg, result.TLSCipher etc.
            }
        }
        log.Println("Result processing finished.")
    }()
    ```

4.  **Start Probing:** Run the probing process. This is a blocking call in the context of its own internal operations, but the overall `Run()` method manages this with goroutines and waits.
    ```go
    err := runner.Run() // This will block until httpx and result/error processing goroutines finish.
    if err != nil {
        log.Fatalf("HTTPX probing run failed: %v", err)
    }
    log.Println("HTTPX Probing completed.")
    ```
    Note: `runner.Close()` is called internally by `Run()` via a defer statement in the goroutine that executes `httpxRunner.RunEnumeration()`. This ensures channels are closed, allowing the result and error processing goroutines to terminate.

## Dependencies

-   **Go Standard Library:** `fmt`, `log`, `strconv`, `strings`, `sync`.
-   **External Libraries:**
    *   `github.com/projectdiscovery/httpx/common/customheader`
    *   `github.com/projectdiscovery/httpx/runner`
-   **Internal Packages:**
    *   `monsterinc/internal/models`: For `ProbeResult` and `Technology` structs.

This package provides a streamlined way to integrate `httpx`'s robust probing capabilities into MonsterInc, focusing on ease of configuration and use specific to the application's needs. 