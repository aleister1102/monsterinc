package httpxrunner

import (
	"fmt"
	"log"
	"monsterinc/internal/models"
	"strconv"
	"strings"
	"sync"

	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
	// Thêm import cho clients và wappalyzer nếu thực sự cần và có sẵn
	// "github.com/projectdiscovery/httpx/common/httpx/clients"
	// "github.com/projectdiscovery/wappalyzergo"
)

// Runner wraps the httpx library runner and provides MonsterInc-specific functionality
type Runner struct {
	httpxRunner *runner.Runner
	options     *runner.Options
	config      *Config // Store the passed config
	results     chan *models.ProbeResult
	errors      chan error
	wg          sync.WaitGroup
}

// Config holds the configuration for the httpx runner
type Config struct {
	// Target configuration
	Targets []string // Can be URLs or file paths

	// HTTP configuration
	Method          string
	RequestURIs     []string
	FollowRedirects bool
	Timeout         int // Timeout in seconds
	Retries         int
	Threads         int
	RateLimit       int // Added RateLimit (requests per second)

	// Output configuration
	OutputFormat string // Currently not used by this wrapper but kept for future flexibility
	Verbose      bool

	// Headers and proxy
	CustomHeaders map[string]string
	Proxy         string

	// Data extraction flags - mapped to httpx.Options
	TechDetect           bool
	ExtractTitle         bool
	ExtractStatusCode    bool
	ExtractLocation      bool
	ExtractContentLength bool
	ExtractServerHeader  bool
	ExtractContentType   bool
	ExtractIPs           bool
	ExtractBody          bool
	ExtractHeaders       bool
	ExtractCNAMEs        bool // Added for CNAME extraction
	ExtractASN           bool // Added for ASN extraction
	ExtractTLSData       bool // Added for TLS data extraction
}

