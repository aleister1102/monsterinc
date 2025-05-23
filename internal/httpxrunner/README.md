# HTTPX Library Integration Documentation

## Core Components

### Runner Package (`github.com/projectdiscovery/httpx/runner`)

#### Main Types

1. **Runner**
   - Main struct that handles HTTP probing operations
   - Created using `New(options *Options) (*Runner, error)`
   - Methods:
     - `Close()` - Cleanup resources
     - `RunEnumeration()` - Start the enumeration process
     - `RunEnumerationWithOpts()` - Run with custom options

2. **Options**
   - Configuration struct for the runner
   - Created using `ParseOptions() *Options`
   - Key fields:
     - `InputTargetHost` - Target host to probe
     - `Methods` - HTTP methods to use
     - `RequestURIs` - URIs to probe
     - `Output` - Output format
     - `Threads` - Number of concurrent threads
     - `Timeout` - Request timeout
     - `Retries` - Number of retries
     - `FollowRedirects` - Whether to follow redirects
     - `CustomHeaders` - Custom HTTP headers
     - `Proxy` - Proxy URL

3. **Result**
   - Contains the result of a probe
   - Key fields:
     - `URL` - The probed URL
     - `StatusCode` - HTTP status code
     - `ContentLength` - Response content length
     - `ContentType` - Response content type
     - `Title` - Page title
     - `WebServer` - Server header
     - `Technologies` - Detected technologies
     - `IP` - Resolved IP
     - `CNAME` - CNAME record
     - `Error` - Error if any

### Browser Package (`github.com/projectdiscovery/httpx/browser`)

Used for browser-based probing and JavaScript rendering.

### Common Package (`github.com/projectdiscovery/httpx/common`)

Contains shared utilities and types used across the library.

## Integration Points

1. **Configuration**
   - Use `runner.Options` to configure probing behavior
   - Map MonsterInc configuration to httpx options

2. **Execution**
   - Use `runner.New()` to create runner instance
   - Configure options before running
   - Use `RunEnumeration()` to start probing

3. **Result Handling**
   - Implement `OnResultCallback` to process results
   - Parse `Result` struct to extract required information
   - Map to MonsterInc's internal result structure

## Example Usage

```go
options := &runner.Options{
    InputTargetHost: "example.com",
    Methods: []string{"GET"},
    Threads: 40,
    Timeout: 5,
    FollowRedirects: true,
}

runner, err := runner.New(options)
if err != nil {
    // Handle error
}
defer runner.Close()

// Run enumeration
err = runner.RunEnumeration()
if err != nil {
    // Handle error
}
```

## Notes

- The library supports both synchronous and asynchronous operation
- Results can be processed through callbacks or collected in memory
- Technology detection requires additional setup (signature files)
- Browser-based probing requires Chrome/Chromium installation 