// NewRunner creates a new instance of the httpx runner wrapper
func NewRunner(config *Config) *Runner {
	resultsChan := make(chan *models.ProbeResult, 100) // Buffer size can be tuned
	errorChan := make(chan error, 10)                  // Buffer for runner-level errors

	options := &runner.Options{
		// Sensible defaults for library usage
		Methods:                 "GET",
		FollowRedirects:         true,
		Timeout:                 10, // Default timeout in seconds
		Retries:                 1,
		Threads:                 25,
		MaxRedirects:            10,
		RespectHSTS:             true,
		NoColor:                 true, // Recommended for programmatic use
		Silent:                  true, // Overridden by Verbose if true
		OmitBody:                false,
		ResponseHeadersInStdout: true,
		TechDetect:              true,
		OutputIP:                true,
		StatusCode:              true,
		ContentLength:           true,
		OutputContentType:       true,
		ExtractTitle:            true,
		OutputServerHeader:      true,
		Location:                true,
		RateLimit:               0,    // Default rate limit (0 often means no limit or use threads)
		OutputCName:             true, // Default to true, controlled by ExtractCNAMEs
		Asn:                     true, // Default to true, controlled by ExtractASN
		TLSProbe:                true, // Default to true, controlled by ExtractTLSData
	}

	// Apply MonsterInc's specific configuration
	if config != nil {
		if len(config.Targets) > 0 {
			// httpx runner.Options expects InputTargetHost to be a slice of hosts/URLs.
			// If a target is a file path, we'll need to read it.
			// For now, assume Targets are directly usable as InputTargetHost.
			// This logic will be refined in Initialize() or Run().
			// options.InputTargetHost = config.Targets // This will be handled by loadTargetsFromFileOrDirect
		}
		if config.Method != "" {
			options.Methods = config.Method
		}
		if len(config.RequestURIs) > 0 {
			options.RequestURI = config.RequestURIs[0] // httpx seems to take one primary request URI
		}
		options.FollowRedirects = config.FollowRedirects
		if config.Timeout > 0 {
			options.Timeout = config.Timeout // map our Config.Timeout (seconds) to options.Timeout (seconds)
		}
		if config.Retries >= 0 { // Allow 0 retries
			options.Retries = config.Retries
		}
		if config.Threads > 0 {
			options.Threads = config.Threads
		}
		if config.RateLimit > 0 { // If RateLimit is specified in our config
			options.RateLimit = config.RateLimit
		}
		if len(config.CustomHeaders) > 0 {
			headers := customheader.CustomHeaders{}
			for k, v := range config.CustomHeaders {
				headerVal := k + ": " + v
				if err := headers.Set(headerVal); err != nil {
					log.Printf("[WARN] HTTPXRunner: Failed to set custom header %s: %v", headerVal, err)
					continue
				}
			}
			options.CustomHeaders = headers
		}
		if config.Proxy != "" {
			options.Proxy = config.Proxy // Corrected field name
		}

		options.Verbose = config.Verbose
		if config.Verbose {
			options.Silent = false // If verbose, not silent
		}

		options.TechDetect = config.TechDetect
		options.ExtractTitle = config.ExtractTitle
		options.StatusCode = config.ExtractStatusCode
		options.Location = config.ExtractLocation
		options.ContentLength = config.ExtractContentLength
		options.OutputServerHeader = config.ExtractServerHeader
		options.OutputContentType = config.ExtractContentType
		options.OutputIP = config.ExtractIPs
		options.OmitBody = !config.ExtractBody // If ExtractBody is true, OmitBody should be false
		options.ResponseHeadersInStdout = config.ExtractHeaders
		options.OutputCName = config.ExtractCNAMEs
		options.Asn = config.ExtractASN
		options.TLSProbe = config.ExtractTLSData
	}

	// OnResult Callback: Maps httpx.Result to our ProbeResult
	options.OnResult = func(res runner.Result) {
		probeResult := &models.ProbeResult{
			InputURL:      res.Input, // Corrected: res.Input is the input URL string
			Method:        res.Method,
			Timestamp:     res.Timestamp,
			StatusCode:    res.StatusCode,
			ContentLength: int64(res.ContentLength),
			ContentType:   res.ContentType,
			Error:         res.Error,
			FinalURL:      res.URL, // res.URL is the final URL after redirects
			Title:         res.Title,
			WebServer:     res.WebServer,
			Body:          res.ResponseBody, // Assuming OmitBody=false allows this
		}

		if res.ResponseTime != "" {
			durationStr := strings.TrimSuffix(res.ResponseTime, "s")
			if dur, err := strconv.ParseFloat(durationStr, 64); err == nil {
				probeResult.Duration = dur
			}
		}

		if len(res.ResponseHeaders) > 0 {
			probeResult.Headers = make(map[string]string)
			for k, v := range res.ResponseHeaders { // v is interface{}
				switch val := v.(type) {
				case string:
					probeResult.Headers[k] = val
				case []string:
					probeResult.Headers[k] = strings.Join(val, ", ")
				case []interface{}: // Handle cases where it might be []interface{} containing strings
					var strVals []string
					for _, iv := range val {
						if sv, ok := iv.(string); ok {
							strVals = append(strVals, sv)
						}
					}
					probeResult.Headers[k] = strings.Join(strVals, ", ")
				default:
					// Optionally log or handle other unexpected types for header values
					// log.Printf("[WARN] HTTPXRunner: Unexpected type for header '%s': %T", k, v)
				}
			}
		}

		if len(res.Technologies) > 0 {
			probeResult.Technologies = make([]models.Technology, 0, len(res.Technologies))
			for _, techName := range res.Technologies {
				tech := models.Technology{Name: techName}
				// Version/Category might come from wappalyzer integration if enabled/configured
				probeResult.Technologies = append(probeResult.Technologies, tech)
			}
		}

		if len(res.A) > 0 {
			probeResult.IPs = res.A
		}
		if len(res.CNAMEs) > 0 { // Map CNAMEs if available
			probeResult.CNAMEs = res.CNAMEs
		}

		// Map ASN if available
		if res.ASN != nil {
			if res.ASN.AsNumber != "" {
				asnNumber, err := strconv.Atoi(strings.ReplaceAll(res.ASN.AsNumber, "AS", ""))
				if err == nil {
					probeResult.ASN = asnNumber
				}
			}
			probeResult.ASNOrg = res.ASN.AsName // Corrected to AsName
		}

		if res.TLSData != nil { // Map TLS data if available
			probeResult.TLSVersion = res.TLSData.Version // Direct field from clients.Response
			probeResult.TLSCipher = res.TLSData.Cipher   // Direct field from clients.Response

			// Mapping for certificate details depends on the exact structure of res.TLSData.CertificateResponse
			// Assuming res.TLSData has a field like `CertificateResponse` which is a struct/pointer.
			// Example structure: res.TLSData.CertificateResponse.IssuerCN []string and res.TLSData.CertificateResponse.NotAfter time.Time
			// This is a common pattern but needs to be verified against the exact `clients.Response` structure used by `httpx`.
			if certData := res.TLSData.CertificateResponse; certData != nil { // Check if CertificateResponse exists
				if certData.IssuerCN != "" {
					probeResult.TLSCertIssuer = certData.IssuerCN // Directly assign if it's a string and not empty
				}
				// Check if NotAfter is a non-zero time
				if !certData.NotAfter.IsZero() {
					probeResult.TLSCertExpiry = certData.NotAfter
				}
			}
		}

		resultsChan <- probeResult
	}

	return &Runner{
		options: options,
		config:  config, // Store the config
		results: resultsChan,
		errors:  errorChan,
	}
}

// Initialize sets up the httpx runner with the provided options
// This can be called implicitly by Run or explicitly if needed.
func (r *Runner) Initialize() error {
	if r.httpxRunner != nil {
		return nil // Already initialized
	}

	if r.config == nil {
		// This case should ideally be caught before calling Initialize if config is mandatory.
		// However, if it can happen, it implies no targets.
		log.Println("[WARN] HTTPXRunner: Config is nil during Initialize, cannot proceed.")
		return fmt.Errorf("runner configuration is nil, cannot initialize")
	}

	if len(r.config.Targets) == 0 {
		log.Println("[INFO] HTTPXRunner: No targets specified in config. Runner will not be initialized.")
		return nil // Not an error to have no targets, Run() will handle this.
	}

	// Directly use the targets from the config.
	r.options.InputTargetHost = r.config.Targets

	httpxRunner, err := runner.New(r.options)
	if err != nil {
		return fmt.Errorf("failed to initialize httpx runner: %w", err)
	}
	r.httpxRunner = httpxRunner
	log.Printf("[INFO] HTTPXRunner: Initialized with %d targets.", len(r.options.InputTargetHost))
	return nil
}

// Close performs cleanup of the httpx runner
func (r *Runner) Close() {
	// Ensure underlying httpx runner is closed first.
	// This should signal its internal goroutines (like the one calling OnResult) to stop.
	if r.httpxRunner != nil {
		r.httpxRunner.Close()
	}

	// Now, close the channels that our runner exposes.
	// This signals the goroutines in Run() (for results and errors processing) that no more data will be sent.
	// It's important to do this after httpxRunner.Close() to avoid sending to a closed channel if OnResult was still somehow active.
	// Add a sync.Once or check if channels are already closed if Close() could be called multiple times.
	// For simplicity here, assume Close() is called once by the defer in Run().
	close(r.results)
	close(r.errors)
	log.Println("[INFO] HTTPXRunner: Results and Errors channels closed.")
}

// Run executes the httpx probing with the configured targets
func (r *Runner) Run() error {
	if r.config == nil {
		// Ensure channels are closed if Run is called with nil config and they were somehow created.
		// This is defensive, as NewRunner likely creates them.
		if r.results != nil {
			close(r.results)
		}
		if r.errors != nil {
			close(r.errors)
		}
		return fmt.Errorf("runner configuration is nil")
	}

	if err := r.Initialize(); err != nil {
		// An error from Initialize here means runner.New() failed for a reason other than no targets.
		// Ensure channels are closed if Initialize failed mid-way after creating them (though unlikely with current NewRunner).
		close(r.results) // Safe to close multiple times if already closed by Initialize error path, but not ideal.
		close(r.errors)  // Better to have Initialize not create channels if it fails, or manage once logic.
		return fmt.Errorf("failed to initialize httpx runner for run: %w", err)
	}

	// Check if httpxRunner was initialized. If not, it means there were no targets.
	if r.httpxRunner == nil {
		log.Println("[INFO] HTTPXRunner: No targets to probe (httpxRunner not initialized). Skipping run.")
		close(r.results) // Close channels to signal no results
		close(r.errors)
		return nil
	}

	log.Println("[INFO] HTTPXRunner: Starting HTTPX probing run...")

	var runErr error // To capture error from RunEnumeration if it's ever changed to return one

	r.wg.Add(3) // One for RunEnumeration, one for results processing, one for errors processing

	// Goroutine to execute httpx's RunEnumeration
	go func() {
		defer r.wg.Done()
		defer r.Close() // Ensure runner resources are cleaned up when enumeration ends or panics
		// This also closes r.results and r.errors, signaling other goroutines.
		r.httpxRunner.RunEnumeration()
		// RunEnumeration is blocking and handles its own errors internally via OnResult.
		// If it were to return an error, we'd capture it: runErr = r.httpxRunner.RunEnumeration()
		log.Println("[INFO] HTTPXRunner: RunEnumeration completed.")
	}()

	// Goroutine to process results
	resultsCount := 0
	go func() {
		defer r.wg.Done()
		for result := range r.Results() { // Will unblock when r.results is closed by r.Close()
			resultsCount++
			if result.Error != "" {
				log.Printf("[RESULT] HTTPX Probe for %s FAILED: %s (Status: %d)", result.InputURL, result.Error, result.StatusCode)
			} else {
				log.Printf("[RESULT] HTTPX Probe for %s SUCCESS: Status %d, Length %d, Type %s, FinalURL: %s, Title: %s, WebServer: %s",
					result.InputURL, result.StatusCode, result.ContentLength, result.ContentType, result.FinalURL, result.Title, result.WebServer)
				if len(result.IPs) > 0 {
					log.Printf("    IPs: %s", strings.Join(result.IPs, ", "))
				}
				if result.HasTechnologies() {
					var techNames []string
					for _, tech := range result.Technologies {
						name := tech.Name
						if tech.Version != "" {
							name += " (" + tech.Version + ")"
						}
						techNames = append(techNames, name)
					}
					log.Printf("    Technologies: %s", strings.Join(techNames, ", "))
				}
			}
		}
		log.Println("[INFO] HTTPXRunner: Results processing goroutine finished.")
	}()

	// Goroutine to process errors
	go func() {
		defer r.wg.Done()
		for err := range r.Errors() { // Will unblock when r.errors is closed by r.Close()
			log.Printf("[ERROR] HTTPXRunner global error: %v", err)
		}
		log.Println("[INFO] HTTPXRunner: Errors processing goroutine finished.")
	}()

	r.wg.Wait() // Wait for RunEnumeration, results processing, and errors processing to complete

	log.Printf("[INFO] HTTPXRunner: HTTPX Probing finished. Processed %d results.", resultsCount)
	// log.Printf("[INFO] HTTPXRunner: Summary - Targets Attempted: %d, Results Processed: %d",
	// 	len(r.options.InputTargetHost), // This might be inaccurate if files were used
	// 	resultsCount)
	return runErr // Currently runErr will always be nil
}

// Results returns a channel that receives probe results
func (r *Runner) Results() <-chan *models.ProbeResult {
	return r.results
}

// Errors returns a channel that receives errors during probing
func (r *Runner) Errors() <-chan error {
	return r.errors
}

// GetOptions returns the current httpx runner.Options. Useful for debugging or advanced scenarios.
func (r *Runner) GetOptions() *runner.Options {
	return r.options
}

// SetTargets is deprecated if targets are part of Config and processed in Initialize/Run.
// func (r *Runner) SetTargets(targets []string) {
// 	r.options.InputTargetHost = targets
// }

// processResults and processErrors are now incorporated into the Run method as goroutines.
// func (r *Runner) processResults() { ... }
// func (r *Runner) processErrors() { ... }
